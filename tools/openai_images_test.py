from __future__ import annotations

import base64
import json
import mimetypes
import os
import pathlib
import time
import urllib.parse
import urllib.request
import uuid
from dataclasses import dataclass, asdict
from typing import Any

try:
    import requests
except ImportError:  # pragma: no cover - runtime dependency guard
    requests = None


DEFAULT_BASE_URL_ENV_VAR = "SUB2API_BASE_URL"
DEFAULT_API_KEY_ENV_VAR = "SUB2API_API_KEY"
DEFAULT_BASE_URL = None
DEFAULT_API_KEY = os.getenv(DEFAULT_API_KEY_ENV_VAR)
DEFAULT_TIMEOUT = 300
DEFAULT_OUTPUT_ROOT = pathlib.Path("tmp/openai-image-tests")
MAX_IMAGE_EDGE = 3840
MIN_IMAGE_PIXELS = 655360
MAX_IMAGE_PIXELS = 8294400
MAX_IMAGE_RATIO = 3
MAX_IMAGE_N = 10
MAX_PARTIAL_IMAGES = 3
MAX_OUTPUT_COMPRESSION = 100
MAX_UPLOAD_BYTES = 50 << 20
ALLOWED_RESPONSE_FORMATS = {"b64_json"}
OFFICIAL_SIZE_VALUES = ("auto", "1024x1024", "1536x1024", "1024x1536")
GATEWAY_COMPATIBLE_SIZE_EXAMPLES = (
    "1792x1024",
    "1024x1792",
    "2048x2048",
    "2048x1152",
    "1152x2048",
    "3840x2160",
    "2160x3840",
)


@dataclass
class ImageArtifact:
    index: int
    saved_path: str | None
    base64_saved_path: str | None
    url_saved_path: str | None
    source_type: str
    source_value: str
    revised_prompt: str | None
    output_format: str | None


@dataclass
class CallResult:
    endpoint: str
    url: str
    request_content_type: str
    status_code: int
    ok: bool
    response_content_type: str
    output_dir: str
    payload: dict[str, Any]
    response_json: dict[str, Any] | None
    response_text: str | None
    artifacts: list[ImageArtifact]


def require_requests() -> None:
    if requests is None:
        raise RuntimeError("This script requires the requests package: python3 -m pip install requests")


def normalize_base_url(base_url: str) -> str:
    return base_url.rstrip("/")


def resolve_runtime_config(base_url: str | None = None, api_key: str | None = None) -> tuple[str, str]:
    resolved_base_url = normalize_base_url(base_url or os.getenv(DEFAULT_BASE_URL_ENV_VAR) or "")
    if not resolved_base_url:
        raise ValueError(
            f"base_url is required; set {DEFAULT_BASE_URL_ENV_VAR} or pass base_url explicitly"
        )
    resolved_api_key = api_key or os.getenv(DEFAULT_API_KEY_ENV_VAR) or DEFAULT_API_KEY
    if not resolved_api_key:
        raise ValueError(f"api_key is required; set {DEFAULT_API_KEY_ENV_VAR} or pass api_key explicitly")
    return resolved_base_url, resolved_api_key


def build_url(base_url: str, endpoint: str) -> str:
    return normalize_base_url(base_url) + endpoint


def ensure_output_dir(output_dir: str | pathlib.Path | None, endpoint: str) -> pathlib.Path:
    if output_dir is not None:
        path = pathlib.Path(output_dir)
    else:
        stamp = time.strftime("%Y%m%d-%H%M%S")
        endpoint_name = endpoint.strip("/").replace("/", "_")
        path = DEFAULT_OUTPUT_ROOT / f"{stamp}_{endpoint_name}"
    path.mkdir(parents=True, exist_ok=True)
    return path


def maybe_set(payload: dict[str, Any], key: str, value: Any) -> None:
    if value is not None:
        payload[key] = value


def parse_size(size: str) -> tuple[int, int]:
    parts = size.lower().split("x", 1)
    if len(parts) != 2:
        raise ValueError(f"invalid size format: expected widthxheight, got {size!r}")
    return int(parts[0]), int(parts[1])


def normalize_optional_text(value: str | None) -> str:
    return (value or "").strip().lower()


def validate_size(size: str | None) -> None:
    normalized = normalize_optional_text(size)
    if not normalized or normalized == "auto":
        return
    width, height = parse_size(normalized)
    if width % 16 != 0 or height % 16 != 0:
        raise ValueError("size requires both width and height to be multiples of 16")
    if width > MAX_IMAGE_EDGE or height > MAX_IMAGE_EDGE:
        raise ValueError(f"size exceeds maximum edge length {MAX_IMAGE_EDGE}")
    longer = max(width, height)
    shorter = min(width, height)
    if shorter == 0 or longer > shorter * MAX_IMAGE_RATIO:
        raise ValueError(f"size aspect ratio exceeds {MAX_IMAGE_RATIO}:1")
    total_pixels = width * height
    if total_pixels < MIN_IMAGE_PIXELS or total_pixels > MAX_IMAGE_PIXELS:
        raise ValueError(f"size total pixels must be between {MIN_IMAGE_PIXELS} and {MAX_IMAGE_PIXELS}")


