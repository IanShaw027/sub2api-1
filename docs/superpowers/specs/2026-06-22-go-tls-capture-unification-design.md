# Go TLS Capture Unification Design

## Goal
Unify the current `sub2api` TLS fingerprint capture pipeline with the richer capture capabilities from `/opt/tls.zip` inside the Go main service, while making every persisted capture sample a complete replayable artifact and preserving task-driven capture control.

## Scope
- Replace the current HTTP/1.1-only native TLS capture runtime with a new multi-protocol capture subsystem inside the Go main service.
- Support capture and task-driven sampling across:
  - HTTP/1.1
  - HTTP/2
  - WebSocket over HTTP/1.1
  - WebSocket over HTTP/2
- Persist richer TLS and request metadata, including JA3/JA4 and transport facts.
- Keep the capture control plane task-driven.
- Make canonical capture samples dedupe by `TLS replay hash + transport`.
- Require every persisted canonical sample to remain fully replayable.
- Return session-friendly mock success responses for JSON, SSE, and WebSocket capture flows.

## Non-Goals
- Do not preserve compatibility with the current capture task/sample schema or DTO shape.
- Do not make the collector emulate real upstream business behavior in high fidelity.
- Do not use JA3 or JA4 as the primary identity or dedupe key.
- Do not merge non-replayable observational metadata into the long-term TLS profile source of truth.

## Confirmed Product Decisions
- Runtime form: fully integrated into the existing Go main service.
- Compatibility: no need to preserve the old capture schema or old capture API shape.
- Sample contract: persisted canonical samples must be complete replayable samples.
- Replay boundary: replay includes TLS and transport type, but not full upstream business-session semantics.
- Control plane: remain task-driven.
- Dedupe unit: `TLS replay hash + transport`.
- Mock behavior target: session-friendly, not high-fidelity upstream emulation.

## Recommended Approach
Adopt a new unified capture core instead of incrementally extending the current native capture listener.

Rationale:
- The current listener is built around one captured `ClientHello` and one HTTP/1.1 request path, which does not fit HTTP/2 multi-stream or WebSocket multi-turn behavior.
- The current service model mixes parsing and orchestration; the unified design needs clean boundaries between TLS parsing, session handling, sample construction, persistence, and mock response behavior.
- Replayable TLS identity and observational metadata must be separated cleanly so richer analysis does not break dedupe or upstream replay.

## Architecture

```text
TCP Accept
  -> TLS Capture Listener
     - capture raw ClientHello
     - negotiate ALPN
     - create connection context

  -> Transport Session Router
     - HTTP/1.1 session
     - HTTP/2 session
     - WebSocket over HTTP/1.1 session
     - WebSocket over HTTP/2 session

  -> Capture Builder
     - raw ClientHello -> ObservedClientHello
     - ObservedClientHello -> ReplayProfile
     - derive replay hash / JA3 / JA4 / ALPN fingerprint
     - merge request and transport metadata

  -> Capture Task Service
     - apply task filters
     - validate replayability
     - dedupe by replay hash + transport
     - persist session and canonical sample
     - update task progress and completion

  -> Mock Responders
     - HTTP JSON success
     - /v1/responses SSE success stream
     - WebSocket multi-turn success events
```

### Layer Responsibilities
- `listener` owns TCP accept, TLS handshake, ALPN negotiation, and raw `ClientHello` capture.
- `transport session` owns HTTP/1.1, HTTP/2, and WebSocket lifecycle handling.
- `capture builder` owns replayability validation and canonical sample construction.
- `task service` owns task-level filtering, target accounting, dedupe, and persistence coordination.
- `mock responders` own client-visible success behavior and must not influence replay truth or dedupe identity.

## Data Model
The current `capture task/sample` design is too narrow. Replace it with a three-level model:

```text
capture_task
  - control plane

capture_session
  - one TLS connection / negotiated session

capture_session_event
  - one request, stream, message, mock emission, or session error event

capture_sample
  - one canonical replayable asset with a confirmed transport
```

### `capture_task`
Task remains the control plane object.

Required fields:
- `id`
- `name`
- `status`
- `token`
- `targets`
- `counts`
- `ua_keywords`
- `created_at`
- `updated_at`
- `completed_at`

New fields:
- `transport_targets`
- `capture_filters`
- `sample_schema_version`
- `task_stats`

