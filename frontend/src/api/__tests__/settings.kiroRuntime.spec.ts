import { describe, expect, it } from "vitest";

import {
  KIRO_CACHE_MIN_BLOCK_TOKENS_MAX,
  normalizeKiroRuntimeSettingsForUpdate,
  validateKiroRuntimeSettings,
} from "@/api/admin/settings";

describe("admin settings kiro runtime helpers", () => {
  it("accepts valid kiro runtime cache settings", () => {
    expect(
      validateKiroRuntimeSettings({
        cache_hit_rate_scale: 80,
        cache_min_block_tokens: 0,
        cache_independent_ttl_seconds: 3600,
        cache_prefix_ttl_seconds: 300,
      }),
    ).toBeNull();
  });

  it("rejects invalid cache ranges and prefix ttl ordering", () => {
    expect(
      validateKiroRuntimeSettings({
        cache_hit_rate_scale: 101,
      }),
    ).toBe("cache_hit_rate_scale_range");

    expect(
      validateKiroRuntimeSettings({
        cache_min_block_tokens: -1,
      }),
    ).toBe("cache_min_block_tokens_range");

    expect(
      validateKiroRuntimeSettings({
        cache_min_block_tokens: KIRO_CACHE_MIN_BLOCK_TOKENS_MAX + 1,
      }),
    ).toBe("cache_min_block_tokens_range");

    expect(
      validateKiroRuntimeSettings({
        cache_independent_ttl_seconds: 59,
      }),
    ).toBe("cache_independent_ttl_seconds_range");

    expect(
      validateKiroRuntimeSettings({
        cache_prefix_ttl_seconds: 3601,
      }),
    ).toBe("cache_prefix_ttl_seconds_range");

    expect(
      validateKiroRuntimeSettings({
        cache_independent_ttl_seconds: 600,
        cache_prefix_ttl_seconds: 601,
      }),
    ).toBe("cache_prefix_ttl_seconds_exceeds_independent");
  });

  it("validates ttl ordering against cleared-field default reset values", () => {
    expect(
      validateKiroRuntimeSettings({
        cache_independent_ttl_seconds: 120,
        cache_prefix_ttl_seconds: null,
      }),
    ).toBe("cache_prefix_ttl_seconds_exceeds_independent");

    expect(
      validateKiroRuntimeSettings({
        cache_independent_ttl_seconds: null,
        cache_prefix_ttl_seconds: 3600,
      }),
    ).toBeNull();
  });

  it("normalizes string fields and keeps optional cache values compact", () => {
    expect(
      normalizeKiroRuntimeSettingsForUpdate({
        kiro_version: " 0.10.0 ",
        kiro_commit: " abc123 ",
        system_version: " darwin#24.6.0 ",
        node_version: " 22.21.1 ",
        kiro_code_execution_sandbox_command: "  sandbox-run --kiro  ",
        cache_hit_rate_scale: 75.9,
        cache_min_block_tokens: 128.4,
        cache_independent_ttl_seconds: 3600.8,
      }),
    ).toEqual({
      kiro_version: "0.10.0",
      kiro_commit: "abc123",
      system_version: "darwin#24.6.0",
      node_version: "22.21.1",
      kiro_code_execution_sandbox_command: "sandbox-run --kiro",
      cache_hit_rate_scale: 75,
      cache_min_block_tokens: 128,
      cache_independent_ttl_seconds: 3600,
    });
  });

  it("maps cleared cache values to the backend defaults", () => {
    expect(
      normalizeKiroRuntimeSettingsForUpdate({
        cache_hit_rate_scale: null,
        cache_min_block_tokens: null,
        cache_independent_ttl_seconds: null,
        cache_prefix_ttl_seconds: null,
      }),
    ).toEqual({
      cache_hit_rate_scale: 85,
      cache_min_block_tokens: 1024,
      cache_independent_ttl_seconds: 3600,
      cache_prefix_ttl_seconds: 3600,
    });
  });

  it("omits unspecified string fields to preserve partial update semantics", () => {
    expect(
      normalizeKiroRuntimeSettingsForUpdate({
        cache_hit_rate_scale: 80,
      }),
    ).toEqual({
      cache_hit_rate_scale: 80,
    });
  });
});