def validate_common_inputs(
    *,
    model: str,
    n: int,
    size: str | None,
    response_format: str | None,
    background: str | None,
    output_format: str | None,
    input_fidelity: str | None,
    output_compression: int | None,
    partial_images: int | None,
) -> None:
    if n < 1 or n > MAX_IMAGE_N:
        raise ValueError(f"n must be between 1 and {MAX_IMAGE_N}")
    validate_size(size)
    normalized_response_format = normalize_optional_text(response_format)
    if normalized_response_format and normalized_response_format not in ALLOWED_RESPONSE_FORMATS:
        raise ValueError("response_format=url is not supported for current gpt-image models; use b64_json")
    if normalize_optional_text(model) == "gpt-image-2" and normalize_optional_text(input_fidelity):
        raise ValueError("input_fidelity is not supported for gpt-image-2")
    normalized_output = normalize_optional_text(output_format)
    normalized_background = normalize_optional_text(background)
    if normalized_background == "transparent" and normalized_output and normalized_output not in {"png", "webp"}:
        raise ValueError("background=transparent requires output_format png or webp")
    if output_compression is not None and not (0 <= output_compression <= MAX_OUTPUT_COMPRESSION):
        raise ValueError(f"output_compression must be between 0 and {MAX_OUTPUT_COMPRESSION}")
    if output_compression is not None and normalized_output and normalized_output not in {"jpeg", "jpg", "webp"}:
        raise ValueError("output_compression is only supported when output_format is jpeg or webp")
    if partial_images is not None and not (0 <= partial_images <= MAX_PARTIAL_IMAGES):
        raise ValueError(f"partial_images must be between 0 and {MAX_PARTIAL_IMAGES}")


def validate_upload_path(path: str | pathlib.Path) -> pathlib.Path:
    file_path = pathlib.Path(path)
    if not file_path.exists():
        raise FileNotFoundError(file_path)
    if file_path.stat().st_size > MAX_UPLOAD_BYTES:
        raise ValueError(f"upload file exceeds {MAX_UPLOAD_BYTES} bytes: {file_path}")
    return file_path


def auth_headers(api_key: str) -> dict[str, str]:
    return {
        "Authorization": f"Bearer {api_key}",
        "Accept": "application/json, text/event-stream",
    }


def to_pretty_json(data: Any) -> str:
    return json.dumps(data, ensure_ascii=False, indent=2)


def guess_extension(output_format: str | None, fallback: str = ".png") -> str:
    if not output_format:
        return fallback
    normalized = output_format.lower().strip()
    if normalized in {"jpg", "jpeg"}:
        return ".jpg"
    if normalized == "webp":
        return ".webp"
    if normalized == "png":
        return ".png"
    if "/" in normalized:
        guessed = mimetypes.guess_extension(normalized)
        if guessed:
            return guessed
    return fallback


def build_random_file_name(prefix: str, extension: str) -> str:
    ext = extension if extension.startswith(".") else f".{extension}"
    return f"{prefix}_{uuid.uuid4().hex}{ext}"


def save_bytes(output_dir: pathlib.Path, name: str, data: bytes) -> pathlib.Path:
    path = output_dir / name
    path.write_bytes(data)
    return path


def save_text(output_dir: pathlib.Path, name: str, text: str) -> pathlib.Path:
    path = output_dir / name
    path.write_text(text, encoding="utf-8")
    return path


def normalize_data_url(value: str) -> tuple[str | None, bytes | None]:
    if not value.startswith("data:") or "," not in value:
        return None, None
    header, encoded = value.split(",", 1)
    mime_type = header[5:].split(";", 1)[0] or "application/octet-stream"
    try:
        return mime_type, base64.b64decode(encoded)
    except Exception:
        return mime_type, None


def extract_sse_payloads(body: str) -> list[dict[str, Any]]:
    payloads: list[dict[str, Any]] = []
    buffer: list[str] = []
    for raw_line in body.splitlines():
        line = raw_line.rstrip("\r")
        if line.startswith("data:"):
            value = line[5:].lstrip()
            if value == "[DONE]":
                if buffer:
                    try:
                        payloads.append(json.loads("\n".join(buffer)))
                    except json.JSONDecodeError:
                        pass
                    buffer = []
                continue
            buffer.append(value)
            continue
        if line == "":
            if not buffer:
                continue
            try:
                payloads.append(json.loads("\n".join(buffer)))
            except json.JSONDecodeError:
                pass
            buffer = []
    if buffer:
        try:
            payloads.append(json.loads("\n".join(buffer)))
        except json.JSONDecodeError:
            pass
    return payloads