`task_stats` should hold aggregates such as:
- `by_transport`
- `by_client_type`
- `by_platform`

Task completion semantics:
- `targets` continue to mean canonical sample targets by platform.
- `transport_targets` mean canonical sample targets by transport.
- If `transport_targets` is empty, task completion is governed only by `targets`.
- If `transport_targets` is set, the task completes only after both configured platform targets and configured transport targets are satisfied by canonical samples that passed filtering.

### `capture_session`
One task-qualified TLS connection creates one session row after the first accepted request, stream, or message resolves task identity.

Required fields:
- `task_id`
- `session_id`
- `client_ip`
- `alpn_negotiated`
- `raw_client_hello`
- `observed_client_hello`
- `replay_profile`
- `derived_fingerprint`
- `session_status`
- `error_summary`
- `opened_at`
- `closed_at`

`observed_client_hello` stores rich parsed facts, including:
- `record_version`
- `client_hello_version`
- `sni_present`
- `sni_kind`
- `grease_values`
- `extensions_order`
- `extension_metadata`
- `cipher_suites`
- `curves`
- `point_formats`
- `signature_algorithms`
- `signature_algorithms_cert`
- `alpn_protocols`
- `supported_versions`
- `key_share_groups`
- `psk_modes`
- `compress_cert_algos`
- `delegated_credentials_algorithms`
- `application_settings_protocols`
- `ech_present`
- `padding_len`

`derived_fingerprint` stores derived identifiers and analysis values:
- `replay_hash`
- `replay_hash_version`
- `ja3_raw`
- `ja3_hash`
- `ja4`
- `alpn_fingerprint`
- `http2_fingerprint`
- `parse_version`

### `capture_session_event`
Persist per-request, per-stream, per-message, and per-mock-response session facts that must not be collapsed into a single canonical sample.

Required fields:
- `task_id`
- `session_id`
- `event_id`
- `event_type`
- `transport`
- `request_sequence`
- `stream_id`
- `request_path`
- `http_method`
- `is_websocket`
- `websocket_protocol`
- `client_type`
- `model`
- `request_kind`
- `streaming`
- `response_mode`
- `user_agent`
- `originator`
- `stainless_metadata`
- `headers_snapshot`
- `body_summary`
- `event_status`
- `event_error`
- `created_at`

`capture_session_event` is the persistence target for:
- later WebSocket turns after the canonical sample is fixed
- additional HTTP/2 streams after the canonical sample is fixed
- mock response emission facts
- parse or replayability failures that should appear in admin inspection
- prewarm versus turn distinctions used by the mock responders

### `capture_sample`
Only canonical, complete, replayable, transport-confirmed assets become samples.

Required fields:
- `task_id`
- `session_id`
- `platform`
- `transport`
- `request_path`
- `http_method`
- `is_websocket`
- `websocket_protocol`
- `client_type`
- `model`
- `request_kind`
- `streaming`
- `response_mode`
- `user_agent`
- `originator`
- `stainless_metadata`
- `replay_profile`
- `replay_hash`
- `ja3_raw`
- `ja3_hash`
- `ja4`
- `http2_fingerprint`
- `raw_client_hello`
- `captured_at`

Behavior:
- `capture_session` is the connection-level truth.
- `capture_session_event` is the request/message-level truth.
- `capture_sample` is the durable replayable asset.
- A session may carry multiple requests or messages.
- A session may produce up to one canonical sample per transport identity observed on that session.
- Within one transport identity, default canonical sampling takes the first valid interaction that satisfies replayability and task filters after transport is fully known.

## Dedupe and Replay Rules

### Dedupe
Canonical sample dedupe key:
- `task_id + replay_hash + transport`

Rationale:
- `transport` must be part of identity so `h2` and `websocket` do not collapse into `http1`.
- `request_path`, `client_type`, and `model` must not be part of the primary identity because they would fragment one transport-replayable artifact into many copies.

### Replay Identity
- `replay_hash` remains the primary replay identity.
- JA3 and JA4 are secondary derived analysis keys only.
- Replay truth comes from `replay_profile`, not from JA3 or JA4.

### Transport Replay Contract
Transport replay means the system can replay the captured sample through the same transport family and protocol shape:
- `http1`
- `h2`
- `websocket-http1`
- `websocket-h2`

