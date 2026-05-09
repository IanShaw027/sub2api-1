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
      openai_image_web2api_enabled: false,
      image_rate_independent: false,
      image_rate_multiplier: 1,
      image_price_1k: null,
      image_price_2k: null,
      image_price_4k: null,
      images2api_price_1k: null,
      images2api_price_2k: null,
      images2api_price_4k: null,
    };

    normalizeOpenAIImageTypeSelection(form);

    expect(form.openai_image_codex_enabled).toBe(true);
    expect(form.openai_image_web2api_enabled).toBe(false);
  });

  it("preserves multiplier and inactive route pricing while normalizing route flags", () => {
    const payload = {
      platform: "openai",
      allow_image_generation: true,
      openai_image_codex_enabled: true,
      openai_image_web2api_enabled: false,
      image_generation_route: "web2api" as const,
      image_rate_independent: true,
      image_rate_multiplier: 1.75,
      image_price_1k: 0.1,
      image_price_2k: 0.2,
      image_price_4k: 0.4,
      images2api_price_1k: 1.1,
      images2api_price_2k: 1.2,
      images2api_price_4k: 1.4,
    };

    applyOpenAIImageTypeSelection(payload);

    expect(payload.image_generation_route).toBe("codex");
    expect(payload.image_rate_independent).toBe(false);
    expect(payload.image_rate_multiplier).toBe(1.75);
    expect(payload.image_price_1k).toBe(0.1);
    expect(payload.images2api_price_1k).toBe(1.1);
    expect("openai_image_codex_enabled" in payload).toBe(false);
    expect("openai_image_web2api_enabled" in payload).toBe(false);
  });

  it("preserves stored OpenAI multiplier when hydrating edit state", () => {
    const formState = deriveOpenAIImageFormState({
      platform: "openai",
      allow_image_generation: true,
      image_generation_route: "web2api",
      image_rate_independent: true,
      image_rate_multiplier: 2.5,
      image_price_1k: 0.25,
      images2api_price_1k: 1.25,
    });

    expect(formState.openai_image_codex_enabled).toBe(false);
    expect(formState.openai_image_web2api_enabled).toBe(true);
    expect(formState.image_rate_multiplier).toBe(2.5);
    expect(formState.image_price_1k).toBe(0.25);
    expect(formState.images2api_price_1k).toBe(1.25);
  });
});