def pick_image_items(response_json: dict[str, Any] | None, response_text: str | None) -> list[dict[str, Any]]:
    if response_json and isinstance(response_json.get("data"), list):
        return [item for item in response_json["data"] if isinstance(item, dict)]
    if response_text:
        items: list[dict[str, Any]] = []
        for payload in extract_sse_payloads(response_text):
            if payload.get("b64_json") or payload.get("url"):
                items.append(payload)
        return items
    return []


def save_artifacts(
    output_dir: pathlib.Path,
    response_json: dict[str, Any] | None,
    response_text: str | None,
) -> list[ImageArtifact]:
    items = pick_image_items(response_json, response_text)
    artifacts: list[ImageArtifact] = []
    for index, item in enumerate(items, start=1):
        revised_prompt = item.get("revised_prompt")
        output_format = item.get("output_format")
        if item.get("b64_json"):
            image_bytes = base64.b64decode(item["b64_json"])
            ext = guess_extension(output_format)
            saved = save_bytes(output_dir, build_random_file_name("image", ext), image_bytes)
            saved_base64 = save_text(output_dir, build_random_file_name("image_base64", ".txt"), item["b64_json"])
            artifacts.append(
                ImageArtifact(
                    index=index,
                    saved_path=str(saved),
                    base64_saved_path=str(saved_base64),
                    url_saved_path=None,
                    source_type="b64_json",
                    source_value=f"{len(item['b64_json'])} base64 chars",
                    revised_prompt=revised_prompt,
                    output_format=output_format,
                )
            )
            continue
        if item.get("url"):
            mime_type, data = normalize_data_url(item["url"])
            saved_url = save_text(output_dir, build_random_file_name("image_url", ".txt"), item["url"])
            if data is not None:
                ext = guess_extension(output_format or mime_type, fallback=".bin")
                saved = save_bytes(output_dir, build_random_file_name("image", ext), data)
                artifacts.append(
                    ImageArtifact(
                        index=index,
                        saved_path=str(saved),
                        base64_saved_path=None,
                        url_saved_path=str(saved_url),
                        source_type="data_url",
                        source_value=item["url"][:120],
                        revised_prompt=revised_prompt,
                        output_format=output_format,
                    )
                )
                continue
            artifacts.append(
                ImageArtifact(
                    index=index,
                    saved_path=None,
                    base64_saved_path=None,
                    url_saved_path=str(saved_url),
                    source_type="url",
                    source_value=item["url"],
                    revised_prompt=revised_prompt,
                    output_format=output_format,
                )
            )
    return artifacts


def print_result(result: CallResult) -> None:
    print(f"endpoint={result.endpoint}")
    print(f"url={result.url}")
    print(f"status={result.status_code}")
    print(f"ok={result.ok}")
    print(f"request_content_type={result.request_content_type}")
    print(f"response_content_type={result.response_content_type}")
    print(f"output_dir={result.output_dir}")
    if result.payload:
        print("payload:")
        print(to_pretty_json(result.payload))
    if result.response_json is not None:
        print("response_json:")
        print(to_pretty_json(result.response_json))
    elif result.response_text:
        print("response_text:")
        print(result.response_text)
    if result.artifacts:
        print("artifacts:")
        print(to_pretty_json([asdict(item) for item in result.artifacts]))


def request_json_endpoint(
    *,
    base_url: str,
    api_key: str,
    endpoint: str,
    payload: dict[str, Any],
    timeout: int = DEFAULT_TIMEOUT,
    output_dir: str | pathlib.Path | None = None,
) -> CallResult:
    require_requests()
    url = build_url(base_url, endpoint)
    output_path = ensure_output_dir(output_dir, endpoint)
    response = requests.post(
        url,
        headers={**auth_headers(api_key), "Content-Type": "application/json"},
        json=payload,
        timeout=timeout,
    )
    response_json: dict[str, Any] | None = None
    response_text: str | None = None
    content_type = response.headers.get("Content-Type", "")
    if "application/json" in content_type:
        try:
            response_json = response.json()
        except json.JSONDecodeError:
            response_text = response.text
    else:
        response_text = response.text
        try:
            response_json = response.json()
        except Exception:
            pass
    save_text(output_path, "request.json", to_pretty_json(payload))
    save_text(output_path, "response_headers.json", to_pretty_json(dict(response.headers)))
    if response_json is not None:
        save_text(output_path, "response.json", to_pretty_json(response_json))
    elif response_text is not None:
        save_text(output_path, "response.txt", response_text)
    artifacts = save_artifacts(output_path, response_json, response_text)
    result = CallResult(
        endpoint=endpoint,
        url=url,
        request_content_type="application/json",
        status_code=response.status_code,
        ok=response.ok,
        response_content_type=content_type,
        output_dir=str(output_path),
        payload=payload,
        response_json=response_json,
        response_text=response_text,
        artifacts=artifacts,
    )
    print_result(result)
    return result


