"""G0 contract tests CLM-P0-03/04 — semantic goldens + structural checks. No pip install."""
from __future__ import annotations
import copy, hashlib, json, re, shutil, subprocess, sys, unittest
from pathlib import Path

try:
    import yaml
except ImportError:
    print("FAIL: PyYAML required via requirements.lock.txt", file=sys.stderr); sys.exit(2)
try:
    from jsonschema import Draft202012Validator
except ImportError:
    print("FAIL: jsonschema required", file=sys.stderr); sys.exit(2)

ROOT = Path(__file__).resolve().parents[3]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))
from tools.g0.contracts.validate_p0_03_04 import management_binding_errors
from tools.g0.contracts.extract_routes_dto import (
    _go_result_types,
    _response_schema_is_complete,
    analyze_body,
    build_go_symbol_index,
    build_handler_index,
    extract_handler_body,
    extract_routes_from_file,
    parse_go_structs,
)
from tools.g0.contracts.sync_management_contract import build_operation

BASELINE = "clomapi-g0-baseline-20260720T021059Z"
HEAD = "a44cc392bb526763f345db7da3e766617184374c"
OID_RE = re.compile(r"^[a-z][A-Za-z0-9]+$")
OP_RE = re.compile(r"^op\.[a-z0-9_]+(\.[a-z0-9_]+)*$")

def load_yaml(rel):
    return yaml.safe_load((ROOT / rel).read_text(encoding="utf-8"))

def find_route(method, path):
    routes = json.loads((ROOT / "contracts/openapi/evidence/extraction/routes.json").read_text())
    hits = [r for r in routes if r["method"] == method and r["path"] == path]
    assert hits, f"missing route {method} {path}"
    return hits[0]

def find_oa_op(method, path):
    oa = load_yaml("contracts/openapi/management-v1.yaml")
    # find by x-clomapi-legacy-path
    for pth, item in (oa.get("paths") or {}).items():
        op = item.get(method.lower())
        if isinstance(op, dict) and op.get("x-clomapi-legacy-path") == path:
            return op
    raise AssertionError(f"missing openapi op {method} {path}")


def binding_documents():
    routes = json.loads((ROOT / "contracts/openapi/evidence/extraction/routes.json").read_text())
    openapi = load_yaml("contracts/openapi/management-v1.yaml")
    index = json.loads((ROOT / "contracts/openapi/evidence/management-route-index.json").read_text())
    return routes, openapi, index


def mutable_oa_op(openapi, method, legacy_path):
    hits = []
    for path_item in (openapi.get("paths") or {}).values():
        operation = path_item.get(method.lower()) if isinstance(path_item, dict) else None
        if isinstance(operation, dict) and operation.get("x-clomapi-legacy-path") == legacy_path:
            hits.append(operation)
    assert len(hits) == 1, f"expected one OpenAPI operation for {method} {legacy_path}, got {len(hits)}"
    return hits[0]

