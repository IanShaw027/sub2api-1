# OpenAI/Claude Compatibility Rollout

This document covers the rollout, verification, and merge order for the `OpenAI/Claude` compatibility work on branch `opt/openai-claude-compat-optimization-20260422`.

## Scope

Completed work on this branch:

1. Claude tool-name identity preservation across the bridge
2. Capability-aware Codex/OpenAI request shaping
3. Stable `prompt_cache_key` request encoding
4. Fallback-only embedded instructions
5. Golden fixtures for bridge stability
6. Centralized compat model capability registry
7. Runtime observability counters for compat behavior
8. Billing fallback aliases for newly explicit compat models
9. Unified usage model resolution for requested/billing/upstream views

## Recommended Merge Order

Apply the branch commits in this order:

1. `6ad6b3f7` `fix(openai): preserve claude tool identity across bridge`
2. `5c200185` `refactor(openai): make codex shaping capability-aware`
3. `74bd2281` `perf(openai): stabilize prompt cache request encoding`
4. `fe67bd25` `refactor(openai): limit embedded instructions to fallback use`
5. `00f07ec5` `docs: document openai claude compatibility contract`
6. `de62997f` `test(openai): add golden fixtures for claude bridge`
7. `f3cff230` `refactor(openai): centralize compat model capability registry`
8. `aa2e7042` `feat(openai): add compat runtime observability metrics`
9. `32cea731` `refactor(openai): formalize usage model resolution`

Rationale for this order:

- `6ad6b3f7` to `fe67bd25` are the bridge-behavior core.
- `00f07ec5` must land before later billing-sensitive verification because it restores fallback price matching for explicit compat model names.
- `de62997f` locks the behavior with fixtures before the registry and observability refactors expand the surface area.
- `f3cff230` and `aa2e7042` are cross-cutting infrastructure.
- `32cea731` is the final consolidation step for usage recording and should land after the compat behavior is stable.

## Verification Before Sync

Run these commands from `backend/` before syncing to `main`:

```bash
go test ./...
go test ./internal/pkg/apicompat ./internal/service -run 'GoldenFixtures|Golden|ResponsesToAnthropic|ToolUse'
go test ./internal/service -run 'ResolveOpenAIModelCapabilities|ResolveCodexRequestProfile|ShouldAutoInjectPromptCacheKeyForCompat|ApplyCodexOAuthTransform'
go test ./internal/service -run 'OpenAICompatRuntimeMetrics|RecordsCompat'
go test ./internal/service -run 'ResolveUsageModelView|OpenAIGatewayServiceRecordUsage|BillingModel|RequestedModelAndUpstreamModel|ChannelMapped|BillsMappedRequests'
```

Expected outcome:

- all commands pass
- no golden fixture drift
- no regression in compat request shaping
- no regression in usage recording

## Canary Rollout Plan

Use a staged rollout instead of pushing the entire compat line into all OpenAI/Claude traffic at once.

### Stage 1: Internal Verification

- deploy only to internal or low-risk groups
- verify Claude Code tool calls can continue across multi-turn tool use
- verify Codex/OAuth requests still succeed when `verbosity`, `temperature`, or `top_p` are stripped
- verify prompt cache hit behavior remains stable for repeated identical requests

### Stage 2: Limited Canary

- route a small subset of production groups to the new build
- keep the old build available for fast rollback
- monitor the counters introduced by compat observability

Recommended watchpoints:

- `StrippedVerbosityTotal`
- `StrippedTemperatureTotal`
- `StrippedTopPTotal`
- `PromptCacheInjectedTotal`
- `ToolContinuationDetectedTotal`
- `Upstream4xxByModel`
- `Upstream5xxByModel`

### Stage 3: Wider Rollout

- expand only after Stage 2 remains stable for a full traffic cycle
- compare upstream `4xx` and `5xx` rate against pre-rollout baseline
- spot check usage logs for `requested_model` and `upstream_model`
- confirm no unexpected increase in compat-related customer complaints

## Rollback Triggers

Rollback immediately when any of the following appears after rollout:

- Claude tool continuation breaks across the bridge
- upstream `4xx` rises materially for mapped OpenAI/Codex models
- upstream `5xx` rises materially for the compat endpoints
- prompt cache hit rate drops sharply on repeated Codex-style requests
- usage logs show obviously inconsistent requested/upstream model attribution

## Rollback Procedure

1. stop routing new compat traffic to the new deployment
2. restore the previous stable build
3. keep the new usage logs and runtime counters for postmortem comparison
4. compare failing requests against the golden fixture scenarios first
5. only re-roll after the failing shape is covered by a regression test

## Notes On Billing

This branch does not change the global billing policy to "always bill by upstream model".

The billing-related changes here are narrower:

- explicit compat model names now resolve to fallback prices correctly
- usage recording now resolves `requested`, `billing`, and `upstream` model views through a shared helper

The current repository still supports `requested`, `upstream`, and `channel_mapped` billing sources. Any later change to always bill by `upstream_model` should be handled as a separate policy refactor.