def request_multipart_endpoint(
    *,
    base_url: str,
    api_key: str,
    endpoint: str,
    data: dict[str, Any],
    files: list[tuple[str, tuple[str, Any, str]]],
    timeout: int = DEFAULT_TIMEOUT,
    output_dir: str | pathlib.Path | None = None,
) -> CallResult:
    require_requests()
    url = build_url(base_url, endpoint)
    output_path = ensure_output_dir(output_dir, endpoint)
    opened_files: list[Any] = []
    try:
        request_files: list[tuple[str, tuple[str, Any, str]]] = []
        for field_name, (file_name, file_value, content_type) in files:
            if isinstance(file_value, (str, pathlib.Path)):
                handle = open(file_value, "rb")
                opened_files.append(handle)
                request_files.append((field_name, (file_name, handle, content_type)))
            else:
                request_files.append((field_name, (file_name, file_value, content_type)))
        response = requests.post(
            url,
            headers=auth_headers(api_key),
            data=data,
            files=request_files,
            timeout=timeout,
        )
    finally:
        for handle in opened_files:
            handle.close()
    response_json: dict[str, Any] | None = None
    response_text: str | None = None
    content_type = response.headers.get("Content-Type", "")
    if "application/json" in content_type:
        try:
            response_json = response.json()
        except json.JSONDecodeError:
            response_text = response.text
    else:
        response_text = response.text
        try:
            response_json = response.json()
        except Exception:
            pass
    save_text(output_path, "request_form.json", to_pretty_json(data))
    save_text(
        output_path,
        "request_files.json",
        to_pretty_json(
            [
                {
                    "field_name": field_name,
                    "file_name": item[0],
                    "content_type": item[2],
                }
                for field_name, item in files
            ]
        ),
    )
    save_text(output_path, "response_headers.json", to_pretty_json(dict(response.headers)))
    if response_json is not None:
        save_text(output_path, "response.json", to_pretty_json(response_json))
    elif response_text is not None:
        save_text(output_path, "response.txt", response_text)
    artifacts = save_artifacts(output_path, response_json, response_text)
    result = CallResult(
        endpoint=endpoint,
        url=url,
        request_content_type="multipart/form-data",
        status_code=response.status_code,
        ok=response.ok,
        response_content_type=content_type,
        output_dir=str(output_path),
        payload=data,
        response_json=response_json,
        response_text=response_text,
        artifacts=artifacts,
    )
    print_result(result)
    return result


def build_common_payload(
    *,
    model: str,
    prompt: str,
    conversation_id: str | None = None,
    parent_message_id: str | None = None,
    n: int = 1,
    size: str | None = None,
    response_format: str | None = "b64_json",
    stream: bool = False,
    quality: str | None = None,
    background: str | None = None,
    output_format: str | None = None,
    moderation: str | None = None,
    input_fidelity: str | None = None,
    style: str | None = None,
    output_compression: int | None = None,
    partial_images: int | None = None,
) -> dict[str, Any]:
    """
    当前仓库 images/images2api 接口实际支持的通用字段说明:

    - model:
      - 必填
      - 当前代码要求图片模型, 实际应传 `gpt-image-*`
      - 常用: `gpt-image-2`

    - prompt:
      - 必填
      - 生图/改图提示词

    - conversation_id:
      - 选填
      - 仅对 `images2api/web` 连续对话有意义
      - 传入后表示继续已有 ChatGPT Web 会话

    - parent_message_id:
      - 选填
      - 用于显式指定连续对话挂载到哪条父消息
      - 当前后端也支持“只给 `conversation_id`，由后端尽量自动补 parent”

    - n:
      - 生成张数
      - 当前校验要求 > 0
      - 建议先传 1

    - size:
      - 可选
      - 官方文档当前显式列出的尺寸: `auto`, `1024x1024`, `1536x1024`, `1024x1536`
      - 当前仓库网关仍兼容更大的合法尺寸，例如:
        `1792x1024`, `1024x1792`, `2048x2048`, `2048x1152`, `1152x2048`, `3840x2160`, `2160x3840`
      - 网关尺寸约束: 最大边长 `<= 3840`，宽高都必须是 `16` 的倍数，长宽比 `<= 3:1`，
        总像素范围 `655,360 - 8,294,400`

    - response_format:
      - 可选
      - 当前官方/仓库行为都以 `b64_json` 为主；显式传 `""` 时也会按默认 base64 返回处理
      - `url` 不再作为当前 gpt-image 模型的合法请求参数

    - stream:
      - 可选
      - 枚举: `True`, `False`

    - quality:
      - 可选
      - 常见枚举: `low`, `medium`, `high`, `auto`

    - background:
      - 可选
      - 常见枚举: `opaque`, `transparent`, `auto`
      - 若传 `transparent`，当前要求 `output_format` 必须是 `png` 或 `webp`

    - output_format:
      - 可选
      - 常见枚举: `png`, `jpeg`, `webp`

    - moderation:
      - 可选
      - 常见枚举: `auto`, `low`

    - input_fidelity:
      - 可选
      - 常见枚举: `low`, `high`
      - 当前 `gpt-image-2` 不支持传这个字段

    - style:
      - 可选
      - 常见枚举: `vivid`, `natural`

    - output_compression:
      - 可选
      - 整数, 常见 0-100
      - 当前仅允许搭配 `jpeg/jpg/webp`

    - partial_images:
      - 可选
      - 整数, 常见 `0` 或 `1`
    """
    validate_common_inputs(
        model=model,
        n=n,
        size=size,
        response_format=response_format,
        background=background,
        output_format=output_format,
        input_fidelity=input_fidelity,
        output_compression=output_compression,
        partial_images=partial_images,
    )
    payload: dict[str, Any] = {
        "model": model,
        "prompt": prompt,
        "n": n,
        "stream": stream,
    }
    maybe_set(payload, "conversation_id", conversation_id)
    maybe_set(payload, "parent_message_id", parent_message_id)
    maybe_set(payload, "size", size)
    maybe_set(payload, "response_format", response_format)
    maybe_set(payload, "quality", quality)
    maybe_set(payload, "background", background)
    maybe_set(payload, "output_format", output_format)
    maybe_set(payload, "moderation", moderation)
    maybe_set(payload, "input_fidelity", input_fidelity)
    maybe_set(payload, "style", style)
    maybe_set(payload, "output_compression", output_compression)
    maybe_set(payload, "partial_images", partial_images)
    return payload


