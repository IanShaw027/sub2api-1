<template>
<!-- OpenAI Messages 调度配置（OpenAI 与 Composite 平台） -->
<div
  v-if="supportsMessagesDispatchPlatform(form.platform)"
  class="border-t border-line  pt-4 mt-4"
>
  <h4 class="text-sm font-medium text-foreground mb-3">
    {{ t("admin.groups.openaiMessages.title") }}
  </h4>

  <!-- 允许 Messages 调度开关 -->
  <div class="flex items-center justify-between">
    <label class="text-sm text-muted">{{
      t("admin.groups.openaiMessages.allowDispatch")
    }}</label>
    <button
      type="button"
      @click="
        form.allow_messages_dispatch =
          !form.allow_messages_dispatch
      "
      class="relative inline-flex h-6 w-12 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none"
      :class="form.allow_messages_dispatch
 ? 'bg-accent'
 : 'bg-surface-3 '"
    >
      <span
        class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-surface shadow ring-0 transition duration-200 ease-in-out"
        :class="form.allow_messages_dispatch
 ? 'translate-x-6'
 : 'translate-x-1'"
      />
    </button>
  </div>
  <p class="text-xs text-muted mt-1">
    {{ t("admin.groups.openaiMessages.allowDispatchHint") }}
  </p>

  <div
    v-if="
      form.platform === 'openai' &&
      form.allow_messages_dispatch
    "
    class="mt-3"
  >
    <div
      class="relative overflow-hidden rounded-xl border border-line bg-surface shadow-sm"
    >
      <div
        class="border-b border-line bg-surface-2 px-4 py-3"
      >
        <div class="flex items-center gap-2">
          <div class="h-2 w-2 rounded-full bg-accent-500"></div>
          <label
            class="text-sm font-medium text-foreground"
            >{{
              t("admin.groups.openaiMessages.familyMappingTitle")
            }}</label
          >
        </div>
        <p class="mt-1 text-xs text-muted">
          {{ t("admin.groups.openaiMessages.familyMappingHint") }}
        </p>
      </div>
      <div class="p-4">
        <div class="grid gap-4 md:grid-cols-3">
          <div>
            <label class="input-label">{{
              t("admin.groups.openaiMessages.opusModel")
            }}</label>
            <input
              v-model="form.opus_mapped_model"
              type="text"
              :placeholder="
                t('admin.groups.openaiMessages.opusModelPlaceholder')
              "
              class="input"
            />
          </div>
          <div>
            <label class="input-label">{{
              t("admin.groups.openaiMessages.sonnetModel")
            }}</label>
            <input
              v-model="form.sonnet_mapped_model"
              type="text"
              :placeholder="
                t('admin.groups.openaiMessages.sonnetModelPlaceholder')
              "
              class="input"
            />
          </div>
          <div>
            <label class="input-label">{{
              t("admin.groups.openaiMessages.haikuModel")
            }}</label>
            <input
              v-model="form.haiku_mapped_model"
              type="text"
              :placeholder="
                t('admin.groups.openaiMessages.haikuModelPlaceholder')
              "
              class="input"
            />
          </div>
        </div>
      </div>
    </div>

    <div
      class="mt-5 relative overflow-hidden rounded-xl border border-[color-mix(in_oklch,var(--accent)_28%,transparent)] bg-surface shadow-sm "
    >
      <div
        class="border-b border-[color-mix(in_oklch,var(--accent)_18%,transparent)] bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] px-4 py-3  "
      >
        <div class="flex items-start justify-between gap-3">
          <div>
            <div class="flex items-center gap-2">
              <div class="h-2 w-2 rounded-full bg-accent"></div>
              <label
                class="text-sm font-medium text-accent "
                >{{
                  t("admin.groups.openaiMessages.exactMappingTitle")
                }}</label
              >
            </div>
            <p
              class="mt-1 text-xs text-accent/90 "
            >
              {{ t("admin.groups.openaiMessages.exactMappingHint") }}
            </p>
          </div>
        </div>
      </div>

      <div class="p-4 bg-surface-2">
        <div
          v-if="form.exact_model_mappings.length === 0"
          class="flex items-center justify-between gap-3 rounded-xl border-2 border-dashed border-[color-mix(in_oklch,var(--accent)_28%,transparent)] bg-surface px-5 py-4 text-sm text-accent transition-colors hover:border-accent   "
        >
          <span>{{
            t("admin.groups.openaiMessages.noExactMappings")
          }}</span>
          <button
            type="button"
            @click="addMapping"
            class="flex items-center gap-1.5 text-sm font-medium text-accent transition-colors hover:text-accent  "
          >
            <Icon name="plus" size="sm" />
            {{ t("admin.groups.openaiMessages.addExactMapping") }}
          </button>
        </div>

        <div v-else class="space-y-3">
          <div
            v-for="row in form.exact_model_mappings"
            :key="getRowKey(row)"
            class="group relative rounded-xl border border-line bg-surface p-4 shadow-sm transition-all hover:border-accent hover:shadow-[var(--shadow-hover)] "
          >
            <div class="flex items-center gap-4">
              <div
                class="grid flex-1 gap-4 md:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] md:items-start"
              >
                <div>
                  <label class="input-label">{{
                    t("admin.groups.openaiMessages.claudeModel")
                  }}</label>
                  <input
                    v-model="row.claude_model"
                    type="text"
                    :placeholder="
                      t(
                        'admin.groups.openaiMessages.claudeModelPlaceholder',
                      )
                    "
                    class="input bg-surface-2 focus:bg-surface "
                  />
                </div>
                <div
                  class="hidden md:flex md:justify-center md:pt-7 text-accent "
                >
                  <Icon
                    name="arrowRight"
                    size="sm"
                    class="transition-transform group-hover:translate-x-1"
                  />
                </div>
                <div>
                  <label class="input-label">{{
                    t("admin.groups.openaiMessages.targetModel")
                  }}</label>
                  <input
                    v-model="row.target_model"
                    type="text"
                    :placeholder="
                      t(
                        'admin.groups.openaiMessages.targetModelPlaceholder',
                      )
                    "
                    class="input bg-surface-2 focus:bg-surface "
                  />
                </div>
              </div>
              <button
                type="button"
                @click="removeMapping(row)"
                class="mt-6 flex h-9 w-9 items-center justify-center rounded-lg text-muted transition-colors hover:bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] hover:text-danger-text  "
                :title="
                  t('admin.groups.openaiMessages.removeExactMapping')
                "
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </div>

          <button
            type="button"
            @click="addMapping"
            class="flex w-full items-center justify-center gap-2 rounded-xl border-2 border-dashed border-line bg-surface py-3 text-sm font-medium text-muted transition-all hover:border-accent hover:bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] hover:text-accent   "
          >
            <Icon name="plus" size="sm" />
            {{ t("admin.groups.openaiMessages.addExactMapping") }}
          </button>
        </div>
      </div>
    </div>
  </div>
</div>

</template>

<script setup lang="ts">
import Icon from "@/components/icons/Icon.vue";
import { useI18n } from "vue-i18n";
import { createStableObjectKeyResolver } from "@/utils/stableObjectKey";
import type { GroupPlatform } from "@/types";
import {
  supportsMessagesDispatchPlatform,
  type MessagesDispatchFormState,
  type MessagesDispatchMappingRow,
} from "@/views/admin/groupsMessagesDispatch";

const form = defineModel<MessagesDispatchFormState & { platform: GroupPlatform }>(
  "form",
  { required: true },
);

const { t } = useI18n();

const resolveRowKey =
  createStableObjectKeyResolver<MessagesDispatchMappingRow>(
    "messages-dispatch-row",
  );
const getRowKey = (row: MessagesDispatchMappingRow) => resolveRowKey(row);

const addMapping = () => {
  form.value.exact_model_mappings.push({ claude_model: "", target_model: "" });
};

const removeMapping = (row: MessagesDispatchMappingRow) => {
  const index = form.value.exact_model_mappings.indexOf(row);
  if (index !== -1) {
    form.value.exact_model_mappings.splice(index, 1);
  }
};
</script>
