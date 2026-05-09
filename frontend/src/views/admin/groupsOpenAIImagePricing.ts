export type OpenAIImageRoute = "codex" | "web2api";

export type OpenAIImageSelectionFormState = {
  platform: string;
  allow_image_generation: boolean;
  image_generation_route: OpenAIImageRoute;
  openai_image_codex_enabled: boolean;
  openai_image_web2api_enabled: boolean;
  image_rate_independent: boolean;
  image_rate_multiplier: number | string | null;
  image_price_1k: number | string | null;
  image_price_2k: number | string | null;
  image_price_4k: number | string | null;
  images2api_price_1k: number | string | null;
  images2api_price_2k: number | string | null;
  images2api_price_4k: number | string | null;
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
  images2api_price_1k?: number | null;
  images2api_price_2k?: number | null;
  images2api_price_4k?: number | null;
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
  if (!form.openai_image_codex_enabled && !form.openai_image_web2api_enabled) {
    form.openai_image_codex_enabled = true;
  }
}

export function applyOpenAIImageTypeSelection(
  payload: OpenAIImageSelectionPayload,
): void {
  if (payload.platform !== "openai") {
    delete payload.openai_image_codex_enabled;
    delete payload.openai_image_web2api_enabled;
    return;
  }

  if (payload.allow_image_generation !== true) {
    payload.image_generation_route = "codex";
    payload.image_rate_independent = false;
    payload.image_rate_multiplier = 1;
    payload.image_price_1k = null;
    payload.image_price_2k = null;
    payload.image_price_4k = null;
    payload.images2api_price_1k = null;
    payload.images2api_price_2k = null;
    payload.images2api_price_4k = null;
    delete payload.openai_image_codex_enabled;
    delete payload.openai_image_web2api_enabled;
    return;
  }

  const codexEnabled = payload.openai_image_codex_enabled === true;
  const web2apiEnabled = payload.openai_image_web2api_enabled === true;

  payload.image_generation_route = codexEnabled ? "codex" : "web2api";
  payload.image_rate_independent = web2apiEnabled;

  delete payload.openai_image_codex_enabled;
  delete payload.openai_image_web2api_enabled;
}

export function deriveOpenAIImageFormState(
  group: OpenAIImageGroupLike,
): Pick<
  OpenAIImageSelectionFormState,
  | "allow_image_generation"
  | "image_generation_route"
  | "openai_image_codex_enabled"
  | "openai_image_web2api_enabled"
  | "image_rate_independent"
  | "image_rate_multiplier"
  | "image_price_1k"
  | "image_price_2k"
  | "image_price_4k"
  | "images2api_price_1k"
  | "images2api_price_2k"
  | "images2api_price_4k"
> {
  const allowImageGeneration = group.allow_image_generation === true;
  const route = group.image_generation_route || "codex";
  return {
    allow_image_generation: allowImageGeneration,
    image_generation_route: route,
    openai_image_codex_enabled:
      group.platform === "openai" && allowImageGeneration ? route === "codex" : false,
    openai_image_web2api_enabled:
      group.platform === "openai" && allowImageGeneration
        ? group.image_rate_independent === true || route === "web2api"
        : false,
    image_rate_independent: group.image_rate_independent ?? false,
    image_rate_multiplier: group.image_rate_multiplier ?? 1,
    image_price_1k: group.image_price_1k ?? null,
    image_price_2k: group.image_price_2k ?? null,
    image_price_4k: group.image_price_4k ?? null,
    images2api_price_1k: group.images2api_price_1k ?? null,
    images2api_price_2k: group.images2api_price_2k ?? null,
    images2api_price_4k: group.images2api_price_4k ?? null,
  };
}