It does not require replaying the entire original client session transcript or every original request header byte-for-byte.

Protocol-specific replay metadata that must be persisted on the canonical sample or derivable from the session includes:
- `transport`
- `request_path`
- `http_method`
- `websocket_protocol`
- `http2_fingerprint` when the transport family is `h2` or `websocket-h2`
- request mode metadata such as `request_kind`, `streaming`, and `response_mode` when they affect transport behavior

### Profile Import
- Import from canonical capture samples into long-term TLS profiles must continue to use only `replay_profile`.
- `request_path`, `client_type`, `model`, `JA3`, `JA4`, and runtime headers remain sample metadata and do not become part of the persistent TLS profile source of truth.

## TLS Parsing and Fingerprint Boundaries
The current code already has an ingress-to-replay shape:
- raw `ClientHello` is captured
- the service parses it into a replayable TLS profile
- replay later rebuilds a `uTLS` spec from that profile

The unified design should formalize this into explicit types:
- `ObservedClientHello`
- `ReplayProfile`
- `DerivedFingerprint`

### ReplayProfile
Contains only values that can be faithfully replayed through the existing TLS dialer path.

It must include the current replay fields and add:
- `signature_algorithms_cert`

### DerivedFingerprint
Contains:
- `replay_hash`
- `replay_hash_version`
- `ja3_raw`
- `ja3_hash`
- `ja4`
- `alpn_fingerprint`
- `parse_version`

### ObservedClientHello
Contains richer observational metadata that is useful for analysis and debugging but is not itself the replay truth.

## Protocol Handling

### HTTP/1.1
- After TLS handshake, parse the HTTP/1.1 request.
- Extract token, platform, request path, method, `User-Agent`, `Originator`, `X-Stainless-*`, and request-body model metadata.
- Build a canonical sample once the request is fully parsed.
- For normal JSON flows, return a minimal `200` success JSON response.
- For `/v1/responses` with `stream=true`, return a minimal SSE success stream:
  - `response.created`
  - optional visible output
  - `response.completed`
  - `[DONE]`
- For prewarm-like requests such as `generate=false`, return a successful stream without visible text.

### HTTP/2
- When ALPN negotiates `h2`, enter an HTTP/2 session.
- Parse per-stream `:method`, `:path`, optional `:protocol`, and body content.
- Capture and derive the client HTTP/2 fingerprint from connection-level settings and retain it for canonical samples and event inspection.
- Support normal JSON responses and `/v1/responses` SSE-style success responses.
- One connection may carry multiple streams.
- Default canonical sampling still uses the first valid stream for the `h2` transport identity that yields a replayable sample and matches task filters.

### WebSocket over HTTP/1.1
- Complete HTTP/1.1 upgrade first.
- Create session-level connection state.
- Use the first valid application text message to finalize the canonical sample.
- Persist transport as `websocket-http1`.
- Support:
  - multi-turn text messages
  - ping/pong
  - fragmented frame reassembly
  - idle close
  - close handshake
- Later messages attach as session events, not new canonical samples by default.

### WebSocket over HTTP/2
- Use extended CONNECT with `:protocol=websocket`.
- Sampling behavior matches WebSocket over HTTP/1.1.
- Persist transport as `websocket-h2`.
- First implementation target is capture correctness and transport replayability, not high-fidelity upstream behavior.

## Canonical Sample Finalization Rules
- `http1` and `h2` normal requests: finalize after full request parse.
- `responses` SSE requests: finalize after request parse, before sending the success stream.
- `websocket` sessions: finalize after upgrade succeeds and the first valid application message arrives.

Reasons:
- TLS identity is known.
- Transport identity is known.
- Client type, model, and request-kind metadata are usually available by that point.
- The persisted object satisfies the required replay contract.

## Mock Response Boundary
Provide only session-friendly success behavior.

### Must Do
- Minimal success JSON
- Minimal success SSE
- Minimal success WebSocket multi-turn event flow

### Must Not Do
- Full upstream business emulation
- Rich error-matrix compatibility
- Real tool-call execution
- Full upstream state synchronization

The collector is responsible for allowing clients to complete a successful capture interaction, not for acting like a full upstream model service.

