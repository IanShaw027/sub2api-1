# Creation Workspace Contract

## Product Baseline

The creation center adapts the workflows of `pigzwy/chat-vue` at
`80649c38cb40ceeba191161cfddd9d6d83d85f1f`; it is not an embedded copy of
that application or a generic administration table. User-provided screenshots
also guide the mode navigation and uncluttered workspace layout.

- A creation category in the shared application sidebar with chat, images,
  videos, voice, and inspiration entries. Each entry uses AppLayout, the shared
  header, breadcrumb and page header; there is no separate full-screen shell.
- Canonical routes are `/studio/chat`, `/studio/image`, `/studio/video`,
  `/studio/voice` and `/studio/gallery`. Legacy `/studio?mode=...` links redirect
  to these routes while preserving prompt links. Cross-mode creation actions
  navigate through these same routes and keep existing local drafts.
- User menus are grouped into API services, creation, subscriptions/billing,
  account and support. Feature flags and route authorization still apply.
- Only chat has a persistent conversation sidebar.
- Images and videos use a task wall, contextual result actions, local history,
  selection/batch operations, and a bottom composer with model parameters.
- Parameters, groups, permissions and billing follow Sub2API capabilities,
  not upstream hard-coded provider IDs, account balances or static prices.
- Unsupported upstream capabilities must not be presented as working controls.

## Media Storage Boundary

New image and video creation is local-first. Browser IndexedDB contains task
history, prompts, parameter snapshots, source media and completed media blobs,
scoped by account. Object URLs are display handles, not persistent storage.
No cloud session or permanent private artwork is created by the new media flow.

Generation necessarily sends prompts and selected source material to the
gateway and upstream provider. Short-lived task execution records and the
existing authentication, usage and billing records are distinct from a
permanent artwork library. Local-first does not mean inference runs offline or
that the gateway controls the upstream provider's retention policy.

Saving locally and publishing are separate actions:

- Local results remain local unless their owner explicitly publishes them.
- "Publish to cloud" means public publication, not private cloud backup.
- Publication transfers the selected media blob and the user-confirmed title,
  prompt and model metadata, with an idempotent request identifier.
- Only published records appear in the inspiration wall. Unpublished local
  data and legacy private cloud records never become public automatically.
- Unpublishing removes access through the application. It cannot revoke copies
  that other users already downloaded while the work was public.
- Browser storage can be cleared or evicted. Storage failures must be visible;
  failed local writes cannot be reported as saved. Download remains an explicit
  backup operation.

Legacy cloud image history remains a separate private history view. Importing
an existing result into local storage does not regenerate, charge, delete the
original, or publish it. Legacy cloud chat is also an explicit read/import
drawer; new conversations do not write the legacy session or message APIs.

## Local Chat

New conversations, drafts, attachments, votes, generation settings, reasoning,
sources and chart tool results are stored in account-scoped IndexedDB. Editing
or regenerating creates a branch and preserves the original conversation.
Stopping retains received text; incomplete or failed streams are not marked
successful. Export and explicit legacy import do not generate or publish data.

Image, PDF and text attachments are sent as native provider content or inline
text, subject to model capability and size checks. Cross-provider PDF bridges
reject unsupported sources rather than silently dropping them. Chart tools
validate model-supplied data and use a bounded five-round execution loop; they
do not query an external statistical source. Random reference weather output
and disabled reference sharing APIs are intentionally not presented as features.

## Media Operations and Billing

OpenAI/Grok image workflows retain their gateway adapters. Gemini image
generation and editing use native generateContent through the existing gateway,
including permission, concurrency, moderation and billing. Gemini image tiers
and ratios are sent as imageConfig, not OpenAI size strings.

Video generation, editing and extension are separate operations. Editing sends
the source video with model and prompt, not generation-only size settings.
Extension has its own source-duration and added-duration constraints. Source
files remain local and are submitted inline to inference, not published first.

Quotes use the application's pricing configuration and account/group media
multipliers. Unknown, token-dependent or ambiguous prices remain unavailable,
not zero. Receipts resolve the owned task and its private billing association,
then read committed usage; a pending receipt is not a second generation or
charge. Quote and receipt routes use JWT and feature guards without creating a
new generation key or applying a new-generation balance gate.

