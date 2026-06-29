import { describe, expect, it } from "vitest";

import {
  applyOpenAIImageTypeSelection,
  deriveOpenAIImageFormState,
  normalizeOpenAIImageTypeSelection,
  type OpenAIImageSelectionFormState,
} from "../groupsOpenAIImagePricing";

describe("groupsOpenAIImagePricing", () => {
  it("ensures at least one OpenAI image route stays enabled", () => {
    const form: OpenAIImageSelectionFormState = {
      platform: "openai",
      allow_image_generation: true,
      image_generation_route: "codex",
      openai_image_codex_enabled: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      image_price_1k: null,
      image_price_2k: null,
      image_price_4k: null,
    };

    normalizeOpenAIImageTypeSelection(form);

    expect(form.openai_image_codex_enabled).toBe(true);
  });

  it("preserves codex pricing while normalizing route flags", () => {
    const payload = {
      platform: "openai",
      allow_image_generation: true,
      openai_image_codex_enabled: true,
      image_generation_route: "codex" as const,
      image_rate_independent: true,
      image_rate_multiplier: 1.75,
      image_price_1k: 0.1,
      image_price_2k: 0.2,
      image_price_4k: 0.4,
    };

    applyOpenAIImageTypeSelection(payload);

    expect(payload.image_generation_route).toBe("codex");
    expect(payload.image_rate_independent).toBe(false);
    expect(payload.image_rate_multiplier).toBe(1.75);
    expect(payload.image_price_1k).toBe(0.1);
    expect("openai_image_codex_enabled" in payload).toBe(false);
  });

  it("forces Grok create payloads to native image route", () => {
    const payload = {
      platform: "grok",
      allow_image_generation: true,
      image_generation_route: "codex" as const,
      openai_image_codex_enabled: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      image_price_1k: 0.134,
      image_price_2k: null,
      image_price_4k: null,
    };

    applyOpenAIImageTypeSelection(payload);

    expect(payload.image_generation_route).toBe("native");
    expect(payload.image_rate_independent).toBe(false);
    expect("openai_image_codex_enabled" in payload).toBe(false);
  });

  it("keeps Grok image prices but forces native route when image generation is disabled", () => {
    const payload = {
      platform: "grok",
      allow_image_generation: false,
      image_generation_route: "native" as const,
      openai_image_codex_enabled: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      image_price_1k: 0.134,
      image_price_2k: 0.201,
      image_price_4k: 0.268,
    };

    applyOpenAIImageTypeSelection(payload);

    expect(payload.image_generation_route).toBe("native");
    expect(payload.image_rate_independent).toBe(false);
    expect(payload.image_price_1k).toBe(0.134);
    expect(payload.image_price_2k).toBe(0.201);
    expect(payload.image_price_4k).toBe(0.268);
  });

  it("preserves stored OpenAI multiplier when hydrating edit state", () => {
    const formState = deriveOpenAIImageFormState({
      platform: "openai",
      allow_image_generation: true,
      image_rate_independent: true,
      image_rate_multiplier: 2.5,
      image_price_1k: 0.25,
    });

    expect(formState.openai_image_codex_enabled).toBe(true);
    expect(formState.image_rate_multiplier).toBe(2.5);
    expect(formState.image_price_1k).toBe(0.25);
  });

  it("preserves non-OpenAI image independent pricing when hydrating edit state", () => {
    const formState = deriveOpenAIImageFormState({
      platform: "anthropic",
      allow_image_generation: true,
      image_rate_independent: true,
      image_rate_multiplier: 3,
      image_price_1k: 0.3,
      image_price_2k: 0.4,
      image_price_4k: 0.5,
    });

    expect(formState.openai_image_codex_enabled).toBe(false);
    expect(formState.image_rate_independent).toBe(true);
    expect(formState.image_rate_multiplier).toBe(3);
    expect(formState.image_price_1k).toBe(0.3);
    expect(formState.image_price_2k).toBe(0.4);
    expect(formState.image_price_4k).toBe(0.5);
  });

  it("defaults Grok image route to native", () => {
    const formState = deriveOpenAIImageFormState({
      platform: "grok",
      allow_image_generation: true,
      image_generation_route: "web2api",
      image_rate_independent: true,
    });

    expect(formState.image_generation_route).toBe("native");
    expect(formState.openai_image_codex_enabled).toBe(false);
  });

  it("forces Grok payloads to native route handling through hydration only", () => {
    const formState = deriveOpenAIImageFormState({
      platform: "grok",
      allow_image_generation: true,
      image_generation_route: "codex",
      image_rate_independent: true,
    });

    expect(formState.image_generation_route).toBe("native");
    expect(formState.image_rate_independent).toBe(true);
  });
});