## Failure Handling
- TLS parse failure: terminate the connection.
- Transport parse failure: respond with the smallest protocol-appropriate error and close.
- If the interaction does not satisfy task filters:
  - client-facing success behavior may still complete
  - no canonical sample is persisted
  - the interaction may still be recorded as a session event
- If the interaction is not fully replayable:
  - do not persist a canonical sample
  - record the failure as a session event for admin inspection

## Code Organization
Do not keep expanding the current listener and service files. Introduce a new structure.

```text
backend/internal/pkg/tlsfingerprint/
  parser/
  replay/
  transport/
  http2/

backend/internal/service/tls_capture/
  listener/
  session/
  event/
  sample/
  mock/
  task/
```

### `backend/internal/pkg/tlsfingerprint/`
- `parser/`
  - raw `ClientHello` -> `ObservedClientHello`
  - JA3 / JA4 / ALPN fingerprint
  - replay hash
- `replay/`
  - `ObservedClientHello` -> `ReplayProfile`
  - `ReplayProfile` -> `uTLS ClientHelloSpec`
- `transport/`
  - transport enum
  - session ids
  - request metadata types
- `http2/`
  - HTTP/2 settings capture
  - HTTP/2 fingerprint derivation

### `backend/internal/service/tls_capture/`
- `listener/`
  - TCP accept
  - TLS handshake
  - ALPN routing
- `session/`
  - HTTP/1.1 session
  - HTTP/2 session
  - WebSocket over HTTP/1.1 session
  - WebSocket over HTTP/2 session
- `event/`
  - session/request/message/mock-response event persistence
- `sample/`
  - session/request event -> canonical sample
- `mock/`
  - JSON responder
  - SSE responder
  - WebSocket responder
- `task/`
  - task orchestration
  - filtering
  - target counting
  - dedupe
  - persistence coordination

## Migration Direction
- Replace the current narrow capture runtime instead of incrementally extending it.
- Rebuild capture persistence around `task/session/sample`.
- Keep replay truth isolated in `replay_profile`.
- Do not push observational metadata into persistent TLS profiles.

## Testing Strategy

### Parser Unit Tests
Cover:
- raw `ClientHello` -> `ObservedClientHello`
- `ObservedClientHello` -> `ReplayProfile`
- replay hash
- JA3
- JA4
- GREASE handling
- SNI presence
- ALPN parsing
- TLS 1.2 and TLS 1.3 cases
- `signature_algorithms_cert`

### Replay Unit Tests
Verify that replay profiles can rebuild stable `uTLS` specs with the expected key fields.

### Session and Transport Integration Tests
Cover:
- HTTP/1.1 JSON
- HTTP/1.1 `/v1/responses` SSE
- HTTP/2 JSON
- HTTP/2 SSE
- HTTP/2 settings capture and `http2_fingerprint` derivation
- WebSocket over HTTP/1.1 single-turn
- WebSocket over HTTP/1.1 multi-turn
- fragmented WebSocket frames
- ping/pong
- idle close
- WebSocket over HTTP/2 basic path

### Task and Persistence Tests
Cover:
- dedupe by `replay_hash + transport`
- target counting
- task completion
- task filters
- task completion with both `targets` and `transport_targets`
- one session producing canonical samples for more than one transport identity when applicable
- non-replayable interactions rejected from canonical samples
- successful client flow with filtered-out sample

### Admin and Frontend Tests
Cover:
- new DTO rendering
- sample detail rendering
- filtering by `transport`, `client_type`, `ja3`, and `ja4`

## Delivery Sequence
1. Build the new `pkg/tlsfingerprint` parsing, derivation, and replay core.
2. Introduce the new `task/session/sample` persistence model and repository layer.
3. Implement new runtime handling for HTTP/1.1 plus SSE success flows.
4. Add HTTP/2 capture and response support.
5. Add WebSocket over HTTP/1.1 and HTTP/2 session handling plus multi-turn mock responders.
6. Switch admin API and frontend to the new model.
7. Remove the old capture runtime after the new path is complete.

## Risk Controls
- Gate the new capture runtime behind its own configuration path while it is being integrated.
- Keep `replay_hash + transport` as the only canonical sample identity.
- Require replayability validation before canonical sample persistence.
- Keep mock responders isolated from sample building and dedupe.
- Treat `websocket-h2` as correctness-first in the first implementation, not fidelity-first.
