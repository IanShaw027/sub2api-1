<template>
<!-- 视频生成计费配置（仅 Grok 平台） -->
<div
  v-if="supportsVideoPricingPlatform(form.platform)"
  class="border-t pt-4"
>
  <label
    class="block mb-2 font-medium text-foreground"
  >
    {{ t(videoPricingI18nKey("title")) }}
  </label>
  <p class="text-xs text-muted mb-3">
    {{ t(videoPricingI18nKey("description")) }}
  </p>
  <div class="mb-4">
    <label class="flex items-center gap-2 text-sm text-foreground">
      <input
        v-model="form.video_rate_independent"
        type="checkbox"
        class="rounded border-line text-accent focus:ring-accent-500"
      />
      {{ t(videoPricingI18nKey("independentMultiplier")) }}
    </label>
  </div>
  <div
    v-if="form.video_rate_independent"
    class="mb-4"
  >
    <label class="input-label">{{
      t(videoPricingI18nKey("videoMultiplier"))
    }}</label>
    <input
      v-model.number="form.video_rate_multiplier"
      type="number"
      step="0.0001"
      min="0"
      class="input"
      placeholder="1"
    />
  </div>
  <div class="grid grid-cols-3 gap-3">
    <div>
      <label class="input-label">480p ($/s)</label>
      <input
        v-model.number="form.video_price_480p"
        type="number"
        step="0.001"
        min="0"
        class="input"
        :placeholder="getVideoPricePlaceholder(form.platform, 'video_price_480p')"
      />
    </div>
    <div>
      <label class="input-label">720p ($/s)</label>
      <input
        v-model.number="form.video_price_720p"
        type="number"
        step="0.001"
        min="0"
        class="input"
        :placeholder="getVideoPricePlaceholder(form.platform, 'video_price_720p')"
      />
    </div>
    <div>
      <label class="input-label">1080p ($/s)</label>
      <input
        v-model.number="form.video_price_1080p"
        type="number"
        step="0.001"
        min="0"
        class="input"
        :placeholder="getVideoPricePlaceholder(form.platform, 'video_price_1080p')"
      />
    </div>
  </div>
  <div
    class="mt-4 border-t border-dashed border-line pt-4"
    :data-testid="`${testIdPrefix}-grok-video-model-prices`"
  >
    <p class="text-sm font-medium text-foreground">
      {{ t("admin.groups.videoPricing.modelOverridesTitle") }}
    </p>
    <p class="mt-1 text-xs text-muted">
      {{ t("admin.groups.videoPricing.modelOverridesDescription") }}
    </p>
    <div class="mt-3 space-y-3">
      <div
        v-for="family in videoModelPriceFamilyRows(form.video_model_prices)"
        :key="family.key"
        class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_repeat(3,minmax(0,7rem))] sm:items-end"
      >
        <div class="min-w-0 pb-1 font-mono text-xs text-foreground">
          {{ family.label }}
        </div>
        <label
          v-for="resolution in grokVideoPriceResolutions"
          :key="resolution.key"
          class="block"
        >
          <span class="mb-1 block text-xs text-muted">
            {{ resolution.label }} ($/s)
          </span>
          <input
            v-model.number="form.video_model_prices[family.key][resolution.key]"
            type="number"
            step="0.001"
            min="0"
            class="input"
            :data-testid="`${testIdPrefix}-grok-video-price-${family.key}-${resolution.key}`"
          />
        </label>
      </div>
    </div>
  </div>
  <p class="mt-3 text-xs text-muted">
    {{ t(videoPricingI18nKey("modeHint")) }}
  </p>
  <div class="mt-2 rounded-lg bg-surface-2 p-3 text-xs text-foreground ">
    <div class="mb-1 font-medium">
      {{ t(videoPricingI18nKey("finalPricePreview")) }}
    </div>
    <div class="grid grid-cols-3 gap-2">
      <div
        v-for="item in videoFinalPricePreview"
        :key="item.label"
      >
        {{ item.label }}: {{ item.value }}
      </div>
    </div>
  </div>
</div>

</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import {
  getDefaultVideoPreviewPrice,
  getVideoPricePlaceholder,
  supportsVideoPricingPlatform,
  videoPricingI18nKey,
} from "@/views/admin/groupsImagePricing";
import {
  grokVideoPriceResolutions,
  videoModelPriceFamilyRows,
} from "@/views/admin/groupsVideoModelPricing";
import {
  normalizePreviewNumber,
  parsePreviewPrice,
  videoPricingTiers,
  type VideoPricingFormState,
} from "./groupFormShared";

const form = defineModel<VideoPricingFormState>("form", { required: true });
defineProps<{
  testIdPrefix: string;
}>();

const { t } = useI18n();

const formatVideoPricePreview = (value: number | string | null | undefined) => {
  if (value === null || value === undefined || value === "") {
    return t("admin.groups.videoPricing.notConfigured");
  }
  const price = Number(value);
  if (!Number.isFinite(price) || price < 0) {
    return t("admin.groups.videoPricing.notConfigured");
  }
  return `$${price.toFixed(6).replace(/0+$/, "").replace(/\.$/, "")}`;
};

const buildVideoFinalPricePreview = (form: VideoPricingFormState) => {
  const multiplier = form.video_rate_independent
    ? normalizePreviewNumber(form.video_rate_multiplier, 1)
    : normalizePreviewNumber(form.rate_multiplier, 1);
  return videoPricingTiers.map((tier) => {
    const basePrice =
      parsePreviewPrice(form[tier.key]) ??
      getDefaultVideoPreviewPrice(form.platform, tier.key);
    return {
      label: tier.label,
      value: basePrice !== null
        ? formatVideoPricePreview(basePrice * multiplier)
        : t("admin.groups.videoPricing.notConfigured"),
    };
  });
};

const videoFinalPricePreview = computed(() =>
  buildVideoFinalPricePreview(form.value),
);
</script>
