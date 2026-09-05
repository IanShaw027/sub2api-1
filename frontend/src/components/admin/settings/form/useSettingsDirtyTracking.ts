import { computed, ref, watch, type Ref } from "vue";
import type { SettingsForm, SettingsTab } from "../useSettingsForm";

export function useSettingsDirtyTracking(
  form: SettingsForm,
  activeTab: Ref<SettingsTab>,
  saving: Ref<boolean>,
  loadSettings: () => Promise<void>,
) {
  const settingsSnapshot = ref<string>("");

  const settingsDirtyKeys = ref<string[]>([]);

  const settingsDirtyTabs = ref<SettingsTab[]>([]);

  const settingsDirtyTrackingReady = ref(false);

  function readSettingsSnapshot(): Record<string, unknown> | null {
    if (!settingsSnapshot.value) return null;
    try {
      return JSON.parse(settingsSnapshot.value) as Record<string, unknown>;
    } catch {
      return null;
    }
  }

  function captureSettingsSnapshot(): void {
    try {
      settingsSnapshot.value = JSON.stringify(form);
    } catch {
      settingsSnapshot.value = "";
    }
    settingsDirtyKeys.value = [];
    settingsDirtyTabs.value = [];
    settingsDirtyTrackingReady.value = true;
  }

  function computeSettingsDirtyKeys(): string[] {
    const base = readSettingsSnapshot();
    if (!base) return [];
    const current = form as unknown as Record<string, unknown>;
    const keys: string[] = [];
    for (const key of Object.keys(current)) {
      let a: string | undefined;
      let b: string | undefined;
      try {
        a = JSON.stringify(current[key]);
        b = JSON.stringify(base[key]);
      } catch {
        continue;
      }
      if (a !== b) keys.push(key);
    }
    return keys;
  }

  const settingsDirtyCount = computed(() => settingsDirtyKeys.value.length);

  function isSettingsTabDirty(tab: SettingsTab): boolean {
    return settingsDirtyTabs.value.includes(tab);
  }

  watch(
    form,
    () => {
      if (!settingsDirtyTrackingReady.value) return;
      const keys = computeSettingsDirtyKeys();
      settingsDirtyKeys.value = keys;
      if (keys.length === 0) {
        settingsDirtyTabs.value = [];
        return;
      }
      if (!settingsDirtyTabs.value.includes(activeTab.value)) {
        settingsDirtyTabs.value = [...settingsDirtyTabs.value, activeTab.value];
      }
    },
    { deep: true },
  );

  async function resetSettingsForm(): Promise<void> {
    if (saving.value) return;
    settingsDirtyTrackingReady.value = false;
    await loadSettings();
  }

  return {
    settingsSnapshot,
    settingsDirtyKeys,
    settingsDirtyTabs,
    settingsDirtyTrackingReady,
    readSettingsSnapshot,
    captureSettingsSnapshot,
    computeSettingsDirtyKeys,
    settingsDirtyCount,
    isSettingsTabDirty,
    resetSettingsForm,
  };
}
