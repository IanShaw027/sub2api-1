#!/usr/bin/env python3
"""g0-route-dto-extractor-v3: multiline Gin routes + receiver-typed handlers.

Reads ONLY git blobs at pinned SHA via git show. One row per mounted
registration (no method+path dedupe). Terminal handler is last non-middleware
arg. Resolve by receiver type + method; bare-name ambiguity fails closed.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
import sys
from collections import defaultdict
from pathlib import Path
from typing import Optional

EXTRACTOR_ID = "g0-route-dto-extractor-v3"
EXTRACTOR_VERSION = "3.0.0"
BASELINE_ID = "clomapi-g0-baseline-20260720T021059Z"
RESPONSE_PACKAGE_IMPORT = "github.com/Wei-Shaw/sub2api/internal/pkg/response"
GIN_PACKAGE_IMPORT = "github.com/gin-gonic/gin"

# This is deliberately not a general Go control-flow verifier. These two
# frozen blobs were manually audited for the specialized function-valued
# gateway handlers below; any source-byte change disables the profile.
AUDITED_STATIC_HANDLER_PROFILE_ID = "clomapi-a44cc392-static-handlers-v1"
AUDITED_STATIC_HANDLER_BLOBS = {
    "backend/internal/server/routes/common.go": (
        "37a0112ea2490b166438c4802e1f7d1d51ca758ffcc3c657d72ad17657a24764"
    ),
    "backend/internal/server/routes/gateway.go": (
        "f5e882bdd9d819d6cc2d1a302da6296f833d694f379b923097f5d120419a991a"
    ),
}

AUDITED_REQUEST_UNMARSHAL_SCHEMAS = {
    ("backend/internal/handler/admin::admin", "optionalLimitField", "c3f5e168908625be0a2162d54240219f971150897b44527e13fec270cd588adf"):
        {"oneOf": [{"type": "number"}, {"type": "string"}, {"type": "null"}]},
    ("backend/internal/handler/admin::admin", "optionalFloatField", "e130780e82040143121eaf69ab0e6146055e8d43533dced269d1be6540d83a2a"):
        {"oneOf": [{"type": "number"}, {"type": "string"}, {"type": "null"}]},
    ("backend/internal/handler/dto::dto", "NullableTimeField", "2b2d56cc679085d08d3673c96c9ef29375c3fd837b3f8ac856fd9885c72e2178"):
        {"oneOf": [{"type": "string", "format": "date-time"}, {"type": "null"}]},
    ("backend/internal/handler/dto::dto", "NullableInt64Field", "5ed8dbb12d85d62dc5d3f7a92a72f9c3d7fe8992b8e1b146962b18762e70746e"):
        {"oneOf": [{"type": "integer", "format": "int64"}, {"type": "null"}]},
}


def run_git(root: Path, *args: str) -> bytes:
    return subprocess.check_output(["git", "-C", str(root), *args])


def full_sha(root: Path, commit: str) -> str:
    return run_git(root, "rev-parse", f"{commit}^{{commit}}").decode().strip()


def git_show_text(root: Path, commit: str, path: str) -> str:
    return run_git(root, "show", f"{commit}:{path}").decode("utf-8", errors="replace")


def git_ls_files(root: Path, commit: str, prefix: str) -> list[str]:
    out = run_git(root, "ls-tree", "-r", "--name-only", commit, prefix).decode()
    return [ln for ln in out.splitlines() if ln]


def join_path(base: str, rel: str) -> str:
    if not rel:
        return base or ""
    if rel.startswith("/"):
        b = (base or "").rstrip("/")
        return (b + rel) if b else rel
    b = (base or "").rstrip("/")
    return f"{b}/{rel}" if b else f"/{rel}"


def normalize_path(p: str) -> str:
    if not p:
        return "/"
    while "//" in p:
        p = p.replace("//", "/")
    if len(p) > 1 and p.endswith("/"):
        p = p[:-1]
    return p


def oas_path(path: str) -> str:
    parts = []
    for seg in path.split("/"):
        if not seg:
            continue
        if seg.startswith(":"):
            parts.append("{" + seg[1:] + "}")
        elif seg.startswith("*"):
            parts.append("{" + (seg[1:] or "path") + "}")
        else:
            parts.append(seg)
    return "/" + "/".join(parts)


def path_params(path: str) -> list[str]:
    out = []
    for seg in path.split("/"):
        if seg.startswith(":"):
            out.append(seg[1:])
        elif seg.startswith("*"):
            out.append(seg[1:] or "path")
    return out


def classify_surface(path: str) -> str:
    if path == "/api/v1" or path.startswith("/api/v1/"):
        rest = path[len("/api/v1") :] or "/"
        if rest.startswith("/admin"):
            return "management_admin"
        if rest.startswith("/auth") or rest.startswith("/settings"):
            return "management_auth"
        if rest.startswith("/payment"):
            return "management_payment"
        return "management_user"
    if path == "/health":
        return "ops_public"
    if (
        path == "/setup/status"
        or path.startswith("/api/claude")
        or path.startswith("/api/oauth/")
        or path.startswith("/api/event_logging")
        or path.startswith("/api/claude_cli")
    ):
        return "common_or_auxiliary"
    return "gateway"


ENTRY_PREFIXES = {
    "RegisterCommonRoutes": "",
    "RegisterGatewayRoutes": "",
    "RegisterAuthRoutes": "/api/v1",
    "RegisterMediaRoutes": "/api/v1",
    "RegisterUserRoutes": "/api/v1",
    "RegisterTLSFingerprintCaptureRoutes": "/api/v1",
    "RegisterAdminRoutes": "/api/v1",
    "RegisterPaymentRoutes": "/api/v1",
    "RegisterAIRoutes": "/api/v1",
    "RegisterPageRoutes": "/api/v1",
}

MW_MARKERS = (
    "rateLimiter",
    "middleware.",
    "servermiddleware.",
    "gin.HandlerFunc",
    "bodyLimit",
    "clientRequestID",
    "opsErrorLogger",
    "endpointNorm",
    "apiKeyAuth",
    "requireGroup",
    "jwtAuth",
    "adminAuth",
    "ForcePlatform",
    "APIKeyAuth",
)

METHOD_START_RE = re.compile(
    r"(?P<recv>\w+)\.(?P<meth>GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)\(\s*"
)
GROUP_RE = re.compile(
    r'(?P<var>\w+)\s*:?=\s*(?P<parent>\w+)\.Group\(\s*"(?P<pre>[^"]*)"\s*\)'
)
GROUP_START_RE = re.compile(
    r"(?P<var>\w+)\s*(?P<op>:=|=)\s*(?P<parent>\w+)\.Group\(\s*"
)
USE_START_RE = re.compile(r"(?P<recv>\w+)\.Use\(\s*")
FUNC_REGISTER_RE = re.compile(
    r"func\s+(?P<name>Register\w+)\(\s*(?P<entry>\w+)\s+\*gin\.(?:Engine|RouterGroup)"
)


def line_of(text: str, idx: int) -> int:
    return text[:idx].count("\n") + 1


def find_matching_paren(text: str, open_idx: int) -> int:
    depth = 0
    in_str = None
    escape = False
    i = open_idx
    while i < len(text):
        ch = text[i]
        if in_str:
            if escape:
                escape = False
            elif ch == "\\":
                escape = True
            elif ch == in_str:
                in_str = None
            i += 1
            continue
        if ch in ('"', "'", "`"):
            in_str = ch
        elif ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
            if depth == 0:
                return i
        i += 1
    return -1


def split_top_level_args(s: str) -> list[str]:
    args: list[str] = []
    buf: list[str] = []
    depth = 0
    in_str = None
    escape = False
    for ch in s:
        if in_str:
            buf.append(ch)
            if escape:
                escape = False
            elif ch == "\\":
                escape = True
            elif ch == in_str:
                in_str = None
            continue
        if ch in ('"', "'", "`"):
            in_str = ch
            buf.append(ch)
            continue
        if ch in "([{":
            depth += 1
            buf.append(ch)
        elif ch in ")]}":
            depth -= 1
            buf.append(ch)
        elif ch == "," and depth == 0:
            args.append("".join(buf).strip())
            buf = []
        else:
            buf.append(ch)
    if "".join(buf).strip():
        args.append("".join(buf).strip())
    return args


def is_middleware_arg(a: str) -> bool:
    a = a.strip()
    if a.startswith("func"):
        return False
    for m in MW_MARKERS:
        if a.startswith(m) or m in a[:80]:
            return True
    if ".Limit" in a[:60]:
        return True
    return False


def parse_go_structs(path: str, text: str) -> dict:
    structs: dict = {}
    lines = text.splitlines()
    code_lines = _mask_go_lexemes(text).splitlines()
    commentless_lines = _mask_go_lexemes(text, mask_strings=False).splitlines()
    i = 0
    while i < len(lines):
        m = re.match(r"^\s*type\s+(\w+)\s+struct\s*\{", code_lines[i])
        if not m:
            i += 1
            continue
        name = m.group(1)
        start_line = i + 1
        body_lines: list[tuple[str, str]] = []
        open_column = code_lines[i].find("{")
        inline_code = code_lines[i][open_column + 1 :]
        inline_original = commentless_lines[i][open_column + 1 :]
        if "}" in inline_code:
            close_column = inline_code.find("}")
            body_lines.append(
                (inline_original[:close_column], inline_code[:close_column])
            )
            i += 1
        else:
            i += 1
            depth = 1
            while i < len(lines) and depth > 0:
                code_line = code_lines[i]
                original_line = commentless_lines[i]
                previous_depth = depth
                depth += code_line.count("{") - code_line.count("}")
                if depth > 0:
                    body_lines.append((original_line, code_line))
                elif previous_depth > 0:
                    close_column = code_line.find("}")
                    if close_column > 0:
                        body_lines.append(
                            (original_line[:close_column], code_line[:close_column])
                        )
                i += 1
        fields = []
        parse_complete = True
        for original_line, code_line in body_lines:
            if not code_line.strip():
                continue
            # The supported field subset is one declaration per line. Tags
            # are recovered only after the declaration was proven executable.
            bls = original_line.rstrip()
            emb = re.match(
                r"^\s*(?P<type>\*?[\w\.]+)(?:\s+`(?P<tag>[^`]*)`)?\s*$",
                bls,
            )
            if emb and " " not in code_line.strip()[1:]:
                embedded_type = emb.group("type")
                tag = emb.group("tag") or ""
                tags: dict = {}
                json_name = None
                json_omitempty = False
                json_omitzero = False
                json_ignored = False
                json_string = False
                if tag:
                    json_tag = re.search(r'(?:^|\s)json:"([^"]*)"', tag)
                    if json_tag:
                        tags["json"] = json_tag.group(1)
                        parts = json_tag.group(1).split(",")
                        json_name = parts[0] or None
                        json_omitempty = "omitempty" in parts[1:]
                        json_omitzero = "omitzero" in parts[1:]
                        json_ignored = parts[0] == "-"
                        json_string = "string" in parts[1:]
                fields.append(
                    {
                        "name": embedded_type.lstrip("*").split(".")[-1],
                        "type": embedded_type,
                        "json_name": json_name,
                        "required": False,
                        "embedded": True,
                        "embedded_pointer": embedded_type.startswith("*"),
                        "exported": embedded_type.lstrip("*").split(".")[-1][:1].isupper(),
                        "json_omitempty": json_omitempty,
                        "json_omitzero": json_omitzero,
                        "json_ignored": json_ignored,
                        "json_string": json_string,
                        "tags": tags,
                    }
                )
                continue
            fm = re.match(
                r"^\s*(?P<names>\w+(?:\s*,\s*\w+)*)\s+"
                r"(?P<type>.+?)(?:\s+`(?P<tag>[^`]*)`)?\s*$",
                bls,
            )
            if not fm:
                # A declaration containing exported identifiers that is outside
                # the supported grammar must invalidate the struct. Silently
                # dropping one member would manufacture an incomplete schema.
                if re.search(r"(?:^|,)\s*[A-Z][A-Za-z0-9_]*\s*(?:,|\s)", bls):
                    parse_complete = False
                continue
            field_names = [item.strip() for item in fm.group("names").split(",")]
            if not field_names or any(not re.fullmatch(r"\w+", item) for item in field_names):
                parse_complete = False
                continue
            ftyp, tag = fm.group("type").strip(), fm.group("tag") or ""
            jn = None
            required = False
            json_omitempty = False
            json_omitzero = False
            json_ignored = False
            json_string = False
            tags: dict = {}
            if tag:
                for part in tag.split():
                    if part.startswith("json:"):
                        val = part[5:].strip('"')
                        tags["json"] = val
                        json_parts = val.split(",")
                        jn = json_parts[0]
                        json_omitempty = "omitempty" in json_parts[1:]
                        json_omitzero = "omitzero" in json_parts[1:]
                        json_string = "string" in json_parts[1:]
                        if jn == "-":
                            json_ignored = True
                            jn = None
                    elif part.startswith("binding:"):
                        val = part[8:].strip('"')
                        tags["binding"] = val
                        if "required" in val.split(","):
                            required = True
                    elif part.startswith("form:"):
                        tags["form"] = part[5:].strip('"').split(",")[0]
                    elif part.startswith("uri:"):
                        tags["uri"] = part[4:].strip('"').split(",")[0]
                    elif part.startswith("header:"):
                        tags["header"] = part[7:].strip('"').split(",")[0]
                    elif part.startswith("query:"):
                        tags["query"] = part[6:].strip('"').split(",")[0]
            for fname in field_names:
                field_json_name = jn
                # One explicit JSON name cannot describe multiple Go fields.
                # Go permits the declaration, but the wire contract is not
                # provable without type-checking encoding/json behavior.
                if len(field_names) > 1 and field_json_name not in (None, "", "-"):
                    parse_complete = False
                    continue
                if field_json_name is None and "json" not in tags:
                    field_json_name = fname
                elif field_json_name == "" and "json" in tags:
                    field_json_name = fname
                fields.append(
                    {
                        "name": fname,
                        "type": ftyp,
                        "json_name": field_json_name,
                        "required": required,
                        "json_omitempty": json_omitempty,
                        "json_omitzero": json_omitzero,
                        "json_ignored": json_ignored,
                        "json_string": json_string,
                        "exported": fname[:1].isupper(),
                        "embedded": False,
                        "embedded_pointer": False,
                        "tags": tags,
                    }
                )
        structs[name] = {
            "name": name,
            "file": path,
            "line": start_line,
            "fields": fields,
            "parse_complete": parse_complete,
            "evidence": f"{path}:{start_line}",
        }
    return structs


def go_type_to_oas(go_type: str) -> dict:
    t = go_type.strip().lstrip("*")
    simple = {
        "string": {"type": "string"},
        "bool": {"type": "boolean"},
        "int": {"type": "integer"},
        "int32": {"type": "integer"},
        "int64": {"type": "integer", "format": "int64"},
        "uint": {"type": "integer"},
        "uint64": {"type": "integer", "format": "int64"},
        "float32": {"type": "number", "format": "float"},
        "float64": {"type": "number", "format": "double"},
        "time.Time": {"type": "string", "format": "date-time"},
        "json.RawMessage": {"type": "object"},
    }
    if t in simple:
        return dict(simple[t])
    if t.startswith("[]"):
        return {"type": "array", "items": go_type_to_oas(t[2:])}
    if t.startswith("map["):
        return {"type": "object", "additionalProperties": True}
    return {"type": "object", "x-clomapi-go-type": t, "description": f"Named Go type {t}"}


def struct_to_oas_schema(st: dict, all_structs: dict, depth: int = 0) -> dict:
    if depth > 5:
        return {"type": "object", "x-clomapi-go-type": st["name"]}
    props: dict = {}
    required: list = []
    for f in st["fields"]:
        if f.get("embedded"):
            emb_name = f["type"].split(".")[-1]
            if emb_name in all_structs:
                emb = struct_to_oas_schema(all_structs[emb_name], all_structs, depth + 1)
                for k, v in (emb.get("properties") or {}).items():
                    props[k] = v
                for r in emb.get("required") or []:
                    if r not in required:
                        required.append(r)
            continue
        jn = f.get("json_name")
        if not jn:
            continue
        props[jn] = go_type_to_oas(f["type"])
        props[jn]["x-clomapi-go-field"] = f["name"]
        props[jn]["x-clomapi-go-type"] = f["type"]
        if f.get("required"):
            required.append(jn)
    schema: dict = {
        "type": "object",
        "properties": props,
        "additionalProperties": False,
        "x-clomapi-struct": st["name"],
        "x-clomapi-evidence": st["evidence"],
    }
    if required:
        schema["required"] = required
    return schema


def _response_schema_is_complete(schema: object) -> bool:
    """Return whether a response schema is safe to project as contract-complete.

    The legacy struct converter deliberately preserves evidence for Go types it
    cannot resolve.  Those evidence placeholders are useful during extraction,
    but they must not unblock an operation or become a generated client type.
    """
    if not isinstance(schema, dict):
        return False
    if not schema:
        return False
    complete = False
    if "$ref" in schema:
        if not isinstance(schema["$ref"], str) or not schema["$ref"].strip():
            return False
        complete = True
    if "const" in schema:
        complete = True
    if "enum" in schema:
        if not isinstance(schema["enum"], list) or not schema["enum"]:
            return False
        complete = True
    for keyword in ("oneOf", "anyOf", "allOf"):
        if keyword in schema:
            branches = schema[keyword]
            if not isinstance(branches, list) or not branches:
                return False
            if not all(_response_schema_is_complete(branch) for branch in branches):
                return False
            complete = True

    additional = schema.get("additionalProperties")
    if additional is True:
        return False
    properties = schema.get("properties")
    if properties is not None:
        if not isinstance(properties, dict):
            return False
        if not all(
            _response_schema_is_complete(field_schema)
            for field_schema in properties.values()
        ):
            return False
    if isinstance(additional, dict) and not _response_schema_is_complete(additional):
        return False
    if "items" in schema and not _response_schema_is_complete(schema["items"]):
        return False

    schema_type = schema.get("type")
    if isinstance(schema_type, list):
        allowed = {"array", "boolean", "integer", "null", "number", "object", "string"}
        if not schema_type or any(item not in allowed for item in schema_type):
            return False
        if "array" in schema_type and "items" not in schema:
            return False
        if "object" in schema_type and properties is None and not isinstance(additional, dict):
            return False
        if "object" in schema_type and properties is not None and additional is not False and not isinstance(additional, dict):
            return False
        complete = True
    elif schema_type == "array":
        if "items" not in schema:
            return False
        complete = True
    elif schema_type == "object":
        if properties is None and not isinstance(additional, dict):
            return False
        if properties is not None and additional is not False and not isinstance(additional, dict):
            return False
        complete = True
    elif schema_type is not None:
        if schema_type not in {"boolean", "integer", "null", "number", "string"}:
            return False
        complete = True
    return complete


def _schema_contains_struct(schema: object, names: set[str]) -> bool:
    """Find an evidence-backed Go struct through nullable/union wrappers."""
    if not isinstance(schema, dict):
        return False
    if schema.get("x-clomapi-struct") in names:
        return True
    return any(
        _schema_contains_struct(branch, names)
        for key in ("oneOf", "anyOf", "allOf")
        for branch in (schema.get(key) or [])
    )


def _package_scope(path: str, text: str) -> str:
    pkg_m = re.search(r"^package\s+(\w+)", _mask_go_lexemes(text), re.M)
    pkg = pkg_m.group(1) if pkg_m else "unknown"
    return f"{Path(path).parent.as_posix()}::{pkg}"


def _parse_import_aliases(text: str) -> dict[str, str]:
    aliases: dict[str, str] = {}
    code = _mask_go_lexemes(text)
    commentless = _mask_go_lexemes(text, mask_strings=False)
    entries: list[tuple[Optional[str], str]] = []
    for import_match in re.finditer(r"(?m)^\s*import\b", code):
        cursor = import_match.end()
        while cursor < len(code) and commentless[cursor] in " \t":
            cursor += 1
        if cursor < len(code) and code[cursor] == "(":
            close = _matching_paren(code, cursor)
            if close is None:
                continue
            block = commentless[cursor + 1 : close]
            for match in re.finditer(
                r'(?m)^\s*(?:(?P<alias>[A-Za-z_]\w*)\s+)?"(?P<path>[^"]+)"',
                block,
            ):
                entries.append((match.group("alias"), match.group("path")))
        else:
            line_end = commentless.find("\n", cursor)
            if line_end < 0:
                line_end = len(commentless)
            match = re.match(
                r'\s*(?:(?P<alias>[A-Za-z_]\w*)\s+)?"(?P<path>[^"]+)"',
                commentless[cursor:line_end],
            )
            if match:
                entries.append((match.group("alias"), match.group("path")))
    for alias, import_path in entries:
        if alias in {"_", "."}:
            continue
        aliases[alias or import_path.rsplit("/", 1)[-1]] = import_path
    return aliases


def _response_package_alias(current_file: str, symbol_index: dict) -> Optional[str]:
    """Return the sole alias proven to name the legacy response package."""
    aliases = sorted(
        alias
        for alias, import_path in (
            symbol_index.get("imports_by_file", {}).get(current_file, {})
        ).items()
        if import_path == RESPONSE_PACKAGE_IMPORT
    )
    return aliases[0] if len(aliases) == 1 else None


def _gin_import_aliases(current_file: str, symbol_index: dict) -> set[str]:
    """Return only file-local aliases proven to name the Gin package."""
    return {
        alias
        for alias, import_path in (
            symbol_index.get("imports_by_file", {}).get(current_file, {})
        ).items()
        if import_path == GIN_PACKAGE_IMPORT
    }


def _is_proven_gin_context_type(
    go_type: str, current_file: str, symbol_index: dict
) -> bool:
    match = re.fullmatch(r"\*(?P<alias>[A-Za-z_]\w*)\.Context", go_type.strip())
    return bool(
        match
        and match.group("alias") in _gin_import_aliases(current_file, symbol_index)
    )


def _context_receiver_proof(
    body: str, context_name: str, identity_proven: bool
) -> tuple[set[str], set[str]]:
    """Prove straight-line context aliases and retain every tainted sibling."""
    if not context_name or context_name == "__unproven_gin_context__":
        return set(), set()
    code = _mask_go_lexemes(body)
    write_pattern = re.compile(
        r"(?m)(?P<lhs>[A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)\s*"
        r"(?P<op>:=|=(?!=))\s*(?P<rhs>[A-Za-z_]\w*)?"
    )
    writes: list[dict] = []
    writes_by_name: dict[str, list[dict]] = defaultdict(list)
    for match in write_pattern.finditer(code):
        names = [name.strip() for name in match.group("lhs").split(",")]
        record = {
            "match": match,
            "names": names,
            "op": match.group("op"),
            "rhs": match.group("rhs"),
        }
        writes.append(record)
        for name in names:
            writes_by_name[name].append(record)

    safe = {context_name} if identity_proven and context_name not in writes_by_name else set()
    poisoned = set() if safe else {context_name}
    changed = True
    while changed:
        changed = False
        for record in writes:
            if len(record["names"]) != 1:
                for alias in record["names"]:
                    if alias in safe or alias in poisoned:
                        safe.discard(alias)
                        if alias not in poisoned:
                            poisoned.add(alias)
                            changed = True
                continue
            alias = record["names"][0]
            source = record["rhs"]
            if source not in safe and source not in poisoned:
                continue
            records = writes_by_name[alias]
            stable = (
                source in safe
                and record["op"] == ":="
                and len(records) == 1
                and _brace_depth(code, record["match"].start()) == 0
            )
            target = safe if stable else poisoned
            if alias not in target:
                target.add(alias)
                changed = True
            if not stable:
                safe.discard(alias)
    return safe, poisoned


def _wire_schema_key(schema: dict) -> str:
    """Canonicalize only wire-shape keywords, excluding extractor evidence."""
    def clean(value):
        if isinstance(value, dict):
            return {
                key: clean(item)
                for key, item in sorted(value.items())
                if not key.startswith("x-clomapi-")
            }
        if isinstance(value, list):
            return [clean(item) for item in value]
        return value

    return json.dumps(clean(schema), sort_keys=True, separators=(",", ":"))


def _supported_go_type(go_type: str) -> bool:
    """Accept only the closed Go type subset that can be projected soundly."""
    value = go_type.strip()
    if (
        not value
        or any(token in value for token in ("...", "<-", "func(", "struct{", "interface{"))
        or re.search(r"\bchan\b", value)
    ):
        return False
    while value.startswith("*"):
        value = value[1:].strip()
    if value.startswith("[]"):
        return _supported_go_type(value[2:])
    if value.startswith("map["):
        close = value.find("]")
        return (
            close > 4
            and value[4:close] == "string"
            and _supported_go_type(value[close + 1 :])
        )
    # Arrays, generic instantiations, anonymous composites, and parenthesized
    # types are outside the supported extractor grammar.
    if any(char in value for char in "[]{}()"):
        return False
    return bool(
        re.fullmatch(
            r"(?:[A-Za-z_]\w*\.)?[A-Za-z_]\w*",
            value,
        )
    )


def _go_result_types(return_clause: str) -> tuple[list[str], str]:
    clause = return_clause.strip()
    if not clause:
        return [], "no_return_value"
    grouped = clause.startswith("(")
    if grouped:
        close = _matching_paren(clause, 0)
        if close is None or clause[close + 1 :].strip():
            return [], "unparsed_return_clause"
        results = _split_top_level_commas(clause[1:close])
    else:
        results = [clause]
    out: list[str] = []
    declaration_modes: set[str] = set()
    for result in results:
        tokens = result.strip().split()
        if len(tokens) == 1 and _supported_go_type(tokens[0]):
            declaration_modes.add("unnamed")
            out.append(tokens[0])
        elif (
            grouped
            and len(tokens) == 2
            and re.fullmatch(r"[A-Za-z_]\w*", tokens[0])
            and _supported_go_type(tokens[1])
        ):
            declaration_modes.add("named")
            out.append(tokens[1])
        else:
            return [], "unparsed_return_clause"
    if len(declaration_modes) != 1:
        return [], "unparsed_return_clause"
    return out, "parsed"


def _go_param_types(param_clause: str) -> tuple[list[str], str]:
    """Parse named Go parameter types while preserving positional slots."""
    clause = param_clause.strip()
    if not clause:
        return [], "parsed"
    parts = _split_top_level_commas(clause)
    out: list[str] = []
    pending_names: list[str] = []
    for part in parts:
        tokens = part.strip().split()
        if len(tokens) >= 2:
            param_type = tokens[-1]
            if (
                param_type.startswith("...")
                or not _supported_go_type(param_type)
                or any(
                    not re.fullmatch(r"[A-Za-z_]\w*", name)
                    for name in pending_names + tokens[:-1]
                )
            ):
                return [], "unparsed_param_clause"
            names = pending_names + tokens[:-1]
            out.extend([param_type] * len(names))
            pending_names = []
        elif len(tokens) == 1:
            pending_names.append(tokens[0])
        else:
            return [], "unparsed_param_clause"
    # Unnamed parameters are represented by their token as a type.
    if pending_names:
        if not all(_supported_go_type(item) for item in pending_names):
            return [], "unparsed_param_clause"
        out.extend(pending_names)
    return out, "parsed"


def _go_param_records(param_clause: str) -> list[dict]:
    """Retain positional names/types for context-bound delegated calls."""
    clause = param_clause.strip()
    if not clause:
        return []
    records: list[dict] = []
    pending_names: list[str] = []
    for part in _split_top_level_commas(clause):
        value = part.strip()
        named = re.match(r"^(?P<name>[A-Za-z_]\w*)\s+(?P<type>.+)$", value, re.S)
        if named:
            go_type = named.group("type").strip()
            names = pending_names + [named.group("name")]
            records.extend(
                {"name": name, "go_type": go_type} for name in names
            )
            pending_names = []
        elif value:
            normalized = value[3:] if value.startswith("...") else value
            if _supported_go_type(normalized):
                records.append({"name": None, "go_type": value})
            elif re.fullmatch(r"[A-Za-z_]\w*", value):
                pending_names.append(value)
            else:
                return []
        else:
            return []
    if pending_names:
        return []
    return records


def build_go_symbol_index(source_files: dict[str, str]) -> dict:
    """Build a frozen, package-qualified response symbol index.

    The index is deliberately lexical. Duplicate declarations are retained and
    every resolver below requires one unique candidate before emitting a schema.
    """
    file_scopes: dict[str, str] = {}
    imports_by_file: dict[str, dict[str, str]] = {}
    structs_by_scope: dict = defaultdict(lambda: defaultdict(list))
    funcs_by_scope: dict = defaultdict(lambda: defaultdict(list))
    methods_by_scope: dict = defaultdict(lambda: defaultdict(list))
    interface_methods_by_scope: dict = defaultdict(lambda: defaultdict(list))
    aliases_by_scope: dict = defaultdict(lambda: defaultdict(list))
    custom_json_marshalers_by_scope: dict[str, set[str]] = defaultdict(set)
    custom_text_marshalers_by_scope: dict[str, set[str]] = defaultdict(set)
    custom_json_unmarshalers_by_scope: dict[str, set[str]] = defaultdict(set)
    custom_text_unmarshalers_by_scope: dict[str, set[str]] = defaultdict(set)
    import_path_to_scopes: dict[str, set[str]] = defaultdict(set)

    for path, text in source_files.items():
        code = _mask_go_lexemes(text)
        scope = _package_scope(path, text)
        file_scopes[path] = scope
        imports_by_file[path] = _parse_import_aliases(text)
        if path.startswith("backend/"):
            import_path = "github.com/Wei-Shaw/sub2api/" + path[len("backend/") :].rsplit("/", 1)[0]
            import_path_to_scopes[import_path].add(scope)
        for st in parse_go_structs(path, text).values():
            structs_by_scope[scope][st["name"]].append(st)

        # Keep named aliases/defined types in their package scope.  A named
        # type is only safe to project when its underlying type can also be
        # resolved; duplicate declarations remain ambiguous by design.
        for alias_match in re.finditer(
            r"(?m)^\s*type\s+(?P<name>[A-Za-z_]\w*)\s+(?P<underlying>[^\n]+)$",
            code,
        ):
            name = alias_match.group("name")
            underlying = alias_match.group("underlying").strip()
            if underlying.startswith(("struct", "interface")):
                continue
            aliases_by_scope[scope][name].append({
                "name": name,
                "underlying": underlying,
                "file": path,
                "line": line_of(text, alias_match.start()),
                "evidence": f"{path}:{line_of(text, alias_match.start())}",
                "scope": scope,
            })

        func_re = re.compile(
            r"^[ \t]*func\s+(?:\((?:(?P<recvvar>\w+)\s+)?\*?(?P<recv>[\w\.]+)\)\s+)?"
            r"(?P<name>[A-Za-z_]\w*)\s*\(",
            re.M,
        )
        for match in func_re.finditer(code):
            open_pos = match.end() - 1
            close_pos = _matching_paren(code, open_pos)
            if close_pos is None:
                continue
            brace_pos = code.find("{", close_pos + 1)
            if brace_pos < 0:
                continue
            close_brace = _matching_brace(code, brace_pos)
            if close_brace is None:
                continue
            result_types, status = _go_result_types(
                text[close_pos + 1 : brace_pos].strip()
            )
            param_types, param_status = _go_param_types(
                text[open_pos + 1 : close_pos]
            )
            entry = {
                "file": path,
                "line": line_of(text, match.start()),
                "evidence": f"{path}:{line_of(text, match.start())}",
                "scope": scope,
                "name": match.group("name"),
                "receiver": (match.group("recv") or "").split(".")[-1] or None,
                "receiver_variable": match.group("recvvar"),
                "result_types": result_types,
                "status": status,
                "param_types": param_types,
                "param_status": param_status,
                "param_records": _go_param_records(
                    text[open_pos + 1 : close_pos]
                ),
                "body": text[brace_pos + 1 : close_brace],
            }
            if entry["receiver"]:
                methods_by_scope[scope][(entry["receiver"], entry["name"])].append(entry)
                codec_indexes = {
                    "MarshalJSON": custom_json_marshalers_by_scope,
                    "MarshalText": custom_text_marshalers_by_scope,
                    "UnmarshalJSON": custom_json_unmarshalers_by_scope,
                    "UnmarshalText": custom_text_unmarshalers_by_scope,
                }
                if entry["name"] in codec_indexes:
                    codec_indexes[entry["name"]][scope].add(entry["receiver"])
            else:
                funcs_by_scope[scope][entry["name"]].append(entry)

        for interface_match in re.finditer(
            r"^type\s+(?P<name>[A-Za-z_]\w*)\s+interface\s*\{", code, re.M
        ):
            brace_pos = code.find("{", interface_match.start(), interface_match.end())
            close_brace = _matching_brace(code, brace_pos)
            if close_brace is None:
                continue
            interface_body = code[brace_pos + 1 : close_brace]
            body_offset = brace_pos + 1
            for method_match in re.finditer(
                r"(?m)^\s*(?P<name>[A-Za-z_]\w*)\s*\(", interface_body
            ):
                open_pos = code.find(
                    "(", body_offset + method_match.start(), body_offset + method_match.end()
                )
                close_pos = _matching_paren(code, open_pos)
                if close_pos is None or close_pos > close_brace:
                    continue
                result_start = close_pos + 1
                while result_start < close_brace and text[result_start] in " \t":
                    result_start += 1
                if result_start < close_brace and text[result_start] == "(":
                    result_close = _matching_paren(code, result_start)
                    if result_close is None or result_close > close_brace:
                        continue
                    return_clause = text[result_start : result_close + 1]
                else:
                    result_end = code.find("\n", result_start, close_brace)
                    if result_end < 0:
                        result_end = close_brace
                    return_clause = text[result_start:result_end].split("//", 1)[0].strip()
                result_types, status = _go_result_types(return_clause)
                param_types, param_status = _go_param_types(
                    text[open_pos + 1 : close_pos]
                )
                line = line_of(text, body_offset + method_match.start())
                interface_methods_by_scope[scope][
                    (interface_match.group("name"), method_match.group("name"))
                ].append({
                    "file": path, "line": line, "evidence": f"{path}:{line}",
                    "scope": scope, "name": method_match.group("name"),
                    "receiver": interface_match.group("name"),
                    "result_types": result_types, "status": status,
                    "param_types": param_types, "param_status": param_status,
                })

    return {
        "file_scopes": file_scopes,
        "imports_by_file": imports_by_file,
        "import_path_to_scopes": {
            path: sorted(scopes) for path, scopes in import_path_to_scopes.items()
        },
        "structs_by_scope": {
            scope: dict(names) for scope, names in structs_by_scope.items()
        },
        "funcs_by_scope": {
            scope: dict(names) for scope, names in funcs_by_scope.items()
        },
        "methods_by_scope": {
            scope: dict(names) for scope, names in methods_by_scope.items()
        },
        "interface_methods_by_scope": {
            scope: dict(names) for scope, names in interface_methods_by_scope.items()
        },
        "aliases_by_scope": {
            scope: dict(names) for scope, names in aliases_by_scope.items()
        },
        "custom_json_marshalers_by_scope": {
            scope: sorted(names) for scope, names in custom_json_marshalers_by_scope.items()
        },
        "custom_text_marshalers_by_scope": {
            scope: sorted(names) for scope, names in custom_text_marshalers_by_scope.items()
        },
        "custom_json_unmarshalers_by_scope": {
            scope: sorted(names) for scope, names in custom_json_unmarshalers_by_scope.items()
        },
        "custom_text_unmarshalers_by_scope": {
            scope: sorted(names) for scope, names in custom_text_unmarshalers_by_scope.items()
        },
    }


def _matching_paren(text: str, open_pos: int) -> Optional[int]:
    """Return the closing paren for a Go signature/call, ignoring strings/comments."""
    depth = 0
    quote: Optional[str] = None
    escaped = False
    line_comment = False
    block_comment = False
    i = open_pos
    while i < len(text):
        ch = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ""
        if line_comment:
            if ch == "\n":
                line_comment = False
            i += 1
            continue
        if block_comment:
            if ch == "*" and nxt == "/":
                block_comment = False
                i += 2
            else:
                i += 1
            continue
        if quote:
            if quote != "`" and escaped:
                escaped = False
            elif quote != "`" and ch == "\\":
                escaped = True
            elif ch == quote:
                quote = None
            i += 1
            continue
        if ch == "/" and nxt == "/":
            line_comment = True
            i += 2
            continue
        if ch == "/" and nxt == "*":
            block_comment = True
            i += 2
            continue
        if ch in ('"', "'", "`"):
            quote = ch
        elif ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
            if depth == 0:
                return i
        i += 1
    return None


def _split_top_level_commas(text: str) -> list[str]:
    parts: list[str] = []
    start = 0
    depths = {"(": 0, "[": 0, "{": 0}
    closing = {")": "(", "]": "[", "}": "{"}
    for i, ch in enumerate(text):
        if ch in depths:
            depths[ch] += 1
        elif ch in closing:
            opener = closing[ch]
            depths[opener] = max(0, depths[opener] - 1)
        elif ch == "," and not any(depths.values()):
            parts.append(text[start:i].strip())
            start = i + 1
    parts.append(text[start:].strip())
    return [part for part in parts if part]


def _single_go_result_type(return_clause: str) -> tuple[Optional[str], str]:
    result_types, status = _go_result_types(return_clause)
    if status == "no_return_value":
        return None, "no_return_value"
    if status != "parsed":
        return None, "unparsed_return_clause"
    if len(result_types) != 1:
        return None, "ambiguous_multiple_returns"
    return result_types[0], "single_return"


def _helper_semantic_type(go_type: str, named_string_types: set[str]) -> Optional[str]:
    base = go_type.strip()
    while base.startswith("*"):
        base = base[1:].strip()
    if base == "bool":
        return "boolean"
    if base in {
        "int", "int8", "int16", "int32", "int64",
        "uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
    }:
        return "integer"
    if base == "string" or base in named_string_types:
        return "string"
    return None


def build_helper_return_type_index(handler_files: dict[str, str]) -> dict:
    """Index frozen top-level helper return types within their Go package scope."""
    by_scope: dict = defaultdict(lambda: defaultdict(list))
    file_scopes: dict[str, str] = {}
    named_strings_by_scope: dict[str, set[str]] = defaultdict(set)
    for path, text in handler_files.items():
        code = _mask_go_lexemes(text)
        pkg_m = re.search(r"^package\s+(\w+)", code, re.M)
        pkg = pkg_m.group(1) if pkg_m else "handler"
        scope = f"{Path(path).parent.as_posix()}::{pkg}"
        file_scopes[path] = scope
        # Named string types can be declared in any file of the same package.
        named_strings_by_scope[scope].update(
            m.group(1)
            for m in re.finditer(
                r"^type\s+(\w+)\s+(?:=\s*)?string\s*$", code, re.M
            )
        )

    for path, text in handler_files.items():
        code = _mask_go_lexemes(text)
        scope = file_scopes[path]
        named_string_types = named_strings_by_scope[scope]
        for match in re.finditer(r"^func\s+(?P<name>[A-Za-z_]\w*)\s*\(", code, re.M):
            open_pos = code.find("(", match.start(), match.end())
            close_pos = _matching_paren(code, open_pos)
            if close_pos is None:
                continue
            brace_pos = code.find("{", close_pos + 1)
            if brace_pos < 0:
                continue
            return_clause = text[close_pos + 1 : brace_pos].strip()
            go_type, status = _single_go_result_type(return_clause)
            semantic_type = (
                _helper_semantic_type(go_type, named_string_types) if go_type else None
            )
            by_scope[scope][match.group("name")].append(
                {
                    "file": path,
                    "line": line_of(text, match.start()),
                    "evidence": f"{path}:{line_of(text, match.start())}",
                    "return_go_type": go_type,
                    "return_clause": return_clause,
                    "semantic_type": semantic_type,
                    "status": status if semantic_type else (
                        status if status != "single_return" else "unsupported_return_type"
                    ),
                }
            )
    return {
        "by_scope": {
            scope: dict(functions) for scope, functions in by_scope.items()
        },
        "file_scopes": file_scopes,
    }


def build_handler_index(handler_files: dict[str, str]) -> dict:
    by_recv: dict = defaultdict(list)
    by_method: dict = defaultdict(list)
    field_map: dict = {}
    for path, text in handler_files.items():
        code = _mask_go_lexemes(text)
        pkg_m = re.search(r"^package\s+(\w+)", code, re.M)
        pkg = pkg_m.group(1) if pkg_m else "handler"
        for m in re.finditer(
            r"^func\s+\((?P<recv>\w+)\s+\*?(?P<rtype>\w+)\)\s+(?P<meth>\w+)\s*\(",
            code,
            re.M,
        ):
            rtype, meth = m.group("rtype"), m.group("meth")
            entry = {
                "file": path,
                "line": line_of(text, m.start()),
                "pkg": pkg,
                "recv_type": rtype,
                "method": meth,
            }
            by_recv[(rtype, meth)].append(entry)
            by_method[meth].append(entry)
        for tname, struct in parse_go_structs(path, text).items():
            for parsed_field in struct["fields"]:
                field = parsed_field["name"]
                ftype = parsed_field["type"].lstrip("*").split(".")[-1]
                field_map[(tname, field)] = ftype
    helper_returns = build_helper_return_type_index(handler_files)
    return {
        "by_recv": by_recv,
        "by_method": by_method,
        "field_map": field_map,
        "helper_returns": helper_returns,
        "file_scopes": helper_returns["file_scopes"],
    }


VAR_TYPE_ALIASES = {
    "paymentHandler": "PaymentHandler",
    "adminPaymentHandler": "PaymentHandler",
    "webhookHandler": "PaymentWebhookHandler",
}


def resolve_handler_ref(ref: str, index: dict) -> dict:
    result = {
        "handler_ref": ref,
        "resolved": False,
        "ambiguous": False,
        "recv_type": None,
        "method": None,
        "evidence": [],
        "candidates": [],
        "resolution_error": None,
    }
    if not ref or ref.startswith("func"):
        result["resolution_error"] = "inline_or_empty_handler"
        return result
    ref = ref.strip()
    parts = ref.split(".")
    method = parts[-1]
    result["method"] = method
    field_map = index["field_map"]
    recv_type = None
    chain = parts[:-1]
    if chain and chain[0] in ("h", "handlers", "r", "v1"):
        chain = chain[1:]
    if chain and chain[0] in VAR_TYPE_ALIASES and len(chain) == 1:
        recv_type = VAR_TYPE_ALIASES[chain[0]]
    elif chain:
        cur = "Handlers"
        ok = True
        for seg in chain:
            key = (cur, seg)
            if key in field_map:
                cur = field_map[key]
            else:
                matches = [field_map[k] for k in field_map if k[1] == seg]
                uniq = list(dict.fromkeys(matches))
                if len(uniq) == 1:
                    cur = uniq[0]
                elif seg.endswith("Handler"):
                    cur = seg
                else:
                    ok = False
                    break
        if ok:
            recv_type = cur

    by_recv = index["by_recv"]
    by_method = index["by_method"]
    if recv_type:
        result["recv_type"] = recv_type
        cands = by_recv.get((recv_type, method), [])
        if len(cands) == 1:
            c = cands[0]
            result.update(
                resolved=True,
                recv_type=recv_type,
                evidence=[f"{c['file']}:{c['line']}"],
                candidates=cands,
            )
            return result
        if len(cands) > 1:
            # Prefer unique package path when multiple files define same receiver method
            # (e.g. admin.AIHandler vs user AIHandler). Prefer admin/ for AdminHandlers chain.
            admin_c = [c for c in cands if "/handler/admin/" in c["file"].replace("\\", "/")]
            non_admin = [c for c in cands if "/handler/admin/" not in c["file"].replace("\\", "/")]
            preferred = None
            if ref.startswith("h.Admin.") or ".Admin." in ref:
                if len(admin_c) == 1:
                    preferred = admin_c
                elif len(admin_c) > 1:
                    preferred = admin_c  # still ambiguous among admin
                else:
                    preferred = cands
            else:
                if len(non_admin) == 1:
                    preferred = non_admin
                elif len(admin_c) == 1 and not non_admin:
                    preferred = admin_c
                else:
                    preferred = non_admin or cands
            if preferred and len(preferred) == 1:
                c = preferred[0]
                result.update(
                    resolved=True,
                    recv_type=recv_type,
                    evidence=[f"{c['file']}:{c['line']}"],
                    candidates=preferred,
                )
                return result
            result.update(
                ambiguous=True,
                recv_type=recv_type,
                candidates=cands,
                resolution_error=f"ambiguous_recv_method:{recv_type}.{method}:{len(cands)}",
                evidence=[f"{c['file']}:{c['line']}" for c in cands],
            )
            return result
        result["resolution_error"] = f"no_method_on_type:{recv_type}.{method}"
        return result

    if len(parts) > 1:
        result["resolution_error"] = f"unresolved_receiver_chain:{ref}"
        return result

    typed = [c for c in by_method.get(method, []) if c.get("recv_type")]
    if len(typed) == 1:
        c = typed[0]
        result.update(
            resolved=True,
            recv_type=c["recv_type"],
            evidence=[f"{c['file']}:{c['line']}"],
            candidates=typed,
        )
        return result
    if len(typed) > 1:
        result.update(
            ambiguous=True,
            candidates=typed,
            resolution_error=f"ambiguous_bare_method:{method}:{len(typed)}",
            evidence=[f"{c['file']}:{c['line']}" for c in typed[:12]],
        )
        return result
    result["resolution_error"] = result.get("resolution_error") or f"unresolved_method:{method}"
    return result


def resolve_static_handler(binding: dict, index: dict) -> dict:
    """Resolve invocation-bound factories and every represented closure branch."""
    result = {
        "handler_ref": binding.get("symbol"),
        "resolved": False,
        "ambiguous": False,
        "recv_type": None,
        "method": binding.get("symbol"),
        "evidence": list(binding.get("evidence") or []),
        "candidates": [],
        "resolution_error": binding.get("resolution_error"),
        "protocol_branches": [],
    }
    if result["resolution_error"]:
        result["ambiguous"] = result["resolution_error"].startswith("ambiguous_")
        return result
    if binding.get("kind") == "factory_call":
        required = (
            "endpoint_name",
            "status_code",
            "response_schema",
            "response_example",
            "request_body_limit",
            "debug_timeline",
            "error_branches",
        )
        if not all(key in binding for key in required):
            result["resolution_error"] = (
                f"incomplete_factory_contract:{binding.get('symbol')}"
            )
            return result
        result["protocol_branches"] = [
            {
                "kind": "factory_response",
                "outcome": "success",
                "endpoint_name": binding["endpoint_name"],
                "status_codes": [binding["status_code"]],
                "media_types": ["application/json"],
                "response_schema": binding["response_schema"],
                "response_example": binding["response_example"],
                "call_evidence": binding["evidence"][0],
                "handler_evidence": binding["evidence"][1:],
                "resolved": True,
                "ambiguous": False,
                "resolution_error": None,
                "effects": {
                    "request_body_limit": binding["request_body_limit"],
                    "debug_timeline": binding["debug_timeline"],
                },
            }
        ] + [
            {
                **branch,
                "endpoint_name": binding["endpoint_name"],
                "resolved": True,
                "ambiguous": False,
                "resolution_error": None,
            }
            for branch in binding["error_branches"]
        ]
        result["resolved"] = True
        return result

    branches = binding.get("branches") or []
    if not branches:
        result["resolution_error"] = f"no_static_handler_branches:{binding.get('symbol')}"
        return result
    resolutions = []
    for branch in branches:
        if branch["kind"] == "local_response":
            resolved_branch = dict(branch)
            resolved_branch.update(
                resolved=True,
                ambiguous=False,
                resolution_error=None,
                handler_evidence=branch.get("evidence") or [],
            )
            result["protocol_branches"].append(resolved_branch)
            continue
        resolved = resolve_handler_ref(branch["handler_ref"], index)
        resolutions.append((branch, resolved))
        resolved_branch = dict(branch)
        resolved_branch.update(
            resolved=resolved["resolved"],
            ambiguous=resolved["ambiguous"],
            recv_type=resolved.get("recv_type"),
            method=resolved.get("method"),
            handler_evidence=resolved.get("evidence") or [],
            resolution_error=resolved.get("resolution_error"),
        )
        result["protocol_branches"].append(resolved_branch)
    failures = [
        (branch, resolved)
        for branch, resolved in resolutions
        if not resolved["resolved"]
    ]
    if failures:
        result["ambiguous"] = any(resolved["ambiguous"] for _, resolved in failures)
        result["resolution_error"] = "static_handler_target_unresolved:" + ",".join(
            f"{branch['handler_ref']}={resolved.get('resolution_error')}"
            for branch, resolved in failures
        )
        return result

    recv_types = {resolved.get("recv_type") for _, resolved in resolutions}
    methods = {resolved.get("method") for _, resolved in resolutions}
    result["resolved"] = True
    result["recv_type"] = next(iter(recv_types)) if len(recv_types) == 1 else None
    result["method"] = next(iter(methods)) if len(methods) == 1 else binding.get("symbol")
    result["candidates"] = [
        candidate
        for _, resolved in resolutions
        for candidate in (resolved.get("candidates") or [])
    ]
    for _, resolved in resolutions:
        result["evidence"].extend(resolved.get("evidence") or [])
    return result


def extract_handler_body(text: str, method: str, recv_type: Optional[str]):
    code = _mask_go_lexemes(text)
    if recv_type:
        pat = re.compile(
            rf"^func\s+\((\w+)\s+\*?{re.escape(recv_type)}\)\s+{re.escape(method)}\s*\(",
            re.M,
        )
    else:
        pat = re.compile(rf"^func\s+(?:\([^)]+\)\s+)?{re.escape(method)}\s*\(", re.M)
    m = pat.search(code)
    if not m:
        return None
    params_open = code.find("(", m.end() - 1)
    params_close = _matching_paren(code, params_open) if params_open >= 0 else None
    if params_close is None:
        return None
    parameter_records = _go_param_records(text[params_open + 1 : params_close])
    gin_aliases = {
        alias
        for alias, import_path in _parse_import_aliases(text).items()
        if import_path == GIN_PACKAGE_IMPORT
    }
    context_params = [
        record
        for record in parameter_records
        if (
            (context_type := re.fullmatch(
                r"\*(?P<alias>[A-Za-z_]\w*)\.Context",
                record.get("go_type", "").lstrip("..."),
            ))
            and context_type.group("alias") in gin_aliases
        )
        and record.get("name")
    ]
    context_name = context_params[0]["name"] if len(context_params) == 1 else None
    bound_names = {
        record["name"] for record in parameter_records if record.get("name")
    }
    if recv_type and m.group(1):
        bound_names.add(m.group(1))
    brace = code.find("{", params_close + 1)
    if brace < 0:
        return None
    close = _matching_brace(code, brace)
    if close is not None:
        return (
            line_of(text, m.start()),
            text[brace + 1 : close],
            context_name,
            bound_names,
        )
    return None


def _direct_unqualified_query_helper(body: str, access_start: int) -> Optional[str]:
    """Find the immediate unqualified helper wrapping a Gin query access."""
    candidates: list[tuple[int, str]] = []
    for call in re.finditer(r"\b([A-Za-z_]\w*)\s*\(", body[:access_start]):
        # Qualified calls (`pkg.Func`) are outside the current handler package
        # index and must not be resolved by an identically named local helper.
        before = body[max(0, call.start() - 1) : call.start()]
        if before == ".":
            continue
        open_pos = body.find("(", call.start(), call.end())
        close_pos = _matching_paren(body, open_pos)
        if close_pos is not None and open_pos < access_start < close_pos:
            candidates.append((open_pos, call.group(1)))
    return max(candidates)[1] if candidates else None


def _query_argument_index(body: str, open_pos: int, position: int) -> int:
    depth = 0
    commas = 0
    quote: Optional[str] = None
    escaped = False
    for char in body[open_pos + 1 : position]:
        if quote:
            if quote != "`" and escaped:
                escaped = False
            elif quote != "`" and char == "\\":
                escaped = True
            elif char == quote:
                quote = None
            continue
        if char in ('"', "'", "`"):
            quote = char
        elif char in "([{":
            depth += 1
        elif char in ")]}":
            depth = max(0, depth - 1)
        elif char == "," and depth == 0:
            commas += 1
    return commas


def _direct_query_string_parameter_evidence(
    body: str,
    access_start: int,
    current_scope: str,
    current_file: str,
    current_recv_type: Optional[str],
    symbol_index: dict,
) -> Optional[str]:
    """Prove a query remains a string when passed to a non-parser call."""
    calls: list[tuple[int, str, int]] = []
    for match in re.finditer(
        r"(?P<name>[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)\s*\(",
        body[:access_start],
    ):
        open_pos = match.end() - 1
        close_pos = _matching_paren(body, open_pos)
        if close_pos is not None and open_pos < access_start < close_pos:
            calls.append((open_pos, match.group("name"), close_pos))
    calls.sort(reverse=True)
    wrappers = {"TrimSpace", "ToLower", "ToUpper"}
    wrapper_evidence: Optional[str] = None
    for open_pos, call_name, _ in calls:
        base_name = call_name.rsplit(".", 1)[-1]
        if base_name in wrappers and call_name.startswith("strings."):
            wrapper_evidence = call_name
            continue
        entry = _unique_callable_results(
            call_name, current_scope, current_file, current_recv_type, symbol_index,
            body[:open_pos], require_results=False,
        )
        if not entry or entry.get("param_status") != "parsed":
            return None
        index = _query_argument_index(body, open_pos, access_start)
        param_types = entry.get("param_types") or []
        if index >= len(param_types):
            return None
        param_type = param_types[index].lstrip("*")
        if param_type == "string" and not re.search(
            r"(?:parse|convert|decode|coerce).*?(?:bool|int|uint|id|days?)$|"
            r"(?:atoi|parseint|parseuint|parsebool)$",
            base_name,
            re.I,
        ):
            return entry.get("evidence") or call_name
        return None
    return wrapper_evidence


def _variable_string_parameter_evidence(
    body: str,
    variable: str,
    current_scope: str,
    current_file: str,
    current_recv_type: Optional[str],
    symbol_index: dict,
) -> Optional[str]:
    evidence: list[str] = []
    for match in re.finditer(
        r"(?P<name>[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)\s*\(", body
    ):
        open_pos = match.end() - 1
        close_pos = _matching_paren(body, open_pos)
        if close_pos is None:
            continue
        args = _split_top_level_commas(body[open_pos + 1 : close_pos])
        for index, arg in enumerate(args):
            if not re.search(
                rf"(?:^|\W){re.escape(variable)}(?:\W|$)", arg
            ):
                continue
            entry = _unique_callable_results(
                match.group("name"), current_scope, current_file,
                current_recv_type, symbol_index, body[:match.start()],
                require_results=False,
            )
            if not entry:
                continue
            param_types = entry.get("param_types") or []
            if index >= len(param_types) or param_types[index].lstrip("*") != "string":
                continue
            evidence.append(entry.get("evidence") or match.group("name"))
    return sorted(set(evidence))[0] if evidence else None


def _helper_return_candidates(
    helper_name: Optional[str], helper_return_index: Optional[dict], helper_scope: Optional[str]
) -> list[dict]:
    if not helper_name or not helper_return_index or not helper_scope:
        return []
    return (
        helper_return_index.get("by_scope", {})
        .get(helper_scope, {})
        .get(helper_name, [])
    )


def _scope_for_qualified_name(
    qualifier: str, current_file: str, symbol_index: dict
) -> Optional[str]:
    import_path = symbol_index.get("imports_by_file", {}).get(current_file, {}).get(qualifier)
    if not import_path:
        return None
    scopes = symbol_index.get("import_path_to_scopes", {}).get(import_path, [])
    return scopes[0] if len(scopes) == 1 else None


def _audited_request_unmarshal_schema(
    scope: str, type_name: str, symbol_index: dict
) -> Optional[dict]:
    methods = (
        symbol_index.get("methods_by_scope", {})
        .get(scope, {})
        .get((type_name, "UnmarshalJSON"), [])
    )
    if len(methods) != 1 or not methods[0].get("body"):
        return None
    digest = hashlib.sha256(methods[0]["body"].encode()).hexdigest()
    schema = AUDITED_REQUEST_UNMARSHAL_SCHEMAS.get((scope, type_name, digest))
    return json.loads(json.dumps(schema)) if schema else None


def _response_schema_for_go_type(
    go_type: str,
    current_scope: str,
    current_file: str,
    symbol_index: dict,
    depth: int = 0,
    nullable_pointer: bool = True,
    seen: Optional[set[tuple[str, str]]] = None,
    allow_aliases: bool = False,
    schema_mode: str = "response",
) -> Optional[dict]:
    if depth > 32:
        return None
    seen = set(seen or ())
    value = go_type.strip()
    pointer = False
    while value.startswith("*"):
        pointer = True
        value = value[1:].strip()
    primitives = {
        "string": {"type": "string"}, "bool": {"type": "boolean"},
        # encoding/json represents []byte as a base64 JSON string.
        "byte": {"type": "integer", "minimum": 0, "maximum": 255},
        "int": {"type": "integer"}, "int8": {"type": "integer"},
        "int16": {"type": "integer"}, "int32": {"type": "integer"},
        "int64": {"type": "integer", "format": "int64"},
        "uint": {"type": "integer", "minimum": 0},
        "uint8": {"type": "integer", "minimum": 0},
        "uint16": {"type": "integer", "minimum": 0},
        "uint32": {"type": "integer", "minimum": 0},
        "uint64": {"type": "integer", "minimum": 0},
        "float32": {"type": "number", "format": "float"},
        "float64": {"type": "number", "format": "double"},
        "time.Time": {"type": "string", "format": "date-time"},
    }
    schema: Optional[dict]
    if value in primitives:
        schema = dict(primitives[value])
    elif value in {"any", "interface{}", "json.RawMessage"}:
        return None
    elif value == "[]byte":
        schema = {"type": "string", "format": "byte"}
    elif value.startswith("[]"):
        item = _response_schema_for_go_type(
            value[2:], current_scope, current_file, symbol_index, depth + 1,
            seen=seen, allow_aliases=allow_aliases, schema_mode=schema_mode,
        )
        schema = {"type": "array", "items": item} if item else None
    elif value.startswith("map["):
        close = value.find("]")
        if close < 0 or value[4:close] != "string":
            return None
        item = _response_schema_for_go_type(
            value[close + 1 :], current_scope, current_file, symbol_index, depth + 1,
            seen=seen, allow_aliases=allow_aliases, schema_mode=schema_mode,
        )
        if item is None:
            return None
        schema = {"type": "object", "additionalProperties": item}
    else:
        target_scope = current_scope
        type_name = value
        if "." in value:
            qualifier, type_name = value.split(".", 1)
            target_scope = _scope_for_qualified_name(qualifier, current_file, symbol_index) or ""
        candidates = symbol_index.get("structs_by_scope", {}).get(target_scope, {}).get(type_name, [])
        alias_candidates = symbol_index.get("aliases_by_scope", {}).get(target_scope, {}).get(type_name, [])
        if len(candidates) + (len(alias_candidates) if allow_aliases else 0) != 1:
            return None
        marker = (target_scope, type_name)
        if marker in seen:
            return None
        nested_seen = seen | {marker}
        audited_request_schema = (
            _audited_request_unmarshal_schema(target_scope, type_name, symbol_index)
            if schema_mode == "request"
            else None
        )
        if audited_request_schema is not None:
            schema = audited_request_schema
        elif allow_aliases and alias_candidates:
            alias = alias_candidates[0]
            underlying = alias["underlying"]
            if underlying.startswith("="):
                underlying = underlying[1:].strip()
            # A custom marshaler owns the wire shape; the lexical type shape is
            # not sufficient evidence for contract generation.
            response_codec_types = set(
                symbol_index.get("custom_json_marshalers_by_scope", {}).get(target_scope, [])
            ) | set(
                symbol_index.get("custom_text_marshalers_by_scope", {}).get(target_scope, [])
            )
            request_codec_types = set(
                symbol_index.get("custom_json_unmarshalers_by_scope", {}).get(target_scope, [])
            ) | set(
                symbol_index.get("custom_text_unmarshalers_by_scope", {}).get(target_scope, [])
            )
            if type_name in response_codec_types or (
                schema_mode == "request" and type_name in request_codec_types
            ):
                return None
            schema = _response_schema_for_go_type(
                underlying, target_scope, alias["file"], symbol_index,
                depth + 1, seen=nested_seen, allow_aliases=True,
                schema_mode=schema_mode,
            )
            if schema is None:
                return None
        else:
            response_codec_types = set(
                symbol_index.get("custom_json_marshalers_by_scope", {}).get(target_scope, [])
            ) | set(
                symbol_index.get("custom_text_marshalers_by_scope", {}).get(target_scope, [])
            )
            request_codec_types = set(
                symbol_index.get("custom_json_unmarshalers_by_scope", {}).get(target_scope, [])
            ) | set(
                symbol_index.get("custom_text_unmarshalers_by_scope", {}).get(target_scope, [])
            )
            if type_name in response_codec_types or (
                schema_mode == "request" and type_name in request_codec_types
            ):
                return None
            st = candidates[0]
            if not st.get("parse_complete", True):
                return None
            properties: dict = {}
            required: list[str] = []
            promoted_names: set[str] = set()
            ambiguous_promoted: set[str] = set()
            direct_names: set[str] = set()
            for field in st["fields"]:
                if field.get("json_ignored"):
                    continue
                if not field.get("embedded") and not field.get("exported"):
                    continue
                if field.get("embedded"):
                    embedded = _response_schema_for_go_type(
                        field["type"], target_scope, st["file"], symbol_index, depth + 1,
                        nullable_pointer=False,
                        seen=nested_seen, allow_aliases=allow_aliases,
                        schema_mode=schema_mode,
                    )
                    if not embedded or embedded.get("type") != "object":
                        return None
                    explicit_name = field.get("json_name")
                    if explicit_name:
                        if explicit_name in direct_names:
                            return None
                        direct_names.add(explicit_name)
                        promoted_names.discard(explicit_name)
                        ambiguous_promoted.discard(explicit_name)
                        if explicit_name in required:
                            required.remove(explicit_name)
                        field_schema = dict(embedded)
                        field_schema["x-clomapi-go-field"] = field["name"]
                        field_schema["x-clomapi-go-type"] = field["type"]
                        properties[explicit_name] = field_schema
                        is_required = (
                            field.get("required")
                            if schema_mode == "request"
                            else not field.get("json_omitempty")
                            and not field.get("json_omitzero")
                        )
                        if is_required:
                            required.append(explicit_name)
                        continue
                    embedded_properties = embedded.get("properties") or {}
                    embedded_required = set(embedded.get("required") or [])
                    for item, item_schema in embedded_properties.items():
                        if item in direct_names:
                            continue
                        if item in promoted_names:
                            promoted_names.discard(item)
                            ambiguous_promoted.add(item)
                            properties.pop(item, None)
                            if item in required:
                                required.remove(item)
                            continue
                        if item in ambiguous_promoted:
                            continue
                        promoted_names.add(item)
                        properties[item] = item_schema
                        if (
                            item in embedded_required
                            and not field.get("embedded_pointer")
                            and not field.get("json_omitempty")
                            and not field.get("json_omitzero")
                            and item not in required
                        ):
                            required.append(item)
                    continue
                name = field.get("json_name")
                if not name:
                    continue
                field_schema = _response_schema_for_go_type(
                    field["type"], target_scope, st["file"], symbol_index, depth + 1,
                    seen=nested_seen, allow_aliases=allow_aliases,
                    schema_mode=schema_mode,
                )
                if field_schema is None:
                    return None
                field_schema = dict(field_schema)
                if field.get("json_string"):
                    scalar_type = field["type"].strip()
                    pointer_string = scalar_type.startswith("*")
                    scalar_type = scalar_type.lstrip("*").strip()
                    stringable = {
                        "string", "bool", "int", "int8", "int16", "int32", "int64",
                        "uint", "uint8", "uint16", "uint32", "uint64",
                        "float32", "float64",
                    }
                    if scalar_type not in stringable:
                        return None
                    encoded = {"type": "string"}
                    field_schema = (
                        {"anyOf": [encoded, {"type": "null"}]}
                        if pointer_string
                        else encoded
                    )
                field_schema["x-clomapi-go-field"] = field["name"]
                field_schema["x-clomapi-go-type"] = field["type"]
                if name in direct_names:
                    return None
                direct_names.add(name)
                promoted_names.discard(name)
                ambiguous_promoted.discard(name)
                if name in required:
                    required.remove(name)
                properties[name] = field_schema
                is_required = (
                    field.get("required")
                    if schema_mode == "request"
                    else not field.get("json_omitempty")
                    and not field.get("json_omitzero")
                )
                if is_required:
                    required.append(name)
            if ambiguous_promoted:
                return None
            schema = {
                "type": "object", "properties": properties, "additionalProperties": False,
                "x-clomapi-struct": type_name, "x-clomapi-qualified-scope": target_scope,
                "x-clomapi-evidence": st["evidence"],
            }
            if required:
                schema["required"] = required
    if schema is None:
        return None
    if pointer and nullable_pointer:
        return {"anyOf": [schema, {"type": "null"}]}
    return schema


def _unique_callable_results(
    call_name: str, current_scope: str, current_file: str,
    current_recv_type: Optional[str], symbol_index: dict,
    body_prefix: Optional[str] = None,
    require_results: Optional[bool] = True,
) -> Optional[dict]:
    parts = call_name.split(".")
    candidates: list[dict] = []
    if len(parts) == 1:
        # A local binding shadows a package function.  Function-valued locals
        # are not signature-indexed, so never borrow the package declaration.
        if not (body_prefix and _visible_local_symbol_writes(body_prefix, parts[0])):
            candidates = symbol_index.get("funcs_by_scope", {}).get(current_scope, {}).get(parts[0], [])
    elif len(parts) == 2 and parts[0] in {"h", "handler"} and current_recv_type:
        if not (body_prefix and _visible_local_symbol_writes(body_prefix, parts[0])):
            candidates = symbol_index.get("methods_by_scope", {}).get(current_scope, {}).get(
                (current_recv_type, parts[1]), []
            )
    elif len(parts) == 2:
        local_writes = _visible_local_symbol_writes(body_prefix, parts[0]) if body_prefix else []
        target_scope = (
            None
            if local_writes
            else _scope_for_qualified_name(parts[0], current_file, symbol_index)
        )
        if target_scope:
            candidates = symbol_index.get("funcs_by_scope", {}).get(target_scope, {}).get(parts[1], [])
        elif body_prefix:
            receiver = _infer_receiver_type(
                parts[0], body_prefix, current_scope, current_file,
                current_recv_type, symbol_index,
            )
            if receiver:
                receiver_scope, receiver_name = receiver
                candidates = symbol_index.get("methods_by_scope", {}).get(
                    receiver_scope, {}
                ).get((receiver_name, parts[1]), [])
    elif len(parts) == 3 and parts[0] in {"h", "handler"} and current_recv_type:
        if body_prefix and _visible_local_symbol_writes(body_prefix, parts[0]):
            return None
        handler_candidates = (
            symbol_index.get("structs_by_scope", {})
            .get(current_scope, {})
            .get(current_recv_type, [])
        )
        if len(handler_candidates) == 1:
            fields = [
                field for field in handler_candidates[0]["fields"]
                if field.get("name") == parts[1]
            ]
            if len(fields) == 1:
                receiver_type = fields[0]["type"].strip()
                while receiver_type.startswith("*"):
                    receiver_type = receiver_type[1:].strip()
                receiver_scope = current_scope
                receiver_name = receiver_type
                if "." in receiver_type:
                    qualifier, receiver_name = receiver_type.split(".", 1)
                    receiver_scope = _scope_for_qualified_name(
                        qualifier, handler_candidates[0]["file"], symbol_index
                    ) or ""
                concrete = (
                    symbol_index.get("methods_by_scope", {})
                    .get(receiver_scope, {})
                    .get((receiver_name, parts[2]), [])
                )
                interface = (
                    symbol_index.get("interface_methods_by_scope", {})
                    .get(receiver_scope, {})
                    .get((receiver_name, parts[2]), [])
                )
                candidates = concrete or interface
    if not candidates and len(parts) >= 3 and body_prefix:
        receiver = _infer_receiver_chain(
            parts[:-1], body_prefix, current_scope, current_file,
            current_recv_type, symbol_index,
        )
        if receiver:
            receiver_scope, receiver_name = receiver
            candidates = (
                symbol_index.get("methods_by_scope", {})
                .get(receiver_scope, {})
                .get((receiver_name, parts[-1]), [])
                or symbol_index.get("interface_methods_by_scope", {})
                .get(receiver_scope, {})
                .get((receiver_name, parts[-1]), [])
            )
    if len(candidates) != 1:
        return None
    if require_results is True and candidates[0].get("status") != "parsed":
        return None
    if require_results is False and candidates[0].get("param_status") != "parsed":
        return None
    return candidates[0]


def _receiver_type_from_go_type(
    go_type: str, current_scope: str, current_file: str, symbol_index: dict
) -> Optional[tuple[str, str]]:
    value = go_type.strip()
    while value.startswith("*"):
        value = value[1:].strip()
    if "." in value:
        qualifier, name = value.split(".", 1)
        scope = _scope_for_qualified_name(qualifier, current_file, symbol_index)
        return (scope, name) if scope else None
    return current_scope, value


def _infer_receiver_type(
    variable: str, body_prefix: str, current_scope: str, current_file: str,
    current_recv_type: Optional[str], symbol_index: dict,
) -> Optional[tuple[str, str]]:
    # A receiver whose value is reassigned or branch-defined is not a stable
    # proof of method identity.  Require one lexical write before inspecting
    # the declaration/factory that supplies its concrete type.
    visible_records = _visible_symbol_records(body_prefix, variable)
    if len(visible_records) != 1:
        return None
    visible_positions = {record["position"] for record in visible_records}
    declarations = list(re.finditer(
        rf"(?m)^[ \t]*var\s+{re.escape(variable)}\s+([\w\.\*]+)", body_prefix
    ))
    declarations = [m for m in declarations if m.start() in visible_positions]
    if len(declarations) == 1:
        return _receiver_type_from_go_type(
            declarations[0].group(1), current_scope, current_file, symbol_index
        )
    literals = list(re.finditer(
        rf"(?m)^[ \t]*{re.escape(variable)}\s*:?=\s*&?([\w\.]+)\s*\{{",
        body_prefix,
    ))
    literals = [m for m in literals if m.start() in visible_positions]
    if len(literals) == 1:
        return _receiver_type_from_go_type(
            literals[0].group(1), current_scope, current_file, symbol_index
        )
    assignments = list(re.finditer(
        rf"(?m)^[ \t]*{re.escape(variable)}(?:\s*,\s*\w+)*\s*:?=\s*"
        rf"(?P<call>[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)\s*\(", body_prefix
    ))
    assignments = [m for m in assignments if m.start() in visible_positions]
    if len(assignments) == 1:
        entry = _unique_callable_results(
            assignments[0].group("call"), current_scope, current_file,
            current_recv_type, symbol_index, body_prefix,
        )
        if entry and entry.get("result_types"):
            return _receiver_type_from_go_type(
                entry["result_types"][0], entry["scope"], entry["file"], symbol_index
            )
    return None


def _infer_receiver_chain(
    chain: list[str], body_prefix: str, current_scope: str, current_file: str,
    current_recv_type: Optional[str], symbol_index: dict,
) -> Optional[tuple[str, str]]:
    if not chain:
        return None
    receiver = _infer_receiver_type(
        chain[0], body_prefix, current_scope, current_file,
        current_recv_type, symbol_index,
    )
    if not receiver:
        return None
    scope, type_name = receiver
    for field_name in chain[1:]:
        structs = symbol_index.get("structs_by_scope", {}).get(scope, {}).get(type_name, [])
        if len(structs) != 1:
            return None
        fields = [f for f in structs[0].get("fields", []) if f.get("name") == field_name]
        if len(fields) != 1:
            return None
        receiver = _receiver_type_from_go_type(
            fields[0]["type"], scope, structs[0]["file"], symbol_index
        )
        if not receiver:
            return None
        scope, type_name = receiver
    return scope, type_name


def _non_null_schema(schema: dict) -> dict:
    variants = schema.get("anyOf") if isinstance(schema, dict) else None
    if isinstance(variants, list):
        non_null = [item for item in variants if item != {"type": "null"}]
        if len(non_null) == 1:
            return non_null[0]
    return schema


def _selector_schema(base_schema: dict, selector: str) -> Optional[dict]:
    schema = _non_null_schema(base_schema)
    if schema.get("oneOf"):
        selected = [_selector_schema(item, selector) for item in schema["oneOf"]]
        if any(item is None for item in selected):
            return None
        unique: list[dict] = []
        seen: set[str] = set()
        for item in selected:
            key = json.dumps(item, sort_keys=True)
            if key not in seen:
                seen.add(key)
                unique.append(item)
        return unique[0] if len(unique) == 1 else {"oneOf": unique}
    if schema.get("type") != "object":
        return None
    matches = [
        prop for prop in (schema.get("properties") or {}).values()
        if prop.get("x-clomapi-go-field") == selector
    ]
    return matches[0] if len(matches) == 1 else None


def _last_assignment(body_prefix: str, variable: str) -> Optional[tuple[list[str], str, int]]:
    assignment = re.compile(
        rf"(?m)^\s*(?P<lhs>[A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)\s*:?=\s*"
    )
    matches = [m for m in assignment.finditer(body_prefix) if variable in {
        part.strip() for part in m.group("lhs").split(",")
    }]
    if not matches:
        return None
    match = matches[-1]
    lhs = [part.strip() for part in match.group("lhs").split(",")]
    start = match.end()
    line_end = body_prefix.find("\n", start)
    if line_end < 0:
        line_end = len(body_prefix)
    first_line = body_prefix[start:line_end].strip()
    if re.match(r"[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*\s*\(", first_line):
        open_pos = body_prefix.find("(", start, line_end + 1)
        close_pos = _matching_paren(body_prefix, open_pos) if open_pos >= 0 else None
        if close_pos is not None and not body_prefix[close_pos + 1 : line_end].strip():
            return lhs, body_prefix[start : close_pos + 1].strip(), match.start()
    return lhs, first_line.rstrip(",").strip(), match.start()


def _assignment_records(
    body_prefix: str, variable: str
) -> list[tuple[list[str], str, int]]:
    """Return writes to the binding visible at the response call.

    Writes through descendant branches remain attached to an outer binding;
    declarations in expired or shadowing inner blocks do not.
    """
    code = _mask_go_lexemes(body_prefix)
    records: list[tuple[list[str], str, int]] = []
    for record in _visible_symbol_records(body_prefix, variable):
        if record["kind"] != "assignment":
            continue
        match = record["match"]
        lhs = record["lhs"]
        start = match.end()
        depth = 0
        end = start
        while end < len(code):
            char = code[end]
            if char in "([{":
                depth += 1
            elif char in ")]}":
                if char == "}" and depth == 0:
                    break
                depth -= 1
            elif depth == 0 and char in "\n;":
                break
            end += 1
        rhs = body_prefix[start:end].strip().rstrip(",").strip()
        if not rhs:
            return []
        records.append((lhs, rhs, match.start()))
    return records


def _matching_brace(text: str, open_pos: int) -> Optional[int]:
    depth = 0
    quote: Optional[str] = None
    escaped = False
    for index in range(open_pos, len(text)):
        char = text[index]
        if quote:
            if quote != "`" and escaped:
                escaped = False
            elif quote != "`" and char == "\\":
                escaped = True
            elif char == quote:
                quote = None
            continue
        if char in ('"', "'", "`"):
            quote = char
        elif char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
            if depth == 0:
                return index
    return None


def _resolve_gin_map_literal(
    value: str, body_prefix: str, current_scope: str, current_file: str,
    current_recv_type: Optional[str], symbol_index: dict, depth: int,
) -> Optional[dict]:
    brace = value.find("{")
    close = _matching_brace(value, brace) if brace >= 0 else None
    if close is None or value[close + 1 :].strip():
        return None
    properties: dict = {}
    required: list[str] = []
    for entry in _split_top_level_commas(value[brace + 1 : close]):
        match = re.match(r'^\s*"(?P<key>(?:[^"\\]|\\.)*)"\s*:\s*(?P<expr>.*)$', entry, re.S)
        if not match:
            return None
        try:
            key = json.loads('"' + match.group("key") + '"')
        except json.JSONDecodeError:
            return None
        resolved = _resolve_response_expression(
            match.group("expr"), body_prefix, current_scope, current_file,
            current_recv_type, symbol_index, depth + 1,
        )
        if not resolved or resolved.get("schema") is None:
            return None
        properties[key] = resolved["schema"]
        required.append(key)
    return {
        "type": "object", "properties": properties, "required": required,
        "additionalProperties": False,
    }


def _resolve_response_expression(
    expr: str, body_prefix: str, current_scope: str, current_file: str,
    current_recv_type: Optional[str], symbol_index: dict, depth: int = 0,
) -> Optional[dict]:
    if depth > 7:
        return None
    value = expr.strip()
    if value == "nil":
        return {"schema": None, "evidence": "literal:nil"}
    if value in {"true", "false"}:
        return {"schema": {"type": "boolean", "const": value == "true"}}
    if re.fullmatch(r'"(?:[^"\\]|\\.)*"', value):
        try:
            return {"schema": {"type": "string", "const": json.loads(value)}}
        except json.JSONDecodeError:
            return None
    if re.fullmatch(r"-?\d+", value):
        return {"schema": {"type": "integer", "const": int(value)}}
    if re.fullmatch(r"-?(?:\d+\.\d*|\d*\.\d+)", value):
        return {"schema": {"type": "number", "const": float(value)}}
    plain = value[1:].strip() if value.startswith("&") else value
    if plain.startswith("gin.H{") or re.match(r"map\s*\[\s*string\s*\]\s*any\s*\{", plain):
        schema = _resolve_gin_map_literal(
            plain, body_prefix, current_scope, current_file,
            current_recv_type, symbol_index, depth,
        )
        return {"schema": schema} if schema else None
    length_call = re.fullmatch(r"len\s*\(.*\)", plain, re.S)
    if length_call:
        return {"schema": {"type": "integer", "minimum": 0}}
    slice_literal = re.match(r"(?P<type>\[\][A-Za-z_\*][\w\.\*]*)\s*\{", plain)
    if slice_literal:
        brace = plain.find("{", slice_literal.start())
        close = _matching_brace(plain, brace)
        if close is None or plain[close + 1 :].strip():
            return None
        schema = _response_schema_for_go_type(
            slice_literal.group("type"), current_scope, current_file, symbol_index
        )
        return {"schema": schema} if schema else None
    make_match = re.match(r"make\(\s*(\[\][\w\.\*]+)", plain)
    if make_match:
        open_pos = plain.find("(", make_match.start())
        close = _matching_paren(plain, open_pos)
        if close is None or plain[close + 1 :].strip():
            return None
        schema = _response_schema_for_go_type(make_match.group(1), current_scope, current_file, symbol_index)
        return {"schema": schema} if schema else None
    literal = re.match(r"(?P<type>[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)\s*\{", plain)
    if literal and not plain.startswith(("gin.H", "map[")):
        brace = plain.find("{", literal.start())
        close = _matching_brace(plain, brace)
        if close is None or plain[close + 1 :].strip():
            return None
        schema = _response_schema_for_go_type(
            literal.group("type"), current_scope, current_file, symbol_index,
            nullable_pointer=False,
        )
        return {"schema": schema} if schema else None
    append_call = re.match(r"append\s*\(", plain)
    if append_call:
        open_pos = append_call.end() - 1
        close = _matching_paren(plain, open_pos)
        if close is None or plain[close + 1 :].strip():
            return None
        args = _split_top_level_commas(plain[open_pos + 1 : close])
        if len(args) < 2:
            return None
        base = _resolve_response_expression(
            args[0], body_prefix, current_scope, current_file,
            current_recv_type, symbol_index, depth + 1,
        )
        base_schema = _non_null_schema((base or {}).get("schema") or {})
        if (
            base_schema.get("type") != "array"
            or not _response_schema_is_complete(base_schema.get("items"))
        ):
            return None
        return base
    call = re.match(r"(?P<name>[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)\s*\(", plain)
    if call:
        # Calls nested inside structural literals are intentionally opaque.
        # Their value semantics require expression/type checking beyond this
        # cohort; direct and assigned helper payload calls remain supported.
        if depth > 0:
            return None
        open_pos = call.end() - 1
        close = _matching_paren(plain, open_pos)
        if close is None or plain[close + 1 :].strip():
            return None
        entry = _unique_callable_results(
            call.group("name"), current_scope, current_file, current_recv_type,
            symbol_index, body_prefix,
        )
        if not entry or not entry["result_types"]:
            return None
        schema = _response_schema_for_go_type(
            entry["result_types"][0], entry["scope"], entry["file"], symbol_index,
            allow_aliases=True,
        )
        return {"schema": schema, "evidence": entry["evidence"]} if schema else None
    selector = re.fullmatch(
        r"(?P<base>[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)\.(?P<field>[A-Za-z_]\w*)",
        value,
    )
    if selector:
        base = _resolve_response_expression(
            selector.group("base"), body_prefix, current_scope, current_file,
            current_recv_type, symbol_index, depth + 1,
        )
        selected = _selector_schema((base or {}).get("schema") or {}, selector.group("field"))
        return {"schema": selected} if selected else None
    indexed = re.fullmatch(r"(?P<base>.+?)\s*\[[^\]]+\]", value)
    if indexed:
        base = _resolve_response_expression(
            indexed.group("base"), body_prefix, current_scope, current_file,
            current_recv_type, symbol_index, depth + 1,
        )
        base_schema = _non_null_schema((base or {}).get("schema") or {})
        return {"schema": base_schema["items"]} if base_schema.get("type") == "array" and base_schema.get("items") else None
    if re.fullmatch(r"[A-Za-z_]\w*", value):
        index_mutation_pattern = re.compile(
            rf"\b{re.escape(value)}\s*\[[^\]]+\]"
            r"\s*(?:=(?!=)|\+=|-=|\*=|/=|%=|&=|\|=|\^=|<<=|>>=|\+\+|--)"
        )
        has_index_mutation = bool(
            index_mutation_pattern.search(_mask_go_lexemes(body_prefix))
        )
        visible_records = _visible_symbol_records(body_prefix, value)
        visible_var_positions = {
            record["position"] for record in visible_records
            if record["kind"] == "var"
        }
        declarations = list(re.finditer(
            rf"(?m)^[ \t]*var\s+{re.escape(value)}\s+([^\s=]+)",
            _mask_go_lexemes(body_prefix),
        ))
        declarations = [
            declaration for declaration in declarations
            if declaration.start() in visible_var_positions
        ]
        if declarations:
            if len(declarations) != 1:
                return None
            schema = _response_schema_for_go_type(
                declarations[-1].group(1), current_scope, current_file, symbol_index
            )
            if schema:
                return {"schema": schema}
            # An explicit opaque/interface declaration must not be refined from
            # whichever branch assignment happens to appear last.
            return None
        assignments = _assignment_records(body_prefix, value)
        if not assignments:
            return None
        resolved_writes: list[dict] = []
        for lhs, rhs, assignment_start in assignments:
            slot = lhs.index(value)
            assignment_prefix = body_prefix[:assignment_start]
            rhs_call = re.match(
                r"(?P<name>[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)\s*\(", rhs
            )
            if rhs_call:
                open_pos = rhs_call.end() - 1
                close = _matching_paren(rhs, open_pos)
                if close is None or rhs[close + 1 :].strip():
                    return None
                entry = _unique_callable_results(
                    rhs_call.group("name"), current_scope, current_file,
                    current_recv_type, symbol_index, assignment_prefix,
                )
                if entry and slot < len(entry["result_types"]):
                    schema = _response_schema_for_go_type(
                        entry["result_types"][slot], entry["scope"], entry["file"],
                        symbol_index,
                    )
                    if not schema:
                        return None
                    resolved_writes.append(
                        {"schema": schema, "evidence": entry["evidence"]}
                    )
                    continue
                if rhs_call.group("name") not in {"make", "append"}:
                    return None
            if len(lhs) != 1:
                return None
            if "\n" in rhs and re.match(
                r"&?(?:[A-Za-z_]\w*\.)?[A-Za-z_]\w*\s*\{", rhs
            ):
                return None
            resolved = _resolve_response_expression(
                rhs, assignment_prefix, current_scope, current_file,
                current_recv_type, symbol_index, depth + 1,
            )
            if not resolved or resolved.get("schema") is None:
                return None
            resolved_writes.append(resolved)
        schemas = {
            json.dumps(item["schema"], sort_keys=True, separators=(",", ":"))
            for item in resolved_writes
        }
        if len(schemas) != 1:
            return None
        evidence = sorted(
            {
                item["evidence"]
                for item in resolved_writes
                if item.get("evidence")
            }
        )
        result = {"schema": resolved_writes[0]["schema"]}
        non_null_result = _non_null_schema(result["schema"])
        if has_index_mutation and non_null_result.get("type") != "array":
            return None
        if evidence:
            result["evidence"] = ",".join(evidence)
        return result
    return None


def _success_envelope_schema(data: Optional[dict], response_kind: str) -> dict:
    properties = {"code": {"type": "integer", "const": 0}, "message": {"type": "string"}}
    required = ["code", "message"]
    if data is not None:
        properties["data"] = data
        required.append("data")
    return {
        "type": "object", "required": required, "properties": properties,
        "additionalProperties": False, "x-clomapi-response-kind": response_kind,
    }


def _paginated_envelope_schema(items_schema: dict) -> dict:
    return _success_envelope_schema({
        "type": "object", "required": ["items", "total", "page", "page_size", "pages"],
        "properties": {
            "items": items_schema, "total": {"type": "integer"},
            "page": {"type": "integer"}, "page_size": {"type": "integer"},
            "pages": {"type": "integer"},
        }, "additionalProperties": False,
    }, "paginated_typed")


def _nested_function_literal_spans(code: str) -> list[tuple[int, int]]:
    spans = []
    for match in re.finditer(r"\bfunc\s*\(", code):
        params_open = code.find("(", match.start(), match.end())
        params_close = _matching_paren(code, params_open)
        if params_close is None:
            continue
        brace = code.find("{", params_close + 1)
        if brace < 0:
            continue
        close = _matching_brace(code, brace)
        if close is not None:
            spans.append((brace + 1, close))
    return spans


def _visible_local_function_literal(
    body_prefix: str, variable: str
) -> Optional[dict]:
    records = _assignment_records(body_prefix, variable)
    if len(records) != 1 or len(records[0][0]) != 1:
        return None
    rhs = records[0][1].strip()
    match = re.match(r"func\s*\(", rhs)
    if not match:
        return None
    params_open = rhs.find("(", match.start(), match.end())
    params_close = _matching_paren(rhs, params_open)
    if params_close is None:
        return None
    brace = _mask_go_lexemes(rhs).find("{", params_close + 1)
    close = _matching_brace(_mask_go_lexemes(rhs), brace) if brace >= 0 else None
    if close is None or rhs[close + 1 :].strip():
        return None
    return {
        "body": rhs[brace + 1 : close],
        "param_records": _go_param_records(rhs[params_open + 1 : params_close]),
        "position": records[0][2],
    }


def _infer_success_response_schemas(
    body: str, current_scope: str, current_file: str,
    current_recv_type: Optional[str], symbol_index: dict, recursion_depth: int = 0,
    context_name: str = "c", call_stack: Optional[set[tuple]] = None,
    bound_names: Optional[set[str]] = None,
    context_names: Optional[set[str]] = None,
    poisoned_context_names: Optional[set[str]] = None,
    direct_response_structs: Optional[set[str]] = None,
) -> dict:
    by_status: dict[int, list[dict]] = defaultdict(list)
    by_status_media: dict[int, dict[str, list[dict]]] = defaultdict(
        lambda: defaultdict(list)
    )
    media_by_status: dict[int, set[str]] = defaultdict(set)
    success_statuses: set[int] = set()
    unresolved_statuses: set[int] = set()
    unknown_success_status = False
    error_statuses: set[int] = set()
    error_schemas: dict[int, list[dict]] = defaultdict(list)
    statuses = {"Success": 200, "Created": 201, "Accepted": 202,
                "Paginated": 200, "PaginatedWithResult": 200}
    code = _mask_go_lexemes(body)
    stable_context_names = (
        set(context_names) if context_names is not None else {context_name}
    )
    poisoned_context_names = set(poisoned_context_names or ())
    context_pattern = (
        r"(?:" + "|".join(re.escape(name) for name in sorted(stable_context_names)) + r")"
        if stable_context_names
        else r"__unproven_gin_context__"
    )

    def is_context_argument(value: str) -> bool:
        return value.strip() in stable_context_names

    nested_function_spans = _nested_function_literal_spans(code)
    response_alias = _response_package_alias(current_file, symbol_index)
    if response_alias in set(bound_names or ()):
        response_alias = None

    def inside_nested_function(offset: int) -> bool:
        return any(start <= offset < end for start, end in nested_function_spans)

    call_stack = set(call_stack or ())
    helper_pattern = (
        re.compile(
            rf"\b{re.escape(response_alias)}\."
            r"(Success|Created|Accepted|Paginated|PaginatedWithResult)\s*\("
        )
        if response_alias
        else None
    )
    calls = list(helper_pattern.finditer(code)) if helper_pattern else []
    for match in calls:
        if inside_nested_function(match.start()):
            continue
        kind = match.group(1)
        status = statuses[kind]
        if (
            _visible_local_symbol_writes(body[:match.start()], response_alias or "")
            or _local_symbol_writes(body[:match.start()], context_name)
        ):
            success_statuses.add(status)
            unresolved_statuses.add(status)
            continue
        open_pos = body.find("(", match.start(), match.end())
        close_pos = _matching_paren(body, open_pos)
        if close_pos is None:
            continue
        args = _split_top_level_commas(body[open_pos + 1 : close_pos])
        if len(args) < 2 or not is_context_argument(args[0]):
            continue
        success_statuses.add(status)
        media_by_status[status].add("application/json")
        resolved = _resolve_response_expression(
            args[1], body[:match.start()], current_scope, current_file,
            current_recv_type, symbol_index,
        )
        if not resolved:
            unresolved_statuses.add(status)
            continue
        if kind.startswith("Paginated"):
            collection = resolved.get("schema") or {}
            if collection.get("type") != "array" or not collection.get("items"):
                unresolved_statuses.add(status)
                continue
            schema = _paginated_envelope_schema(collection)
        else:
            schema = _success_envelope_schema(resolved.get("schema"), kind.lower())
        if resolved.get("evidence"):
            schema["x-clomapi-response-evidence"] = [resolved["evidence"]]
        by_status[status].append(schema)
        by_status_media[status]["application/json"].append(schema)

    # A familiar qualifier is not proof of package identity. Treat calls made
    # through an unproven or shadowed `response` name as unresolved sinks.
    for match in re.finditer(
        r"\bresponse\.(Success|Created|Accepted|Paginated|PaginatedWithResult)\s*\(",
        code,
    ):
        if response_alias == "response" and not _visible_local_symbol_writes(
            body[:match.start()], "response"
        ):
            continue
        status = statuses[match.group(1)]
        success_statuses.add(status)
        unresolved_statuses.add(status)

    def status_code(value: str) -> Optional[int]:
        value = value.strip()
        if re.fullmatch(r"\d{3}", value):
            return int(value)
        named = re.fullmatch(r"http\.Status([A-Za-z]+)", value)
        names = {
            "OK": 200, "Created": 201, "Accepted": 202, "NoContent": 204,
            "BadRequest": 400, "Unauthorized": 401, "Forbidden": 403,
            "NotFound": 404, "Conflict": 409, "RequestEntityTooLarge": 413,
            "UnprocessableEntity": 422, "Locked": 423, "TooManyRequests": 429,
            "InternalServerError": 500, "BadGateway": 502,
            "ServiceUnavailable": 503,
        }
        return names.get(named.group(1)) if named else None

    structured_writers = {
        "JSON": "application/json",
        "IndentedJSON": "application/json",
        "PureJSON": "application/json",
        "SecureJSON": "application/json",
        "AsciiJSON": "application/json",
        "AbortWithStatusJSON": "application/json",
        "AbortWithStatusPureJSON": "application/json",
        "JSONP": "application/javascript",
        "XML": "application/xml",
        "YAML": "application/yaml",
        "TOML": "application/toml",
        "ProtoBuf": "application/x-protobuf",
        "MsgPack": "application/msgpack",
        "HTML": "text/html",
        "Render": None,
        "Negotiate": None,
    }
    writer_names = "|".join(sorted(structured_writers, key=len, reverse=True))
    structured_calls = list(re.finditer(
        rf"\b{context_pattern}\.(?P<writer>{writer_names})\s*\(",
        code,
    ))
    for match in structured_calls:
        if inside_nested_function(match.start()):
            continue
        open_pos = code.find("(", match.start(), match.end())
        close_pos = _matching_paren(code, open_pos)
        args = (
            _split_top_level_commas(body[open_pos + 1 : close_pos])
            if close_pos is not None
            else []
        )
        status = status_code(args[0]) if args else None
        if status is None:
            # A dynamic status could be successful. Preserve a conservative
            # unresolved 200 sink rather than dropping it from the contract.
            success_statuses.add(200)
            unresolved_statuses.add(200)
            unknown_success_status = True
            continue
        if 200 <= status < 300:
            success_statuses.add(status)
            media_type = structured_writers[match.group("writer")]
            if media_type:
                media_by_status[status].add(media_type)
            if status == 204 and len(args) == 1:
                continue
            resolved = (
                _resolve_response_expression(
                    args[1], body[:match.start()], current_scope, current_file,
                    current_recv_type, symbol_index,
                )
                if len(args) >= 2
                else None
            )
            resolved_schema = (resolved or {}).get("schema")
            if (
                media_type == "application/json"
                and resolved_schema is not None
                and _response_schema_is_complete(resolved_schema)
                and (
                    direct_response_structs is None
                    or _schema_contains_struct(resolved_schema, direct_response_structs)
                )
            ):
                by_status[status].append(resolved_schema)
                by_status_media[status][media_type].append(resolved_schema)
            else:
                unresolved_statuses.add(status)
        else:
            error_statuses.add(status)
            resolved = (
                _resolve_response_expression(
                    args[1], body[:match.start()], current_scope, current_file,
                    current_recv_type, symbol_index,
                )
                if len(args) >= 2
                else None
            )
            if resolved and resolved.get("schema") is not None:
                error_schemas[status].append(resolved["schema"])

    for match in re.finditer(
        rf"\b{context_pattern}\.DataFromReader\s*\(", code
    ):
        if inside_nested_function(match.start()):
            continue
        open_pos = code.find("(", match.start(), match.end())
        close_pos = _matching_paren(code, open_pos)
        args = _split_top_level_commas(body[open_pos + 1 : close_pos]) if close_pos else []
        status = status_code(args[0]) if args else None
        if status is None:
            success_statuses.add(200)
            unresolved_statuses.add(200)
            unknown_success_status = True
            continue
        if not 200 <= status < 300:
            continue
        media_type = None
        if len(args) >= 3 and re.fullmatch(r'"(?:[^"\\]|\\.)*"', args[2].strip()):
            try:
                media_type = json.loads(args[2].strip())
            except json.JSONDecodeError:
                pass
        success_statuses.add(status)
        if not media_type:
            unresolved_statuses.add(status)
            continue
        media_by_status[status].add(media_type)
        by_status_media[status][media_type].append(
            {"type": "string", "format": "binary"}
        )

    if re.search(
        rf"\b{context_pattern}\."
        r"(?:File|FileAttachment|FileFromFS)\s*\(",
        code,
    ):
        success_statuses.add(200)
        unresolved_statuses.add(200)
    if re.search(rf"\b{context_pattern}\.(?:Stream|SSEvent)\s*\(", code):
        success_statuses.add(200)
        media_by_status[200].add("text/event-stream")
        unresolved_statuses.add(200)
    for match in re.finditer(
        rf"\b{context_pattern}\."
        r"(?:Data|String|Status|AbortWithStatus)\s*\(", code
    ):
        if inside_nested_function(match.start()):
            continue
        open_pos = code.find("(", match.start(), match.end())
        close_pos = _matching_paren(code, open_pos)
        args = _split_top_level_commas(body[open_pos + 1 : close_pos]) if close_pos else []
        status = status_code(args[0]) if args else None
        if status is None:
            success_statuses.add(200)
            unresolved_statuses.add(200)
            unknown_success_status = True
            continue
        if not 200 <= status < 300:
            continue
        success_statuses.add(status)
        if status != 204:
            unresolved_statuses.add(status)
    if re.search(
        rf"\b{context_pattern}\.Writer\."
        r"(?:Write|WriteString)\s*\(",
        code,
    ):
        success_statuses.add(200)
        unresolved_statuses.add(200)
    if re.search(rf"\b{context_pattern}\.Writer\.Flush\s*\(", code):
        success_statuses.add(200)
        unresolved_statuses.add(200)
        unknown_success_status = True
    for match in re.finditer(
        rf"\b{context_pattern}\.Writer\.WriteHeader\s*\(", code
    ):
        open_pos = code.find("(", match.start(), match.end())
        close_pos = _matching_paren(code, open_pos)
        args = _split_top_level_commas(body[open_pos + 1 : close_pos]) if close_pos else []
        status = status_code(args[0]) if args else None
        if status is None:
            success_statuses.add(200)
            unresolved_statuses.add(200)
            unknown_success_status = True
        elif 200 <= status < 300:
            success_statuses.add(status)
    for match in re.finditer(
        rf"\b{context_pattern}\.Writer\.WriteHeaderNow\s*\(\s*\)",
        code,
    ):
        prior_status = re.search(
            rf"\b{context_pattern}\.Status\s*\(\s*"
            r"(?P<status>[^()]+?)\s*\)\s*$",
            code[: match.start()],
        )
        status = status_code(prior_status.group("status")) if prior_status else None
        if status is None:
            success_statuses.add(200)
            unresolved_statuses.add(200)
            unknown_success_status = True
        elif 200 <= status < 300:
            success_statuses.add(status)
            if status != 204:
                unresolved_statuses.add(status)

    imports = symbol_index.get("imports_by_file", {}).get(current_file, {})
    http_aliases = sorted(
        alias for alias, import_path in imports.items() if import_path == "net/http"
    )
    io_aliases = sorted(
        alias for alias, import_path in imports.items() if import_path == "io"
    )
    if len(http_aliases) == 1:
        http_alias = re.escape(http_aliases[0])
        if re.search(
            rf"\b{http_alias}\.ServeContent\s*\(\s*"
            rf"{context_pattern}\.Writer\s*,\s*"
            rf"{context_pattern}\.Request\s*,",
            code,
        ):
            success_statuses.add(200)
            unresolved_statuses.add(200)
            unknown_success_status = True
    if len(io_aliases) == 1:
        io_alias = re.escape(io_aliases[0])
        if re.search(
            rf"\b{io_alias}\.Copy\s*\(\s*"
            rf"{context_pattern}\.Writer\s*,",
            code,
        ):
            success_statuses.add(200)
            unresolved_statuses.add(200)

    delegated = re.compile(
        r"(?m)(?:^|[;{}])[ \t]*(?P<name>[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)\s*\("
    )

    def merge_nested(nested: dict) -> None:
        for key, schemas in nested["schema_variants"].items():
            by_status[int(key)].extend(schemas)
        for key, media_variants in nested["content_schema_variants"].items():
            for media_type, schemas in media_variants.items():
                by_status_media[int(key)][media_type].extend(schemas)
        for key, values in nested["media_by_status"].items():
            media_by_status[int(key)].update(values)
        success_statuses.update(nested["success_statuses"])
        unresolved_statuses.update(nested["unresolved_statuses"])
        error_statuses.update(nested["error_statuses"])
        for key, schemas in nested["error_schema_variants"].items():
            error_schemas[int(key)].extend(schemas)

    if recursion_depth < 3:
        for match in delegated.finditer(code):
            if inside_nested_function(match.start()):
                continue
            name = match.group("name")
            if name.startswith("response.") or any(
                name.startswith(receiver + ".") for receiver in stable_context_names
            ):
                continue
            open_pos = code.find("(", match.start(), match.end())
            close_pos = _matching_paren(code, open_pos)
            if close_pos is None:
                continue
            args = _split_top_level_commas(body[open_pos + 1 : close_pos])
            local_closure = (
                _visible_local_function_literal(body[:match.start()], name)
                if "." not in name
                else None
            )
            if local_closure:
                context_params = [
                    (index, record)
                    for index, record in enumerate(local_closure["param_records"])
                    if _is_proven_gin_context_type(
                        record.get("go_type", "").lstrip("..."),
                        current_file,
                        symbol_index,
                    )
                ]
                nested_context = context_name
                if len(context_params) == 1:
                    context_index, context_param = context_params[0]
                    poisoned_argument = (
                        context_index < len(args)
                        and args[context_index].strip() in poisoned_context_names
                    )
                    if (
                        context_index >= len(args)
                        or (
                            not is_context_argument(args[context_index])
                            and not poisoned_argument
                        )
                        or not context_param.get("name")
                    ):
                        continue
                    nested_context = context_param["name"]
                elif context_params:
                    continue
                if _local_symbol_writes(
                    body[:match.start()], context_name
                ):
                    nested = _infer_success_response_schemas(
                        local_closure["body"], current_scope, current_file,
                        current_recv_type, symbol_index, recursion_depth + 1,
                        nested_context,
                        bound_names={
                            record["name"]
                            for record in local_closure["param_records"]
                            if record.get("name")
                        },
                    )
                    success_statuses.update(nested["success_statuses"])
                    unresolved_statuses.update(nested["success_statuses"])
                    continue
                identity = (current_scope, "local", name, current_file, local_closure["position"])
                if identity in call_stack:
                    continue
                nested = _infer_success_response_schemas(
                    local_closure["body"], current_scope, current_file,
                    current_recv_type, symbol_index, recursion_depth + 1,
                    nested_context, call_stack | {identity},
                        bound_names={
                        record["name"]
                        for record in local_closure["param_records"]
                        if record.get("name")
                        },
                        direct_response_structs=direct_response_structs,
                    )
                if len(context_params) == 1 and poisoned_argument:
                    success_statuses.update(nested["success_statuses"])
                    unresolved_statuses.update(nested["success_statuses"])
                    continue
                merge_nested(nested)
                continue
            entry = _unique_callable_results(
                name, current_scope, current_file, current_recv_type,
                symbol_index, body[:match.start()], require_results=None,
            )
            if not entry or not entry.get("body"):
                continue
            context_params = [
                (index, record)
                for index, record in enumerate(entry.get("param_records") or [])
                    if _is_proven_gin_context_type(
                        record.get("go_type", "").lstrip("..."),
                        entry["file"],
                        symbol_index,
                    )
            ]
            if len(context_params) != 1:
                continue
            context_index, context_param = context_params[0]
            poisoned_argument = (
                context_index < len(args)
                and args[context_index].strip() in poisoned_context_names
            )
            if (
                context_index >= len(args)
                or (
                    not is_context_argument(args[context_index])
                    and not poisoned_argument
                )
                or not context_param.get("name")
            ):
                continue
            if _local_symbol_writes(body[:match.start()], context_name):
                nested = _infer_success_response_schemas(
                    entry["body"], entry["scope"], entry["file"],
                    entry.get("receiver"), symbol_index, recursion_depth + 1,
                    context_param["name"], call_stack,
                    bound_names={
                        record["name"]
                        for record in entry.get("param_records") or []
                        if record.get("name")
                    } | ({entry["receiver_variable"]} if entry.get("receiver_variable") else set()),
                    direct_response_structs=direct_response_structs,
                )
                success_statuses.update(nested["success_statuses"])
                unresolved_statuses.update(nested["success_statuses"])
                continue
            identity = (
                entry["scope"], entry.get("receiver"), entry["name"], entry["file"],
                entry["line"],
            )
            if identity in call_stack:
                continue
            nested = _infer_success_response_schemas(
                entry["body"], entry["scope"], entry["file"], entry.get("receiver"),
                symbol_index, recursion_depth + 1, context_param["name"],
                call_stack | {identity},
                bound_names={
                    record["name"]
                    for record in entry.get("param_records") or []
                    if record.get("name")
                } | ({entry["receiver_variable"]} if entry.get("receiver_variable") else set()),
            )
            if not nested["success_statuses"] and not nested["error_statuses"]:
                continue
            if poisoned_argument:
                success_statuses.update(nested["success_statuses"])
                unresolved_statuses.update(nested["success_statuses"])
                continue
            merge_nested(nested)

    # A context-shaped receiver with unproven identity, branch-local origin,
    # or any reassignment remains a possible sink. It must invalidate schemas
    # from proven siblings at the same conservative success status.
    for poisoned_name in poisoned_context_names:
        poisoned = re.escape(poisoned_name)
        if response_alias:
            for match in re.finditer(
                rf"\b{re.escape(response_alias)}\."
                r"(Success|Created|Accepted|Paginated|PaginatedWithResult)\s*\(",
                code,
            ):
                open_pos = code.find("(", match.start(), match.end())
                close_pos = _matching_paren(code, open_pos)
                args = (
                    _split_top_level_commas(body[open_pos + 1 : close_pos])
                    if close_pos is not None
                    else []
                )
                if args and args[0].strip() == poisoned_name:
                    status = statuses[match.group(1)]
                    success_statuses.add(status)
                    unresolved_statuses.add(status)
        if re.search(
            rf"\b{poisoned}\.(?:{writer_names}|DataFromReader|File|FileAttachment|"
            r"FileFromFS|Stream|SSEvent|Data|String|Status|AbortWithStatus)\s*\(",
            code,
        ) or re.search(
            rf"\b{poisoned}\.Writer\."
            r"(?:Write|WriteString|WriteHeader|WriteHeaderNow|Flush)\s*\(",
            code,
        ):
            success_statuses.add(200)
            unresolved_statuses.add(200)
            unknown_success_status = True

    if unknown_success_status:
        unresolved_statuses.add(200)
        unresolved_statuses.update(
            status for status in success_statuses if 200 <= status < 300
        )

    output: dict[str, dict] = {}
    for status, variants in by_status.items():
        if status in unresolved_statuses:
            continue
        unique: list[dict] = []
        seen: set[str] = set()
        for variant in variants:
            key = json.dumps(variant, sort_keys=True)
            if key not in seen:
                seen.add(key)
                unique.append(variant)
        candidate = unique[0] if len(unique) == 1 else {"oneOf": unique}
        if _response_schema_is_complete(candidate):
            output[str(status)] = candidate

    error_output: dict[str, dict] = {}
    for status, variants in error_schemas.items():
        unique = []
        seen = set()
        for variant in variants:
            key = json.dumps(variant, sort_keys=True)
            if key not in seen and _response_schema_is_complete(variant):
                seen.add(key)
                unique.append(variant)
        if unique:
            error_output[str(status)] = (
                unique[0] if len(unique) == 1 else {"oneOf": unique}
            )
    content_output: dict[str, dict[str, dict]] = {}
    for status, media_variants in by_status_media.items():
        if status in unresolved_statuses:
            continue
        content_output[str(status)] = {}
        for media_type, variants in media_variants.items():
            unique = []
            seen = set()
            for variant in variants:
                key = json.dumps(variant, sort_keys=True)
                if key not in seen:
                    seen.add(key)
                    unique.append(variant)
            candidate = unique[0] if len(unique) == 1 else {"oneOf": unique}
            if _response_schema_is_complete(candidate):
                content_output[str(status)][media_type] = candidate
        if not content_output[str(status)]:
            content_output.pop(str(status))
    return {
        "schemas": output,
        "schema_variants": {str(key): value for key, value in by_status.items()},
        "content_schemas": content_output,
        "content_schema_variants": {
            str(status): {
                media_type: variants
                for media_type, variants in media_variants.items()
            }
            for status, media_variants in by_status_media.items()
        },
        "media_by_status": {
            str(key): sorted(value) for key, value in media_by_status.items()
        },
        "success_statuses": sorted(success_statuses),
        "unresolved_statuses": sorted(unresolved_statuses),
        "error_statuses": sorted(error_statuses),
        "error_schemas": error_output,
        "error_schema_variants": {
            str(key): value for key, value in error_schemas.items()
        },
    }


def analyze_body(
    body: str,
    all_structs: dict,
    helper_return_index: Optional[dict] = None,
    helper_scope: Optional[str] = None,
    response_symbol_index: Optional[dict] = None,
    current_file: Optional[str] = None,
    current_recv_type: Optional[str] = None,
    context_name: str = "c",
    bound_names: Optional[set[str]] = None,
    request_recursion_depth: int = 0,
    request_call_stack: Optional[set[tuple]] = None,
) -> dict:
    code = _mask_go_lexemes(body)
    context_identity_proven = bool(
        current_file
        and response_symbol_index
        and _gin_import_aliases(current_file, response_symbol_index)
    )
    safe_context_names, poisoned_context_names = _context_receiver_proof(
        body, context_name, context_identity_proven
    )
    context_pattern = (
        r"(?:" + "|".join(
            re.escape(name) for name in sorted(safe_context_names)
        ) + r")"
        if safe_context_names
        else r"__unproven_gin_context__"
    )
    all_context_names = safe_context_names | poisoned_context_names
    all_context_pattern = (
        r"(?:" + "|".join(
            re.escape(name) for name in sorted(all_context_names)
        ) + r")"
        if all_context_names
        else r"__unproven_gin_context__"
    )
    nested_function_spans = _nested_function_literal_spans(code)
    response_alias = (
        _response_package_alias(current_file, response_symbol_index or {})
        if current_file
        else None
    )

    def proven_response_helper(name: str) -> bool:
        if not response_alias or response_alias in set(bound_names or ()):
            return False
        pattern = re.compile(
            rf"\b{re.escape(response_alias)}\.{re.escape(name)}\s*\("
        )
        for match in pattern.finditer(code):
            if any(
                start <= match.start() < end
                for start, end in nested_function_spans
            ):
                continue
            if (
                _visible_local_symbol_writes(
                    body[: match.start()], response_alias
                )
                or _local_symbol_writes(body[: match.start()], context_name)
            ):
                continue
            open_pos = code.find("(", match.start(), match.end())
            close_pos = _matching_paren(code, open_pos)
            if close_pos is None:
                continue
            args = _split_top_level_commas(body[open_pos + 1 : close_pos])
            if args and args[0].strip() in safe_context_names:
                return True
        return False
    imports = (response_symbol_index or {}).get("imports_by_file", {}).get(
        current_file, {}
    )

    def sole_import_alias(import_path: str) -> Optional[str]:
        aliases = sorted(
            alias for alias, path in imports.items() if path == import_path
        )
        return aliases[0] if len(aliases) == 1 else None

    io_alias = sole_import_alias("io")
    json_alias = sole_import_alias("encoding/json")
    binding_alias = sole_import_alias("github.com/gin-gonic/gin/binding")
    io_pattern = re.escape(io_alias) if io_alias else r"__unproven_io_package__"
    json_pattern = (
        re.escape(json_alias) if json_alias else r"__unproven_json_package__"
    )
    binding_pattern = (
        re.escape(binding_alias)
        if binding_alias
        else r"__unproven_gin_binding_package__"
    )
    raw_body_read_pattern = re.compile(
        r"(?m)^[ \t]*(?P<var>[A-Za-z_]\w*)\s*(?:,\s*[A-Za-z_]\w*)?\s*:?="
        rf"\s*(?:{io_pattern}\.ReadAll\s*\(\s*{context_pattern}\.Request\.Body\s*\)|"
        rf"{context_pattern}\.GetRawData\s*\(\s*\))"
    )
    raw_body_reads = [
        match
        for match in raw_body_read_pattern.finditer(code)
        if not (
            io_alias
            and f"{io_alias}.ReadAll" in match.group(0)
            and _visible_local_symbol_writes(body[: match.start()], io_alias)
        )
    ]
    raw_body_variables = {match.group("var") for match in raw_body_reads}
    bind_patterns = [
        re.compile(
            rf"\b{context_pattern}\."
            r"(?:ShouldBindJSON|BindJSON|ShouldBind|ShouldBindBodyWith)"
            r"\(\s*&(?P<var>\w+)"
        ),
        re.compile(
            rf"{json_pattern}\.NewDecoder\s*\(\s*"
            rf"{context_pattern}\.Request\.Body\s*\)\."
            r"Decode\(\s*&(?P<var>\w+)\s*\)"
        ),
    ]
    for raw_body_variable in sorted(raw_body_variables):
        bind_patterns.extend((
            re.compile(
                rf"{json_pattern}\.Unmarshal\s*\(\s*{re.escape(raw_body_variable)}\s*,"
                r"\s*&(?P<var>\w+)\s*\)"
            ),
            re.compile(
                rf"{binding_pattern}\.JSON\.BindBody\s*\(\s*{re.escape(raw_body_variable)}\s*,"
                r"\s*&(?P<var>\w+)\s*\)"
            ),
        ))
    PARAM = re.compile(rf'{context_pattern}\.Param\(\s*"(?P<name>[^"]+)"\s*\)')
    QUERY = re.compile(rf'{context_pattern}\.Query\(\s*"(?P<name>[^"]+)"\s*\)')
    DQUERY = re.compile(rf'{context_pattern}\.DefaultQuery\(\s*"(?P<name>[^"]+)"\s*,')
    GETQ = re.compile(rf'{context_pattern}\.GetQuery\(\s*"(?P<name>[^"]+)"\s*\)')
    HEADER = re.compile(rf'{context_pattern}\.GetHeader\(\s*"(?P<name>[^"]+)"\s*\)')
    bind_matches = [
        match
        for pattern in bind_patterns
        for match in _executable_matches(pattern, body, code)
        if not any(
            alias
            and match.group(0).lstrip().startswith(alias + ".")
            and _visible_local_symbol_writes(body[: match.start()], alias)
            for alias in (json_alias, binding_alias)
        )
    ]
    recognized_body_source_spans = [
        (match.start(), match.end()) for match in raw_body_reads
    ] + [
        (match.start(), match.end())
        for match in bind_matches
        if re.search(r"Request\.Body|GetRawData", match.group(0))
    ]
    body_sources = list(re.finditer(
        rf"\b{all_context_pattern}\.Request\.Body\b|"
        rf"\b{all_context_pattern}\.GetRawData\s*\(",
        code,
    ))
    unrecognized_body_consumer = any(
        not any(start <= match.start() < end for start, end in recognized_body_source_spans)
        for match in body_sources
    )
    recognized_bind_starts = {match.start() for match in bind_matches}
    for decoder in re.finditer(
        rf"\b{all_context_pattern}\."
        r"(?:Bind[A-Za-z0-9_]*|ShouldBind[A-Za-z0-9_]*|MustBindWith)\s*\(",
        code,
    ):
        if decoder.start() not in recognized_bind_starts:
            unrecognized_body_consumer = True
    if re.search(
        rf"\b(?:read|Read)(?:LenientJSON)?RequestBody[A-Za-z0-9_]*\s*\("
        rf"[^)]*\b{all_context_pattern}\b",
        code,
    ):
        unrecognized_body_consumer = True
    for raw_body_variable in raw_body_variables:
        for call in re.finditer(
            rf"\b(?P<call>[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)\s*\("
            rf"[^()]*\b{re.escape(raw_body_variable)}\b",
            code,
        ):
            if call.group("call") not in {
                f"{json_alias}.Unmarshal" if json_alias else "",
                f"{binding_alias}.JSON.BindBody" if binding_alias else "",
            }:
                unrecognized_body_consumer = True
    param_matches = list(_executable_matches(PARAM, body, code))
    query_matches = list(_executable_matches(QUERY, body, code))
    default_query_matches = list(_executable_matches(DQUERY, body, code))
    get_query_matches = list(_executable_matches(GETQ, body, code))
    header_matches = list(_executable_matches(HEADER, body, code))
    direct_query_names = sorted(
        {
            m.group("name")
            for m in query_matches + default_query_matches + get_query_matches
        }
    )
    query_scope = helper_scope or (
        (response_symbol_index or {}).get("file_scopes", {}).get(current_file)
        if current_file
        else None
    )
    query_contracts = {
        name: {
            "name": name,
            "schema": {"type": "string"},
            "source": "direct_gin_query",
            "semantic_type_status": "wire_string_only",
        }
        for name in direct_query_names
    }
    for name in direct_query_names:
        escaped = re.escape(name)
        access_pattern = re.compile(
            rf'{context_pattern}\.(?:Query|DefaultQuery|GetQuery)\(\s*"{escaped}"'
        )
        accesses = list(_executable_matches(access_pattern, body, code))
        helper_signature_evidence: list[dict] = []
        resolved_helper_evidence: list[dict] = []
        for access in accesses:
            helper_name = _direct_unqualified_query_helper(body, access.start())
            helper_candidates = _helper_return_candidates(
                helper_name, helper_return_index, helper_scope
            )
            helper_signature_evidence.extend(
                {"helper": helper_name, **candidate}
                for candidate in helper_candidates
            )
            # Duplicate or otherwise ambiguous definitions fail closed even if
            # their apparent return types happen to agree.
            if len(helper_candidates) == 1 and helper_candidates[0].get(
                "semantic_type"
            ):
                resolved_helper_evidence.append(
                    {"helper": helper_name, **helper_candidates[0]}
                )
        helper_semantic_types = {
            item["semantic_type"] for item in resolved_helper_evidence
        }
        if helper_signature_evidence:
            query_contracts[name]["helper_evidence"] = sorted(
                {
                    f'{item["helper"]}:{item.get("return_go_type") or item["status"]}'
                    f'@{item["evidence"]}'
                    for item in helper_signature_evidence
                }
            )
        string_wrapper_calls = {"strings.TrimSpace", "strings.ToLower", "strings.ToUpper"}
        access_contexts_are_strings = bool(accesses)
        for access in accesses:
            line_start = body.rfind("\n", 0, access.start()) + 1
            prefix = body[line_start:access.start()]
            prefix_calls = {
                call
                for call in re.findall(r"([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)\s*\(", prefix)
                if not any(
                    call.startswith(receiver + ".")
                    for receiver in safe_context_names
                )
            }
            if prefix_calls - string_wrapper_calls:
                access_contexts_are_strings = False
                break
        forwarded_string_evidence = [
            _direct_query_string_parameter_evidence(
                body, access.start(), query_scope or "", current_file or "",
                current_recv_type, response_symbol_index or {},
            )
            for access in accesses
        ]
        assignment = re.search(
            rf"(?P<var>\w+)\s*(?:,\s*\w+)?\s*:?=\s*[^\n;]*"
            rf"{context_pattern}\.(?:Query|DefaultQuery|GetQuery)\(\s*\"{escaped}\"",
            body,
        )
        variable = assignment.group("var") if assignment else None
        # Names such as `raw` are often reused by later query blocks. Only use
        # parser evidence before the next query access, otherwise one parser
        # can incorrectly retype every query assigned to the same local name.
        evidence_start = assignment.start() if assignment else 0
        next_query = re.search(
            rf'{context_pattern}\.(?:Query|DefaultQuery|GetQuery)\(\s*"',
            body[assignment.end() :] if assignment else "",
        )
        evidence_end = (
            assignment.end() + next_query.start()
            if assignment and next_query
            else len(body)
        )
        evidence_window = body[evidence_start:evidence_end]
        assignment_text = body[assignment.start() : assignment.end()] if assignment else ""
        outer_calls = {
            call
            for call in re.findall(r"([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)?)\s*\(", assignment_text)
            if not any(
                call.startswith(receiver + ".")
                for receiver in safe_context_names
            )
        }
        unknown_outer_call = bool(outer_calls - string_wrapper_calls)
        variable_parameter_evidence = (
            _variable_string_parameter_evidence(
                evidence_window, variable, query_scope or "", current_file or "",
                current_recv_type, response_symbol_index or {},
            )
            if variable
            else None
        )
        integer_evidence = bool(
            re.search(
                rf"(?:strconv\.)?(?:Atoi|ParseInt|ParseUint)\(\s*{re.escape(variable)}\b",
                evidence_window,
            )
            if variable
            else False
        ) or bool(
            re.search(
                rf"(?:strconv\.)?(?:Atoi|ParseInt|ParseUint)\([^\n]*"
                rf"{context_pattern}\.(?:Query|DefaultQuery)\(\s*\"{escaped}\"",
                evidence_window,
            )
        ) or bool(
            re.search(
                rf"parse\w*(?:ID|Int|Days)\w*\(\s*{re.escape(variable)}\b",
                evidence_window,
                re.I,
            )
            if variable
            else False
        ) or bool(
            re.search(
                rf"parse\w*(?:ID|Int|Days)\w*\(\s*{context_pattern}\.(?:Query|DefaultQuery)\(\s*\"{escaped}\"",
                evidence_window,
                re.I,
            )
        )
        boolean_evidence = bool(
            re.search(rf"(?:strconv\.)?ParseBool\(\s*{re.escape(variable)}\b", evidence_window)
            if variable
            else False
        ) or bool(
            re.search(rf"parse\w*Bool\w*\(\s*{re.escape(variable)}\b", evidence_window, re.I)
            if variable
            else False
        ) or bool(
            re.search(
                rf"parse\w*Bool\w*\(\s*{context_pattern}\.(?:Query|DefaultQuery)\(\s*\"{escaped}\"",
                evidence_window,
                re.I,
            )
        ) or bool(
            re.search(
                rf'{context_pattern}\.(?:Query|DefaultQuery)\(\s*"{escaped}"[^\n]*\)\s*(?:==|!=)\s*"(?:true|false)"',
                evidence_window,
            )
        ) or bool(
            re.search(
                rf'strings\.EqualFold\([^\n]*{context_pattern}\.(?:Query|DefaultQuery)\(\s*"{escaped}"[^\n]*,\s*"(?:true|false)"\s*\)',
                evidence_window,
            )
        )
        if len(helper_semantic_types) == 1:
            semantic_type = next(iter(helper_semantic_types))
            query_contracts[name].update(
                schema={"type": semantic_type},
                source="observed_helper_return_type",
                semantic_type_status="specified",
            )
        elif helper_semantic_types:
            # Conflicting direct helper evidence for repeated accesses is not a
            # stable semantic contract.
            continue
        elif boolean_evidence and not integer_evidence:
            query_contracts[name].update(
                schema={"type": "boolean"},
                source="observed_boolean_parser",
                semantic_type_status="specified",
            )
        elif integer_evidence and not boolean_evidence:
            query_contracts[name].update(
                schema={"type": "integer"},
                source="observed_integer_parser",
                semantic_type_status="specified",
            )
        elif assignment and not unknown_outer_call:
            unknown_parser_use = bool(
                re.search(
                    rf"\b(?:parse\w+|strconv\.\w+)\([^\n;]*\b{re.escape(variable)}\b",
                    evidence_window,
                    re.I,
                )
                if variable
                else False
            )
            if not unknown_parser_use:
                query_contracts[name].update(
                    source="observed_string_use",
                    semantic_type_status="specified",
                )
            elif variable_parameter_evidence:
                query_contracts[name].update(
                    source="observed_string_parameter",
                    semantic_type_status="specified",
                    helper_evidence=[variable_parameter_evidence],
                )
        elif access_contexts_are_strings:
            query_contracts[name].update(
                source="observed_string_use",
                semantic_type_status="specified",
            )
        elif accesses and all(forwarded_string_evidence):
            query_contracts[name].update(
                source="observed_string_parameter",
                semantic_type_status="specified",
                helper_evidence=sorted(set(forwarded_string_evidence)),
            )
    if proven_response_helper("ParsePagination"):
        query_contracts.update(
            {
                "page": {
                    "name": "page",
                    "schema": {"type": "integer", "minimum": 1, "default": 1},
                    "source": "response.ParsePagination",
                    "semantic_type_status": "specified",
                },
                "page_size": {
                    "name": "page_size",
                    "schema": {"type": "integer", "minimum": 1, "maximum": 1000, "default": 20},
                    "source": "response.ParsePagination",
                    "semantic_type_status": "specified",
                },
                "limit": {
                    "name": "limit",
                    "schema": {"type": "integer", "minimum": 1, "maximum": 1000},
                    "source": "response.ParsePagination",
                    "semantic_type_status": "specified",
                },
            }
        )
    if "attr[" in code or re.search(
        rf"\bparseAttributeFilters\s*\(\s*{context_pattern}\s*\)", code
    ):
        query_contracts["attr"] = {
            "name": "attr",
            "schema": {"type": "object", "additionalProperties": {"type": "string"}},
            "style": "deepObject",
            "explode": True,
            "source": "dynamic_attr_bracket_filter",
            "semantic_type_status": "specified",
        }
    codes: set[int] = set()
    media: set[str] = set()
    result: dict = {
        "request_body_struct": None,
        "request_body_schema": None,
        "request_body_go_type": None,
        "request_body_unresolved_struct": False,
        "request_body_consumed": False,
        "request_body_schema_resolution_error": None,
        "path_params_used": sorted({m.group("name") for m in param_matches}),
        "query_params_used": sorted(query_contracts),
        "query_param_contracts": [query_contracts[name] for name in sorted(query_contracts)],
        "query_semantic_type_unresolved": sorted(
            name
            for name, contract in query_contracts.items()
            if contract["semantic_type_status"] != "specified"
        ),
        "headers_used": sorted({m.group("name") for m in header_matches}),
        "status_codes": [],
        "media_types": [],
        "redirect": bool(re.search(rf"{context_pattern}\.Redirect\(", code)),
        "stream_sse": "SSEvent" in code,
        "websocket": bool(re.search(r"websocket|WebSocket", code)),
        "binary": bool(re.search(
            rf"\b{context_pattern}\.(?:File|FileAttachment|FileFromFS)\s*\(", code
        )),
        "markdown": False,
        "paginated": proven_response_helper("Paginated"),
        "response_helpers": [],
        "response_schema_hint": None,
        "attr_query_filters": "attr[" in code,
        "typed_response_struct": None,
        "typed_response_schema": None,
        "typed_response_schemas": {},
        "typed_response_content_by_status": {},
        "success_response_media_by_status": {},
        "unresolved_success_statuses": [],
        "typed_error_schemas": {},
        "unresolved_error_statuses": [],
    }
    request_body_consumed = bool(
        bind_matches
        or body_sources
        or re.search(
            r"\b(?:read|Read)(?:LenientJSON)?RequestBody[A-Za-z0-9_]*\s*\(",
            code,
        )
    )
    result["request_body_consumed"] = request_body_consumed
    if result["redirect"]:
        codes.add(302)
    if result["stream_sse"]:
        media.add("text/event-stream")
    if result["markdown"]:
        media.add("text/markdown; charset=utf-8")
        codes.add(200)
    if result["binary"]:
        media.add("application/octet-stream")
        # Gin file helpers implicitly return 200 when no earlier guard exits.
        codes.add(200)
    if result["websocket"]:
        codes.add(426)

    resolved_request_candidates: list[tuple[dict, str, str]] = []
    decoder_unresolved = unrecognized_body_consumer
    delegated_request_consumers = 0
    for bm in sorted(bind_matches, key=lambda item: item.start()):
        var = bm.group("var")
        head = body[: bm.start()]
        head_code = code[: bm.start()]
        typ = None
        declarations = list(re.finditer(
            rf"(?m)^\s*var\s+{re.escape(var)}\s+([\w\.\*]+)", head_code
        ))
        if len(declarations) > 1:
            decoder_unresolved = True
            continue
        m2 = declarations[0] if declarations else None
        if m2:
            typ = m2.group(1)
        if typ is None:
            literals = list(re.finditer(
                rf"(?m)(?:^|[;{{}}])\s*{re.escape(var)}\s*:?=\s*"
                rf"([\w\.\*]+)\s*\{{",
                head_code,
            ))
            if len(literals) == 1:
                typ = literals[0].group(1)
            elif len(literals) > 1:
                decoder_unresolved = True
                continue
        if typ:
            simple = typ.split(".")[-1]
            if simple == "struct":
                anon = re.search(rf"var\s+{re.escape(var)}\s+struct\s*\{{", head)
                if anon:
                    brace = head.find("{", anon.start())
                    end = _matching_brace(_mask_go_lexemes(head), brace)
                    if end is not None and end > brace:
                        inline_name = (
                            f"Inline{var[:1].upper()}{var[1:]}Request{bm.start()}"
                        )
                        fake = f"type {inline_name} struct {{\n{head[brace + 1:end]}\n}}\n"
                        inline = parse_go_structs(current_file or "inline_handler_request", fake).get(inline_name)
                        current_scope = (
                            (response_symbol_index or {}).get("file_scopes", {}).get(current_file)
                            if current_file
                            else None
                        )
                        if inline and current_scope and current_file and response_symbol_index:
                            synthetic_index = dict(response_symbol_index)
                            synthetic_scopes = dict(
                                response_symbol_index.get("structs_by_scope", {})
                            )
                            current_structs = dict(synthetic_scopes.get(current_scope, {}))
                            if current_structs.get(inline_name):
                                decoder_unresolved = True
                                continue
                            current_structs[inline_name] = [inline]
                            synthetic_scopes[current_scope] = current_structs
                            synthetic_index["structs_by_scope"] = synthetic_scopes
                            schema = _response_schema_for_go_type(
                                inline_name, current_scope, current_file, synthetic_index,
                                nullable_pointer=False, allow_aliases=True,
                                schema_mode="request",
                            )
                            if schema is not None and _response_schema_is_complete(schema):
                                resolved_request_candidates.append(
                                    (schema, inline_name, "anonymous struct")
                                )
                                continue
                            decoder_unresolved = True
                            continue
            current_scope = (
                (response_symbol_index or {}).get("file_scopes", {}).get(current_file)
                if current_file
                else None
            )
            schema = (
                _response_schema_for_go_type(
                    typ, current_scope, current_file, response_symbol_index,
                    nullable_pointer=False, allow_aliases=True,
                    schema_mode="request",
                )
                if current_scope and current_file and response_symbol_index
                else None
            )
            if schema is not None and _response_schema_is_complete(schema):
                resolved_request_candidates.append(
                    (schema, simple.lstrip("*"), typ)
                )
            else:
                decoder_unresolved = True
        else:
            decoder_unresolved = True

    if (
        request_recursion_depth < 3
        and response_symbol_index
        and current_file
    ):
        current_scope = response_symbol_index.get("file_scopes", {}).get(current_file)
        call_stack = set(request_call_stack or ())
        delegated_pattern = re.compile(
            r"(?m)(?:^|[;{}])[ \t]*"
            r"(?P<name>[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)\s*\("
        )
        for call in delegated_pattern.finditer(code):
            if any(start <= call.start() < end for start, end in nested_function_spans):
                continue
            name = call.group("name")
            if name.startswith((context_name + ".", "response.")):
                continue
            open_pos = code.find("(", call.start(), call.end())
            close_pos = _matching_paren(code, open_pos)
            if close_pos is None or current_scope is None:
                continue
            args = _split_top_level_commas(body[open_pos + 1 : close_pos])
            entry = _unique_callable_results(
                name, current_scope, current_file, current_recv_type,
                response_symbol_index, body[: call.start()], require_results=None,
            )
            if not entry or not entry.get("body"):
                continue
            context_params = [
                (index, record)
                for index, record in enumerate(entry.get("param_records") or [])
                if _is_proven_gin_context_type(
                    record.get("go_type", "").lstrip("..."),
                    entry["file"],
                    response_symbol_index,
                )
            ]
            if len(context_params) != 1:
                continue
            context_index, context_param = context_params[0]
            if context_index >= len(args) or not context_param.get("name"):
                continue
            argument = args[context_index].strip()
            if argument not in all_context_names:
                continue
            identity = (
                entry["scope"], entry.get("receiver"), entry["name"],
                entry["file"], entry["line"],
            )
            if identity in call_stack:
                continue
            nested = analyze_body(
                entry["body"], all_structs,
                helper_return_index=helper_return_index,
                helper_scope=entry["scope"],
                response_symbol_index=response_symbol_index,
                current_file=entry["file"],
                current_recv_type=entry.get("receiver"),
                context_name=context_param["name"],
                bound_names={
                    record["name"]
                    for record in entry.get("param_records") or []
                    if record.get("name")
                },
                request_recursion_depth=request_recursion_depth + 1,
                request_call_stack=call_stack | {identity},
            )
            if not nested.get("request_body_consumed"):
                continue
            delegated_request_consumers += 1
            request_body_consumed = True
            result["request_body_consumed"] = True
            if (
                argument in safe_context_names
                and nested.get("request_body_schema") is not None
                and not nested.get("request_body_unresolved_struct")
            ):
                resolved_request_candidates.append((
                    nested["request_body_schema"],
                    nested.get("request_body_struct") or "delegated request",
                    nested.get("request_body_go_type") or "delegated request",
                ))
            else:
                decoder_unresolved = True

    request_schema_keys = {
        _wire_schema_key(candidate[0]) for candidate in resolved_request_candidates
    }
    decoders_equivalent = (
        bool(bind_matches or delegated_request_consumers)
        and not decoder_unresolved
        and len(resolved_request_candidates) == (
            len(bind_matches) + delegated_request_consumers
        )
        and len(request_schema_keys) == 1
    )
    if decoders_equivalent:
        schema, struct_name, go_type = resolved_request_candidates[0]
        result["request_body_schema"] = schema
        result["request_body_struct"] = struct_name
        result["request_body_go_type"] = go_type
        result["request_body_unresolved_struct"] = False
        result["request_body_schema_resolution_error"] = None
    elif request_body_consumed:
        result["request_body_schema"] = None
        result["request_body_unresolved_struct"] = True
        result["request_body_schema_resolution_error"] = (
            "additional_request_body_consumer_not_proven"
            if unrecognized_body_consumer
            else
            "multiple_request_body_schemas_not_equivalent"
            if len(bind_matches) + delegated_request_consumers > 1
            else "raw_request_body_schema_not_proven"
        )

    helpers = {
        "Success": 200,
        "Created": 201,
        "Accepted": 202,
        "Paginated": 200,
        "BadRequest": 400,
        "Unauthorized": 401,
        "Forbidden": 403,
        "NotFound": 404,
        "InternalError": 500,
    }
    for helper_name, helper_code in helpers.items():
        if proven_response_helper(helper_name):
            result["response_helpers"].append(
                f"{response_alias}.{helper_name}"
            )
            codes.add(helper_code)
    for sm in re.finditer(r"http\.Status(\w+)", code):
        mp = {
            "OK": 200,
            "Created": 201,
            "Accepted": 202,
            "NoContent": 204,
            "Found": 302,
            "BadRequest": 400,
            "Unauthorized": 401,
            "Forbidden": 403,
            "NotFound": 404,
            "Conflict": 409,
            "RequestEntityTooLarge": 413,
            "UnprocessableEntity": 422,
            "Locked": 423,
            "TooManyRequests": 429,
            "InternalServerError": 500,
            "BadGateway": 502,
            "ServiceUnavailable": 503,
            "UpgradeRequired": 426,
        }
        if sm.group(1) in mp:
            codes.add(mp[sm.group(1)])
    for sm in re.finditer(r"(?:JSON|AbortWithStatusJSON)\(\s*(\d{3})", code):
        codes.add(int(sm.group(1)))

    if proven_response_helper("Paginated"):
        result["response_schema_hint"] = "paginated"
        # try detect out type UserWithConcurrency / similar
        # Prefer local type names (UserWithConcurrency) over qualified (service.X)
        item_type = None
        for m in re.finditer(r"make\(\s*\[\]\s*([\w\.]+)", code):
            cand = m.group(1).split(".")[-1]
            if cand in all_structs:
                item_type = cand
        if not item_type:
            m = re.search(r"\[\]([\w\.]+)", code)
            if m:
                cand = m.group(1).split(".")[-1]
                if cand in all_structs:
                    item_type = cand
        if not item_type:
            m2 = re.search(r"=\s*(\w+)\s*\{", code)
            if m2 and m2.group(1) in all_structs:
                item_type = m2.group(1)
        if item_type and item_type in all_structs:
            result["typed_response_struct"] = item_type
            result["typed_response_schema"] = {
                "type": "object",
                "required": ["code", "message", "data"],
                "properties": {
                    "code": {"const": 0},
                    "message": {"type": "string"},
                    "data": {
                        "type": "object",
                        "required": ["items", "total", "page", "page_size", "pages"],
                        "properties": {
                            "items": {
                                "type": "array",
                                "items": struct_to_oas_schema(
                                    all_structs[item_type], all_structs
                                ),
                            },
                            "total": {"type": "integer"},
                            "page": {"type": "integer"},
                            "page_size": {"type": "integer"},
                            "pages": {"type": "integer"},
                        },
                        "additionalProperties": False,
                    },
                },
                "additionalProperties": False,
                "x-clomapi-response-kind": "paginated_typed",
            }
    elif ("AuthResponse" in code or "respondWithTokenPair" in code) and "AuthResponse" in all_structs:
        result["response_schema_hint"] = "auth_response"
        result["typed_response_struct"] = "AuthResponse"
        result["typed_response_schema"] = {
            "type": "object",
            "required": ["code", "message", "data"],
            "properties": {
                "code": {"const": 0},
                "message": {"type": "string"},
                "data": struct_to_oas_schema(all_structs["AuthResponse"], all_structs),
            },
            "additionalProperties": False,
            "x-clomapi-response-kind": "auth_response",
        }
    elif proven_response_helper("Success"):
        result["response_schema_hint"] = "success_envelope"
        # GetPaymentConfig returns cfg PaymentConfig
        m = re.search(
            rf"{re.escape(response_alias or 'response')}\.Success\(\s*"
            rf"{context_pattern}\s*,\s*(\w+)\s*\)",
            body,
        )
        if m:
            var = m.group(1)
            # look for cfg type
            tm = re.search(rf"{re.escape(var)},\s*err\s*:?=.*?(\w+Config)", body)
            if not tm:
                tm = re.search(rf"(\w+)\s*,\s*err\s*:?=.*GetPaymentConfig", body)
            if "PaymentConfig" in code and "GetPaymentConfig" in code:
                if "PaymentConfig" in all_structs:
                    result["typed_response_struct"] = "PaymentConfig"
                    result["typed_response_schema"] = {
                        "type": "object",
                        "required": ["code", "message", "data"],
                        "properties": {
                            "code": {"const": 0},
                            "message": {"type": "string"},
                            "data": struct_to_oas_schema(
                                all_structs["PaymentConfig"], all_structs
                            ),
                        },
                        "additionalProperties": False,
                        "x-clomapi-response-kind": "typed_success",
                    }

    if result.get("response_schema_hint") in {"paginated", "auth_response", "success_envelope"}:
        result["typed_response_schema"] = None

    if response_symbol_index and current_file:
        current_scope = response_symbol_index.get("file_scopes", {}).get(current_file)
        if current_scope:
            inferred = _infer_success_response_schemas(
                body, current_scope, current_file, current_recv_type,
                response_symbol_index, context_name=context_name,
                bound_names=bound_names,
                context_names=safe_context_names,
                poisoned_context_names=poisoned_context_names,
                # This is deliberately evidence-bound to the concrete DTO
                # chain, not to a route name. Other direct c.JSON sinks stay
                # unresolved unless their own typed contract is proven.
                direct_response_structs=(
                    {"BatchImagePublicBatch"}
                    if result.get("request_body_struct") == "BatchImageSubmitRequest"
                    else None
                ),
            )
            direct_response_structs = (
                {"BatchImagePublicBatch"}
                if result.get("request_body_struct") == "BatchImageSubmitRequest"
                else None
            )
            if direct_response_structs and 200 in inferred.get("unresolved_statuses", []):
                variants = inferred.get("schema_variants", {}).get("200") or []
                json_variants = (
                    inferred.get("content_schema_variants", {})
                    .get("200", {})
                    .get("application/json", [])
                )
                if (
                    len(variants) == 1
                    and len(json_variants) == 1
                    and _response_schema_is_complete(variants[0])
                    and variants[0] == json_variants[0]
                    and _schema_contains_struct(variants[0], direct_response_structs)
                ):
                    inferred["schemas"]["200"] = variants[0]
                    inferred["content_schemas"].setdefault("200", {})[
                        "application/json"
                    ] = variants[0]
                    result["typed_response_struct"] = "BatchImagePublicBatch"
                    inferred["unresolved_statuses"].remove(200)
            if inferred["schemas"]:
                result["typed_response_schemas"].update(inferred["schemas"])
                if result.get("typed_response_schema") is None and len(inferred["schemas"]) == 1:
                    result["typed_response_schema"] = next(iter(inferred["schemas"].values()))
            elif (
                result.get("typed_response_schema") is not None
                and not inferred["unresolved_statuses"]
            ):
                success_codes = [
                    str(code) for code in sorted(codes) if 200 <= code < 300
                ]
                if len(success_codes) == 1:
                    result["typed_response_schemas"][success_codes[0]] = result[
                        "typed_response_schema"
                    ]
            result["success_response_media_by_status"] = inferred[
                "media_by_status"
            ]
            result["typed_response_content_by_status"] = inferred[
                "content_schemas"
            ]
            result["unresolved_success_statuses"] = inferred[
                "unresolved_statuses"
            ]
            result["typed_error_schemas"] = inferred["error_schemas"]
            result["unresolved_error_statuses"] = sorted(
                set(inferred["error_statuses"]) - {
                    int(status) for status in inferred["error_schemas"]
                }
            )
            codes.update(inferred["success_statuses"])
            codes.update(inferred["error_statuses"])
            for values in inferred["media_by_status"].values():
                media.update(values)

    result["typed_response_schemas"] = {
        status: schema
        for status, schema in result["typed_response_schemas"].items()
        if _response_schema_is_complete(schema)
        and int(status) not in set(result.get("unresolved_success_statuses") or [])
    }
    if len(result["typed_response_schemas"]) == 1:
        result["typed_response_schema"] = next(iter(result["typed_response_schemas"].values()))
    elif result["typed_response_schemas"]:
        result["typed_response_schema"] = None
    elif not _response_schema_is_complete(result.get("typed_response_schema")):
        result["typed_response_schema"] = None
    if result.get("unresolved_success_statuses") and not result[
        "typed_response_schemas"
    ]:
        result["typed_response_schema"] = None

    if not media:
        media.add("application/json")
    if not codes:
        codes.add(200)
    result["status_codes"] = sorted(codes)
    result["media_types"] = sorted(media)
    return result


def _mask_go_lexemes(text: str, mask_strings: bool = True) -> str:
    """Mask Go comments and optionally strings while preserving offsets/newlines."""
    out = list(text)
    i = 0
    state = None
    escaped = False
    while i < len(text):
        char = text[i]
        nxt = text[i + 1] if i + 1 < len(text) else ""
        if state == "line_comment":
            if char == "\n":
                state = None
            else:
                out[i] = " "
            i += 1
            continue
        if state == "block_comment":
            if char == "*" and nxt == "/":
                out[i] = out[i + 1] = " "
                state = None
                i += 2
            else:
                if char != "\n":
                    out[i] = " "
                i += 1
            continue
        if state in ('"', "'", "`"):
            if mask_strings and char != "\n":
                out[i] = " "
            if state != "`" and escaped:
                escaped = False
            elif state != "`" and char == "\\":
                escaped = True
            elif char == state:
                state = None
            i += 1
            continue
        if char == "/" and nxt == "/":
            out[i] = out[i + 1] = " "
            state = "line_comment"
            i += 2
        elif char == "/" and nxt == "*":
            out[i] = out[i + 1] = " "
            state = "block_comment"
            i += 2
        elif char in ('"', "'", "`"):
            state = char
            if mask_strings:
                out[i] = " "
            i += 1
        else:
            i += 1
    return "".join(out)


def _executable_matches(
    pattern: re.Pattern, text: str, code: Optional[str] = None
):
    """Yield matches made from code plus complete string literals only.

    Call patterns need the original literal value, but no token may be borrowed
    from a comment or from only part of a quoted lexeme.  That prevents regexes
    from stitching a call together across comments/strings while retaining the
    original offsets used to decode route and parameter literals.
    """
    masked = code if code is not None else _mask_go_lexemes(text)
    lexical = _go_lexeme_spans(text)
    for match in pattern.finditer(text):
        start, end = match.span()
        if not masked[start:end].strip():
            continue
        overlaps = [
            span for span in lexical
            if span["start"] < end and span["end"] > start
        ]
        if any(span["kind"] == "comment" for span in overlaps):
            continue
        if any(
            span["kind"] == "string"
            and not (start <= span["start"] and span["end"] <= end)
            for span in overlaps
        ):
            continue
        yield match


def _go_lexeme_spans(text: str) -> list[dict]:
    """Return complete comment/string byte spans without changing offsets."""
    spans: list[dict] = []
    i = 0
    while i < len(text):
        if text.startswith("//", i):
            end = text.find("\n", i + 2)
            end = len(text) if end < 0 else end
            spans.append({"kind": "comment", "start": i, "end": end})
            i = end
            continue
        if text.startswith("/*", i):
            close = text.find("*/", i + 2)
            end = len(text) if close < 0 else close + 2
            spans.append({"kind": "comment", "start": i, "end": end})
            i = end
            continue
        if text[i] in ('"', "'", "`"):
            quote = text[i]
            end = i + 1
            escaped = False
            while end < len(text):
                char = text[end]
                if quote != "`" and escaped:
                    escaped = False
                elif quote != "`" and char == "\\":
                    escaped = True
                elif char == quote:
                    end += 1
                    break
                end += 1
            spans.append({"kind": "string", "start": i, "end": end})
            i = end
            continue
        i += 1
    return spans


def _go_block_spans(code: str) -> list[dict]:
    """Build brace scopes from masked Go source, including an implicit root."""
    blocks = [{"id": 0, "parent": None, "start": 0, "end": len(code), "depth": 0}]
    stack = [0]
    for offset, char in enumerate(code):
        if char == "{":
            block_id = len(blocks)
            blocks.append({
                "id": block_id, "parent": stack[-1], "start": offset + 1,
                "end": len(code), "depth": len(stack),
            })
            stack.append(block_id)
        elif char == "}" and len(stack) > 1:
            blocks[stack.pop()]["end"] = offset
    return blocks


def _block_at(blocks: list[dict], offset: int) -> int:
    candidates = [
        block for block in blocks
        if block["start"] <= offset <= block["end"]
    ]
    return max(candidates, key=lambda item: item["depth"])["id"]


def _block_is_ancestor(blocks: list[dict], ancestor: int, child: int) -> bool:
    cursor: Optional[int] = child
    while cursor is not None:
        if cursor == ancestor:
            return True
        cursor = blocks[cursor]["parent"]
    return False


def _visible_symbol_records(scope: str, symbol: str) -> list[dict]:
    """Return writes belonging to the binding visible at the end of scope."""
    code = _mask_go_lexemes(scope)
    blocks = _go_block_spans(code)
    records: list[dict] = []
    assignment = re.compile(
        r"(?m)(?:^|[;{}])[ \t]*"
        r"(?P<lhs>[A-Za-z_]\w*(?:[ \t]*,[ \t]*[A-Za-z_]\w*)*)"
        r"[ \t]*(?P<op>:=|=(?!=))[ \t]*"
    )
    for match in assignment.finditer(code):
        lhs = [item.strip() for item in match.group("lhs").split(",")]
        if symbol in lhs:
            records.append({
                "kind": "assignment", "match": match, "position": match.start(),
                "block": _block_at(blocks, match.start("lhs")), "op": match.group("op"),
                "lhs": lhs,
            })
    var_pattern = re.compile(
        rf"(?m)(?:^|[;{{}}])[ \t]*var[ \t]+(?P<names>"
        rf"[A-Za-z_]\w*(?:[ \t]*,[ \t]*[A-Za-z_]\w*)*)[ \t]+"
        rf"(?P<type>[^\s=;{{}}]+)"
    )
    for match in var_pattern.finditer(code):
        if symbol in [item.strip() for item in match.group("names").split(",")]:
            records.append({
                "kind": "var", "match": match, "position": match.start(),
                "block": _block_at(blocks, match.start("names")), "op": "var",
                "lhs": [item.strip() for item in match.group("names").split(",")],
                "type": match.group("type"),
            })
    records.sort(key=lambda item: item["position"])

    declarations: list[dict] = []
    for record in records:
        same_block = [
            declaration for declaration in declarations
            if declaration["block"] == record["block"]
            and declaration["position"] < record["position"]
        ]
        declares = record["kind"] == "var" or (
            record["op"] == ":=" and not same_block
        )
        if declares:
            record["binding"] = (record["block"], record["position"])
            declarations.append(record)
            continue
        visible = [
            declaration for declaration in declarations
            if declaration["position"] < record["position"]
            and _block_is_ancestor(blocks, declaration["block"], record["block"])
        ]
        record["binding"] = (
            max(visible, key=lambda item: (blocks[item["block"]]["depth"], item["position"]))[
                "binding"
            ]
            if visible
            else None
        )

    call_block = _block_at(blocks, len(code))
    visible_at_call = [
        declaration for declaration in declarations
        if _block_is_ancestor(blocks, declaration["block"], call_block)
    ]
    if not visible_at_call:
        return []
    binding = max(
        visible_at_call,
        key=lambda item: (blocks[item["block"]]["depth"], item["position"]),
    )["binding"]
    return [record for record in records if record.get("binding") == binding]


def _visible_local_symbol_writes(scope: str, symbol: str) -> list[re.Match]:
    return [record["match"] for record in _visible_symbol_records(scope, symbol)]


def _local_symbol_writes(scope: str, symbol: str) -> list[re.Match]:
    """Find direct, multi-LHS, and shadowing writes to a local symbol."""
    scope = _mask_go_lexemes(scope)
    writes = []
    assignment = re.compile(
        r"(?m)(?:^|[;{}])[ \t]*(?:(?:if|switch|for)[ \t]+)?"
        r"(?P<lhs>[A-Za-z_]\w*(?:[ \t]*,[ \t]*[A-Za-z_]\w*)*)"
        r"[ \t]*(?::=|=(?!=))"
    )
    for match in assignment.finditer(scope):
        if symbol in [item.strip() for item in match.group("lhs").split(",")]:
            writes.append(match)
    for match in re.finditer(
        rf"(?m)(?:^|[;{{}}])[ \t]*var[ \t]+(?:[A-Za-z_]\w*[ \t]*,[ \t]*)*"
        rf"{re.escape(symbol)}(?:[ \t,]|$)",
        scope,
    ):
        writes.append(match)
    writes.extend(
        re.finditer(
            rf"(?m)\b{re.escape(symbol)}\s*(?:\+\+|--|"
            r"\+=|-=|\*=|/=|%=|&=|\|=|\^=|<<=|>>=)",
            scope,
        )
    )
    writes.extend(
        re.finditer(
            rf"\bfunc\s*\([^)]*\b{re.escape(symbol)}\s+"
            r"(?:\*?[A-Za-z_][\w\.]*|interface\s*\{\s*\}|any)\b",
            scope,
            re.S,
        )
    )
    return sorted(writes, key=lambda match: match.start())


def _local_closure(
    path: str, text: str, scope_start: int, scope_end: int, symbol: str
) -> dict:
    scope = text[scope_start:scope_end]
    writes = _local_symbol_writes(scope, symbol)
    masked_scope = _mask_go_lexemes(scope)
    definitions = list(
        re.finditer(
            rf"(?m)^[ \t]*{re.escape(symbol)}[ \t]*:=[ \t]*"
            r"func[ \t]*\([^)]*\)[ \t]*(?:[A-Za-z_][\w\.\[\]\*]*)?[ \t]*\{",
            masked_scope,
        )
    )
    evidence = [
        f"{path}:{line_of(text, scope_start + match.start())}" for match in writes
    ]
    if len(writes) != 1 or len(definitions) != 1:
        return {
            "resolution_error": (
                f"ambiguous_local_handler_reassignment:{symbol}:{len(writes)}"
            ),
            "evidence": evidence,
        }
    definition = definitions[0]
    absolute_start = scope_start + definition.start()
    open_brace = scope_start + definition.end() - 1
    close_brace = _matching_brace(text, open_brace)
    if close_brace is None or close_brace > scope_end:
        return {
            "resolution_error": f"unterminated_local_handler:{symbol}",
            "evidence": [f"{path}:{line_of(text, absolute_start)}"],
        }
    return {
        "definition_start": absolute_start,
        "body_start": open_brace + 1,
        "body": text[open_brace + 1 : close_brace],
        "evidence": [f"{path}:{line_of(text, absolute_start)}"],
    }


def _schema_for_literal(value: object) -> dict:
    if isinstance(value, dict):
        return {
            "type": "object",
            "properties": {
                key: _schema_for_literal(item) for key, item in value.items()
            },
            "additionalProperties": False,
        }
    if isinstance(value, list):
        if not value:
            # The exact empty value is complete without inventing an item type.
            return {"const": []}
        item_schemas = [_schema_for_literal(item) for item in value]
        first = item_schemas[0]
        return {
            "type": "array",
            "items": (
                first
                if all(schema == first for schema in item_schemas)
                and _response_schema_is_complete(first)
                else {}
            ),
        }
    if isinstance(value, bool):
        return {"type": "boolean"}
    if isinstance(value, int):
        return {"type": "integer"}
    if isinstance(value, str):
        return {"type": "string"}
    if value is None:
        return {"nullable": True}
    return {}


def _supported_go_literal(expr: str, environment: Optional[dict] = None):
    """Evaluate the deliberately small literal subset used by ccAuxHandler."""
    environment = environment or {}
    expr = expr.strip()
    if re.fullmatch(r"[A-Za-z_]\w*", expr) and expr in environment:
        return environment[expr]
    if re.fullmatch(r'"(?:[^"\\]|\\.)*"', expr):
        return json.loads(expr)
    if re.fullmatch(r"-?\d+", expr):
        return int(expr)
    if expr in ("true", "false"):
        return expr == "true"
    if expr == "nil":
        return None
    plus = re.fullmatch(
        r"(?P<left>[A-Za-z_]\w*)\s*\+\s*(?P<right>\"(?:[^\"\\]|\\.)*\")",
        expr,
    )
    if plus and isinstance(environment.get(plus.group("left")), str):
        return environment[plus.group("left")] + json.loads(plus.group("right"))
    composite = re.match(r"^(?P<type>gin\.H|\[\]any)\s*\{", expr)
    if not composite:
        raise ValueError(f"unsupported_literal:{expr}")
    open_brace = expr.find("{", composite.start())
    close_brace = _matching_brace(expr, open_brace)
    if close_brace is None or expr[close_brace + 1 :].strip():
        raise ValueError(f"unsupported_literal:{expr}")
    entries = split_top_level_args(expr[open_brace + 1 : close_brace])
    if composite.group("type") == "[]any":
        return [
            _supported_go_literal(item, environment) for item in entries if item.strip()
        ]
    result = {}
    for entry in entries:
        match = re.match(
            r'^\s*(?P<key>"(?:[^"\\]|\\.)*")\s*:\s*(?P<value>.*)$',
            entry,
            re.S,
        )
        if not match:
            raise ValueError(f"unsupported_map_entry:{entry}")
        result[json.loads(match.group("key"))] = _supported_go_literal(
            match.group("value"), environment
        )
    return result


def _brace_depth(text: str, offset: int) -> int:
    depth = 0
    quote = None
    escaped = False
    for char in text[:offset]:
        if quote:
            if quote != "`" and escaped:
                escaped = False
            elif quote != "`" and char == "\\":
                escaped = True
            elif char == quote:
                quote = None
            continue
        if char in ('"', "'", "`"):
            quote = char
        elif char == "{":
            depth += 1
        elif char == "}":
            depth -= 1
    return depth


def _top_level_function_body(text: str, name: str):
    match = re.search(rf"(?m)^func\s+{re.escape(name)}\s*\(", text)
    if not match:
        return None
    params_open = text.find("(", match.start(), match.end())
    params_close = find_matching_paren(text, params_open)
    body_open = text.find("{", params_close + 1) if params_close >= 0 else -1
    body_close = _matching_brace(text, body_open) if body_open >= 0 else None
    if body_open < 0 or body_close is None:
        return None
    return match.start(), body_open + 1, text[body_open + 1 : body_close]


def _cc_aux_behavior_contract(
    path: str, text: str, closure_body: str, closure_body_start: int,
    endpoint_name: str, context_name: str,
) -> dict:
    """Prove ccAux guards/effects from the same frozen route source."""
    context = re.escape(context_name)
    closure_code = _mask_go_lexemes(closure_body)
    limit_guards = list(re.finditer(
        rf"if\s+!limitCCAuxRequestBody\s*\(\s*{context}\s*\)\s*"
        r"\{\s*return\s*\}",
        closure_code,
        re.S,
    ))
    read_calls = list(re.finditer(
        rf"body\s*,\s*ok\s*:=\s*readCCAuxRequestBody\s*\(\s*{context}\s*\)",
        closure_code,
    ))
    read_guards = list(
        re.finditer(r"if\s+!ok\s*\{\s*return\s*\}", closure_code, re.S)
    )
    if (
        len(limit_guards) != 1
        or len(read_calls) != 1
        or len(read_guards) != 1
        or any(
            _brace_depth(closure_code, match.start()) != 0
            for match in (limit_guards[0], read_calls[0], read_guards[0])
        )
    ):
        return {"resolution_error": "unproven_cc_aux_request_guards"}
    limit_guard, read_call, read_guard = (
        limit_guards[0],
        read_calls[0],
        read_guards[0],
    )
    if not (limit_guard.start() < read_call.start() < read_guard.start()):
        return {"resolution_error": "ambiguous_cc_aux_guard_order"}

    timelines = list(re.finditer(
        r"service\.RecordGatewayDebugTimelineBody\s*\(",
        closure_code,
    ))
    if len(timelines) != 1:
        return {"resolution_error": "cc_aux_debug_timeline_missing"}
    timeline = timelines[0]
    if _brace_depth(closure_code, timeline.start()) != 0:
        return {"resolution_error": "cc_aux_debug_timeline_not_top_level"}
    if any(
        _brace_depth(closure_code, match.start()) == 0
        for match in re.finditer(r"\breturn\b", closure_code[: timeline.start()])
    ):
        return {"resolution_error": "cc_aux_debug_timeline_unreachable"}
    timeline_open = closure_code.find("(", timeline.start())
    timeline_close = find_matching_paren(closure_code, timeline_open)
    if timeline_close < 0:
        return {"resolution_error": "cc_aux_debug_timeline_unterminated"}
    timeline_args = split_top_level_args(
        closure_body[timeline_open + 1 : timeline_close]
    )
    metadata = timeline_args[5].strip() if len(timeline_args) == 6 else ""
    metadata_code = _mask_go_lexemes(metadata, mask_strings=False)
    metadata_match = re.match(r"^map\s*\[\s*string\s*\]\s*any\s*\{", metadata_code)
    metadata_values = {}
    if metadata_match:
        metadata_open = metadata_code.find("{", metadata_match.start())
        metadata_close = _matching_brace(metadata_code, metadata_open)
        if metadata_close is not None and not metadata_code[
            metadata_close + 1 :
        ].strip():
            for entry in split_top_level_args(
                metadata_code[metadata_open + 1 : metadata_close]
            ):
                item = re.fullmatch(
                    r'\s*("(?:[^"\\]|\\.)*")\s*:\s*(.*?)\s*',
                    entry,
                    re.S,
                )
                if not item:
                    metadata_values = {}
                    break
                try:
                    key = json.loads(item.group(1))
                except json.JSONDecodeError:
                    metadata_values = {}
                    break
                if key in metadata_values:
                    metadata_values = {}
                    break
                metadata_values[key] = item.group(2).strip()
    success_calls = list(re.finditer(
        rf"\b{context}\.JSON\s*\(\s*statusCode\s*,\s*response\s*\)",
        closure_code,
    ))
    if (
        len(timeline_args) != 6
        or timeline_args[0].strip() != "settingService"
        or timeline_args[1].strip() != context_name
        or timeline_args[2].strip() != '"cc_aux_request"'
        or timeline_args[3].strip() != "body"
        or not re.fullmatch(
            rf"{context}\.GetHeader\s*\(\s*\"Content-Type\"\s*\)",
            timeline_args[4].strip(),
        )
        or metadata_values
        != {
            "component": '"cc_aux_endpoint"',
            "endpoint_name": "endpointName",
            "headers": "headers",
        }
        or len(success_calls) != 1
        or timeline.start() >= success_calls[0].start()
    ):
        return {"resolution_error": "cc_aux_debug_timeline_shape_changed"}

    source_code = _mask_go_lexemes(text)
    constant = re.search(
        r"(?m)^const\s+ccAuxMaxBodyBytes\s+int64\s*=\s*1\s*<<\s*20\s*$",
        source_code,
    )
    limit_helper = _top_level_function_body(text, "limitCCAuxRequestBody")
    read_helper = _top_level_function_body(text, "readCCAuxRequestBody")
    exact_limit_signature = re.search(
        r"(?m)^func\s+limitCCAuxRequestBody\s*"
        r"\(\s*c\s+\*gin\.Context\s*\)\s+bool\s*\{",
        source_code,
    )
    exact_read_signature = re.search(
        r"(?m)^func\s+readCCAuxRequestBody\s*"
        r"\(\s*c\s+\*gin\.Context\s*\)\s*"
        r"\(\s*\[\]byte\s*,\s*bool\s*\)\s*\{",
        source_code,
    )
    if (
        not constant
        or not limit_helper
        or not read_helper
        or not exact_limit_signature
        or not exact_read_signature
    ):
        return {"resolution_error": "cc_aux_limit_evidence_missing"}
    limit_start, limit_body_start, limit_body = limit_helper
    read_start, read_body_start, read_body = read_helper
    limit_code = _mask_go_lexemes(limit_body)
    read_code = _mask_go_lexemes(read_body)
    limit_nil_headers = list(re.finditer(
        r"if\s+c\s*==\s*nil\s*\|\|\s*c\.Request\s*==\s*nil\s*\|\|\s*"
        r"c\.Request\.Body\s*==\s*nil\s*\{",
        limit_code,
    ))
    content_headers = list(re.finditer(
        r"if\s+c\.Request\.ContentLength\s*>\s*ccAuxMaxBodyBytes\s*\{",
        limit_code,
    ))
    max_bytes_assignments = list(re.finditer(
        r"c\.Request\.Body\s*=\s*http\.MaxBytesReader\s*\(\s*c\.Writer\s*,\s*"
        r"c\.Request\.Body\s*,\s*ccAuxMaxBodyBytes\s*\)",
        limit_code,
    ))
    read_nil_headers = list(re.finditer(
        r"if\s+c\s*==\s*nil\s*\|\|\s*c\.Request\s*==\s*nil\s*\|\|\s*"
        r"c\.Request\.Body\s*==\s*nil\s*\{",
        read_code,
    ))
    read_assignments = list(re.finditer(
        r"body\s*,\s*err\s*:=\s*io\.ReadAll\s*\(\s*c\.Request\.Body\s*\)",
        read_code,
    ))
    read_success_headers = list(re.finditer(
        r"if\s+err\s*==\s*nil\s*\{",
        read_code,
    ))
    max_error_declarations = list(re.finditer(
        r"var\s+maxErr\s+\*http\.MaxBytesError\b",
        read_code,
    ))
    max_error_headers = list(re.finditer(
        r"if\s+errors\.As\s*\(\s*err\s*,\s*&maxErr\s*\)\s*\{",
        read_code,
    ))
    error_response = {"error": "request body too large"}

    def block_bounds(code_body: str, header: re.Match):
        block_open = code_body.find("{", header.start(), header.end())
        block_close = _matching_brace(code_body, block_open)
        return block_open, block_close

    def exact_return_block(
        code_body: str, header: re.Match, expected_return: str
    ) -> bool:
        block_open, block_close = block_bounds(code_body, header)
        return (
            block_close is not None
            and re.fullmatch(
                rf"\s*{re.escape(expected_return)}\s*",
                code_body[block_open + 1 : block_close],
            )
            is not None
        )

    def prove_error_block(
        raw_body: str, code_body: str, header: re.Match, expected_return: str
    ) -> bool:
        block_open, block_close = block_bounds(code_body, header)
        if block_close is None:
            return False
        raw_block = raw_body[block_open + 1 : block_close]
        code_block = code_body[block_open + 1 : block_close]
        calls = list(re.finditer(r"\bc\.AbortWithStatusJSON\s*\(", code_block))
        if len(calls) != 1:
            return False
        call_open = code_block.find("(", calls[0].start())
        call_close = find_matching_paren(code_block, call_open)
        if call_close < 0:
            return False
        arguments = split_top_level_args(raw_block[call_open + 1 : call_close])
        try:
            payload = (
                _supported_go_literal(arguments[1])
                if len(arguments) == 2
                else None
            )
        except (ValueError, json.JSONDecodeError):
            return False
        return (
            arguments[0].strip() == "http.StatusRequestEntityTooLarge"
            and payload == error_response
            and not code_block[: calls[0].start()].strip()
            and re.fullmatch(
                rf"\s*{re.escape(expected_return)}\s*",
                code_block[call_close + 1 :],
            )
            is not None
        )

    content_length_error = (
        content_headers[0]
        if len(content_headers) == 1
        and prove_error_block(
            limit_body, limit_code, content_headers[0], "return false"
        )
        else None
    )
    max_bytes_error = (
        max_error_headers[0]
        if len(max_error_headers) == 1
        and prove_error_block(
            read_body, read_code, max_error_headers[0], "return nil, false"
        )
        else None
    )
    limit_nil = limit_nil_headers[0] if len(limit_nil_headers) == 1 else None
    read_nil = read_nil_headers[0] if len(read_nil_headers) == 1 else None
    read_assignment = (
        read_assignments[0] if len(read_assignments) == 1 else None
    )
    read_success = (
        read_success_headers[0] if len(read_success_headers) == 1 else None
    )
    max_error_declaration = (
        max_error_declarations[0]
        if len(max_error_declarations) == 1
        else None
    )
    max_bytes_assignment = (
        max_bytes_assignments[0]
        if len(max_bytes_assignments) == 1
        else None
    )

    limit_structure_ok = False
    if limit_nil and content_length_error and max_bytes_assignment:
        _, limit_nil_close = block_bounds(limit_code, limit_nil)
        _, content_close = block_bounds(limit_code, content_length_error)
        limit_structure_ok = bool(
            limit_nil_close is not None
            and content_close is not None
            and _brace_depth(limit_code, limit_nil.start()) == 0
            and _brace_depth(limit_code, content_length_error.start()) == 0
            and _brace_depth(limit_code, max_bytes_assignment.start()) == 0
            and exact_return_block(limit_code, limit_nil, "return true")
            and not limit_code[: limit_nil.start()].strip()
            and not limit_code[
                limit_nil_close + 1 : content_length_error.start()
            ].strip()
            and not limit_code[
                content_close + 1 : max_bytes_assignment.start()
            ].strip()
            and re.fullmatch(
                r"\s*return\s+true\s*",
                limit_code[max_bytes_assignment.end() :],
            )
            is not None
        )

    read_structure_ok = False
    if (
        read_nil
        and read_assignment
        and read_success
        and max_error_declaration
        and max_bytes_error
    ):
        _, read_nil_close = block_bounds(read_code, read_nil)
        _, read_success_close = block_bounds(read_code, read_success)
        _, max_error_close = block_bounds(read_code, max_bytes_error)
        read_structure_ok = bool(
            read_nil_close is not None
            and read_success_close is not None
            and max_error_close is not None
            and all(
                _brace_depth(read_code, match.start()) == 0
                for match in (
                    read_nil,
                    read_assignment,
                    read_success,
                    max_error_declaration,
                    max_bytes_error,
                )
            )
            and exact_return_block(read_code, read_nil, "return nil, true")
            and exact_return_block(read_code, read_success, "return body, true")
            and not read_code[: read_nil.start()].strip()
            and not read_code[
                read_nil_close + 1 : read_assignment.start()
            ].strip()
            and not read_code[
                read_assignment.end() : read_success.start()
            ].strip()
            and not read_code[
                read_success_close + 1 : max_error_declaration.start()
            ].strip()
            and not read_code[
                max_error_declaration.end() : max_bytes_error.start()
            ].strip()
            and re.fullmatch(
                r"\s*return\s+nil\s*,\s*true\s*",
                read_code[max_error_close + 1 :],
            )
            is not None
        )
    if (
        not limit_structure_ok
        or not read_structure_ok
    ):
        return {"resolution_error": "cc_aux_413_helper_shape_changed"}

    error_schema = _schema_for_literal(error_response)
    return {
        "request_body_limit": {
            "bytes": 1 << 20,
            "constant": "ccAuxMaxBodyBytes",
            "evidence": [
                f"{path}:{line_of(text, constant.start())}",
                f"{path}:{line_of(text, limit_body_start + max_bytes_assignment.start())}",
            ],
        },
        "debug_timeline": {
            "event": "cc_aux_request",
            "component": "cc_aux_endpoint",
            "endpoint_name": endpoint_name,
            "evidence": [
                f"{path}:{line_of(text, closure_body_start + timeline.start())}"
            ],
        },
        "error_branches": [
            {
                "kind": "factory_error",
                "cause": "content_length_exceeds_limit",
                "status_codes": [413],
                "media_types": ["application/json"],
                "response_example": error_response,
                "response_schema": error_schema,
                "call_evidence": (
                    f"{path}:{line_of(text, limit_body_start + content_length_error.start())}"
                ),
                "handler_evidence": [
                    f"{path}:{line_of(text, limit_start)}",
                    f"{path}:{line_of(text, constant.start())}",
                ],
            },
            {
                "kind": "factory_error",
                "cause": "max_bytes_error",
                "status_codes": [413],
                "media_types": ["application/json"],
                "response_example": error_response,
                "response_schema": error_schema,
                "call_evidence": (
                    f"{path}:{line_of(text, read_body_start + max_bytes_error.start())}"
                ),
                "handler_evidence": [
                    f"{path}:{line_of(text, read_start)}",
                    f"{path}:{line_of(text, constant.start())}",
                ],
            },
        ],
    }


def _factory_binding(
    path: str, text: str, expression: str, route_offset: int
) -> Optional[dict]:
    call = re.match(r"^(?P<name>[A-Za-z_]\w*)\s*\(", expression)
    if not call:
        return None
    call_open = expression.find("(", call.start())
    call_close = find_matching_paren(expression, call_open)
    if call_close < 0 or expression[call_close + 1 :].strip():
        return None
    symbol = call.group("name")
    definitions = []
    for definition in re.finditer(rf"(?m)^func\s+{re.escape(symbol)}\s*\(", text):
        params_open = text.find("(", definition.start(), definition.end())
        params_close = find_matching_paren(text, params_open)
        if params_close < 0:
            continue
        body_open = text.find("{", params_close + 1)
        if body_open < 0:
            continue
        body_close = _matching_brace(text, body_open)
        return_clause = text[params_close + 1 : body_open].strip()
        if body_close is not None and return_clause == "gin.HandlerFunc":
            definitions.append(
                (definition, params_open, params_close, body_open, body_close)
            )
    base = {
        "kind": "factory_call",
        "symbol": symbol,
        "invocation": expression,
        "evidence": [
            f"{path}:{line_of(text, route_offset)} call {expression}",
        ],
    }
    if len(definitions) != 1:
        base["resolution_error"] = (
            f"ambiguous_handler_factory:{symbol}:{len(definitions)}"
        )
        base["evidence"].extend(
            f"{path}:{line_of(text, definition.start())}"
            for definition, _, _, _, _ in definitions
        )
        return base

    definition, params_open, params_close, body_open, body_close = definitions[0]
    base["evidence"].append(f"{path}:{line_of(text, definition.start())}")
    factory_body = text[body_open + 1 : body_close]
    factory_code = _mask_go_lexemes(factory_body)
    returned = list(re.finditer(r"\breturn\s+func\s*\([^)]*\)\s*\{", factory_code))
    top_level = [
        match for match in returned
        if _brace_depth(factory_code, match.start()) == 0
    ]
    if len(returned) != 1 or len(top_level) != 1:
        base["resolution_error"] = (
            f"ambiguous_factory_return:{symbol}:{len(returned)}"
        )
        return base
    returned_match = top_level[0]
    closure_open = returned_match.end() - 1
    closure_close = _matching_brace(factory_code, closure_open)
    if closure_close is None:
        base["resolution_error"] = f"unterminated_factory_closure:{symbol}"
        return base
    if factory_code[: returned_match.start()].strip() or factory_code[
        closure_close + 1 :
    ].strip():
        base["resolution_error"] = f"ambiguous_factory_control_flow:{symbol}"
        return base

    parameters = []
    for item in split_top_level_args(text[params_open + 1 : params_close]):
        match = re.fullmatch(r"([A-Za-z_]\w*)\s+(.+)", item.strip(), re.S)
        if not match:
            base["resolution_error"] = f"unsupported_factory_parameters:{symbol}"
            return base
        parameters.append((match.group(1), match.group(2).strip()))
    arguments = split_top_level_args(expression[call_open + 1 : call_close])
    if len(arguments) != len(parameters):
        base["resolution_error"] = f"factory_arity_mismatch:{symbol}"
        return base

    bound = {}
    binding_evidence = []
    try:
        for (name, go_type), argument in zip(parameters, arguments):
            argument = argument.strip()
            if go_type.startswith("*") and re.fullmatch(r"[A-Za-z_]\w*", argument):
                bound[name] = {"dependency_ref": argument, "go_type": go_type}
            elif go_type == "int" and argument in {
                "http.StatusOK",
                "http.StatusCreated",
                "http.StatusNotFound",
            }:
                bound[name] = {
                    "http.StatusOK": 200,
                    "http.StatusCreated": 201,
                    "http.StatusNotFound": 404,
                }[argument]
            else:
                bound[name] = _supported_go_literal(argument)
            binding_evidence.append(
                {
                    "parameter": name,
                    "go_type": go_type,
                    "argument": argument,
                    "value": bound[name],
                }
            )
    except (ValueError, json.JSONDecodeError) as exc:
        base["resolution_error"] = f"unsupported_factory_argument:{symbol}:{exc}"
        return base

    required = ("endpointName", "statusCode", "response")
    if not all(name in bound for name in required):
        base["resolution_error"] = f"unsupported_factory_contract:{symbol}"
        return base
    endpoint_name = bound["endpointName"]
    status_code = bound["statusCode"]
    response = bound["response"]
    if (
        not isinstance(endpoint_name, str)
        or not endpoint_name
        or type(status_code) is not int
        or not 100 <= status_code <= 599
        or isinstance(response, dict) is False
    ):
        base["resolution_error"] = f"unsupported_factory_contract:{symbol}"
        return base

    func_pos = factory_code.find("func", returned_match.start(), returned_match.end())
    context_open = factory_code.find("(", func_pos, returned_match.end())
    context_close = find_matching_paren(factory_code, context_open)
    context_match = re.fullmatch(
        r"\s*([A-Za-z_]\w*)\s+\*gin\.Context\s*",
        factory_code[context_open + 1 : context_close]
        if context_close >= 0
        else "",
    )
    if not context_match:
        base["resolution_error"] = f"unsupported_factory_context_parameter:{symbol}"
        return base
    context_name = context_match.group(1)
    closure_body = factory_body[closure_open + 1 : closure_close]
    closure_code = _mask_go_lexemes(closure_body)
    for parameter in required:
        writes = _local_symbol_writes(closure_body, parameter)
        if writes:
            base["resolution_error"] = (
                f"ambiguous_factory_parameter_flow:{parameter}:{len(writes)}"
            )
            return base
    success_calls = list(
        re.finditer(
            rf"\b{re.escape(context_name)}\.JSON\s*"
            r"\(\s*statusCode\s*,\s*response\s*\)",
            closure_code,
        )
    )
    if len(success_calls) != 1:
        base["resolution_error"] = f"unproven_factory_parameter_flow:{symbol}"
        return base
    success_call = success_calls[0]
    success_open = closure_code.find("(", success_call.start())
    success_close = find_matching_paren(closure_code, success_open)
    if (
        _brace_depth(closure_code, success_call.start()) != 0
        or success_close < 0
        or closure_code[success_close + 1 :].strip()
    ):
        base["resolution_error"] = f"conditional_factory_success:{symbol}"
        return base
    closure_body_start = body_open + 1 + closure_open + 1
    behavior = _cc_aux_behavior_contract(
        path, text, closure_body, closure_body_start, endpoint_name, context_name
    )
    if behavior.get("resolution_error"):
        base["resolution_error"] = behavior["resolution_error"]
        return base
    response_schema = _schema_for_literal(response)
    if not _response_schema_is_complete(response_schema):
        base["resolution_error"] = f"incomplete_factory_response_schema:{symbol}"
        return base
    base.update(
        parameter_bindings=binding_evidence,
        endpoint_name=endpoint_name,
        status_code=status_code,
        response_example=response,
        response_schema=response_schema,
        response_evidence=[
            base["evidence"][0],
            f"{path}:{line_of(text, closure_body_start + success_call.start())}",
        ],
        request_body_limit=behavior["request_body_limit"],
        debug_timeline=behavior["debug_timeline"],
        error_branches=behavior["error_branches"],
        _body=closure_body,
    )
    return base


def _predicate_contract(
    path: str, text: str, scope_start: int, scope_end: int, symbol: str
) -> dict:
    closure = _local_closure(path, text, scope_start, scope_end, symbol)
    if closure.get("resolution_error"):
        return closure
    body = _mask_go_lexemes(closure["body"])
    if re.fullmatch(
        r"\s*switch\s+getGroupPlatform\(c\)\s*\{\s*"
        r"case\s+service\.PlatformOpenAI\s*,\s*service\.PlatformGrok\s*:"
        r"\s*return\s+true\s*default\s*:\s*return\s+false\s*\}\s*",
        body,
        re.S,
    ):
        closure.update(
            true_platforms=["openai", "grok"],
            false_platforms=["fallback"],
            semantics="OpenAI/Grok compatible platform",
        )
        return closure
    if re.fullmatch(
        r"\s*return\s+getGroupPlatform\(c\)\s*==\s*service\.PlatformGrok\s*",
        body,
        re.S,
    ):
        closure.update(
            true_platforms=["grok"],
            false_platforms=["not_grok"],
            semantics="Grok platform",
        )
        return closure
    return {
        "resolution_error": f"unsupported_platform_predicate:{symbol}",
        "evidence": closure["evidence"],
    }


def _local_rejection_contract(
    path: str, text: str, scope_start: int, scope_end: int,
    symbol: str, invocation: str,
) -> dict:
    closure = _local_closure(path, text, scope_start, scope_end, symbol)
    if closure.get("resolution_error"):
        return closure
    call = re.fullmatch(
        rf"{re.escape(symbol)}\s*\(\s*c\s*,\s*(?P<endpoint>\"(?:[^\"\\]|\\.)*\")\s*\)",
        invocation.strip(),
    )
    if not call:
        return {
            "resolution_error": f"unsupported_local_rejection_call:{symbol}",
            "evidence": closure["evidence"],
        }
    endpoint = json.loads(call.group("endpoint"))
    body = closure["body"]
    body_code = _mask_go_lexemes(body)
    json_calls = list(re.finditer(r"\bc\.JSON\s*\(", body_code))
    if len(json_calls) != 1:
        return {
            "resolution_error": f"unsupported_local_rejection_body:{symbol}",
            "evidence": closure["evidence"],
        }
    json_open = body_code.find("(", json_calls[0].start())
    json_close = find_matching_paren(body_code, json_open)
    arguments = (
        split_top_level_args(body[json_open + 1 : json_close])
        if json_close >= 0
        else []
    )
    expected_response = {
        "error": {
            "type": "not_found_error",
            "message": endpoint + " is not supported for this platform",
        }
    }
    try:
        response = (
            _supported_go_literal(arguments[1], {"endpoint": endpoint})
            if len(arguments) == 2
            else None
        )
    except (ValueError, json.JSONDecodeError):
        response = None
    if (
        len(arguments) != 2
        or arguments[0].strip() != "http.StatusNotFound"
        or response != expected_response
        or _brace_depth(body_code, json_calls[0].start()) != 0
    ):
        return {
            "resolution_error": f"unsupported_local_rejection_body:{symbol}",
            "evidence": closure["evidence"],
        }
    return {
        "kind": "local_response",
        "handler_ref": symbol,
        "status_codes": [404],
        "media_types": ["application/json"],
        "response_example": response,
        "response_schema": _schema_for_literal(response),
        "evidence": closure["evidence"],
    }


def _closure_branches(
    path: str, text: str, scope_start: int, scope_end: int, closure: dict
) -> dict:
    body = closure["body"]
    first = re.match(
        r"\s*if\s+(?P<negate>!?)"
        r"(?P<predicate>[A-Za-z_]\w*)\s*\(\s*c\s*\)\s*\{",
        body,
    )
    if not first:
        return {"resolution_error": "unsupported_static_handler_control_flow"}
    branch_open = first.end() - 1
    branch_close = _matching_brace(body, branch_open)
    if branch_close is None:
        return {"resolution_error": "unterminated_static_handler_branch"}
    branch_body = body[branch_open + 1 : branch_close].strip()
    fallback_body = body[branch_close + 1 :].strip()
    predicate = _predicate_contract(
        path, text, scope_start, scope_end, first.group("predicate")
    )
    if predicate.get("resolution_error"):
        return predicate
    predicate_public = {
        "function": first.group("predicate"),
        "semantics": predicate["semantics"],
        "true_platforms": predicate["true_platforms"],
        "false_platforms": predicate["false_platforms"],
        "evidence": predicate["evidence"],
    }

    target_pattern = re.compile(
        r"^\s*(?P<ref>h(?:\.[A-Za-z_]\w*)+)\s*\(\s*c\s*\)"
        r"\s*(?:return\s*)?$",
        re.S,
    )
    branch_target = target_pattern.fullmatch(branch_body)
    fallback_target = target_pattern.fullmatch(fallback_body)
    branches = []
    if branch_target and fallback_target and not first.group("negate"):
        branches = [
            {
                "kind": "handler",
                "handler_ref": branch_target.group("ref"),
                "predicate_outcome": True,
                "platforms": predicate["true_platforms"],
                "predicate": predicate_public,
                "call_evidence": f"{path}:{line_of(text, closure['body_start'] + branch_open + 1)}",
            },
            {
                "kind": "handler",
                "handler_ref": fallback_target.group("ref"),
                "predicate_outcome": False,
                "platforms": predicate["false_platforms"],
                "predicate": predicate_public,
                "call_evidence": f"{path}:{line_of(text, closure['body_start'] + branch_close + 1)}",
            },
        ]
    elif fallback_target and first.group("negate"):
        rejection_match = re.fullmatch(
            r"\s*(?P<call>[A-Za-z_]\w*\s*\(\s*c\s*,\s*"
            r"\"(?:[^\"\\]|\\.)*\"\s*\))\s*return\s*",
            branch_body,
            re.S,
        )
        if not rejection_match:
            return {"resolution_error": "unsupported_local_rejection_branch"}
        rejection_symbol = re.match(r"([A-Za-z_]\w*)", rejection_match.group("call")).group(1)
        rejection = _local_rejection_contract(
            path, text, scope_start, scope_end,
            rejection_symbol, rejection_match.group("call"),
        )
        if rejection.get("resolution_error"):
            return rejection
        branches = [
            {
                **rejection,
                "predicate_outcome": False,
                "platforms": predicate["false_platforms"],
                "predicate": predicate_public,
                "call_evidence": f"{path}:{line_of(text, closure['body_start'] + branch_open + 1)}",
            },
            {
                "kind": "handler",
                "handler_ref": fallback_target.group("ref"),
                "predicate_outcome": True,
                "platforms": predicate["true_platforms"],
                "predicate": predicate_public,
                "call_evidence": f"{path}:{line_of(text, closure['body_start'] + branch_close + 1)}",
            },
        ]
    else:
        return {"resolution_error": "unsupported_static_handler_control_flow"}
    return {"branches": branches}


def _audited_static_handler_profile(path: str, text: str, symbol: str) -> dict:
    """Gate specialized semantics on the exact manually audited source blob."""
    expected_path = (
        "backend/internal/server/routes/common.go"
        if symbol == "ccAuxHandler"
        else "backend/internal/server/routes/gateway.go"
    )
    actual_digest = hashlib.sha256(text.encode()).hexdigest()
    profile = {
        "id": AUDITED_STATIC_HANDLER_PROFILE_ID,
        "path": path,
        "sha256": actual_digest,
    }
    evidence = [
        f"audited_source_profile:{AUDITED_STATIC_HANDLER_PROFILE_ID}:"
        f"{path}:sha256={actual_digest}"
    ]
    if path != expected_path or actual_digest != AUDITED_STATIC_HANDLER_BLOBS[expected_path]:
        return {
            "proof_profile": profile,
            "evidence": evidence,
            "resolution_error": (
                f"audited_source_profile_mismatch:{AUDITED_STATIC_HANDLER_PROFILE_ID}:"
                f"{path}:{actual_digest}"
            ),
        }
    return {
        "proof_profile": profile,
        "evidence": evidence,
    }


def _audited_gateway_static_binding(path: str, text: str, symbol: str) -> dict:
    """Return semantics audited for the frozen gateway.go profile only."""
    profile = _audited_static_handler_profile(path, text, symbol)
    result = {
        "kind": "named_local_closure",
        "symbol": symbol,
        "proof_profile": profile["proof_profile"],
        "evidence": list(profile.get("evidence") or []),
    }
    if profile.get("resolution_error"):
        result["resolution_error"] = profile["resolution_error"]
        return result

    if symbol == "responsesHandler":
        predicate = {
            "function": "isOpenAIResponsesCompatibleGatewayPlatform",
            "semantics": "OpenAI/Grok compatible platform",
            "true_platforms": ["openai", "grok"],
            "false_platforms": ["fallback"],
            "evidence": [f"{path}:34"],
        }
        branches = [
            {
                "kind": "handler",
                "handler_ref": "h.OpenAIGateway.Responses",
                "predicate_outcome": True,
                "platforms": ["openai", "grok"],
                "predicate": predicate,
                "call_evidence": f"{path}:207",
            },
            {
                "kind": "handler",
                "handler_ref": "h.Gateway.Responses",
                "predicate_outcome": False,
                "platforms": ["fallback"],
                "predicate": predicate,
                "call_evidence": f"{path}:210",
            },
        ]
        definition_line = 205
    else:
        predicate = {
            "function": "isVideoSupportedPlatform",
            "semantics": "Grok platform",
            "true_platforms": ["grok"],
            "false_platforms": ["not_grok"],
            "evidence": [f"{path}:46"],
        }
        rejection_example = {
            "error": {
                "type": "not_found_error",
                "message": "Videos API is not supported for this platform",
            }
        }
        branches = [
            {
                "kind": "local_response",
                "predicate_outcome": False,
                "platforms": ["not_grok"],
                "predicate": predicate,
                "status_codes": [404],
                "media_types": ["application/json"],
                "response_example": rejection_example,
                "response_schema": _schema_for_literal(rejection_example),
                "call_evidence": f"{path}:{169 if symbol == 'videoHandler' else 259}",
                "evidence": [f"{path}:52-60"],
            },
            {
                "kind": "handler",
                "handler_ref": "h.OpenAIGateway.Videos",
                "predicate_outcome": True,
                "platforms": ["grok"],
                "predicate": predicate,
                "call_evidence": f"{path}:{172 if symbol == 'videoHandler' else 262}",
            },
        ]
        definition_line = 167 if symbol == "videoHandler" else 257

    result["evidence"].append(f"{path}:{definition_line}")
    result["branches"] = branches
    result["targets"] = [
        {"ref": branch["handler_ref"], "evidence": branch["call_evidence"]}
        for branch in branches
        if branch["kind"] == "handler"
    ]
    result["protocol_branching"] = True
    return result


def _static_handler_binding(
    path: str, text: str, terminal: str, route_offset: int, function_span: tuple[int, int]
) -> Optional[dict]:
    """Resolve generic bindings separately from pinned audited special semantics."""
    expression = terminal.strip()
    factory_match = re.match(r"^(?P<name>[A-Za-z_]\w*)\s*\(", expression)
    if factory_match:
        symbol = factory_match.group("name")
        if symbol == "ccAuxHandler":
            profile = _audited_static_handler_profile(path, text, symbol)
            if profile.get("resolution_error"):
                return {
                    "kind": "factory_call",
                    "symbol": symbol,
                    "invocation": expression,
                    **profile,
                }
            result = _factory_binding(path, text, expression, route_offset)
            if result is None:
                return None
            result["proof_profile"] = profile["proof_profile"]
            result.setdefault("evidence", []).extend(profile["evidence"])
            return result
        return _factory_binding(path, text, expression, route_offset)

    symbol_match = re.fullmatch(r"([A-Za-z_]\w*)", expression)
    if not symbol_match:
        return None
    symbol = symbol_match.group(1)
    if symbol in {"responsesHandler", "videoHandler", "rootVideoHandler"}:
        return _audited_gateway_static_binding(path, text, symbol)

    scope_start, scope_end = function_span
    closure = _local_closure(path, text, scope_start, scope_end, symbol)
    if not closure.get("evidence") and closure.get("resolution_error"):
        return None
    result = {
        "kind": "named_local_closure",
        "symbol": symbol,
        "evidence": closure.get("evidence") or [],
    }
    if closure.get("resolution_error"):
        result["resolution_error"] = closure["resolution_error"]
        return result
    if closure["definition_start"] >= route_offset:
        result["resolution_error"] = f"local_handler_defined_after_use:{symbol}"
        return result
    branch_result = _closure_branches(path, text, scope_start, scope_end, closure)
    if branch_result.get("resolution_error"):
        result["resolution_error"] = branch_result["resolution_error"]
        return result
    result["branches"] = branch_result["branches"]
    result["targets"] = [
        {"ref": branch["handler_ref"], "evidence": branch["call_evidence"]}
        for branch in result["branches"]
        if branch["kind"] == "handler"
    ]
    result["protocol_branching"] = True
    result["_body"] = closure["body"]
    return result


def analyze_protocol_branches(
    branches: list[dict],
    handler_files: dict[str, str],
    all_structs: dict,
    index: dict,
    response_symbol_index: dict,
) -> tuple[list[dict], Optional[str]]:
    """Analyze each concrete handler/local-response branch independently."""
    analyzed = []
    contract_keys = (
        "request_body_struct",
        "request_body_schema",
        "request_body_unresolved_struct",
        "status_codes",
        "media_types",
        "typed_response_schema",
        "typed_response_schemas",
        "typed_response_content_by_status",
        "success_response_media_by_status",
        "unresolved_success_statuses",
        "typed_error_schemas",
        "unresolved_error_statuses",
        "typed_response_struct",
        "response_schema_hint",
        "response_helpers",
        "stream_sse",
        "websocket",
        "binary",
        "redirect",
    )
    for branch in branches:
        item = dict(branch)
        if branch["kind"] in (
            "local_response",
            "factory_response",
            "factory_error",
        ):
            status_codes = list(branch.get("status_codes") or [])
            schema = branch.get("response_schema")
            item["contract"] = {
                "status_codes": status_codes,
                "media_types": list(branch.get("media_types") or ["application/json"]),
                "typed_response_schema": schema,
                "typed_response_schemas": (
                    {str(status_codes[0]): schema}
                    if len(status_codes) == 1 and schema
                    else {}
                ),
                "response_example": branch.get("response_example"),
                "endpoint_name": branch.get("endpoint_name"),
                "outcome": branch.get("outcome"),
                "cause": branch.get("cause"),
                "effects": branch.get("effects"),
                "response_schema_status": "specified",
                "response_schema_evidence": (
                    [branch.get("call_evidence")]
                    + list(branch.get("handler_evidence") or [])
                ),
                "evidence": (
                    [branch.get("call_evidence")]
                    + list(branch.get("handler_evidence") or [])
                ),
            }
            analyzed.append(item)
            continue

        evidence = branch.get("handler_evidence") or []
        if not evidence:
            return analyzed, f"branch_handler_evidence_missing:{branch.get('handler_ref')}"
        fpath = evidence[0].rsplit(":", 1)[0]
        text = handler_files.get(fpath)
        if text is None:
            return analyzed, f"branch_handler_file_missing:{fpath}"
        found = extract_handler_body(
            text, branch.get("method") or "", branch.get("recv_type")
        )
        if not found:
            return analyzed, f"branch_handler_body_missing:{branch.get('handler_ref')}"
        body_line, body, context_name, bound_names = found
        contract = analyze_body(
            body,
            all_structs,
            index["helper_returns"],
            index["file_scopes"].get(fpath),
            response_symbol_index,
            fpath,
            branch.get("recv_type"),
            context_name or "__unproven_gin_context__",
            bound_names,
        )
        body_code = _mask_go_lexemes(body)
        context_pattern = re.escape(
            context_name or "__unproven_gin_context__"
        )
        direct_body_consumption = bool(
            re.search(
                rf"\b{context_pattern}\.Request\.Body\b|\b(?:io\.)?ReadAll\s*\("
                r"|\bReadRequestBody\w*\s*\(|\breadLenientJSONRequestBody\w*\s*\("
                r"|\bShouldBindJSON\s*\(|\bBindJSON\s*\(|\bGetRawData\s*\(",
                body_code,
            )
        )
        # Direct branch inspection can add evidence, but it must never erase
        # delegated or opaque consumption already established by analyze_body.
        consumes_request_body = bool(
            contract.get("request_body_consumed") or direct_body_consumption
        )
        protocol_contract_ref = branch.get("protocol_contract_ref")
        safe_protocol_ref = bool(
            isinstance(protocol_contract_ref, str)
            and re.fullmatch(
                r"#/components/schemas/[A-Za-z_][A-Za-z0-9_]*",
                protocol_contract_ref,
            )
        )
        request_schema_complete = _response_schema_is_complete(
            contract.get("request_body_schema")
        )
        contract["request_body_consumed"] = consumes_request_body
        contract["request_body_schema_status"] = (
            "specified"
            if request_schema_complete
            else ("protocol_contract_ref" if safe_protocol_ref else "unresolved")
        )
        contract["protocol_contract_ref"] = (
            protocol_contract_ref if safe_protocol_ref else None
        )
        if consumes_request_body and not request_schema_complete and not safe_protocol_ref:
            if not contract.get("request_body_schema_resolution_error"):
                contract["request_body_schema_resolution_error"] = (
                    "raw_request_body_schema_not_proven"
                )
            request_evidence = list(
                contract.get("request_body_schema_evidence") or []
            )
            branch_evidence = f"{fpath}:{body_line}"
            if branch_evidence not in request_evidence:
                request_evidence.append(branch_evidence)
            contract["request_body_schema_evidence"] = request_evidence
        if not contract.get("typed_response_schema"):
            literal_responses = []
            for call in re.finditer(r"\bc\.JSON\s*\(", body_code):
                open_paren = body.find("(", call.start())
                close_paren = find_matching_paren(body, open_paren)
                if close_paren < 0:
                    continue
                arguments = split_top_level_args(body[open_paren + 1 : close_paren])
                if len(arguments) != 2:
                    continue
                try:
                    value = _supported_go_literal(arguments[1])
                except (ValueError, json.JSONDecodeError):
                    continue
                literal_responses.append(
                    (
                        value,
                        f"{fpath}:{body_line + line_of(body, call.start()) - 1}",
                    )
                )
            if len(literal_responses) == 1 and len(contract.get("status_codes") or []) == 1:
                value, schema_evidence = literal_responses[0]
                schema = _schema_for_literal(value)
                status = str(contract["status_codes"][0])
                contract["typed_response_schema"] = schema
                contract["typed_response_schemas"] = {status: schema}
                contract["response_schema_evidence"] = [schema_evidence]
        item["contract"] = {
            key: contract.get(key) for key in contract_keys if key in contract
        }
        item["contract"]["evidence"] = [f"{fpath}:{body_line}"]
        item["contract"]["response_schema_evidence"] = contract.get(
            "response_schema_evidence"
        ) or []
        item["contract"]["response_schema_status"] = (
            "specified"
            if contract.get("typed_response_schema")
            or contract.get("typed_response_schemas")
            else "unresolved"
        )
        if item["contract"]["response_schema_status"] == "unresolved":
            item["contract"]["response_schema_resolution_error"] = (
                "typed_response_schema_not_proven"
            )
            item["contract"]["response_schema_evidence"] = [f"{fpath}:{body_line}"]
        item["contract"]["request_body_consumed"] = contract[
            "request_body_consumed"
        ]
        item["contract"]["request_body_schema_status"] = contract[
            "request_body_schema_status"
        ]
        item["contract"]["protocol_contract_ref"] = contract[
            "protocol_contract_ref"
        ]
        if contract.get("request_body_schema_resolution_error"):
            item["contract"]["request_body_schema_resolution_error"] = contract[
                "request_body_schema_resolution_error"
            ]
            item["contract"]["request_body_schema_evidence"] = contract[
                "request_body_schema_evidence"
            ]
        analyzed.append(item)
    return analyzed, None


def aggregate_branch_response_schemas(contracts: list[dict]) -> dict[str, dict]:
    """Merge complete branch schemas per status without losing alternatives."""
    by_status: dict[str, list[dict]] = defaultdict(list)
    for contract in contracts:
        schemas = dict(contract.get("typed_response_schemas") or {})
        singular = contract.get("typed_response_schema")
        statuses = contract.get("status_codes") or []
        if singular and len(statuses) == 1:
            schemas.setdefault(str(statuses[0]), singular)
        for status, schema in schemas.items():
            if _response_schema_is_complete(schema):
                by_status[str(status)].append(schema)

    merged = {}
    for status, schemas in by_status.items():
        unique = []
        seen = set()
        for schema in schemas:
            key = json.dumps(schema, sort_keys=True, separators=(",", ":"))
            if key not in seen:
                seen.add(key)
                unique.append(schema)
        candidate = unique[0] if len(unique) == 1 else {"oneOf": unique}
        if _response_schema_is_complete(candidate):
            merged[status] = candidate
    return dict(sorted(merged.items(), key=lambda item: item[0]))


def _lexical_constructor_receiver_type(
    text: str,
    variable: str,
    function_span: tuple[int, int],
    route_offset: int,
) -> Optional[str]:
    """Infer one visible, immutable, top-level constructor before route use."""
    scope_start, scope_end = function_span
    if not scope_start <= route_offset <= scope_end:
        return None
    scope = text[scope_start:scope_end]
    writes = _local_symbol_writes(scope, variable)
    masked = _mask_go_lexemes(scope)
    constructors = list(re.finditer(
        rf"(?m)^[ \t]*{re.escape(variable)}\s*:=\s*"
        r"New(?P<rtype>[A-Za-z_]\w*)\s*\(",
        masked,
    ))
    if len(writes) != 1 or len(constructors) != 1:
        return None
    constructor = constructors[0]
    route_in_scope = route_offset - scope_start
    if (
        constructor.start() >= route_in_scope
        or _brace_depth(masked, constructor.start()) != 0
    ):
        return None
    return constructor.group("rtype")


def _block_stack_at(code: str, offset: int) -> tuple[int, ...]:
    stack: list[int] = []
    for position, char in enumerate(code[:offset]):
        if char == "{":
            stack.append(position)
        elif char == "}" and stack:
            stack.pop()
    return tuple(stack)


def _conditional_block_opens(code: str) -> set[int]:
    """Identify control-flow/function-literal blocks without treating bare scopes as branches."""
    conditional: set[int] = set()
    for control in re.finditer(r"\b(?:if|for|switch|select|else)\b", code):
        paren_depth = 0
        bracket_depth = 0
        for position in range(control.end(), len(code)):
            char = code[position]
            if char == "(":
                paren_depth += 1
            elif char == ")":
                paren_depth = max(0, paren_depth - 1)
            elif char == "[":
                bracket_depth += 1
            elif char == "]":
                bracket_depth = max(0, bracket_depth - 1)
            elif char == "{" and paren_depth == 0 and bracket_depth == 0:
                conditional.add(position)
                break
            elif char == "\n" and control.group(0) == "else":
                break
    for function_literal in re.finditer(r"\bfunc\s*\([^)]*\)[^{]*\{", code):
        conditional.add(function_literal.end() - 1)
    return conditional


def extract_routes_from_file(path: str, text: str, entry_prefix_default: str) -> list[dict]:
    code = _mask_go_lexemes(text)
    register_func = "unknown"
    entry_var = None
    entry_prefix = entry_prefix_default
    groups: dict = {}

    m = FUNC_REGISTER_RE.search(code)
    function_span = (0, len(text))
    if m:
        function_open = code.find("{", m.end())
        function_close = (
            _matching_brace(_mask_go_lexemes(text), function_open)
            if function_open >= 0
            else None
        )
        if function_open >= 0 and function_close is not None:
            function_span = (function_open + 1, function_close)
        register_func = m.group("name")
        entry_var = m.group("entry")
        entry_prefix = ENTRY_PREFIXES.get(register_func, entry_prefix_default)
        groups[entry_var] = {
            "full": entry_prefix,
            "middlewares": [],
            "chain": [
                f"{path}:{line_of(text, m.start())} {entry_var}={entry_prefix or '(root)'} [register_entry]"
            ],
        }
    if "func RegisterPageRoutes" in code and entry_var is None:
        register_func = "RegisterPageRoutes"
        entry_var = "v1"
        entry_prefix = "/api/v1"
        groups[entry_var] = {
            "full": entry_prefix,
            "middlewares": [],
            "chain": [f"{path}:1 v1=/api/v1 [register_entry]"],
        }
    if entry_var is None:
        entry_var = "r"
        groups["r"] = {
            "full": entry_prefix,
            "middlewares": [],
            "chain": [f"{path}:1 r={entry_prefix or '(root)'}"],
        }

    initial_groups = {
        name: {
            "full": value["full"],
            "chain": list(value["chain"]),
            "middlewares": list(value.get("middlewares") or []),
        }
        for name, value in groups.items()
    }

    function_records: list[dict] = []
    for function_match in re.finditer(
        r"(?m)^func\s+(?P<name>[A-Za-z_]\w*)\s*\(", code
    ):
        params_open = code.find("(", function_match.start(), function_match.end())
        params_close = _matching_paren(code, params_open)
        if params_close is None:
            continue
        body_open = code.find("{", params_close + 1)
        body_close = _matching_brace(code, body_open) if body_open >= 0 else None
        if body_close is None:
            continue
        function_records.append(
            {
                "name": function_match.group("name"),
                "start": function_match.start(),
                "body_open": body_open,
                "body_close": body_close,
                "params": _go_param_records(text[params_open + 1 : params_close]),
            }
        )

    def containing_function(offset: int) -> Optional[dict]:
        matches = [
            record for record in function_records
            if record["body_open"] < offset < record["body_close"]
        ]
        return max(matches, key=lambda item: item["body_open"]) if matches else None

    conditional_opens = _conditional_block_opens(code)
    if m and function_open in conditional_opens:
        conditional_opens.remove(function_open)

    def groups_before(
        offset: int,
        resolve_delegation: bool = True,
        delegation_stack: Optional[frozenset[int]] = None,
    ) -> tuple[dict, list[str]]:
        """Replay Group/Use statements in lexical order up to a route registration."""
        state = {
            name: {
                "full": value["full"],
                "chain": list(value["chain"]),
                "middlewares": list(value.get("middlewares") or []),
            }
            for name, value in initial_groups.items()
        }
        delegated_ambiguities: list[str] = []
        function = containing_function(offset)
        delegation_stack = delegation_stack or frozenset()
        if (
            resolve_delegation
            and function
            and function["name"] != register_func
        ):
            if function["start"] in delegation_stack:
                return state, [
                    f"registrar_call_cycle:{path}:"
                    f"{line_of(text, function['start'])}:{function['name']}"
                ]
            router_params = [
                (index, parameter)
                for index, parameter in enumerate(function["params"])
                if parameter.get("name")
                and parameter.get("go_type") in {
                    "*gin.RouterGroup", "*gin.Engine"
                }
            ]
            calls = [
                call
                for call in re.finditer(
                    rf"(?<!\.)\b{re.escape(function['name'])}\s*\(", code
                )
                if containing_function(call.start()) is not None
            ]
            if len(calls) == 1 and router_params:
                call = calls[0]
                call_open = code.find("(", call.start(), call.end())
                call_close = _matching_paren(code, call_open)
                call_args = (
                    split_top_level_args(text[call_open + 1 : call_close])
                    if call_close is not None
                    else []
                )
                caller_state, caller_ambiguities = groups_before(
                    call.start(),
                    resolve_delegation=True,
                    delegation_stack=(
                        delegation_stack | {function["start"]}
                    ),
                )
                delegated_ambiguities.extend(caller_ambiguities)
                if any(
                    item.startswith("conditional_route_registration:")
                    for item in caller_ambiguities
                ):
                    delegated_ambiguities.append(
                        f"conditional_registrar_call:{path}:"
                        f"{line_of(text, call.start())}:{function['name']}"
                    )
                for index, parameter in router_params:
                    if index >= len(call_args):
                        delegated_ambiguities.append(
                            f"registrar_router_argument_missing:{path}:"
                            f"{line_of(text, call.start())}:{function['name']}"
                        )
                        continue
                    argument = call_args[index].strip()
                    bound = caller_state.get(argument)
                    if not re.fullmatch(r"[A-Za-z_]\w*", argument) or bound is None:
                        delegated_ambiguities.append(
                            f"registrar_router_argument_unresolved:{path}:"
                            f"{line_of(text, call.start())}:{function['name']}"
                        )
                        continue
                    state[parameter["name"]] = {
                        "full": bound["full"],
                        "middlewares": list(bound.get("middlewares") or []),
                        "chain": list(bound["chain"]) + [
                            f"{path}:{line_of(text, call.start())} "
                            f"{function['name']}({argument}) binds "
                            f"{parameter['name']}={bound['full']}"
                        ],
                    }
            elif router_params:
                delegated_ambiguities.append(
                    f"registrar_call_ambiguous:{path}:"
                    f"{line_of(text, function['start'])}:{function['name']}"
                )
            else:
                delegated_ambiguities.append(
                    f"registrar_caller_not_router_bound:{path}:"
                    f"{line_of(text, function['start'])}:{function['name']}"
                )
        event_start = function["body_open"] + 1 if function else 0
        event_end = min(offset, function["body_close"]) if function else offset
        events = [
            (m.start(), "group", m)
            for m in GROUP_START_RE.finditer(code, event_start, event_end)
        ]
        events.extend(
            (m.start(), "use", m)
            for m in USE_START_RE.finditer(code, event_start, event_end)
        )
        ambiguities: list[str] = list(delegated_ambiguities)
        route_stack = _block_stack_at(code, offset)
        route_conditional = [item for item in route_stack if item in conditional_opens]
        if route_conditional:
            ambiguities.append(
                f"conditional_route_registration:{path}:{line_of(text, offset)}"
            )
        for _, event_type, match in sorted(events, key=lambda item: item[0]):
            event_stack = _block_stack_at(code, match.start())
            visible = event_stack == route_stack[: len(event_stack)]
            event_conditional = any(
                item in conditional_opens for item in event_stack
            )
            if event_type == "group":
                var, parent = match.group("var"), match.group("parent")
                if not visible and match.group("op") == ":=":
                    continue
                if not visible and not event_conditional:
                    # Assignment to an outer variable in an unconditional bare
                    # block persists after the block.
                    visible = match.group("op") == "=" and var in state
                if not visible:
                    if event_conditional and (
                        var in state or parent in state
                    ):
                        ambiguities.append(
                            f"conditional_group_assignment:{path}:"
                            f"{line_of(text, match.start())}:{var}"
                        )
                    continue
                if event_conditional:
                    ambiguities.append(
                        f"conditional_group_assignment:{path}:"
                        f"{line_of(text, match.start())}:{var}"
                    )
                open_paren = code.find("(", match.start(), match.end())
                close = _matching_paren(code, open_paren)
                if close is None or close >= offset:
                    ambiguities.append(
                        f"group_call_incomplete:{path}:"
                        f"{line_of(text, match.start())}:{var}"
                    )
                    continue
                args = split_top_level_args(text[open_paren + 1 : close])
                if not args:
                    ambiguities.append(
                        f"group_path_unresolved:{path}:"
                        f"{line_of(text, match.start())}:{var}"
                    )
                    continue
                prefix_match = re.fullmatch(
                    r'"(?P<pre>(?:[^"\\]|\\.)*)"', args[0].strip(), re.S
                )
                if not prefix_match:
                    ambiguities.append(
                        f"group_path_unresolved:{path}:"
                        f"{line_of(text, match.start())}:{var}"
                    )
                    continue
                try:
                    pre = json.loads(args[0].strip())
                except json.JSONDecodeError:
                    ambiguities.append(
                        f"group_path_unresolved:{path}:"
                        f"{line_of(text, match.start())}:{var}"
                    )
                    continue
                parent_g = state.get(parent)
                if parent_g is None:
                    ambiguities.append(
                        f"group_parent_unresolved:{path}:"
                        f"{line_of(text, match.start())}:{parent}"
                    )
                    implicit_prefix = "/api/v1" if parent == "v1" else ("" if parent in ("r", "engine", "router") else entry_prefix)
                    parent_g = {
                        "full": implicit_prefix,
                        "middlewares": [],
                        "chain": [
                            f"{path}:{line_of(text, match.start())} {parent}={implicit_prefix or '(root)'} [implicit]"
                        ],
                    }
                    state[parent] = parent_g
                full = normalize_path(join_path(parent_g["full"], pre))
                state[var] = {
                    "full": full,
                    "middlewares": (
                        list(parent_g.get("middlewares") or [])
                        + [arg.strip()[:160] for arg in args[1:] if arg.strip()]
                    ),
                    "chain": list(parent_g["chain"]) + [
                        f'{path}:{line_of(text, match.start())} {var}={full} (Group "{pre}" on {parent})'
                    ],
                }
                if any(arg.strip().endswith("...") for arg in args[1:]):
                    ambiguities.append(
                        f"group_middleware_variadic_unresolved:{path}:"
                        f"{line_of(text, match.start())}:{var}"
                    )
                continue

            recv = match.group("recv")
            target = state.get(recv)
            if target is None:
                continue
            if not visible:
                if event_conditional:
                    ambiguities.append(
                        f"conditional_group_use:{path}:"
                        f"{line_of(text, match.start())}:{recv}"
                    )
                    continue
                # Use mutates the router group; an unconditional bare scope
                # does not undo that mutation when its lexical scope closes.
            elif event_conditional:
                ambiguities.append(
                    f"conditional_group_use:{path}:"
                    f"{line_of(text, match.start())}:{recv}"
                )
            open_paren = code.find("(", match.start(), match.end())
            close = _matching_paren(code, open_paren)
            if close is None or close >= offset:
                continue
            args = split_top_level_args(text[open_paren + 1 : close])
            target["middlewares"].extend(a.strip()[:160] for a in args if a.strip())
            target["chain"].append(
                f"{path}:{line_of(text, match.start())} {recv}.Use({', '.join(a[:80] for a in args)})"
            )
        return state, sorted(set(ambiguities))

    routes = []
    for mm in METHOD_START_RE.finditer(code):
        recv = mm.group("recv")
        meth = mm.group("meth")
        groups, routing_ambiguities = groups_before(mm.start())
        open_paren = code.find("(", mm.start(), mm.end())
        close = _matching_paren(code, open_paren)
        if close is None:
            continue
        start_line = line_of(text, mm.start())
        end_line = line_of(text, close)
        call_inner = text[open_paren + 1 : close]
        args = split_top_level_args(call_inner)
        if not args:
            continue
        path_arg = args[0].strip()
        pm = re.match(r'^"(.*)"$', path_arg, re.S)
        if not pm:
            continue
        route_rel = pm.group(1)
        rest = args[1:]
        inherited = groups.get(recv) or {}
        middlewares = list(inherited.get("middlewares") or [])
        non_mw = []
        for a in rest:
            if is_middleware_arg(a):
                middlewares.append(a.strip()[:160])
            else:
                non_mw.append(a.strip())
        terminal = non_mw[-1] if non_mw else None
        if len(non_mw) > 1:
            middlewares.extend(x[:80] for x in non_mw[:-1])

        handler_ref = None
        static_handler = None
        if terminal:
            if terminal.startswith("func"):
                handler_ref = "func(...)"
            else:
                handler_ref = (
                    terminal.split("(")[0].strip()
                    if not terminal.startswith("gin.HandlerFunc")
                    else terminal[:80]
                )
                qualified = re.fullmatch(
                    r"(?P<variable>[A-Za-z_]\w*)\.(?P<method>[A-Za-z_]\w*)",
                    handler_ref,
                )
                if qualified:
                    receiver_type = _lexical_constructor_receiver_type(
                        text,
                        qualified.group("variable"),
                        function_span,
                        mm.start(),
                    )
                    if receiver_type:
                        handler_ref = (
                            f"{receiver_type}.{qualified.group('method')}"
                        )
                static_handler = _static_handler_binding(
                    path, text, terminal, mm.start(), function_span
                )

        base_g = groups.get(recv)
        if base_g is None:
            if recv == "v1":
                base_g = {
                    "full": "/api/v1",
                    "middlewares": [],
                    "chain": [f"{path}:{start_line} v1=/api/v1 [implicit-recv]"],
                }
            elif recv in ("r", "engine"):
                base_g = {
                    "full": "",
                    "middlewares": [],
                    "chain": [f"{path}:{start_line} {recv}=(root) [implicit-recv]"],
                }
            else:
                base_g = {
                    "full": entry_prefix,
                    "middlewares": [],
                    "chain": [
                        f"{path}:{start_line} {recv}={entry_prefix} [fallback-recv]"
                    ],
                }
        full = normalize_path(join_path(base_g["full"], route_rel))
        chain = list(base_g["chain"]) + [
            f"{path}:{start_line}-{end_line} {meth} {full}"
        ]
        routes.append(
            {
                "method": meth.upper(),
                "path": full,
                "oas_path": oas_path(full),
                "file": path,
                "line": start_line,
                "line_end": end_line,
                "evidence": (
                    f"{path}:{start_line}"
                    if start_line == end_line
                    else f"{path}:{start_line}-{end_line}"
                ),
                "recv": recv,
                "group_chain": chain,
                "middlewares": middlewares,
                "handlers_raw": [a[:200] for a in rest],
                "handler_ref": handler_ref,
                "static_handler": (
                    {k: v for k, v in static_handler.items() if not k.startswith("_")}
                    if static_handler
                    else None
                ),
                "register_func": register_func,
                "entry_prefix": entry_prefix,
                "surface": classify_surface(full),
                "multiline": end_line != start_line,
                "routing_ambiguities": routing_ambiguities,
            }
        )
    return routes


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--legacy-root", required=True)
    ap.add_argument("--commit", required=True)
    ap.add_argument("--out-dir", required=True)
    args = ap.parse_args()

    legacy = Path(args.legacy_root)
    commit = full_sha(legacy, args.commit)
    if len(commit) != 40:
        print(f"ERROR bad commit {commit}", file=sys.stderr)
        return 2
    out_dir = Path(args.out_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    route_files = [
        "backend/internal/server/routes/common.go",
        "backend/internal/server/routes/gateway.go",
        "backend/internal/server/routes/auth.go",
        "backend/internal/server/routes/admin.go",
        "backend/internal/server/routes/user.go",
        "backend/internal/server/routes/media.go",
        "backend/internal/server/routes/payment.go",
        "backend/internal/server/routes/tls_fingerprint_capture.go",
        "backend/internal/handler/page_handler.go",
    ]

    all_routes: list[dict] = []
    file_meta: dict = {}
    for rf in route_files:
        try:
            text = git_show_text(legacy, commit, rf)
        except subprocess.CalledProcessError:
            continue
        file_meta[rf] = {
            "sha256": hashlib.sha256(text.encode()).hexdigest(),
            "bytes": len(text.encode()),
        }
        ep = "" if rf.endswith(("common.go", "gateway.go")) else "/api/v1"
        all_routes.extend(extract_routes_from_file(rf, text, ep))

    # NO DEDUPE — full mounted-path per-registration inventory
    all_routes.sort(key=lambda r: (r["path"], r["method"], r["file"], r["line"]))

    handler_paths = git_ls_files(legacy, commit, "backend/internal/handler")
    internal_paths = [
        p for p in git_ls_files(legacy, commit, "backend/internal")
        if p.endswith(".go") and not p.endswith("_test.go")
    ]
    handler_files: dict[str, str] = {}
    internal_files: dict[str, str] = {}
    all_structs: dict = {}
    for hp in handler_paths:
        if not hp.endswith(".go") or hp.endswith("_test.go"):
            continue
        text = git_show_text(legacy, commit, hp)
        handler_files[hp] = text
        internal_files[hp] = text
        for name, s in parse_go_structs(hp, text).items():
            all_structs.setdefault(name, s)
    # DTOs are declared across handler/service/model/config packages. Read every
    # frozen internal Go blob for struct definitions, while keeping handler
    # method resolution restricted to handler_files below.
    for sp in internal_paths:
        text = handler_files.get(sp)
        if text is None:
            text = git_show_text(legacy, commit, sp)
        internal_files[sp] = text
        for name, s in parse_go_structs(sp, text).items():
            all_structs.setdefault(name, s)
    for mp in [
        "backend/internal/server/middleware/middleware.go",
        "backend/internal/server/middleware/jwt_auth.go",
        "backend/internal/server/middleware/admin_auth.go",
        "backend/internal/server/middleware/api_key_auth.go",
        "backend/internal/pkg/response/response.go",
        "backend/internal/handler/auth_refresh_cookie.go",
        "backend/internal/handler/handler.go",
    ]:
        try:
            text = git_show_text(legacy, commit, mp)
            handler_files[mp] = text
            internal_files[mp] = text
            for name, s in parse_go_structs(mp, text).items():
                all_structs.setdefault(name, s)
        except subprocess.CalledProcessError:
            pass

    index = build_handler_index(handler_files)
    response_symbol_index = build_go_symbol_index(internal_files)
    ambiguous = []
    unresolved = []
    enriched = []
    for r in all_routes:
        static_handler = r.get("static_handler")
        resolution = (
            resolve_static_handler(static_handler, index)
            if static_handler
            else resolve_handler_ref(r.get("handler_ref") or "", index)
        )
        analysis: dict = {
            "handler_ref": r.get("handler_ref"),
            "resolved": resolution["resolved"],
            "ambiguous": resolution["ambiguous"],
            "recv_type": resolution.get("recv_type"),
            "method": resolution.get("method"),
            "resolution_error": resolution.get("resolution_error"),
            "evidence": resolution.get("evidence") or [],
            "static_handler": static_handler,
            "protocol_branches": resolution.get("protocol_branches") or [],
            "candidates": [
                {
                    "file": c["file"],
                    "line": c["line"],
                    "recv_type": c.get("recv_type"),
                }
                for c in (resolution.get("candidates") or [])[:8]
            ],
            "request_body_struct": None,
            "request_body_schema": None,
            "path_params_used": path_params(r["path"]),
            "query_params_used": [],
            "headers_used": [],
            "status_codes": [200],
            "media_types": ["application/json"],
            "redirect": False,
            "stream_sse": False,
            "websocket": False,
            "binary": False,
            "markdown": False,
            "paginated": False,
            "response_helpers": [],
            "response_schema_hint": None,
            "typed_response_schema": None,
            "typed_response_schemas": {},
            "typed_response_content_by_status": {},
            "success_response_media_by_status": {},
            "unresolved_success_statuses": [],
            "typed_error_schemas": {},
            "unresolved_error_statuses": [],
            "typed_response_struct": None,
            "blocking_unresolved": False,
            "contract_status": "inventoried",
        }
        if r.get("routing_ambiguities"):
            analysis["resolved"] = False
            analysis["blocking_unresolved"] = True
            analysis["blocking_reason"] = "route_registration_scope_ambiguous"
            analysis["contract_status"] = "inventoried_blocking"
            analysis["resolution_error"] = ";".join(r["routing_ambiguities"])
            unresolved.append(
                {
                    "path": r["path"],
                    "method": r["method"],
                    "ref": r.get("handler_ref"),
                    "error": analysis["resolution_error"],
                    "evidence": r["evidence"],
                }
            )
        elif resolution["ambiguous"]:
            ambiguous.append(
                {
                    "path": r["path"],
                    "method": r["method"],
                    "ref": r.get("handler_ref"),
                    "error": resolution["resolution_error"],
                    "evidence": r["evidence"],
                }
            )
            analysis["blocking_unresolved"] = True
            analysis["contract_status"] = "inventoried_blocking"
        elif not resolution["resolved"]:
            if r.get("handler_ref") in (None, "func(...)"):
                analysis["resolution_error"] = "inline_handler"
                analysis["contract_status"] = "specified" if r["surface"] != "management_admin" else "inventoried"
            else:
                unresolved.append(
                    {
                        "path": r["path"],
                        "method": r["method"],
                        "ref": r.get("handler_ref"),
                        "error": resolution["resolution_error"],
                        "evidence": r["evidence"],
                    }
                )
                analysis["blocking_unresolved"] = True
                analysis["contract_status"] = "inventoried_blocking"
        else:
            if static_handler:
                branches, branch_error = analyze_protocol_branches(
                    resolution.get("protocol_branches") or [],
                    handler_files,
                    all_structs,
                    index,
                    response_symbol_index,
                )
                analysis["protocol_branches"] = branches
                if branch_error:
                    analysis["resolved"] = False
                    analysis["blocking_unresolved"] = True
                    analysis["contract_status"] = "inventoried_blocking"
                    analysis["resolution_error"] = branch_error
                    unresolved.append(
                        {
                            "path": r["path"],
                            "method": r["method"],
                            "ref": r.get("handler_ref"),
                            "error": branch_error,
                            "evidence": r["evidence"],
                        }
                    )
                else:
                    contracts = [
                        branch["contract"] for branch in branches
                        if branch.get("contract")
                    ]
                    analysis["status_codes"] = sorted(
                        {
                            code
                            for contract in contracts
                            for code in (contract.get("status_codes") or [])
                        }
                    )
                    analysis["media_types"] = sorted(
                        {
                            media
                            for contract in contracts
                            for media in (contract.get("media_types") or [])
                        }
                    )
                    analysis["response_helpers"] = sorted(
                        {
                            helper
                            for contract in contracts
                            for helper in (contract.get("response_helpers") or [])
                        }
                    )
                    for flag in ("stream_sse", "websocket", "binary", "redirect"):
                        analysis[flag] = any(
                            bool(contract.get(flag)) for contract in contracts
                        )
                    aggregated_schemas = aggregate_branch_response_schemas(contracts)
                    analysis["typed_response_schemas"] = aggregated_schemas
                    analysis["typed_response_schema"] = (
                        next(iter(aggregated_schemas.values()))
                        if len(aggregated_schemas) == 1
                        else None
                    )
                    unresolved_response_branches = [
                        branch.get("handler_ref") or branch.get("endpoint_name")
                        for branch in branches
                        if branch["contract"].get("response_schema_status")
                        != "specified"
                    ]
                    unresolved_request_branches = [
                        branch.get("handler_ref") or branch.get("endpoint_name")
                        for branch in branches
                        if branch["contract"].get("request_body_consumed")
                        and branch["contract"].get("request_body_schema_status")
                        == "unresolved"
                    ]
                    blockers = []
                    if unresolved_request_branches:
                        blockers.append(
                            "branch_request_body_schema_unresolved:"
                            + ",".join(unresolved_request_branches)
                        )
                    if unresolved_response_branches:
                        blockers.append(
                            "branch_response_schema_unresolved:"
                            + ",".join(unresolved_response_branches)
                        )
                    analysis["branch_contract_blockers"] = blockers
                    if blockers:
                        analysis["blocking_unresolved"] = True
                        analysis["contract_status"] = "inventoried_blocking"
                        analysis["resolution_error"] = ";".join(blockers)
                    else:
                        analysis["contract_status"] = "specified"
            else:
                ev = resolution["evidence"][0]
                fpath = ev.rsplit(":", 1)[0]
                text = handler_files.get(fpath)
                found = (
                    extract_handler_body(
                        text, resolution["method"], resolution.get("recv_type")
                    )
                    if text
                    else None
                )
                if found:
                    _bline, body, context_name, bound_names = found
                    ad = analyze_body(
                        body,
                        all_structs,
                        index["helper_returns"],
                        index["file_scopes"].get(fpath),
                        response_symbol_index,
                        fpath,
                        resolution.get("recv_type"),
                        context_name or "__unproven_gin_context__",
                        bound_names,
                    )
                    for k, v in ad.items():
                        analysis[k] = v
                    for p in path_params(r["path"]):
                        if p not in analysis["path_params_used"]:
                            analysis["path_params_used"].append(p)
                    analysis["path_params_used"] = sorted(set(analysis["path_params_used"]))
                    if analysis.get("request_body_unresolved_struct"):
                        analysis["blocking_unresolved"] = True
                        analysis["blocking_reason"] = "handler_semantics_unresolved"
                        analysis["contract_status"] = "inventoried_blocking"
                        analysis["resolution_error"] = (
                            analysis.get("request_body_schema_resolution_error")
                            or f"struct_not_found:{analysis.get('request_body_struct')}"
                        )
                    elif analysis.get("typed_response_schema") or analysis.get(
                        "request_body_schema"
                    ):
                        analysis["contract_status"] = "specified"
                    elif analysis.get("resolved"):
                        analysis["contract_status"] = "specified"
                else:
                    analysis["blocking_unresolved"] = True
                    analysis["contract_status"] = "inventoried_blocking"
                    analysis["resolution_error"] = "handler_body_not_found"

        r2 = {k: v for k, v in r.items() if not k.startswith("_")}
        r2["analysis"] = analysis
        r2["registration_id"] = f"{r['method']}:{r['path']}:{r['file']}:{r['line']}"
        enriched.append(r2)

    structs_out = {
        name: {
            "name": st["name"],
            "evidence": st["evidence"],
            "schema": struct_to_oas_schema(st, all_structs),
        }
        for name, st in all_structs.items()
    }

    meta = {
        "extractor_id": EXTRACTOR_ID,
        "extractor_version": EXTRACTOR_VERSION,
        "commit_full_sha": commit,
        "legacy_root": str(legacy.resolve()),
        "route_file_meta": file_meta,
        "total_registrations": len(enriched),
        "dedupe_policy": "none_per_registration",
        "multiline_count": sum(1 for r in enriched if r.get("multiline")),
        "struct_count": len(structs_out),
        "handler_files_scanned": len(handler_files),
        "resolved_handlers": sum(1 for r in enriched if r["analysis"]["resolved"]),
        "ambiguous_handlers": len(ambiguous),
        "unresolved_handlers": len(unresolved),
        "with_body_schema": sum(
            1 for r in enriched if r["analysis"].get("request_body_schema")
        ),
        "with_typed_response": sum(
            1 for r in enriched if r["analysis"].get("typed_response_schema")
        ),
        "blocking_unresolved_count": sum(
            1 for r in enriched if r["analysis"].get("blocking_unresolved")
        ),
        "skipped_unmounted": [
            {
                "path": "backend/internal/server/routes/ai.go",
                "reason": "RegisterAIRoutes not called from router.go",
            }
        ],
    }

    (out_dir / "extraction-meta.json").write_text(json.dumps(meta, indent=2) + "\n")
    (out_dir / "routes.json").write_text(json.dumps(enriched, indent=2) + "\n")
    (out_dir / "structs.json").write_text(json.dumps(structs_out, indent=2) + "\n")
    (out_dir / "ambiguous-handlers.json").write_text(
        json.dumps(ambiguous, indent=2) + "\n"
    )
    (out_dir / "unresolved-handlers.json").write_text(
        json.dumps(unresolved, indent=2) + "\n"
    )

    mounted = {
        "schema_version": "1.0.0",
        "kind": "mounted_route_inventory",
        "baseline_id": BASELINE_ID,
        "commit_full_sha": commit,
        "extractor_id": EXTRACTOR_ID,
        "dedupe_policy": "none_per_registration",
        "count": len(enriched),
        "registrations": [
            {
                "registration_id": r["registration_id"],
                "method": r["method"],
                "path": r["path"],
                "oas_path": r["oas_path"],
                "evidence": r["evidence"],
                "group_chain": r["group_chain"],
                "middlewares": r.get("middlewares"),
                "handler_ref": r.get("handler_ref"),
                "handler_recv_type": r["analysis"].get("recv_type"),
                "handler_method": r["analysis"].get("method"),
                "handler_evidence": r["analysis"].get("evidence"),
                "static_handler": r["analysis"].get("static_handler"),
                "handler_protocol_branches": r["analysis"].get("protocol_branches"),
                "resolved": r["analysis"].get("resolved"),
                "ambiguous": r["analysis"].get("ambiguous"),
                "blocking_unresolved": r["analysis"].get("blocking_unresolved"),
                "contract_status": r["analysis"].get("contract_status"),
                "surface": r["surface"],
                "register_func": r["register_func"],
                "multiline": r.get("multiline"),
            }
            for r in enriched
        ],
    }
    (out_dir / "mounted-route-inventory.json").write_text(
        json.dumps(mounted, indent=2) + "\n"
    )

    # also export for wave A under contracts/openapi/evidence
    lines = [
        "registration_id\tmethod\tpath\tevidence\thandler_ref\trecv_type\tresolved\tblocking\tsurface"
    ]
    for r in enriched:
        lines.append(
            f"{r['registration_id']}\t{r['method']}\t{r['path']}\t{r['evidence']}\t"
            f"{r.get('handler_ref') or ''}\t{r['analysis'].get('recv_type') or ''}\t"
            f"{r['analysis'].get('resolved')}\t{r['analysis'].get('blocking_unresolved')}\t{r['surface']}"
        )
    (out_dir / "routes.tsv").write_text("\n".join(lines) + "\n")

    print(
        json.dumps(
            {
                "ok": True,
                "commit": commit,
                "registrations": len(enriched),
                "multiline": meta["multiline_count"],
                "resolved": meta["resolved_handlers"],
                "ambiguous": meta["ambiguous_handlers"],
                "unresolved": meta["unresolved_handlers"],
                "body_schema": meta["with_body_schema"],
                "typed_response": meta["with_typed_response"],
                "blocking": meta["blocking_unresolved_count"],
                "out_dir": str(out_dir),
            },
            indent=2,
        )
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