def build_multipart_form(payload: dict[str, Any]) -> dict[str, str]:
    form: dict[str, str] = {}
    for key, value in payload.items():
        if value is None:
            continue
        if isinstance(value, bool):
            form[key] = "true" if value else "false"
            continue
        form[key] = str(value)
    return form


def build_file_tuple(path: str | pathlib.Path) -> tuple[str, str, str]:
    file_path = validate_upload_path(path)
    content_type = mimetypes.guess_type(str(file_path))[0] or "application/octet-stream"
    return file_path.name, str(file_path), content_type


def call_images_generations(
    *,
    base_url: str | None = None,
    api_key: str | None = None,
    model: str,
    prompt: str,
    n: int = 1,
    size: str | None = None,
    response_format: str | None = "b64_json",
    stream: bool = False,
    quality: str | None = None,
    background: str | None = None,
    output_format: str | None = None,
    moderation: str | None = None,
    input_fidelity: str | None = None,
    style: str | None = None,
    output_compression: int | None = None,
    partial_images: int | None = None,
    timeout: int = DEFAULT_TIMEOUT,
    output_dir: str | pathlib.Path | None = None,
) -> CallResult:
    base_url, api_key = resolve_runtime_config(base_url, api_key)
    payload = build_common_payload(
        model=model,
        prompt=prompt,
        n=n,
        size=size,
        response_format=response_format,
        stream=stream,
        quality=quality,
        background=background,
        output_format=output_format,
        moderation=moderation,
        input_fidelity=input_fidelity,
        style=style,
        output_compression=output_compression,
        partial_images=partial_images,
    )
    return request_json_endpoint(
        base_url=base_url,
        api_key=api_key,
        endpoint="/v1/images/generations",
        payload=payload,
        timeout=timeout,
        output_dir=output_dir,
    )


