import { describe, expect, it } from "vitest";

import {
  appendAuthSourceDefaultsToUpdateRequest,
  buildAuthSourceDefaultsState,
  findInvalidPlatformQuotaFields,
  normalizePlatformQuotasMap,
  sanitizePlatformQuotasMap,
  type UpdateSettingsRequest,
  type AuthSourcePlatformQuotaOverridesMap,
  type DefaultPlatformQuotasMap,
} from "@/api/admin/settings";

const allInheritedOverrides: AuthSourcePlatformQuotaOverridesMap = {
  anthropic: { daily: undefined, weekly: undefined, monthly: undefined },
  openai: { daily: undefined, weekly: undefined, monthly: undefined },
  gemini: { daily: undefined, weekly: undefined, monthly: undefined },
  antigravity: { daily: undefined, weekly: undefined, monthly: undefined },
  kiro: { daily: undefined, weekly: undefined, monthly: undefined },
  grok: { daily: undefined, weekly: undefined, monthly: undefined },
};

describe("admin settings auth source defaults helpers", () => {
  it("builds auth source defaults state from flat settings fields", () => {
    const state = buildAuthSourceDefaultsState({
      auth_source_default_email_balance: 9.5,
      auth_source_default_email_concurrency: 3,
      auth_source_default_email_subscriptions: [
        { group_id: 1, validity_days: 30 },
      ],
      auth_source_default_email_grant_on_signup: false,
      auth_source_default_email_grant_on_first_bind: true,
      auth_source_default_linuxdo_balance: 6,
      auth_source_default_linuxdo_concurrency: 8,
      auth_source_default_linuxdo_subscriptions: [
        { group_id: 2, validity_days: 60 },
      ],
      auth_source_default_linuxdo_grant_on_signup: true,
      auth_source_default_linuxdo_grant_on_first_bind: false,
    });

    expect(state.email).toEqual({
      enabled: false,
      balance: 9.5,
      concurrency: 3,
      subscriptions: [{ group_id: 1, validity_days: 30 }],
      grant_on_signup: false,
      grant_on_first_bind: true,
      platform_quotas: allInheritedOverrides,
    });
    expect(state.linuxdo).toEqual({
      enabled: true,
      balance: 6,
      concurrency: 8,
      subscriptions: [{ group_id: 2, validity_days: 60 }],
      grant_on_signup: true,
      grant_on_first_bind: false,
      platform_quotas: allInheritedOverrides,
    });
    expect(state.oidc).toEqual({
      enabled: false,
      balance: 0,
      concurrency: undefined,
      subscriptions: [],
      grant_on_signup: false,
      grant_on_first_bind: false,
      platform_quotas: allInheritedOverrides,
    });
    expect(state.wechat).toEqual({
      enabled: false,
      balance: 0,
      concurrency: undefined,
      subscriptions: [],
      grant_on_signup: false,
      grant_on_first_bind: false,
      platform_quotas: allInheritedOverrides,
    });
  });

  it("defaults grant-on-signup to disabled when settings are missing", () => {
    const state = buildAuthSourceDefaultsState({});

    expect(state.email.grant_on_signup).toBe(false);
    expect(state.email.enabled).toBe(false);
    expect(state.linuxdo.grant_on_signup).toBe(false);
    expect(state.oidc.grant_on_signup).toBe(false);
    expect(state.wechat.grant_on_signup).toBe(false);
  });

  it("reads nested platform_quotas from settings into auth source state", () => {
    const state = buildAuthSourceDefaultsState({
      auth_source_default_email_platform_quotas: {
        anthropic: { daily: 10, weekly: 50, monthly: 200 },
        openai: { daily: null },
      } as AuthSourcePlatformQuotaOverridesMap,
    });

    expect(state.email.platform_quotas.anthropic).toEqual({
      daily: 10,
      weekly: 50,
      monthly: 200,
    });
    expect(state.email.platform_quotas.openai).toEqual({
      daily: null,
      weekly: undefined,
      monthly: undefined,
    });
    expect(state.email.platform_quotas.gemini).toEqual({
      daily: undefined,
      weekly: undefined,
      monthly: undefined,
    });
    expect(state.email.platform_quotas.antigravity).toEqual({
      daily: undefined,
      weekly: undefined,
      monthly: undefined,
    });
  });

  it("appends auth source defaults back onto update payload", () => {
    const payload: UpdateSettingsRequest = {
      site_name: "Sub2API",
    };

    const state = buildAuthSourceDefaultsState({});
    state.email = {
      ...state.email,
      enabled: true,
      balance: 1.25,
      concurrency: 2,
      subscriptions: [{ group_id: 3, validity_days: 7 }],
      grant_on_signup: true,
      grant_on_first_bind: false,
      platform_quotas: allInheritedOverrides,
    };
    state.linuxdo = {
      ...state.linuxdo,
      enabled: true,
      balance: 0,
      concurrency: 0,
      subscriptions: [],
      grant_on_signup: false,
      grant_on_first_bind: true,
      platform_quotas: allInheritedOverrides,
    };
    state.oidc = {
      ...state.oidc,
      enabled: true,
      balance: 4,
      concurrency: 9,
      subscriptions: [{ group_id: 9, validity_days: 90 }],
      grant_on_signup: true,
      grant_on_first_bind: true,
      platform_quotas: allInheritedOverrides,
    };
    state.wechat = {
      ...state.wechat,
      enabled: false,
      balance: 2,
      concurrency: 5,
      subscriptions: [],
      grant_on_signup: true,
      grant_on_first_bind: true,
      platform_quotas: allInheritedOverrides,
    };

    appendAuthSourceDefaultsToUpdateRequest(payload, state);

    expect(payload).toMatchObject({
      site_name: "Sub2API",
      auth_source_default_email_balance: 1.25,
      auth_source_default_email_concurrency: 2,
      auth_source_default_email_subscriptions: [
        { group_id: 3, validity_days: 7 },
      ],
      auth_source_default_email_grant_on_signup: true,
      auth_source_default_email_grant_on_first_bind: false,
      auth_source_default_linuxdo_balance: 0,
      auth_source_default_linuxdo_concurrency: 0,
      auth_source_default_linuxdo_subscriptions: [],
      auth_source_default_linuxdo_grant_on_signup: true,
      auth_source_default_linuxdo_grant_on_first_bind: true,
      auth_source_default_oidc_balance: 4,
      auth_source_default_oidc_concurrency: 9,
      auth_source_default_oidc_subscriptions: [
        { group_id: 9, validity_days: 90 },
      ],
      auth_source_default_oidc_grant_on_signup: true,
      auth_source_default_oidc_grant_on_first_bind: true,
      auth_source_default_wechat_balance: 2,
      auth_source_default_wechat_concurrency: 5,
      auth_source_default_wechat_subscriptions: [],
      auth_source_default_wechat_grant_on_signup: false,
      auth_source_default_wechat_grant_on_first_bind: true,
      auth_source_default_email_platform_quotas: {},
      auth_source_default_linuxdo_platform_quotas: {},
      auth_source_default_oidc_platform_quotas: {},
      auth_source_default_wechat_platform_quotas: {},
      auth_source_default_github_platform_quotas: {},
      auth_source_default_google_platform_quotas: {},
      auth_source_default_dingtalk_platform_quotas: {},
    });
  });

  it("preserves omitted concurrency fields while still serializing explicit zero", () => {
    const state = buildAuthSourceDefaultsState({
      auth_source_default_email_balance: 3,
      auth_source_default_linuxdo_balance: 1,
      auth_source_default_linuxdo_concurrency: 0,
      auth_source_default_oidc_concurrency: 6,
    });

    expect(state.email.concurrency).toBeUndefined();
    expect(state.linuxdo.concurrency).toBe(0);
    expect(state.oidc.concurrency).toBe(6);
    expect(state.wechat.concurrency).toBeUndefined();

    const payload: UpdateSettingsRequest = {
      site_name: "Sub2API",
    };

    appendAuthSourceDefaultsToUpdateRequest(payload, state);

    expect(payload).toMatchObject({
      site_name: "Sub2API",
      auth_source_default_linuxdo_concurrency: 0,
      auth_source_default_oidc_concurrency: 6,
    });
    expect(payload).not.toHaveProperty("auth_source_default_email_concurrency");
    expect(payload).not.toHaveProperty("auth_source_default_wechat_concurrency");
  });

  it("appends sanitized nested platform_quotas with non-null values in update payload", () => {
    const payload: UpdateSettingsRequest = {};
    const state = buildAuthSourceDefaultsState({});

    state.email = {
      ...state.email,
      enabled: true,
      platform_quotas: {
        anthropic: { daily: 10, weekly: 50, monthly: 200 },
        openai: { daily: 0, weekly: null, monthly: null },
      },
    };

    appendAuthSourceDefaultsToUpdateRequest(payload, state);

    const emailQuotas = (payload as Record<string, unknown>)[
      "auth_source_default_email_platform_quotas"
    ] as DefaultPlatformQuotasMap;
    expect(emailQuotas.anthropic).toEqual({
      daily: 10,
      weekly: 50,
      monthly: 200,
    });
    expect(emailQuotas.openai?.daily).toBe(0);
    expect(emailQuotas).not.toHaveProperty("gemini");
    expect(emailQuotas).not.toHaveProperty("antigravity");
  });

  it("omits inherited auth-source quota windows while preserving explicit null overrides", () => {
    const payload: UpdateSettingsRequest = {};
    const state = buildAuthSourceDefaultsState({});

    state.email = {
      ...state.email,
      platform_quotas: {
        anthropic: { daily: undefined, weekly: null, monthly: 20 },
      },
    };

    appendAuthSourceDefaultsToUpdateRequest(payload, state);

    expect(payload.auth_source_default_email_platform_quotas).toEqual({
      anthropic: { weekly: null, monthly: 20 },
    });
  });
});

