<template>
<!-- 图片生成计费配置 -->
<div
  v-if="supportsImagePricingPlatform(form.platform)"
  class="border-t pt-4"
>
  <label
    class="block mb-2 font-medium text-foreground"
  >
    {{ t(imagePricingI18nKey(form.platform, "title")) }}
  </label>
  <p class="text-xs text-muted mb-3">
    {{ t(imagePricingI18nKey(form.platform, "description")) }}
  </p>
  <div class="mb-4 grid grid-cols-1 gap-3 md:grid-cols-2">
    <label class="flex items-center gap-2 text-sm text-foreground">
      <input
        v-model="form.allow_image_generation"
        type="checkbox"
        class="rounded border-line text-accent focus:ring-accent-500"
      />
      {{ t(imagePricingI18nKey(form.platform, "allowImageGeneration")) }}
    </label>
    <label class="flex items-center gap-2 text-sm text-foreground">
      <input
        v-model="form.image_rate_independent"
        type="checkbox"
        class="rounded border-line text-accent focus:ring-accent-500"
      />
      {{ t(imagePricingI18nKey(form.platform, "independentMultiplier")) }}
    </label>
  </div>
  <div
    v-if="form.image_rate_independent"
    class="mb-4"
  >
    <label class="input-label">{{
      t(imagePricingI18nKey(form.platform, "imageMultiplier"))
    }}</label>
    <input
      v-model.number="form.image_rate_multiplier"
      type="number"
      step="0.0001"
      min="0"
      class="input"
      placeholder="1"
    />
  </div>
  <div class="grid grid-cols-3 gap-3">
    <div>
      <label class="input-label">1K ($)</label>
      <input
        v-model.number="form.image_price_1k"
        type="number"
        step="0.001"
        min="0"
        class="input"
        :placeholder="getImagePricePlaceholder(form.platform, 'image_price_1k')"
      />
    </div>
    <div>
      <label class="input-label">2K ($)</label>
      <input
        v-model.number="form.image_price_2k"
        type="number"
        step="0.001"
        min="0"
        class="input"
        :placeholder="getImagePricePlaceholder(form.platform, 'image_price_2k')"
      />
    </div>
    <div>
      <label class="input-label">4K ($)</label>
      <input
        v-model.number="form.image_price_4k"
        type="number"
        step="0.001"
        min="0"
        class="input"
        :placeholder="getImagePricePlaceholder(form.platform, 'image_price_4k')"
      />
    </div>
  </div>
  <p class="mt-3 text-xs text-muted">
    {{ t(imagePricingI18nKey(form.platform, "modeHint")) }}
  </p>
  <div class="mt-2 rounded-lg bg-surface-2 p-3 text-xs text-foreground ">
    <div class="mb-1 font-medium">
      {{ t(imagePricingI18nKey(form.platform, "finalPricePreview")) }}
    </div>
    <div class="grid grid-cols-3 gap-2">
      <div
        v-for="item in imageFinalPricePreview"
        :key="item.label"
      >
        {{ item.label }}: {{ item.value }}
      </div>
    </div>
  </div>
  <div v-if="form.platform === 'gemini' && form.allow_image_generation" class="mt-4 border-t border-dashed border-line pt-4">
    <label
      class="flex items-center gap-2 text-sm font-medium text-foreground"
    >
      <input
        v-model="form.allow_batch_image_generation"
        type="checkbox"
        class="rounded border-line text-accent focus:ring-accent-500"
      />
      {{ t("admin.groups.imagePricing.allowBatchImageGeneration") }}
    </label>
    <p class="mt-2 text-xs text-muted">
      {{ t("admin.groups.imagePricing.batchSectionHint") }}
    </p>
    <div
      v-if="form.allow_batch_image_generation"
      class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2"
    >
      <div>
        <label class="input-label">{{
          t("admin.groups.imagePricing.batchDiscountMultiplier")
        }}</label>
        <input
          v-model.number="form.batch_image_discount_multiplier"
          type="number"
          step="0.0001"
          min="0"
          class="input"
          placeholder="0.5"
        />
      </div>
      <div>
        <label class="input-label">{{
          t("admin.groups.imagePricing.batchHoldMultiplier")
        }}</label>
        <input
          v-model.number="form.batch_image_hold_multiplier"
          type="number"
          step="0.0001"
          min="0"
          class="input"
          placeholder="0.6"
        />
      </div>
    </div>
  </div>
  <p
    v-else-if="form.platform !== 'gemini'"
    class="mt-4 border-t border-dashed border-line pt-4 text-xs text-muted"
  >
    {{ t("admin.groups.imagePricing.batchGeminiOnlyHint") }}
  </p>
</div>

</template>

<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import {
  getDefaultImagePreviewPrice,
  getImagePricePlaceholder,
  imagePricingI18nKey,
  supportsImagePricingPlatform,
} from "@/views/admin/groupsImagePricing";
import {
  imagePricingTiers,
  normalizePreviewNumber,
  parsePreviewPrice,
  type ImagePricingFormState,
} from "./groupFormShared";

const form = defineModel<ImagePricingFormState>("form", { required: true });

const { t } = useI18n();

const formatImagePricePreview = (value: number | string | null | undefined) => {
  if (value === null || value === undefined || value === "") {
    return t("admin.groups.imagePricing.notConfigured");
  }
  const price = Number(value);
  if (!Number.isFinite(price) || price < 0) {
    return t("admin.groups.imagePricing.notConfigured");
  }
  return `$${price.toFixed(6).replace(/0+$/, "").replace(/\.$/, "")}`;
};

const buildImageFinalPricePreview = (form: ImagePricingFormState) => {
  const imageMultiplier = form.image_rate_independent
    ? normalizePreviewNumber(form.image_rate_multiplier, 1)
    : normalizePreviewNumber(form.rate_multiplier, 1);
  const multiplier = imageMultiplier;
  return imagePricingTiers.map((tier) => {
    const basePrice =
      parsePreviewPrice(form[tier.key]) ??
      getDefaultImagePreviewPrice(form.platform, tier.key);
    return {
      label: tier.label,
      value: basePrice !== null
        ? formatImagePricePreview(basePrice * multiplier)
        : t("admin.groups.imagePricing.notConfigured"),
    };
  });
};

const imageFinalPricePreview = computed(() =>
  buildImageFinalPricePreview(form.value),
);
</script>