def call_images_edits(
    *,
    base_url: str | None = None,
    api_key: str | None = None,
    model: str,
    prompt: str,
    image_paths: list[str | pathlib.Path] | None = None,
    image_urls: list[str] | None = None,
    mask_path: str | pathlib.Path | None = None,
    mask_image_url: str | None = None,
    n: int = 1,
    size: str | None = None,
    response_format: str | None = "b64_json",
    stream: bool = False,
    quality: str | None = None,
    background: str | None = None,
    output_format: str | None = None,
    moderation: str | None = None,
    input_fidelity: str | None = None,
    style: str | None = None,
    output_compression: int | None = None,
    partial_images: int | None = None,
    timeout: int = DEFAULT_TIMEOUT,
    output_dir: str | pathlib.Path | None = None,
) -> CallResult:
    """
    edits 支持两种输入方式:

    1. multipart 本地文件:
       - image_paths=["/abs/a.png", "/abs/b.png"]
       - mask_path="/abs/mask.png"
       - 当前服务端单文件上限: 50MB

    2. JSON 远程引用:
       - image_urls=["https://example.com/source.png"]
       - mask_image_url="https://example.com/mask.png"

    当前 helper 只暴露 `image_url` 入参；后端实际可接受 `images[].image_url` / `images[].file_id` 的混合顺序。
    """
    base_url, api_key = resolve_runtime_config(base_url, api_key)
    payload = build_common_payload(
        model=model,
        prompt=prompt,
        n=n,
        size=size,
        response_format=response_format,
        stream=stream,
        quality=quality,
        background=background,
        output_format=output_format,
        moderation=moderation,
        input_fidelity=input_fidelity,
        style=style,
        output_compression=output_compression,
        partial_images=partial_images,
    )
    image_paths = image_paths or []
    image_urls = image_urls or []
    if image_paths:
        files: list[tuple[str, tuple[str, Any, str]]] = [
            ("image", build_file_tuple(path)) for path in image_paths
        ]
        if mask_path is not None:
            files.append(("mask", build_file_tuple(mask_path)))
        return request_multipart_endpoint(
            base_url=base_url,
            api_key=api_key,
            endpoint="/v1/images/edits",
            data=build_multipart_form(payload),
            files=files,
            timeout=timeout,
            output_dir=output_dir,
        )
    json_payload = dict(payload)
    json_payload["images"] = [{"image_url": value} for value in image_urls]
    if mask_image_url is not None:
        json_payload["mask"] = {"image_url": mask_image_url}
    return request_json_endpoint(
        base_url=base_url,
        api_key=api_key,
        endpoint="/v1/images/edits",
        payload=json_payload,
        timeout=timeout,
        output_dir=output_dir,
    )


def call_images2api_generations(
    *,
    base_url: str | None = None,
    api_key: str | None = None,
    model: str,
    prompt: str,
    conversation_id: str | None = None,
    parent_message_id: str | None = None,
    n: int = 1,
    size: str | None = None,
    response_format: str | None = "b64_json",
    stream: bool = False,
    quality: str | None = None,
    background: str | None = None,
    output_format: str | None = None,
    moderation: str | None = None,
    input_fidelity: str | None = None,
    style: str | None = None,
    output_compression: int | None = None,
    partial_images: int | None = None,
    timeout: int = DEFAULT_TIMEOUT,
    output_dir: str | pathlib.Path | None = None,
) -> CallResult:
    base_url, api_key = resolve_runtime_config(base_url, api_key)
    payload = build_common_payload(
        model=model,
        prompt=prompt,
        conversation_id=conversation_id,
        parent_message_id=parent_message_id,
        n=n,
        size=size,
        response_format=response_format,
        stream=stream,
        quality=quality,
        background=background,
        output_format=output_format,
        moderation=moderation,
        input_fidelity=input_fidelity,
        style=style,
        output_compression=output_compression,
        partial_images=partial_images,
    )
    return request_json_endpoint(
        base_url=base_url,
        api_key=api_key,
        endpoint="/v1/images2api/generations",
        payload=payload,
        timeout=timeout,
        output_dir=output_dir,
    )


def call_images2api_edits(
    *,
    base_url: str | None = None,
    api_key: str | None = None,
    model: str,
    prompt: str,
    conversation_id: str | None = None,
    parent_message_id: str | None = None,
    original_file_id: str | None = None,
    original_gen_id: str | None = None,
    mask_file_id: str | None = None,
    image_paths: list[str | pathlib.Path] | None = None,
    image_urls: list[str] | None = None,
    mask_path: str | pathlib.Path | None = None,
    mask_image_url: str | None = None,
    n: int = 1,
    size: str | None = None,
    response_format: str | None = "b64_json",
    stream: bool = False,
    quality: str | None = None,
    background: str | None = None,
    output_format: str | None = None,
    moderation: str | None = None,
    input_fidelity: str | None = None,
    style: str | None = None,
    output_compression: int | None = None,
    partial_images: int | None = None,
    timeout: int = DEFAULT_TIMEOUT,
    output_dir: str | pathlib.Path | None = None,
) -> CallResult:
    """
    images2api edits 的输入形态与 images edits 相同:
    - 本地文件: multipart
    - 远程图: JSON `images[].image_url`
    - 已有引用: JSON `images[].file_id`
    - 连续对话: 可额外传 `conversation_id` / `parent_message_id`
    - 旧版 inpainting: 可额外传 `original_file_id` / `original_gen_id` / `mask_file_id`
    """
    base_url, api_key = resolve_runtime_config(base_url, api_key)
    payload = build_common_payload(
        model=model,
        prompt=prompt,
        conversation_id=conversation_id,
        parent_message_id=parent_message_id,
        n=n,
        size=size,
        response_format=response_format,
        stream=stream,
        quality=quality,
        background=background,
        output_format=output_format,
        moderation=moderation,
        input_fidelity=input_fidelity,
        style=style,
        output_compression=output_compression,
        partial_images=partial_images,
    )
    maybe_set(payload, "original_file_id", original_file_id)
    maybe_set(payload, "original_gen_id", original_gen_id)
    maybe_set(payload, "mask_file_id", mask_file_id)
    image_paths = image_paths or []
    image_urls = image_urls or []
    if image_paths:
        files: list[tuple[str, tuple[str, Any, str]]] = [
            ("image", build_file_tuple(path)) for path in image_paths
        ]
        if mask_path is not None:
            files.append(("mask", build_file_tuple(mask_path)))
        return request_multipart_endpoint(
            base_url=base_url,
            api_key=api_key,
            endpoint="/v1/images2api/edits",
            data=build_multipart_form(payload),
            files=files,
            timeout=timeout,
            output_dir=output_dir,
        )
    json_payload = dict(payload)
    json_payload["images"] = [{"image_url": value} for value in image_urls]
    if mask_image_url is not None:
        json_payload["mask"] = {"image_url": mask_image_url}
    return request_json_endpoint(
        base_url=base_url,
        api_key=api_key,
        endpoint="/v1/images2api/edits",
        payload=json_payload,
        timeout=timeout,
        output_dir=output_dir,
    )