describe("normalizePlatformQuotasMap", () => {
  it("fills missing platforms with null windows", () => {
    const result = normalizePlatformQuotasMap({
      anthropic: { daily: 5, weekly: null, monthly: null },
    });
    expect(result.anthropic).toEqual({ daily: 5, weekly: null, monthly: null });
    expect(result.openai).toEqual({ daily: null, weekly: null, monthly: null });
    expect(result.gemini).toEqual({ daily: null, weekly: null, monthly: null });
    expect(result.antigravity).toEqual({ daily: null, weekly: null, monthly: null });
    expect(result.kiro).toEqual({ daily: null, weekly: null, monthly: null });
    expect(result.grok).toEqual({ daily: null, weekly: null, monthly: null });
  });

  it("returns all-null quotas when input is omitted", () => {
    const result = normalizePlatformQuotasMap();
    expect(Object.keys(result)).toHaveLength(6);
    for (const value of Object.values(result)) {
      expect(value).toEqual({ daily: null, weekly: null, monthly: null });
    }
  });

  it("normalizes non-numeric values to null", () => {
    const result = normalizePlatformQuotasMap({
      anthropic: {
        daily: "50" as unknown as number,
        weekly: undefined as unknown as number,
        monthly: null,
      },
    });
    expect(result.anthropic).toEqual({ daily: null, weekly: null, monthly: null });
  });
});