## Voice and Inspiration

Voice supports synthesis, transcription and Grok realtime calls. A JWT request
issues a short-lived, single-use ticket whose hash is stored in Redis. The
same-origin WebSocket sends it as a subprotocol, not a query parameter, and
revalidates identity and access. Microphone capture and playback stop on hangup,
unmount and account change. Already observed audio is settled on abnormal close
as well as normal close. No music service exists in the reference commit.

The curated gallery includes the complete 100-case CC-BY-4.0 collection at the
recorded source revision, with local previews, full prompts and attribution.
Other reference collections are excluded where redistribution permission has
not been verified. Curated cases and user-published artwork remain distinct.

## Verification Expectations

Verify local persistence across reload, account isolation, no automatic cloud
publication, editing and retry without losing source files, temporary-task
recovery, explicit publication confirmation, idempotent publication retry,
owner-only withdrawal, and inaccessible withdrawn content. Exercise desktop
and mobile workspaces with actual media blobs and real browser IndexedDB.

## Execution Recovery

Local image generation uses a separate local-only view of the temporary task
store, with no S3 resolver/uploader. Local and legacy cloud task services reject
each other's records. Completed image results are returned as bounded base64
payloads (32 MiB maximum JSON result) and expire from the task store after one
hour. The browser must download the result into IndexedDB to retain it.

An accepted local video starts a server-side status observer independent of
browser cancellation: five-second polling, a 30-second limit per status request,
and a 30-minute overall observation window. It uses the existing owner/account
binding, original subscription context and task-scoped billing claim. Browser
polling/content downloads share the same completion billing path, not another
charge. Read-only retrieval does not require balance for a new generation.
Observation is capped at 32 local videos across instances sharing Redis. An
atomic queue reservation fails with 503 before any upstream request when Redis
is unavailable or capacity is full. Reservations without a confirmed provider
request ID expire after two minutes. Accepted jobs expire after 30 minutes.
Local-video submission runs independently of browser cancellation with a
90-second total upstream budget, including queueing and account failover. A
higher configured provider response-header timeout does not extend this local
budget; native non-local video endpoints keep their existing timeout behavior.
After a readable acceptance, owner/account and pricing metadata have a separate
10-second persistence budget and queue binding has five seconds. Together with
the reservation round trip, these bounds stay below the two-minute reservation.
The queue retains only internal user/group/API-key/subscription IDs, provider
request ID and lease/deadline metadata, never prompts, source files, JWTs,
cookies, user agents or client IPs. Each poll reloads its billing identity.

Four workers per instance claim jobs with fenced one-minute leases. After an
application restart or instance failure, another worker resumes the original
provider task when its lease expires. Redis itself must retain the queue; a
Redis data loss is not covered by application-process recovery. Successful
billable tasks are removed only after the durable billing-dedup record exists.
An interrupted in-memory billing claim can be released and retried using the
existing stable task billing ID, whose database uniqueness prevents a second
charge. Failed/expired provider tasks and elapsed observation windows are
removed without retaining artwork.

Upstream acceptance and local request-ID binding cannot be one transaction.
If the upstream accepts a task but the process dies before receiving/persisting
its ID, recovery cannot infer that ID without a provider idempotency contract.
The same uncertainty applies when a network failure or submission deadline
prevents receipt of an already-accepted provider response.
If Redis binding fails after a readable acceptance response, the gateway makes
a bounded persistence retry, logs the task ID, and still returns HTTP 202 with
the original request ID and `observation_persisted: false`. The browser warns
and continues polling that same task; it must not resubmit generation. Browser
recovery can continue while the native Redis ownership binding remains valid
(currently 24 hours). Provider content has its own expiry. Temporary execution
and billing metadata are not a permanent artwork library.

Existing safety and operational audit policies are not disabled by local-first
storage. Moderation may retain redacted input excerpts according to configured
retention (defaults: flagged 180 days, non-flagged 3 days). This remains distinct
from creating a permanent image/video history or a public gallery asset.
