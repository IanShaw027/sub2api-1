<template>
  <BaseDialog :show="show" :title="t('admin.groups.sortOrder')" width="normal" @close="handleClose">
    <div class="space-y-4">
      <p class="text-sm text-muted">
        {{ t("admin.groups.sortOrderHint") }}
      </p>
      <VueDraggable v-model="sortableGroups" :animation="200" class="space-y-2">
        <div
          v-for="group in sortableGroups"
          :key="group.id"
          class="flex cursor-grab items-center gap-3 rounded-lg border border-line bg-surface p-3 transition-shadow hover:shadow-[var(--shadow-hover)] active:cursor-grabbing"
        >
          <div class="text-muted">
            <Icon name="menu" size="md" />
          </div>
          <div class="flex-1">
            <div class="font-medium text-foreground">
              {{ group.name }}
            </div>
            <div class="text-xs text-muted">
              <span
                :class="[
                  'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium',
                  group.platform === 'anthropic'
                    ? 'bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] text-warning-700  '
                    : group.platform === 'openai'
                      ? 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text  '
                      : group.platform === 'antigravity'
                        ? 'bg-accent-500/15 text-accent-700  '
                        : group.platform === 'kiro'
                          ? 'bg-accent-500/15 text-accent-700  '
                          : group.platform === 'grok'
                            ? 'bg-surface-3 text-foreground  '
                            : group.platform === 'kimi'
                              ? 'bg-accent-500/15 text-accent-700  '
                              : group.platform === 'zhipu'
                                ? 'bg-accent-500/15 text-accent-700  '
                                : group.platform === 'deepseek'
                                  ? 'bg-success-500/15 text-success-700  '
                                  : 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent  ',
                ]"
              >
                {{ t("admin.groups.platforms." + group.platform) }}
              </span>
            </div>
          </div>
          <div class="text-sm text-muted">#{{ group.id }}</div>
        </div>
      </VueDraggable>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3 pt-4">
        <button @click="handleClose" type="button" class="btn-glass-secondary">
          {{ t("common.cancel") }}
        </button>
        <button @click="saveSortOrder" :disabled="sortSubmitting" class="btn-glass-primary">
          <svg v-if="sortSubmitting" class="-ml-1 mr-2 h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            ></path>
          </svg>
          {{ sortSubmitting ? t("common.saving") : t("common.save") }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { VueDraggable } from "vue-draggable-plus";
import { useAppStore } from "@/stores/app";
import { adminAPI } from "@/api/admin";
import type { AdminGroup } from "@/types";
import BaseDialog from "@/components/common/BaseDialog.vue";
import Icon from "@/components/icons/Icon.vue";

const props = defineProps<{ show: boolean }>();
const emit = defineEmits<{ close: []; saved: [] }>();

const { t } = useI18n();
const appStore = useAppStore();

const sortableGroups = ref<AdminGroup[]>([]);
const sortSubmitting = ref(false);

const loadSortableGroups = async () => {
  try {
    const allGroups = await adminAPI.groups.getAll();
    sortableGroups.value = [...allGroups].sort((a, b) => a.sort_order - b.sort_order);
  } catch (error) {
    appStore.showError(t("admin.groups.failedToLoad"));
    console.error("Error loading groups for sorting:", error);
  }
};

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      void loadSortableGroups();
    } else {
      sortableGroups.value = [];
    }
  },
);

const handleClose = () => {
  emit("close");
};

const saveSortOrder = async () => {
  sortSubmitting.value = true;
  try {
    const updates = sortableGroups.value.map((g, index) => ({
      id: g.id,
      sort_order: index * 10,
    }));
    await adminAPI.groups.updateSortOrder(updates);
    appStore.showSuccess(t("admin.groups.sortOrderUpdated"));
    emit("saved");
    emit("close");
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t("admin.groups.failedToUpdateSortOrder"));
    console.error("Error updating sort order:", error);
  } finally {
    sortSubmitting.value = false;
  }
};
</script>