describe("sanitizePlatformQuotasMap", () => {
  it("keeps valid positive and zero values", () => {
    const result = sanitizePlatformQuotasMap({
      anthropic: { daily: 10.5, weekly: 0, monthly: null },
    });
    expect(result.anthropic?.daily).toBe(10.5);
    expect(result.anthropic?.weekly).toBe(0);
    expect(result.anthropic?.monthly).toBe(null);
  });

  it("converts empty-string numeric inputs to null", () => {
    const result = sanitizePlatformQuotasMap({
      anthropic: { daily: "" as unknown as number, weekly: null, monthly: null },
    });
    expect(result.anthropic?.daily).toBe(null);
  });

  it("converts negative values to null", () => {
    const result = sanitizePlatformQuotasMap({
      openai: { daily: -1, weekly: null, monthly: null },
    });
    expect(result.openai?.daily).toBe(null);
  });

  it("converts NaN and Infinity to null", () => {
    const result = sanitizePlatformQuotasMap({
      gemini: { daily: NaN, weekly: Infinity, monthly: null },
    });
    expect(result.gemini?.daily).toBe(null);
    expect(result.gemini?.weekly).toBe(null);
  });

  it("fills missing platforms with all-null quotas", () => {
    const result = sanitizePlatformQuotasMap({});
    expect(Object.keys(result)).toHaveLength(6);
    for (const value of Object.values(result)) {
      expect(value).toEqual({ daily: null, weekly: null, monthly: null });
    }
  });
});

describe("findInvalidPlatformQuotaFields", () => {
  it("allows empty-string clears but flags invalid intermediate values", () => {
    const result = findInvalidPlatformQuotaFields({
      anthropic: { daily: "" as unknown as number, weekly: null, monthly: null },
      openai: { daily: Number.NaN, weekly: null, monthly: null },
      gemini: { daily: "-" as unknown as number, weekly: null, monthly: null },
      antigravity: { daily: -1, weekly: null, monthly: null },
    });

    expect(result).toEqual(["openai.daily", "gemini.daily", "antigravity.daily"]);
  });
});
