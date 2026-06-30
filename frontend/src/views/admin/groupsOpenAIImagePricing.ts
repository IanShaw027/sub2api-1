export type OpenAIImageRoute = "codex" | "web2api" | "native";

export type OpenAIImageSelectionFormState = {
  platform: string;
  allow_image_generation: boolean;
  image_generation_route: OpenAIImageRoute;
  openai_image_codex_enabled: boolean;
  image_rate_independent: boolean;
  image_rate_multiplier: number | string | null;
  image_price_1k: number | string | null;
  image_price_2k: number | string | null;
  image_price_4k: number | string | null;
};

export type OpenAIImageGroupLike = {
  platform: string;
  allow_image_generation?: boolean | null;
  image_generation_route?: OpenAIImageRoute | null;
  image_rate_independent?: boolean | null;
  image_rate_multiplier?: number | null;
  image_price_1k?: number | null;
  image_price_2k?: number | null;
  image_price_4k?: number | null;
};

export type OpenAIImageSelectionPayload = Partial<OpenAIImageSelectionFormState> & {
  platform?: string;
};

export function normalizeOpenAIImageTypeSelection(
  form: OpenAIImageSelectionFormState,
): void {
  if (form.platform !== "openai" || !form.allow_image_generation) {
    return;
  }
  if (!form.openai_image_codex_enabled) {
    form.openai_image_codex_enabled = true;
  }
}

export function applyOpenAIImageTypeSelection(
  payload: OpenAIImageSelectionPayload,
): void {
  const platform = (payload.platform || "").trim().toLowerCase();
  if (platform === "grok") {
    payload.image_generation_route = "native";
    payload.image_rate_independent = false;
    payload.image_rate_multiplier = 1;
    delete payload.openai_image_codex_enabled;
    return;
  }

  if (platform !== "openai") {
    delete payload.openai_image_codex_enabled;
    return;
  }

  if (payload.allow_image_generation !== true) {
    payload.image_generation_route = "codex";
    payload.image_rate_independent = false;
    payload.image_rate_multiplier = 1;
    payload.image_price_1k = null;
    payload.image_price_2k = null;
    payload.image_price_4k = null;
    delete payload.openai_image_codex_enabled;
    return;
  }

  payload.image_generation_route = "codex";
  payload.image_rate_independent = false;

  delete payload.openai_image_codex_enabled;
}

export function deriveOpenAIImageFormState(
  group: OpenAIImageGroupLike,
): Pick<
  OpenAIImageSelectionFormState,
  | "allow_image_generation"
  | "image_generation_route"
  | "openai_image_codex_enabled"
  | "image_rate_independent"
  | "image_rate_multiplier"
  | "image_price_1k"
  | "image_price_2k"
  | "image_price_4k"
> {
  const allowImageGeneration = group.allow_image_generation === true;
  const platform = group.platform.trim().toLowerCase();
  const storedRoute = group.image_generation_route;
  const route =
    platform === "grok"
      ? "native"
      : platform === "openai"
        ? "codex"
        : storedRoute === "native" || storedRoute === "web2api" || storedRoute === "codex"
          ? storedRoute
          : "codex";
  const isCodexRoute = route === "codex";
  return {
    allow_image_generation: allowImageGeneration,
    image_generation_route: route as OpenAIImageRoute,
    openai_image_codex_enabled: platform === "openai" && allowImageGeneration && isCodexRoute,
    image_rate_independent:
      platform === "openai" || platform === "grok"
        ? false
        : group.image_rate_independent === true,
    image_rate_multiplier:
      platform === "grok" ? 1 : group.image_rate_multiplier ?? 1,
    image_price_1k: group.image_price_1k ?? null,
    image_price_2k: group.image_price_2k ?? null,
    image_price_4k: group.image_price_4k ?? null,
  };
}