def download_http_url(url: str, output_dir: str | pathlib.Path | None = None) -> pathlib.Path:
    output_path = ensure_output_dir(output_dir, "/download")
    parsed = urllib.parse.urlparse(url)
    suffix = pathlib.Path(parsed.path).suffix or ".bin"
    target = output_path / build_random_file_name("downloaded", suffix)
    with urllib.request.urlopen(url) as response:
        target.write_bytes(response.read())
    return target


def main() -> None:
    # 默认从环境变量读取:
    # - SUB2API_BASE_URL
    # - SUB2API_API_KEY
    # 显式传参会覆盖环境变量。取消对应示例的注释即可直接运行。
    #
    # 参数备注总览:
    # - model: 图片模型, 当前应传 gpt-image-*; 常用 `gpt-image-2`
    # - prompt: 提示词
    # - n: 生成张数, 建议 1
    # - size: 官方文档当前显式列出 `auto` / `1024x1024` / `1536x1024` / `1024x1536`
    #   - 当前仓库网关额外兼容的常见扩展尺寸:
    #     `1792x1024` / `1024x1792` / `2048x2048` / `2048x1152` / `1152x2048` / `3840x2160` / `2160x3840`
    #   - 网关约束: 最大边长 <= 3840, 宽高都是 16 的倍数, 长宽比 <= 3:1, 总像素在 655,360 - 8,294,400
    # - response_format: 当前官方/仓库行为都以 `b64_json` 为主; 传空字符串等价于用默认 base64 返回
    # - stream: 枚举 True / False
    # - quality: 常见 `low` / `medium` / `high` / `auto`
    # - background: 常见 `opaque` / `transparent` / `auto`
    # - output_format: 常见 `png` / `jpeg` / `webp`
    # - moderation: 常见 `auto` / `low`
    # - input_fidelity: 常见 `low` / `high`
    # - style: 常见 `vivid` / `natural`
    # - output_compression: 整数, 常见 0-100
    # - partial_images: 整数, 常见 0 / 1
    # - multipart 本地文件上传: 当前单文件上限 50MB
    #
    # 输出说明:
    # - 脚本会保存请求、响应头、响应体
    # - 如果返回 b64_json:
    #   - 保存一份随机文件名的图片
    #   - 额外保存一份 base64 文本
    # - 如果未来上游返回 url 字段:
    #   - 脚本仍会保存 url 原值
    #   - 若是 data:image/... 会自动解码落盘
    #   - 若是真实 http(s) 链接，可再用 download_http_url() 下载

    # 示例 1: /v1/images/generations
    # call_images_generations(
    #     base_url=DEFAULT_BASE_URL,
    #     api_key=DEFAULT_API_KEY,
    #     model="gpt-image-2",                  # 必填; 当前应传 gpt-image-*; 常用 gpt-image-2
    #     prompt="生成一张雨夜霓虹街景海报，电影感，中文招牌，反射丰富",  # 必填; 提示词
    #     n=1,                                  # 生成张数; 建议 1
    #     size="1024x1024",                     # 官方显式尺寸; 也可传仓库兼容扩展尺寸, 但需满足网关约束
    #     response_format="b64_json",           # 当前官方/仓库行为都以 b64_json 为主; 也可留空走默认返回
    #     stream=False,                         # 枚举: True/False
    #     quality="high",                       # 常见: low/medium/high/auto
    #     background="opaque",                  # 常见: opaque/transparent/auto
    #     output_format="png",                  # 常见: png/jpeg/webp
    #     moderation="auto",                    # 常见: auto/low
    #     style="vivid",                        # 常见: vivid/natural
    #     output_compression=90,                # 整数; 常见 0-100
    #     partial_images=0,                     # 整数; 常见 0/1
    # )

    # 示例 2: /v1/images/edits，本地文件 multipart
    # call_images_edits(
    #     base_url=DEFAULT_BASE_URL,
    #     api_key=DEFAULT_API_KEY,
    #     model="gpt-image-2",
    #     prompt="把人物后面的背景替换成蓝色极光，保留人物主体和光影方向",
    #     image_paths=["/absolute/path/source.png"],
    #     mask_path="/absolute/path/mask.png",
    #     n=1,
    #     size="1024x1024",
    #     response_format="b64_json",
    #     stream=False,
    #     quality="high",
    #     background="transparent",
    #     output_format="png",
    #     moderation="auto",
    #     style="natural",
    #     output_compression=90,
    #     partial_images=0,
    # )

    # 示例 4.1: /v1/images2api/generations，连续对话
    # call_images2api_generations(
    #     base_url=DEFAULT_BASE_URL,
    #     api_key=DEFAULT_API_KEY,
    #     model="gpt-image-2",
    #     prompt="继续上一轮风格，换成更克制的配色",
    #     conversation_id="69fdf36f-3d48-83ea-8c7a-265960703d04",
    #     parent_message_id="c512d6a3-85b0-4c75-ae18-b76f52a88067",
    #     n=1,
    #     size="1024x1024",
    #     response_format="b64_json",
    # )

    # 示例 3: /v1/images/edits，JSON image_url
    # call_images_edits(
    #     base_url=DEFAULT_BASE_URL,
    #     api_key=DEFAULT_API_KEY,
    #     model="gpt-image-2",
    #     prompt="把背景改成赛博朋克城市，主体不变",
    #     image_urls=["https://example.com/source.png"],
    #     mask_image_url="https://example.com/mask.png",
    #     n=1,
    #     size="1024x1024",
    #     response_format="b64_json",
    #     stream=False,
    #     quality="high",
    #     background="opaque",
    #     output_format="png",
    #     moderation="auto",
    #     style="vivid",
    #     output_compression=90,
    #     partial_images=0,
    # )

    # 示例 4: /v1/images2api/generations
    # call_images2api_generations(
    #     base_url=DEFAULT_BASE_URL,
    #     api_key=DEFAULT_API_KEY,
    #     model="gpt-image-2",
    #     prompt="生成一张极简科技产品 KV，白底，柔光，商业摄影风格",
    #     n=1,
    #     size="1024x1024",
    #     response_format="b64_json",
    #     stream=False,
    #     quality="high",
    #     background="opaque",
    #     output_format="png",
    #     moderation="auto",
    #     style="natural",
    #     output_compression=90,
    #     partial_images=0,
    # )

    # 示例 5: /v1/images2api/edits，本地文件 multipart
    # call_images2api_edits(
    #     base_url=DEFAULT_BASE_URL,
    #     api_key=DEFAULT_API_KEY,
    #     model="gpt-image-2",
    #     prompt="把背景换成黄昏海边，保留主体构图",
    #     conversation_id="69fdf36f-3d48-83ea-8c7a-265960703d04",
    #     parent_message_id="c512d6a3-85b0-4c75-ae18-b76f52a88067",
    #     original_file_id="file_123",
    #     original_gen_id="gen_456",
    #     mask_file_id="mask_789",
    #     image_paths=["/absolute/path/source.png"],
    #     mask_path="/absolute/path/mask.png",
    #     n=1,
    #     size="1024x1024",
    #     response_format="b64_json",
    #     stream=False,
    #     quality="high",
    #     background="transparent",
    #     output_format="png",
    #     moderation="auto",
    #     style="natural",
    #     output_compression=90,
    #     partial_images=0,
    # )

    # 示例 6: /v1/images2api/edits，JSON image_url
    # call_images2api_edits(
    #     base_url=DEFAULT_BASE_URL,
    #     api_key=DEFAULT_API_KEY,
    #     model="gpt-image-2",
    #     prompt="把图片改成复古杂志封面风格",
    #     conversation_id="69fdf36f-3d48-83ea-8c7a-265960703d04",
    #     parent_message_id="c512d6a3-85b0-4c75-ae18-b76f52a88067",
    #     original_file_id="file_123",
    #     original_gen_id="gen_456",
    #     mask_file_id="mask_789",
    #     image_urls=["https://example.com/source.png"],
    #     mask_image_url="https://example.com/mask.png",
    #     n=1,
    #     size="1024x1024",
    #     response_format="b64_json",
    #     stream=False,
    #     quality="high",
    #     background="opaque",
    #     output_format="png",
    #     moderation="auto",
    #     style="vivid",
    #     output_compression=90,
    #     partial_images=0,
    # )

    # 示例 7: 若未来某条兼容链路真返回 http(s) 远端链接，可手动下载
    # download_http_url("https://example.com/path/to/image.png")
    pass


if __name__ == "__main__":
    main()