class TestP003P004(unittest.TestCase):
    def test_pinned_batch_image_submit_resolves_dto_and_public_response(self):
        handler_path = "backend/internal/handler/batch_image_handler.go"
        service_path = "backend/internal/service/batch_image_public.go"
        handler = subprocess.check_output(
            ["git", "show", f"{HEAD}:{handler_path}"], text=True
        )
        service = subprocess.check_output(
            ["git", "show", f"{HEAD}:{service_path}"], text=True
        )

        def analyze(handler_source, service_source):
            sources = {handler_path: handler_source, service_path: service_source}
            structs = {}
            for path, source in sources.items():
                for name, struct in parse_go_structs(path, source).items():
                    structs.setdefault(name, struct)
            symbol_index = build_go_symbol_index(sources)
            handler_index = build_handler_index({handler_path: handler_source})
            found = extract_handler_body(
                handler_source, "Submit", "BatchImageHandler"
            )
            self.assertIsNotNone(found)
            line, body, context, bound = found
            return analyze_body(
                body,
                structs,
                handler_index["helper_returns"],
                handler_index["file_scopes"][handler_path],
                symbol_index,
                handler_path,
                "BatchImageHandler",
                context,
                bound,
            )

        analysis = analyze(handler, service)
        self.assertEqual(analysis["request_body_struct"], "BatchImageSubmitRequest")
        self.assertEqual(analysis["request_body_go_type"], "service.BatchImageSubmitRequest")
        self.assertEqual(analysis["typed_response_struct"], "BatchImagePublicBatch")
        self.assertTrue(analysis["typed_response_schema"])
        self.assertEqual(analysis["unresolved_success_statuses"], [])
        request = analysis["request_body_schema"]
        self.assertEqual(
            request["properties"]["items"]["items"]["properties"]["reference_images"]
            ["items"]["properties"]["data"],
            {"type": "string", "format": "byte", "x-clomapi-go-field": "Data", "x-clomapi-go-type": "[]byte"},
        )
        self.assertEqual(
            analysis["typed_response_schema"]["anyOf"][0]["x-clomapi-struct"],
            "BatchImagePublicBatch",
        )

        request_mutation = handler.replace(
            "service.BatchImageSubmitRequest", "service.BatchImagePublicService", 1
        )
        mutated_request = analyze(request_mutation, service)
        self.assertTrue(mutated_request["request_body_unresolved_struct"])
        self.assertIsNone(mutated_request["request_body_schema"])

        response_mutation = service.replace(
            "(*BatchImagePublicBatch, error)", "(*BatchImagePublicService, error)", 1
        )
        mutated_response = analyze(handler, response_mutation)
        self.assertIsNone(mutated_response["typed_response_schema"])
        self.assertIn(200, mutated_response["unresolved_success_statuses"])

        sink_mutation = handler.replace(
            "c.JSON(http.StatusOK, got)", "c.HTML(http.StatusOK, got)", 1
        )
        mutated_sink = analyze(sink_mutation, service)
        self.assertEqual(mutated_sink["success_response_media_by_status"], {"200": ["text/html"]})
        self.assertIn(200, mutated_sink["unresolved_success_statuses"])
        self.assertIsNone(mutated_sink["typed_response_schema"])

        status_mutation = handler.replace(
            "c.JSON(http.StatusOK, got)", "c.JSON(http.StatusCreated, got)", 1
        )
        mutated_status = analyze(status_mutation, service)
        self.assertIn(200, mutated_status["unresolved_success_statuses"])
        self.assertNotIn("200", mutated_status["typed_response_schemas"])

        unlink_mutation = handler.replace(
            "c.JSON(http.StatusOK, got)", "c.JSON(http.StatusOK, req)", 1
        )
        mutated_link = analyze(unlink_mutation, service)
        self.assertIn(200, mutated_link["unresolved_success_statuses"])
        self.assertIsNone(mutated_link["typed_response_schema"])

    def test_extractor_v3_meta(self):
        meta = json.loads((ROOT / "contracts/openapi/evidence/extraction/extraction-meta.json").read_text())
        self.assertEqual(meta["extractor_id"], "g0-route-dto-extractor-v3")
        self.assertEqual(meta["commit_full_sha"], HEAD)
        self.assertEqual(meta["dedupe_policy"], "none_per_registration")
        self.assertEqual(meta["ambiguous_handlers"], 0)
        self.assertGreater(meta["multiline_count"], 0)
        self.assertGreater(meta["total_registrations"], 600)

    def test_mounted_inventory_no_dedupe(self):
        inv = json.loads((ROOT / "contracts/openapi/evidence/mounted-route-inventory.json").read_text())
        self.assertEqual(inv["dedupe_policy"], "none_per_registration")
        self.assertEqual(inv["count"], len(inv["registrations"]))
        ids = [r["registration_id"] for r in inv["registrations"]]
        self.assertEqual(len(ids), len(set(ids)))

    def test_semantic_golden_admin_users(self):
        g = json.loads((ROOT / "contracts/openapi/evidence/semantic-goldens.json").read_text())["get_admin_users"]
        r = find_route(g["method"], g["path"])
        a = r["analysis"]
        self.assertEqual(a.get("recv_type"), g["expect_handler_recv"])
        self.assertEqual(a.get("method"), g["expect_handler_method"])
        self.assertTrue(any(g["expect_handler_evidence_contains"] in e for e in (a.get("evidence") or [])))
        for q in g["expect_query_params_contains"]:
            self.assertIn(q, a.get("query_params_used") or [])
        self.assertEqual(a.get("typed_response_struct"), g["expect_typed_response_struct"])
        self.assertIsNone(a.get("typed_response_schema"))
        self.assertEqual(a.get("typed_response_schemas"), {})
        op = find_oa_op(g["method"], g["path"])
        self.assertEqual(op["x-clomapi-handler-recv-type"], "UserHandler")
        # body must not be EmptyObject
        self.assertNotIn("EmptyObject", json.dumps(op))
        # Partial legacy structs remain candidate evidence and must not unblock.
        self.assertEqual(op["x-clomapi-status"], "inventoried_blocking")
        self.assertIn("untyped_success_response", op["x-clomapi-blocking-reasons"])
        resp = op["responses"]["200"]
        self.assertNotIn("content", resp)
        self.assertTrue(resp["x-clomapi-inventoried-blocking"])
        self.assertEqual(resp["x-clomapi-response-media-status"], "observed")
        self.assertEqual(resp["x-clomapi-observed-media-types"], ["application/json"])

    def test_admin_users_query_contracts_are_evidence_typed(self):
        route = find_route("GET", "/api/v1/admin/users")
        contracts = {
            item["name"]: item for item in route["analysis"]["query_param_contracts"]
        }
        self.assertEqual(
            contracts["page"]["schema"],
            {"type": "integer", "minimum": 1, "default": 1},
        )
        self.assertEqual(
            contracts["page_size"]["schema"],
            {"type": "integer", "minimum": 1, "maximum": 1000, "default": 20},
        )
        self.assertEqual(
            contracts["limit"]["schema"],
            {"type": "integer", "minimum": 1, "maximum": 1000},
        )
        self.assertEqual(contracts["api_key_group_id"]["schema"], {"type": "integer"})
        self.assertEqual(contracts["include_subscriptions"]["schema"], {"type": "boolean"})
        self.assertEqual(contracts["include_usage_stats"]["schema"], {"type": "boolean"})
        self.assertEqual(contracts["attr"]["schema"], {
            "type": "object", "additionalProperties": {"type": "string"},
        })
        self.assertEqual(contracts["attr"]["style"], "deepObject")
        self.assertIs(contracts["attr"]["explode"], True)
        self.assertEqual(contracts["search"]["semantic_type_status"], "specified")
        self.assertNotIn("search", route["analysis"]["query_semantic_type_unresolved"])

        operation = find_oa_op("GET", "/api/v1/admin/users")
        parameters = {
            item["name"]: item for item in operation["parameters"] if item["in"] == "query"
        }
        self.assertEqual(parameters["page"]["schema"], contracts["page"]["schema"])
        self.assertEqual(parameters["include_subscriptions"]["schema"], {"type": "boolean"})
        self.assertEqual(parameters["attr"]["style"], "deepObject")
        self.assertIs(parameters["attr"]["explode"], True)

    def test_query_type_inference_is_lexically_scoped_and_fails_closed(self):
        handler_file = "backend/internal/handler/list.go"
        handler_files = {
            handler_file: '''package handler
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct{}
'''
        }
        analysis = analyze_body(
            '''func (h *Handler) List(c *gin.Context) {
                page, size := response.ParsePagination(c)
                if raw := c.Query("account_id"); raw != "" {
                    id, _ := strconv.ParseInt(raw, 10, 64)
                    _ = id
                }
                if raw, ok := c.GetQuery("enabled"); ok {
                    value := parseBoolQueryWithDefault(raw, true)
                    _ = value
                }
                featured := parseOptionalBool(c.Query("featured"))
                _ = featured
                search := strings.TrimSpace(c.Query("search"))
                _ = search
                limitRaw := c.Query("custom_limit")
                limit := parsePositiveInt(limitRaw)
                _ = limit
                mode := parseMode(c.Query("mode"))
                _ = mode
                opaque := c.Query("opaque")
                _ = opaque
                filter := Filter{
                    Status: strings.TrimSpace(c.Query("status")),
                    SortBy: c.DefaultQuery("sort_by", "created_at"),
                }
                _ = filter
                attrs := parseAttributeFilters(c)
                _ = attrs
            }''',
            {},
            response_symbol_index=build_go_symbol_index(handler_files),
            current_file=handler_file,
            current_recv_type="Handler",
        )
        contracts = {item["name"]: item for item in analysis["query_param_contracts"]}
        self.assertEqual(contracts["account_id"]["schema"], {"type": "integer"})
        self.assertEqual(contracts["enabled"]["schema"], {"type": "boolean"})
        self.assertEqual(contracts["featured"]["schema"], {"type": "boolean"})
        self.assertEqual(contracts["search"]["semantic_type_status"], "specified")
        self.assertEqual(contracts["custom_limit"]["schema"], {"type": "integer"})
        self.assertEqual(contracts["mode"]["semantic_type_status"], "wire_string_only")
        self.assertEqual(contracts["opaque"]["semantic_type_status"], "specified")
        self.assertEqual(contracts["status"]["semantic_type_status"], "specified")
        self.assertEqual(contracts["sort_by"]["semantic_type_status"], "specified")
        self.assertEqual(analysis["query_semantic_type_unresolved"], ["mode"])
        self.assertEqual(contracts["attr"]["style"], "deepObject")

        route = {
            "registration_id": "GET:/api/v1/test:synthetic:1",
            "method": "GET",
            "path": "/api/v1/test",
            "oas_path": "/api/v1/test",
            "evidence": "synthetic:1",
            "group_chain": [],
            "middlewares": [],
            "handler_ref": "synthetic.List",
            "analysis": analysis,
        }
        operation, index_row = build_operation(route)
        self.assertIn("query_semantic_type_unresolved", operation["x-clomapi-blocking-reasons"])
        self.assertEqual(operation["x-clomapi-status"], "inventoried_blocking")
        self.assertEqual(index_row["query_semantic_type_unresolved"], ["mode"])

    def test_query_helper_return_index_is_package_scoped_and_fails_closed(self):
        handler_files = {
            "backend/internal/handler/filters.go": '''package handler
import "github.com/gin-gonic/gin"
type SkillScope string
func decodeEnabled(raw string) bool { return raw == "true" }
func decodeOptional(raw string) *bool { return nil }
func coerceCount(raw string) int64 { return 0 }
func normalizeIntent(raw string) string { return raw }
func canonicalScope(raw string) SkillScope { return SkillScope(raw) }
func decodeOpaque(raw string) any { return raw }
func splitRange(raw string) (string, error) { return raw, nil }
''',
            # Same helper name in another package must not make the handler
            # package lookup ambiguous or leak its boolean return type.
            "backend/internal/handler/admin/filters.go": '''package admin
func canonicalScope(raw string) bool { return raw == "all" }
''',
        }
        index = build_handler_index(handler_files)
        scope = index["file_scopes"]["backend/internal/handler/filters.go"]
        analysis = analyze_body(
            '''func (h *Handler) List(c *gin.Context) {
                enabled := decodeEnabled(c.Query("enabled"))
                optional := decodeOptional(c.Query("optional"))
                count := coerceCount(c.Query("count"))
                intent := normalizeIntent(c.Query("intent"))
                scope := canonicalScope(c.Query("scope"))
                opaque := decodeOpaque(c.Query("opaque"))
                start, err := splitRange(c.Query("range"))
                qualified := external.Normalize(c.Query("qualified"))
                _, _, _, _, _, _, _, _ = enabled, optional, count, intent, scope, opaque, start, err
                _ = qualified
            }''',
            {},
            index["helper_returns"],
            scope,
            response_symbol_index=build_go_symbol_index(handler_files),
            current_file="backend/internal/handler/filters.go",
            current_recv_type="Handler",
        )
        contracts = {item["name"]: item for item in analysis["query_param_contracts"]}
        self.assertEqual(contracts["enabled"]["schema"], {"type": "boolean"})
        self.assertEqual(contracts["optional"]["schema"], {"type": "boolean"})
        self.assertEqual(contracts["count"]["schema"], {"type": "integer"})
        self.assertEqual(contracts["intent"]["schema"], {"type": "string"})
        self.assertEqual(contracts["scope"]["schema"], {"type": "string"})
        self.assertTrue(contracts["scope"]["helper_evidence"][0].startswith(
            "canonicalScope:SkillScope@backend/internal/handler/filters.go:"
        ))
        # Unsupported/multiple return types cannot retype the query, but the
        # proven string input slots still establish the wire contract.
        self.assertEqual(contracts["opaque"]["semantic_type_status"], "specified")
        self.assertEqual(contracts["opaque"]["source"], "observed_string_parameter")
        self.assertEqual(contracts["range"]["semantic_type_status"], "specified")
        self.assertEqual(contracts["range"]["source"], "observed_string_parameter")
        self.assertEqual(contracts["qualified"]["semantic_type_status"], "wire_string_only")
        self.assertEqual(
            analysis["query_semantic_type_unresolved"],
            ["qualified"],
        )

    def test_query_helper_signature_mutation_changes_contract_type(self):
        template = '''package handler
import "github.com/gin-gonic/gin"
func decode(raw string) RETURN_TYPE { panic("not called") }
'''
        body = 'value := decode(c.Query("value"))'
        expected = {
            "bool": "boolean",
            "*bool": "boolean",
            "uint32": "integer",
            "string": "string",
        }
        for return_type, semantic_type in expected.items():
            with self.subTest(return_type=return_type):
                files = {
                    "backend/internal/handler/helpers.go": template.replace(
                        "RETURN_TYPE", return_type
                    )
                }
                index = build_handler_index(files)
                scope = index["file_scopes"]["backend/internal/handler/helpers.go"]
                analysis = analyze_body(
                    body, {}, index["helper_returns"], scope,
                    response_symbol_index=build_go_symbol_index(files),
                    current_file="backend/internal/handler/helpers.go",
                    current_recv_type="Handler",
                )
                contract = analysis["query_param_contracts"][0]
                self.assertEqual(contract["schema"], {"type": semantic_type})
                self.assertEqual(contract["source"], "observed_helper_return_type")

        ambiguous_files = {
            "backend/internal/handler/helpers.go": template.replace(
                "RETURN_TYPE", "(bool, error)"
            )
        }
        ambiguous_index = build_handler_index(ambiguous_files)
        ambiguous_scope = ambiguous_index["file_scopes"][
            "backend/internal/handler/helpers.go"
        ]
        ambiguous = analyze_body(
            body, {}, ambiguous_index["helper_returns"], ambiguous_scope,
            response_symbol_index=build_go_symbol_index(ambiguous_files),
            current_file="backend/internal/handler/helpers.go",
            current_recv_type="Handler",
        )
        ambiguous_contract = ambiguous["query_param_contracts"][0]
        self.assertEqual(ambiguous_contract["schema"], {"type": "string"})
        self.assertEqual(ambiguous_contract["semantic_type_status"], "specified")
        self.assertEqual(ambiguous_contract["source"], "observed_string_parameter")

    def test_query_input_slots_resolve_grouped_params_and_receiver_chains(self):
        files = {
            "backend/internal/handler/list.go": '''package handler
import "github.com/Wei-Shaw/sub2api/internal/service"
import "github.com/gin-gonic/gin"
type Handler struct{}
func (h *Handler) Module() (*service.Module, error) { return nil, nil }
func parseRange(start, end, timezone string) (int64, int64, error) { return 0, 0, nil }
func redirectError(description string) {}
''',
            "backend/internal/service/query.go": '''package service
type Filter struct{}
func (f *Filter) SetSort(sortBy, sortOrder string) {}
type Queries struct{}
func (q *Queries) List(status, search string) {}
type Module struct { Queries *Queries }
func ParseKind(raw string) (int16, error) { return 0, nil }
func ConsumeInteger(value int64) {}
''',
        }
        symbols = build_go_symbol_index(files)
        analysis = analyze_body(
            '''kind := strings.TrimSpace(c.Query("kind"))
parsed, err := service.ParseKind(kind)
_, _ = parsed, err
parseRange(c.Query("start"), c.Query("end"), c.Query("timezone"))
redirectError(c.Query("description"))
filter := &service.Filter{}
filter.SetSort(c.Query("sort_by"), c.Query("sort_order"))
module, err := h.Module()
module.Queries.List(c.Query("status"), c.Query("search"))
service.ConsumeInteger(c.Query("must_fail_closed"))''',
            {}, response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        contracts = {item["name"]: item for item in analysis["query_param_contracts"]}
        for name in (
            "kind", "start", "end", "timezone", "description", "sort_by",
            "sort_order", "status", "search",
        ):
            with self.subTest(name=name):
                self.assertEqual(contracts[name]["schema"], {"type": "string"})
                self.assertEqual(contracts[name]["semantic_type_status"], "specified")
        self.assertEqual(
            contracts["must_fail_closed"]["semantic_type_status"],
            "wire_string_only",
        )
        self.assertEqual(analysis["query_semantic_type_unresolved"], ["must_fail_closed"])

    def test_response_converter_is_package_qualified_and_serialization_aware(self):
        files = {
            "backend/internal/handler/list.go": '''package handler
import "github.com/Wei-Shaw/sub2api/internal/handler/dto"
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct{}
''',
            "backend/internal/handler/dto/mappers.go": '''package dto
type User struct {
    ID int64 `json:"id"`
    Note *string `json:"note,omitempty"`
}
func UserFromService(input any) *User { return nil }
''',
            "backend/internal/service/types.go": '''package service
type User struct { Secret string `json:"secret"` }
''',
        }
        symbols = build_go_symbol_index(files)
        analysis = analyze_body(
            '''out := dto.UserFromService(input)
response.Success(c, out)''',
            {}, response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        schema = analysis["typed_response_schemas"]["200"]
        data = schema["properties"]["data"]
        user = data["anyOf"][0]
        self.assertEqual(user["x-clomapi-struct"], "User")
        self.assertIn("handler/dto", user["x-clomapi-qualified-scope"])
        self.assertEqual(user["required"], ["id"])
        self.assertNotIn("secret", user["properties"])
        self.assertEqual(user["properties"]["note"]["anyOf"][1], {"type": "null"})

    def test_response_converter_ambiguity_fails_closed(self):
        files = {
            "backend/internal/handler/list.go": '''package handler
type Handler struct{}
func convert(input any) Result { return Result{} }
func convert(input string) Result { return Result{} }
type Result struct { ID int64 `json:"id"` }
''',
        }
        symbols = build_go_symbol_index(files)
        analysis = analyze_body(
            '''out := convert(input)
response.Success(c, out)''',
            {}, response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertEqual(analysis["typed_response_schemas"], {})
        self.assertIsNone(analysis["typed_response_schema"])

        dynamic_map_files = {
            "backend/internal/handler/list.go": '''package handler
type Handler struct{}
func dynamic(input any) map[string]any { return nil }
''',
        }
        dynamic_symbols = build_go_symbol_index(dynamic_map_files)
        dynamic = analyze_body(
            'response.Success(c, dynamic(input))', {},
            response_symbol_index=dynamic_symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertEqual(dynamic["typed_response_schemas"], {})

    def test_legacy_response_schema_never_unblocks_without_qualified_evidence(self):
        handler_file = "backend/internal/handler/items.go"
        handler_files = {
            handler_file: '''package handler
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct{}
'''
        }
        proof = {
            "response_symbol_index": build_go_symbol_index(handler_files),
            "current_file": handler_file,
            "current_recv_type": "Handler",
        }
        complete = {
            "name": "CompleteItem",
            "evidence": "backend/internal/handler/items.go:10",
            "fields": [
                {
                    "name": "ID", "type": "int64", "json_name": "id",
                    "required": True, "embedded": False,
                },
            ],
        }
        incomplete = {
            "name": "IncompleteItem",
            "evidence": "backend/internal/handler/items.go:20",
            "fields": [
                {
                    "name": "Payload", "type": "json.RawMessage",
                    "json_name": "payload", "required": True,
                    "embedded": False,
                },
            ],
        }

        complete_analysis = analyze_body(
            "items := make([]CompleteItem, 0)\nresponse.Paginated(c, items, 0, 1, 20)",
            {"CompleteItem": complete},
            **proof,
        )
        self.assertEqual(complete_analysis["typed_response_struct"], "CompleteItem")
        self.assertIsNone(complete_analysis["typed_response_schema"])

        incomplete_analysis = analyze_body(
            "items := make([]IncompleteItem, 0)\nresponse.Paginated(c, items, 0, 1, 20)",
            {"IncompleteItem": incomplete},
            **proof,
        )
        self.assertIsNone(incomplete_analysis["typed_response_schema"])
        self.assertEqual(incomplete_analysis["typed_response_schemas"], {})

    def test_response_schema_completeness_is_keyword_aware_and_fail_closed(self):
        opaque = {"type": "object", "x-clomapi-go-type": "Unknown"}
        rejected = [
            None,
            {},
            {"$ref": ""},
            {"type": "object"},
            {"type": "object", "properties": {}, "additionalProperties": True},
            {"type": "array"},
            {"oneOf": []},
            {"oneOf": [{"type": "string"}, opaque]},
            {"oneOf": [{"type": "string"}], "additionalProperties": True},
            {"type": ["object", "null"]},
        ]
        for schema in rejected:
            with self.subTest(schema=schema):
                self.assertFalse(_response_schema_is_complete(schema))

        accepted = [
            {"type": "object", "properties": {}, "additionalProperties": False},
            {
                "type": "object",
                "additionalProperties": {"type": "string"},
            },
            {"type": "array", "items": {"type": "integer"}},
            {"anyOf": [{"type": "string"}, {"type": "null"}]},
            {"$ref": "#/components/schemas/Complete"},
        ]
        for schema in accepted:
            with self.subTest(schema=schema):
                self.assertTrue(_response_schema_is_complete(schema))

    def test_complete_status_schema_replaces_incomplete_legacy_singular_schema(self):
        files = {
            "backend/internal/handler/items.go": '''package handler
import "github.com/Wei-Shaw/sub2api/internal/handler/dto"
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct{}
''',
            "backend/internal/handler/dto/items.go": '''package dto
type Item struct { ID int64 `json:"id"` }
''',
        }
        legacy_item = {
            "name": "Item",
            "evidence": "backend/internal/handler/items.go:20",
            "fields": [
                {
                    "name": "Payload", "type": "json.RawMessage",
                    "json_name": "payload", "required": True,
                    "embedded": False,
                },
            ],
        }
        analysis = analyze_body(
            "items := make([]dto.Item, 0)\nresponse.Paginated(c, items, 0, 1, 20)",
            {"Item": legacy_item},
            response_symbol_index=build_go_symbol_index(files),
            current_file="backend/internal/handler/items.go",
            current_recv_type="Handler",
        )
        status_schema = analysis["typed_response_schemas"]["200"]
        self.assertEqual(analysis["typed_response_schema"], status_schema)
        item_schema = status_schema["properties"]["data"]["properties"]["items"]["items"]
        self.assertEqual(sorted(item_schema["properties"]), ["id"])

    def test_response_call_suffix_fails_closed_instead_of_using_whole_return(self):
        files = {
            "backend/internal/handler/items.go": '''package handler
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct{}
type Result struct { Primary string `json:"primary"` }
func Make() Result { return Result{} }
''',
        }
        symbols = build_go_symbol_index(files)
        exact = analyze_body(
            "response.Success(c, Make())", {}, response_symbol_index=symbols,
            current_file="backend/internal/handler/items.go", current_recv_type="Handler",
        )
        self.assertIn("200", exact["typed_response_schemas"])

        for body in (
            "response.Success(c, Make().Primary)",
            "out := Make().Primary\nresponse.Success(c, out)",
        ):
            with self.subTest(body=body):
                suffixed = analyze_body(
                    body, {}, response_symbol_index=symbols,
                    current_file="backend/internal/handler/items.go",
                    current_recv_type="Handler",
                )
                self.assertEqual(suffixed["typed_response_schemas"], {})
                self.assertIsNone(suffixed["typed_response_schema"])

    def test_direct_helper_receiver_mutations_fail_closed(self):
        files = {
            "backend/internal/handler/list.go": '''package handler
import "github.com/Wei-Shaw/sub2api/internal/handler/dto"
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct{}
type Converter struct{}
type Other struct{}
func (o *Converter) Convert() dto.Item { return dto.Item{} }
''',
            "backend/internal/handler/dto/types.go": '''package dto
type Item struct { ID int64 }
func Convert(input any) Item { return Item{} }
''',
            "backend/internal/other/dto/types.go": '''package dto
type Item struct { Wrong string }
func Convert(input any) Item { return Item{} }
''',
        }
        symbols = build_go_symbol_index(files)
        exact = analyze_body(
            "response.Success(c, dto.Convert(input))", {},
            response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertIn("200", exact["typed_response_schemas"])

        wrong_receiver = analyze_body(
            "other := &Other{}\nresponse.Success(c, other.Convert())", {},
            response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertEqual(wrong_receiver["typed_response_schemas"], {})

        shadowed_import = analyze_body(
            "dto := &Other{}\nresponse.Success(c, dto.Convert())", {},
            response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertEqual(shadowed_import["typed_response_schemas"], {})

        shadowed_function = dict(files)
        shadowed_function["backend/internal/handler/list.go"] += '''
type LocalItem struct { Local string }
func Convert() dto.Item { return dto.Item{} }
'''
        local_shadow = analyze_body(
            "Convert := func() LocalItem { return LocalItem{} }\n"
            "response.Success(c, Convert())", {},
            response_symbol_index=build_go_symbol_index(shadowed_function),
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertEqual(local_shadow["typed_response_schemas"], {})

        overloaded = dict(files)
        overloaded["backend/internal/handler/dto/ambiguous.go"] = '''package dto
func Convert(input string) Item { return Item{} }
'''
        ambiguous = analyze_body(
            "response.Success(c, dto.Convert(input))", {},
            response_symbol_index=build_go_symbol_index(overloaded),
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertEqual(ambiguous["typed_response_schemas"], {})

        reassigned = dict(files)
        reassigned["backend/internal/handler/list.go"] += '''
func NewConverter() *Converter { return &Converter{} }
'''
        ambiguous_receiver = analyze_body(
            "converter := NewConverter()\nif flag { converter = &Other{} }\n"
            "response.Success(c, converter.Convert())", {},
            response_symbol_index=build_go_symbol_index(reassigned),
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertEqual(ambiguous_receiver["typed_response_schemas"], {})

        branch_value = analyze_body(
            "var out any\n"
            "if flag { out = dto.Convert(input) } else { out = &Other{} }\n"
            "response.Success(c, out)", {},
            response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertEqual(branch_value["typed_response_schemas"], {})

    def test_direct_helper_recursive_alias_embedded_and_wire_guards(self):
        files = {
            "backend/internal/handler/list.go": '''package handler
import "github.com/Wei-Shaw/sub2api/internal/handler/dto"
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct{}
''',
            "backend/internal/handler/dto/types.go": '''package dto
type ID = string
type Base struct { ID ID }
type Item struct {
    *Base
    Note *ID
}
func Convert(input any) *Item { return nil }
''',
        }
        symbols = build_go_symbol_index(files)
        complete = analyze_body(
            "response.Success(c, dto.Convert(input))", {},
            response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        data = complete["typed_response_schemas"]["200"]["properties"]["data"]["anyOf"][0]
        self.assertEqual(sorted(data["properties"]), ["ID", "Note"])
        self.assertEqual(data["properties"]["ID"]["type"], "string")

        mutations = {
            "unresolved_nested": '''package dto
type Item struct { Nested Missing }
func Convert(input any) *Item { return nil }
''',
            "custom_marshaler": '''package dto
type Item struct { ID int64 }
func (*Item) MarshalJSON() ([]byte, error) { return nil, nil }
func Convert(input any) *Item { return nil }
''',
            "cycle": '''package dto
type Item struct { Next *Item }
func Convert(input any) *Item { return nil }
''',
        }
        for name, source in mutations.items():
            with self.subTest(name=name):
                mutated = dict(files)
                mutated["backend/internal/handler/dto/types.go"] = source
                result = analyze_body(
                    "response.Success(c, dto.Convert(input))", {},
                    response_symbol_index=build_go_symbol_index(mutated),
                    current_file="backend/internal/handler/list.go",
                    current_recv_type="Handler",
                )
                self.assertEqual(result["typed_response_schemas"], {})

    def test_lexical_scope_is_preserved_for_assigned_and_receiver_calls(self):
        files = {
            "backend/internal/handler/list.go": '''package handler
import "github.com/Wei-Shaw/sub2api/internal/handler/dto"
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct { service *Service }
type Service struct{}
type Other struct{}
func (s *Service) Convert() dto.Item { return dto.Item{} }
func Same() dto.Item { return dto.Item{} }
func Different() dto.Other { return dto.Other{} }
''',
            "backend/internal/handler/dto/types.go": '''package dto
type Item struct { ID int64 `json:"id"` }
type Other struct { Name string `json:"name"` }
func Convert() Item { return Item{} }
''',
        }
        symbols = build_go_symbol_index(files)
        common = {
            "response_symbol_index": symbols,
            "current_file": "backend/internal/handler/list.go",
            "current_recv_type": "Handler",
        }
        same_writes = analyze_body(
            '''out := Same()
if flag { out = Same() }
response.Success(c, out)''',
            {}, **common,
        )
        self.assertIn("200", same_writes["typed_response_schemas"])

        for body in (
            '''out := Same()
if flag { out = Different() }
response.Success(c, out)''',
            '''dto := &Other{}
out := dto.Convert()
response.Success(c, out)''',
            '''h := &Other{}
response.Success(c, h.service.Convert())''',
            '''var out any
if flag { out = Same() } else { out = Different() }
response.Success(c, out)''',
        ):
            with self.subTest(body=body):
                result = analyze_body(body, {}, **common)
                self.assertEqual(result["typed_response_schemas"], {})

    def test_go_code_mask_blocks_route_response_status_and_bind_fabrication(self):
        source = '''package routes
// r.GET("/comment", h.Comment)
const fake = "r.POST(\\"/string\\", h.String)"
const raw = `r.DELETE("/raw", h.Raw)`
func RegisterAdminRoutes(r *gin.RouterGroup) {
    r.GET("/real", h.Real)
}
'''
        routes = extract_routes_from_file("routes.go", source, "/api/v1")
        self.assertEqual(
            [(item["method"], item["path"]) for item in routes],
            [("GET", "/api/v1/real")],
        )

        files = {
            "backend/internal/handler/list.go": '''package handler
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct{}
type Item struct { UserID string }
''',
        }
        body = '''// response.Created(c, Item{UserID: "comment"})
_ = "response.Accepted(c, Item{UserID: \\"string\\"})"
_ = `response.Success(c, Item{UserID: "raw"})`
response.Success(c, Item{UserID: "real"})
// c.JSON(299, nil)
'''
        analysis = analyze_body(
            body, {}, response_symbol_index=build_go_symbol_index(files),
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertEqual(analysis["status_codes"], [200])
        self.assertEqual(sorted(analysis["typed_response_schemas"]), ["200"])
        self.assertEqual(
            list(analysis["typed_response_schemas"]["200"]["properties"]["data"]["properties"]),
            ["UserID"],
        )
        fabricated_bind = analyze_body(
            '// c.ShouldBindJSON(&req)\n_ = "c.ShouldBindJSON(&req)"',
            {}, response_symbol_index=build_go_symbol_index(files),
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertIsNone(fabricated_bind["request_body_struct"])

    def test_request_dto_resolution_is_qualified_recursive_and_json_correct(self):
        files = {
            "backend/internal/handler/create.go": '''package handler
import request "github.com/Wei-Shaw/sub2api/internal/requesta"
import "github.com/gin-gonic/gin"
type Handler struct{}
''',
            "backend/internal/requesta/types.go": '''package requesta
type Embedded struct {
    EmbeddedID string `binding:"required"`
}
type OptionalEmbedded struct {
    OptionalID string `binding:"required"`
}
type Nested struct { Value string `json:"value"` }
type Payload struct {
    Embedded
    *OptionalEmbedded
    UserID string
    private string
    Display string `json:"display,omitempty"`
    Ignored string `json:"-"`
    Nested Nested `json:"nested" binding:"required"`
}
''',
            "backend/internal/requestb/types.go": '''package requestb
type Payload struct { Wrong string `json:"wrong"` }
''',
        }
        symbols = build_go_symbol_index(files)
        analysis = analyze_body(
            'var req request.Payload\nc.ShouldBindJSON(&req)',
            {}, response_symbol_index=symbols,
            current_file="backend/internal/handler/create.go",
            current_recv_type="Handler",
        )
        schema = analysis["request_body_schema"]
        self.assertEqual(
            sorted(schema["properties"]),
            ["EmbeddedID", "OptionalID", "UserID", "display", "nested"],
        )
        self.assertNotIn("private", schema["properties"])
        self.assertNotIn("Ignored", schema["properties"])
        self.assertEqual(schema["required"], ["EmbeddedID", "nested"])
        self.assertNotIn("wrong", schema["properties"])

        mutations = {
            "missing_import": 'var req ghost.Payload\nc.ShouldBindJSON(&req)',
            "missing_nested": 'var req request.MissingPayload\nc.ShouldBindJSON(&req)',
        }
        mutated_files = dict(files)
        mutated_files["backend/internal/requesta/unsafe.go"] = '''package requesta
type MissingPayload struct { Nested Missing `json:"nested"` }
type AnyPayload struct { Value any `json:"value"` }
type CyclePayload struct { Next *CyclePayload `json:"next"` }
type MarshaledPayload struct { Value string `json:"value"` }
func (*MarshaledPayload) MarshalJSON() ([]byte, error) { return nil, nil }
'''
        mutated_symbols = build_go_symbol_index(mutated_files)
        mutations.update({
            "any": 'var req request.AnyPayload\nc.ShouldBindJSON(&req)',
            "cycle": 'var req request.CyclePayload\nc.ShouldBindJSON(&req)',
            "custom": 'var req request.MarshaledPayload\nc.ShouldBindJSON(&req)',
        })
        for name, body in mutations.items():
            with self.subTest(name=name):
                result = analyze_body(
                    body, {}, response_symbol_index=mutated_symbols,
                    current_file="backend/internal/handler/create.go",
                    current_recv_type="Handler",
                )
                self.assertTrue(result["request_body_unresolved_struct"])
                self.assertIsNone(result["request_body_schema"])

    def test_go_return_clause_parser_rejects_unsupported_grammar(self):
        accepted = {
            "Result": ["Result"],
            "*dto.Result": ["*dto.Result"],
            "(Result, error)": ["Result", "error"],
            "(result *dto.Result, err error)": ["*dto.Result", "error"],
            "[]dto.Result": ["[]dto.Result"],
            "map[string]dto.Result": ["map[string]dto.Result"],
        }
        for clause, expected in accepted.items():
            with self.subTest(clause=clause):
                self.assertEqual(_go_result_types(clause), (expected, "parsed"))
        for clause in (
            "chan Result", "<-chan Result", "chan<- Result",
            "(a, b Result)", "(result Result, error)", "...Result",
            "Result[T]", "[2]Result", "func() Result", "struct{ X int }",
            "interface{ M() }",
        ):
            with self.subTest(clause=clause):
                self.assertEqual(
                    _go_result_types(clause),
                    ([], "unparsed_return_clause"),
                )

    def test_unresolved_success_branch_cannot_use_message_updated_escape(self):
        files = {
            "backend/internal/handler/update.go": '''package handler
type Handler struct{}
func unknown() any { return nil }
''',
        }
        result = analyze_body(
            '''if flag {
    response.Success(c, gin.H{"message": "updated"})
    return
}
response.Success(c, unknown())''',
            {}, response_symbol_index=build_go_symbol_index(files),
            current_file="backend/internal/handler/update.go",
            current_recv_type="Handler",
        )
        self.assertEqual(result["typed_response_schemas"], {})
        self.assertIsNone(result["typed_response_schema"])
        self.assertNotEqual(result.get("response_schema_hint"), "message_updated")

    def test_response_converter_return_signature_mutation_changes_schema(self):
        files = {
            "backend/internal/handler/list.go": '''package handler
import "github.com/Wei-Shaw/sub2api/internal/handler/dto"
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct{}
''',
            "backend/internal/handler/dto/mappers.go": '''package dto
import "github.com/Wei-Shaw/sub2api/internal/service"
func Convert(input any) *service.Result { return nil }
''',
            "backend/internal/service/types.go": '''package service
type Result struct {
    Value string `json:"value"`
}
''',
        }
        symbols = build_go_symbol_index(files)
        analysis = analyze_body(
            'response.Success(c, dto.Convert(input))', {},
            response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        data = analysis["typed_response_schemas"]["200"]["properties"]["data"]
        self.assertEqual(data["anyOf"][0]["properties"]["value"]["type"], "string")
        self.assertIn("internal/service", data["anyOf"][0]["x-clomapi-qualified-scope"])

    def test_response_local_slice_pagination_and_created_status(self):
        files = {
            "backend/internal/handler/list.go": '''package handler
import "github.com/Wei-Shaw/sub2api/internal/handler/dto"
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct{}
''',
            "backend/internal/handler/dto/types.go": '''package dto
type Item struct { ID int64 `json:"id"` }
''',
        }
        symbols = build_go_symbol_index(files)
        paginated = analyze_body(
            '''items := make([]dto.Item, 0)
response.Paginated(c, items, int64(0), 1, 20)''',
            {}, response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        items = paginated["typed_response_schemas"]["200"]["properties"]["data"]["properties"]["items"]
        self.assertEqual(items["type"], "array")
        self.assertEqual(items["items"]["x-clomapi-struct"], "Item")

        created = analyze_body(
            'response.Created(c, dto.Item{ID: 1})',
            {}, response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertEqual(sorted(created["typed_response_schemas"]), ["201"])
        self.assertEqual(
            created["typed_response_schemas"]["201"]["properties"]["data"]["x-clomapi-struct"],
            "Item",
        )

    def test_inline_gin_map_requires_every_value_and_preserves_variants(self):
        files = {
            "backend/internal/handler/list.go": '''package handler
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct{}
func unknown() any { return nil }
''',
        }
        symbols = build_go_symbol_index(files)
        typed = analyze_body(
            '''items := []string{"a"}
if len(items) > 0 {
    response.Success(c, gin.H{"state": "ready", "count": len(items), "nested": gin.H{"ok": true}})
    return
}
response.Success(c, gin.H{"state": "empty", "count": 0, "nested": gin.H{"ok": false}})''',
            {}, response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        variants = typed["typed_response_schemas"]["200"]["oneOf"]
        self.assertEqual(len(variants), 2)
        first_data = variants[0]["properties"]["data"]
        self.assertEqual(first_data["properties"]["count"]["type"], "integer")
        self.assertEqual(
            first_data["properties"]["nested"]["properties"]["ok"]["type"],
            "boolean",
        )

        dynamic = analyze_body(
            'response.Success(c, gin.H{"value": unknown()})', {},
            response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertEqual(dynamic["typed_response_schemas"], {})

    def test_service_receiver_tuple_selector_index_and_interface_resolution(self):
        files = {
            "backend/internal/handler/list.go": '''package handler
import "github.com/Wei-Shaw/sub2api/internal/service"
import "github.com/Wei-Shaw/sub2api/internal/pkg/response"
import "github.com/gin-gonic/gin"
type Handler struct {
    service *service.Service
    reader service.Reader
}
''',
            "backend/internal/service/service.go": '''package service
type Item struct {
    ID int64 `json:"id"`
}
type Result struct {
    Items []Item `json:"items"`
    Primary *Item `json:"primary"`
}
type Service struct{}
func (s *Service) List() (Result, error) { return Result{}, nil }
type Reader interface {
    Fetch() (*Result, error)
}
''',
        }
        symbols = build_go_symbol_index(files)
        selected = analyze_body(
            '''result, err := h.service.List()
_ = err
response.Success(c, result.Primary)''', {},
            response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        primary = selected["typed_response_schemas"]["200"]["properties"]["data"]
        self.assertEqual(primary["anyOf"][0]["x-clomapi-struct"], "Item")

        paginated = analyze_body(
            '''result, err := h.service.List()
_ = err
response.Paginated(c, result.Items, int64(0), 1, 20)''', {},
            response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        item = paginated["typed_response_schemas"]["200"]["properties"]["data"]["properties"]["items"]["items"]
        self.assertEqual(item["x-clomapi-struct"], "Item")

        interfaced = analyze_body(
            '''result, err := h.reader.Fetch()
_ = err
response.Success(c, result)''', {},
            response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        interface_data = interfaced["typed_response_schemas"]["200"]["properties"]["data"]
        self.assertEqual(interface_data["anyOf"][0]["x-clomapi-struct"], "Result")

        wrong_slot = analyze_body(
            '''result, err := h.service.List()
_ = result
response.Success(c, err)''', {},
            response_symbol_index=symbols,
            current_file="backend/internal/handler/list.go",
            current_recv_type="Handler",
        )
        self.assertEqual(wrong_slot["typed_response_schemas"], {})

    def test_status_specific_schema_does_not_leak_to_other_success_status(self):
        schema = {
            "type": "object",
            "required": ["code", "message"],
            "properties": {"code": {"const": 0}, "message": {"type": "string"}},
        }
        route = {
            "registration_id": "POST:/api/v1/test:synthetic:1",
            "method": "POST", "path": "/api/v1/test", "oas_path": "/api/v1/test",
            "evidence": "synthetic:1", "group_chain": [], "middlewares": [],
            "handler_ref": "synthetic.Create",
            "analysis": {
                "recv_type": "Handler", "method": "Create", "evidence": ["synthetic:2"],
                "request_body_schema": None, "path_params_used": [],
                "query_params_used": [], "query_param_contracts": [],
                "query_semantic_type_unresolved": [], "headers_used": [],
                "status_codes": [200, 201], "media_types": ["application/json"],
                "typed_response_schema": schema,
                "typed_response_schemas": {"201": schema},
                "binary": False, "stream_sse": False, "websocket": False,
                "blocking_unresolved": False,
            },
        }
        operation, _ = build_operation(route)
        self.assertTrue(operation["responses"]["200"]["x-clomapi-inventoried-blocking"])
        self.assertNotIn("x-clomapi-inventoried-blocking", operation["responses"]["201"])
        self.assertIn("untyped_success_response", operation["x-clomapi-blocking-reasons"])

    def test_semantic_golden_payment_config(self):
        g = json.loads((ROOT / "contracts/openapi/evidence/semantic-goldens.json").read_text())["put_admin_payment_config"]
        r = find_route(g["method"], g["path"])
        a = r["analysis"]
        self.assertEqual(a.get("recv_type"), g["expect_handler_recv"])
        self.assertEqual(a.get("request_body_struct"), g["expect_body_struct"])
        schema = a.get("request_body_schema") or {}
        props = schema.get("properties") or {}
        for f in g["expect_body_fields_contains"]:
            self.assertIn(f, props)
        self.assertEqual(a.get("response_schema_hint"), g["expect_response_hint"])
        op = find_oa_op(g["method"], g["path"])
        body = op["requestBody"]["content"]["application/json"]["schema"]
        self.assertEqual(body["x-clomapi-struct"], "UpdatePaymentConfigRequest")
        self.assertIn("enabled", body["properties"])
        # distinct from other /config routes
        r2 = find_route("PUT", "/api/v1/admin/risk-control/config")
        self.assertNotEqual(r["handler_ref"], r2["handler_ref"])

    def test_semantic_golden_login_register(self):
        goldens = json.loads((ROOT / "contracts/openapi/evidence/semantic-goldens.json").read_text())
        for key in ("post_auth_login", "post_auth_register"):
            g = goldens[key]
            r = find_route(g["method"], g["path"])
            a = r["analysis"]
            self.assertEqual(a.get("recv_type"), g["expect_handler_recv"])
            self.assertEqual(a.get("request_body_struct"), g["expect_body_struct"])
            props = (a.get("request_body_schema") or {}).get("properties") or {}
            for f in g["expect_body_fields_contains"]:
                self.assertIn(f, props)
            self.assertTrue(r.get("multiline"))
            self.assertTrue(r.get("middlewares"))
            op = find_oa_op(g["method"], g["path"])
            body = op["requestBody"]["content"]["application/json"]["schema"]
            self.assertEqual(body["x-clomapi-struct"], g["expect_body_struct"])
        # login typed AuthResponse
        r = find_route("POST", "/api/v1/auth/login")
        self.assertEqual(r["analysis"].get("typed_response_struct"), "AuthResponse")

    def test_no_empty_object_dishonest(self):
        text = (ROOT / "contracts/openapi/management-v1.yaml").read_text()
        self.assertNotIn("defer to P1", text)
        # Must not define EmptyObject schema component (description text may mention the anti-pattern)
        self.assertNotRegex(text, r"(?m)^\s*EmptyObject:")
        self.assertNotIn("#/components/schemas/EmptyObject", text)
        if "InventoriedUntypedBody" in text:
            self.assertIn("x-clomapi-inventoried-blocking", text)

    def test_path_params_required_true(self):
        oa = load_yaml("contracts/openapi/management-v1.yaml")
        n = 0
        for pth, item in (oa.get("paths") or {}).items():
            for method, op in item.items():
                if not isinstance(op, dict): continue
                for p in op.get("parameters") or []:
                    if p.get("in") == "path":
                        self.assertIs(p.get("required"), True)
                        n += 1
        self.assertGreater(n, 100)

    def test_aliases_exclude_primary(self):
        proto = load_yaml("docs/rebuild/inventory/protocols.yaml")
        for op in proto["operations"]:
            prim = op.get("primary") or {}
            for a in op.get("aliases") or []:
                self.assertFalse(
                    a.get("method") == prim.get("method") and a.get("path") == prim.get("path"),
                    f"primary listed as alias in {op['id']}",
                )
        # openai.responses should have aliases without primary
        resp = next(o for o in proto["operations"] if o["id"] == "op.openai.responses")
        self.assertEqual(resp["primary"]["path"], "/v1/responses")
        self.assertTrue(all(a["path"] != "/v1/responses" or a["method"] != "POST" for a in resp["aliases"]))
        self.assertGreater(resp["alias_count"], 0)

    def test_terminal_fixtures_complete(self):
        need = [
            "terminal_sse_anthropic_success.json",
            "terminal_sse_anthropic_error.json",
            "terminal_sse_openai_responses_success.json",
            "terminal_sse_openai_responses_error.json",
            "terminal_sse_openai_chat_done.json",
            "terminal_ws_openai_responses.json",
            "terminal_ws_openai_responses_error.json",
            "terminal_ws_openai_responses_cancel.json",
            "terminal_gemini_stream.json",
            "terminal_gemini_stream_error.json",
            "terminal_batch_async.json",
            "terminal_binary_download.json",
        ]
        for n in need:
            p = ROOT / "contracts/gateway/fixtures" / n
            self.assertTrue(p.is_file(), n)
            data = json.loads(p.read_text())
            self.assertIn("terminal_semantics_key", data)
            if data.get("kind") == "gateway_terminal_wire_fixture":
                for field in (
                    "schema_version", "fixture_id", "protocol", "scenario", "frames", "expected"
                ):
                    self.assertIn(field, data, f"{n} missing {field}")
                self.assertTrue(data["frames"], n)
                self.assertIn("failover_after_commit", data["expected"], n)
            else:
                self.assertIn(data["terminal_semantics_key"], {"batch_async", "binary"}, n)
                for field in ("success", "error", "eof", "cancel", "commit"):
                    self.assertIn(field, data, f"{n} missing {field}")
        # golden cc aux
        gdir = ROOT / "contracts/gateway/fixtures/golden"
        self.assertGreaterEqual(len(list(gdir.glob("*.json"))), 8)

    def test_protocols_terminal_on_stream_ops(self):
        proto = load_yaml("docs/rebuild/inventory/protocols.yaml")
        for op in proto["operations"]:
            modes = (op.get("streaming") or {}).get("modes") or []
            if any(m in modes for m in ("sse", "websocket")) or op["id"].startswith(
                ("op.openai.videos", "op.media.batch", "op.search", "op.cc.")
            ):
                self.assertIn("terminal_semantics", op, op["id"])
                ts = op["terminal_semantics"]
                for k in ("success", "error", "eof", "cancel", "commit", "failover"):
                    self.assertIn(k, ts, f"{op['id']} missing {k}")

    def test_clients_no_generics_versions_honest(self):
        clients = load_yaml("docs/rebuild/inventory/clients.yaml")
        ids = {c["id"] for c in clients["clients"]}
        for need in ["client.claude_code","client.codex_cli","client.gemini_cli","client.grok_cli",
                     "client.opencode","client.cc_switch"]:
            self.assertIn(need, ids)
        for banned in ["client.agents_generic","client.ide_generic","client.app_generic"]:
            self.assertNotIn(banned, ids)
        closed = {
            "client.claude_code": "@anthropic-ai/claude-code@2.1.216",
            "client.codex_cli": "@openai/codex@0.145.0",
            "client.gemini_cli": "@google/gemini-cli@0.51.0",
            "client.opencode": "anomalyco/opencode@v1.18.4",
            "client.cc_switch": "farion1231/cc-switch@v3.18.0",
            "client.anthropic_sdk": "@anthropic-ai/sdk@0.112.4",
        }
        for client_id, coordinate in closed.items():
            version = next(c for c in clients["clients"] if c["id"] == client_id)["tested_or_observed_versions"]["authoritative_version"]
            self.assertEqual(version["status"], "authoritative")
            self.assertEqual(version["package_coordinate"], coordinate)
            self.assertRegex(version["artifact_sha256"], r"^[0-9a-f]{64}$")
            provenance = version["provenance"]
            self.assertRegex(provenance["metadata_sha256"], r"^[0-9a-f]{64}$")
            self.assertTrue(provenance["metadata_path"].startswith(
                "contracts/gateway/evidence/client-versions/metadata/"
            ))
            self.assertNotIn("g0_blocker_ref", version)
        self.assertEqual(
            {row["client_id"] for row in clients["version_pin_blockers"]},
            {"client.grok_cli", "client.openai_sdk", "client.google_sdk"},
        )
        for c in clients["clients"]:
            if c["id"] == "client.web_spa":
                continue
            v = c.get("tested_or_observed_versions") or {}
            if c["id"] in closed:
                continue
            # must not fabricate version strings as if pinned
            if v.get("client_binary_version_status") == "unavailable" or v.get("sdk_version_status") == "unavailable":
                self.assertTrue(v.get("client_binary_version") in (None, "") or "status" in str(v))
                self.assertEqual(v.get("g0_blocker_ref"), "BLK-CLIENT-BINARY-VERSIONS")
        # cc_switch model constant is evidence but not app version
        ccs = next(c for c in clients["clients"] if c["id"] == "client.cc_switch")
        self.assertEqual(ccs["tested_or_observed_versions"].get("openai_model_constant"), "gpt-5.5")

    def test_workloads_blocking_not_oos(self):
        wl = load_yaml("docs/rebuild/inventory/workloads.yaml")
        self.assertEqual(wl["status"], "specified")  # catalog specified; samples unavailable
        self.assertTrue(wl.get("g0_blockers"))
        for w in wl["workloads"]:
            self.assertNotEqual(w["status"], "out_of_scope")
            ms = w["measured_samples"]
            self.assertEqual(ms["status"], "unavailable")
            self.assertTrue(ms.get("g0_blocker") or ms.get("blocking"))
            self.assertTrue(str(ms.get("unavailable_id","")).startswith("UNAV-"))

    def test_g0_blockers_file(self):
        blk = load_yaml("docs/rebuild/inventory/G0_BLOCKERS.yaml")
        ids = {b["id"] for b in blk["blockers"]}
        for need in ["BLK-UNAV-PROD-SLO-SAMPLES","BLK-CLIENT-BINARY-VERSIONS","BLK-HANDLER-UNRESOLVED"]:
            self.assertIn(need, ids)
            b = next(x for x in blk["blockers"] if x["id"] == need)
            self.assertTrue(b.get("blocking") or b.get("severity"))

    def test_no_api_v2(self):
        oa = load_yaml("contracts/openapi/management-v1.yaml")
        for s in oa.get("servers") or []:
            self.assertNotIn("/api/v2", s.get("url",""))

    def test_gateway_coverage(self):
        inv = json.loads((ROOT / "contracts/openapi/evidence/extraction/routes.json").read_text())
        proto = load_yaml("docs/rebuild/inventory/protocols.yaml")
        inv_set = {(r["method"], r["path"]) for r in inv if r["surface"] in ("gateway","common_or_auxiliary","ops_public")}
        cov = set()
        for op in proto["operations"]:
            for a in op.get("all_paths") or ([op.get("primary")] + (op.get("aliases") or [])):
                if a:
                    cov.add((a["method"], a["path"]))
        self.assertEqual(inv_set, cov)

    def test_handler_resolution_receiver_typed(self):
        r = find_route("GET", "/api/v1/admin/users")
        self.assertEqual(r["analysis"]["recv_type"], "UserHandler")
        self.assertTrue(r["analysis"]["resolved"])
        self.assertFalse(r["analysis"]["ambiguous"])
        # multi-mounted :id distinct
        u = find_route("GET", "/api/v1/admin/users/:id")
        self.assertEqual(u["analysis"]["recv_type"], "UserHandler")
        self.assertEqual(u["analysis"]["method"], "GetByID")

    def test_payment_invoice_group_reuse_is_lexically_scoped(self):
        expected = {
            ("GET", "/api/v1/payment/invoices"),
            ("POST", "/api/v1/payment/invoices"),
            ("GET", "/api/v1/payment/invoices/:id"),
            ("POST", "/api/v1/payment/invoices/:id/cancel"),
            ("GET", "/api/v1/payment/invoices/:id/download"),
        }
        routes = json.loads((ROOT / "contracts/openapi/evidence/extraction/routes.json").read_text())
        keys = {(r["method"], r["path"]) for r in routes}
        self.assertTrue(expected <= keys)
        self.assertNotIn(("POST", "/api/v1/admin/payment/invoices"), keys)
        self.assertNotIn(("GET", "/api/v1/admin/payment/invoices/:id/download"), keys)
        management = [r for r in routes if r["path"].startswith("/api/v1")]
        self.assertEqual(len(management), 614)
        self.assertEqual(len({(r["method"], r["path"]) for r in management}), 614)
        for method, path in expected:
            op = find_oa_op(method, path)
            self.assertEqual(op["x-clomapi-auth-mode"], "mgmt_dual")
            self.assertEqual(len(op["security"]), 2)
        with self.assertRaisesRegex(AssertionError, "missing openapi op"):
            find_oa_op("POST", "/api/v1/admin/payment/invoices")
        with self.assertRaisesRegex(AssertionError, "missing openapi op"):
            find_oa_op("GET", "/api/v1/admin/payment/invoices/:id/download")

    def test_group_auth_and_public_binary_semantics(self):
        bind = find_route("POST", "/api/v1/auth/oauth/bind-token")
        self.assertTrue(any("jwtAuth" in m for m in bind["middlewares"]))
        image = find_route("GET", "/api/v1/pages/:slug/images/*filename")
        self.assertFalse(any("jwtAuth" in m or "adminAuth" in m for m in image["middlewares"]))
        self.assertIn(200, image["analysis"]["status_codes"])
        self.assertIn(404, image["analysis"]["status_codes"])
        self.assertIn("application/octet-stream", image["analysis"]["media_types"])

        bind_op = find_oa_op("POST", "/api/v1/auth/oauth/bind-token")
        self.assertEqual(bind_op["x-clomapi-auth-mode"], "mgmt_dual")
        self.assertEqual(
            bind_op["security"],
            [{"legacyBearer": []}, {"browserAccessBearer": []}],
        )
        self.assertEqual(set(bind_op["responses"]), {"204"})
        self.assertNotIn("content", bind_op["responses"]["204"])

        image_op = find_oa_op("GET", "/api/v1/pages/:slug/images/*filename")
        self.assertEqual(image_op["x-clomapi-auth-mode"], "public_none")
        self.assertEqual(image_op["security"], [])
        binary = image_op["responses"]["200"]
        self.assertNotIn("content", binary)
        self.assertTrue(binary["x-clomapi-inventoried-blocking"])
        self.assertEqual(binary["x-clomapi-response-kind"], "opaque_binary")
        self.assertEqual(binary["x-clomapi-response-media-status"], "unresolved")
        error = image_op["responses"]["404"]
        self.assertNotIn("content", error)
        self.assertEqual(error["x-clomapi-error-schema-status"], "unresolved")

    def test_management_binding_is_one_to_one_and_hash_bound(self):
        routes, openapi, index = binding_documents()
        self.assertEqual(management_binding_errors(routes, openapi, index), [])
        self.assertEqual(openapi["x-clomapi-management-operation-count"], 614)
        self.assertEqual(
            openapi["x-clomapi-counts"],
            {
                "operations_in_document": 616,
                "management_api_v1": 614,
                "typed_management_successes": 314,
                "blocking_management_operations": 299,
                "blocking_document_operations": 301,
                "component_schemas": 212,
                "total_registrations": 685,
            },
        )
        self.assertEqual(index["count"], 614)
        self.assertEqual(len(index["operations"]), 614)
        for row in index["operations"]:
            self.assertRegex(row["semantic_sha256"], r"^[0-9a-f]{64}$")
            self.assertRegex(row["operation_sha256"], r"^[0-9a-f]{64}$")

    def test_management_routes_tsv_is_current_authoritative_projection(self):
        index = json.loads(
            (ROOT / "contracts/openapi/evidence/management-route-index.json").read_text()
        )
        lines = (ROOT / "contracts/openapi/evidence/management-routes.tsv").read_text().splitlines()
        self.assertEqual(lines[:3], [
            "# method path operation_id auth_profiles source",
            f"# baseline_id={BASELINE}",
            "# count=614",
        ])
        rows = [line.split("\t") for line in lines[3:] if line]
        self.assertEqual(len(rows), 614)
        self.assertEqual(
            [(row[0], row[1], row[2]) for row in rows],
            [(row["method"], row["path"], row["operation_id"]) for row in index["operations"]],
        )

    def test_stale_v1_route_inventory_is_retired(self):
        self.assertFalse(
            (ROOT / "contracts/openapi/evidence/route-inventory.json").exists()
        )
        retirement = load_yaml("docs/rebuild/inventory/RETIREMENT_NOTE.yaml")
        entry = next(
            item for item in retirement["retirements"]
            if item["id"] == "RET-ROUTE-INVENTORY-V1"
        )
        self.assertEqual(
            entry["replacement"],
            "contracts/openapi/evidence/mounted-route-inventory.json",
        )

    def test_management_binding_rejects_stale_embedded_counts(self):
        routes, baseline_openapi, index = binding_documents()
        for field in (
            "operations_in_document",
            "management_api_v1",
            "typed_management_successes",
            "blocking_management_operations",
        ):
            with self.subTest(field=field):
                openapi = copy.deepcopy(baseline_openapi)
                openapi["x-clomapi-counts"][field] = -1
                errors = management_binding_errors(routes, openapi, index)
                self.assertTrue(
                    any("OpenAPI x-clomapi-counts drift" in error for error in errors),
                    errors,
                )

    def test_management_binding_rejects_semantic_mutations(self):
        routes, baseline_openapi, index = binding_documents()

        mutations = [
            (
                "auth",
                "POST",
                "/api/v1/auth/oauth/bind-token",
                lambda op: op.update({"security": []}),
                "security",
            ),
            (
                "status",
                "POST",
                "/api/v1/auth/oauth/bind-token",
                lambda op: op["responses"].update({"200": {"description": "drift"}}),
                "responses",
            ),
            (
                "media",
                "GET",
                "/api/v1/pages/:slug/images/*filename",
                lambda op: op["responses"]["200"].update(
                    {"x-clomapi-response-kind": "drifted_binary"}
                ),
                "responses",
            ),
            (
                "body",
                "POST",
                "/api/v1/auth/login",
                lambda op: op["requestBody"]["content"]["application/json"]["schema"]["properties"].pop("password"),
                "requestBody",
            ),
            (
                "handler",
                "GET",
                "/api/v1/admin/users",
                lambda op: op.update({"x-clomapi-handler-ref": "h.Admin.User.Drifted"}),
                "x-clomapi-handler-ref",
            ),
            (
                "query",
                "GET",
                "/api/v1/admin/users",
                lambda op: next(
                    item for item in op["parameters"]
                    if item.get("in") == "query" and item.get("name") == "page"
                )["schema"].update({"type": "string"}),
                "parameters",
            ),
        ]
        for name, method, path, mutate, changed_field in mutations:
            with self.subTest(name=name):
                openapi = copy.deepcopy(baseline_openapi)
                mutate(mutable_oa_op(openapi, method, path))
                errors = management_binding_errors(routes, openapi, index)
                self.assertTrue(
                    any("OpenAPI operation hash mismatch" in error for error in errors),
                    errors,
                )
                self.assertTrue(
                    any(
                        "OpenAPI operation semantic drift" in error and changed_field in error
                        for error in errors
                    ),
                    errors,
                )

        drifted_index = copy.deepcopy(index)
        null_body_row = next(
            row for row in drifted_index["operations"]
            if row["method"] == "GET" and row["path"] == "/api/v1/admin/users"
        )
        self.assertIsNone(null_body_row.pop("request_body_schema"))
        errors = management_binding_errors(routes, baseline_openapi, drifted_index)
        self.assertTrue(
            any(
                "management index semantic fields missing GET /api/v1/admin/users" in error
                and "request_body_schema" in error
                for error in errors
            ),
            errors,
        )

    def test_schemas_draft_2020_12(self):
        for rel, schema_rel in [
            ("docs/rebuild/inventory/protocols.yaml", "docs/rebuild/schemas/protocols.schema.json"),
            ("docs/rebuild/inventory/clients.yaml", "docs/rebuild/schemas/clients.schema.json"),
            ("docs/rebuild/inventory/workloads.yaml", "docs/rebuild/schemas/workloads.schema.json"),
        ]:
            if not (ROOT / schema_rel).exists():
                continue
            doc = load_yaml(rel)
            schema = json.loads((ROOT / schema_rel).read_text())
            Draft202012Validator.check_schema(schema)
            Draft202012Validator(schema).validate(doc)

if __name__ == "__main__":
    suite = unittest.defaultTestLoader.loadTestsFromTestCase(TestP003P004)
    result = unittest.TextTestRunner(verbosity=2).run(suite)
    sys.exit(0 if result.wasSuccessful() else 1)
