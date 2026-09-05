// 从 useSettingsForm.ts 拆分而来（frontend-health-cleanup 16）：设置页 Tab 导航（hash 同步、
// 键盘左右/上下/Home/End 切换、移动端 select）。逐字保留原实现，仅将闭包变量迁移到独立 composable。
import { ref } from "vue";
import type { SettingsTab } from "../useSettingsForm";

export function useSettingsTabNav() {
  const activeTab = ref<SettingsTab>("general");

  const settingsTabs = [
    { key: "general" as SettingsTab, icon: "home" as const },
    { key: "agreement" as SettingsTab, icon: "document" as const },
    { key: "features" as SettingsTab, icon: "bolt" as const },
    { key: "security" as SettingsTab, icon: "shield" as const },
    { key: "users" as SettingsTab, icon: "user" as const },
    { key: "gateway" as SettingsTab, icon: "server" as const },
    // NOTE (subagent 10A, glass-ui-redesign task 10.1): reordered email/backup/payment
    // to match prototype reference 06 nav sequence (通用设置/登录条款/功能开关/安全与认证/
    // 用户默认值/网关服务/邮件设置/数据备份/支付设置). No tab content changed — only the
    // order of these three entries. Flag for 10B (owner of Payment/Email/Backup tabs)
    // in case this conflicts with other in-flight edits to this array.
    { key: "email" as SettingsTab, icon: "mail" as const },
    { key: "backup" as SettingsTab, icon: "database" as const },
    { key: "payment" as SettingsTab, icon: "creditCard" as const },
  ];

  const settingsTabKeyboardActions = {
    ArrowLeft: -1,
    ArrowUp: -1,
    ArrowRight: 1,
    ArrowDown: 1,
    Home: "first",
    End: "last",
  } as const;

  const settingsTabKeys = settingsTabs.map((item) => item.key);

  function isSettingsTab(value: string): value is SettingsTab {
    return (settingsTabKeys as string[]).includes(value);
  }

  function applySettingsTabFromHash(): void {
    try {
      const raw = window.location.hash.replace(/^#/, "");
      if (raw && isSettingsTab(raw)) {
        activeTab.value = raw;
      }
    } catch {
      /* noop */
    }
  }

  function selectSettingsTab(tab: SettingsTab): void {
    activeTab.value = tab;
    try {
      window.history.replaceState(window.history.state, "", `#${tab}`);
    } catch {
      /* noop */
    }
  }

  function onSettingsMobileTabChange(event: Event): void {
    const value = (event.target as HTMLSelectElement).value as SettingsTab;
    selectSettingsTab(value);
  }

  function focusSettingsTab(tab: SettingsTab): void {
    window.requestAnimationFrame(() => {
      document.getElementById(`settings-tab-${tab}`)?.focus();
    });
  }

  function handleSettingsTabKeydown(event: KeyboardEvent, tab: SettingsTab): void {
    const action =
      settingsTabKeyboardActions[
        event.key as keyof typeof settingsTabKeyboardActions
      ];
    if (action === undefined) {
      return;
    }

    event.preventDefault();
    const currentIndex = settingsTabs.findIndex((item) => item.key === tab);
    let nextIndex = currentIndex < 0 ? 0 : currentIndex;

    if (action === "first") {
      nextIndex = 0;
    } else if (action === "last") {
      nextIndex = settingsTabs.length - 1;
    } else {
      nextIndex =
        (nextIndex + action + settingsTabs.length) % settingsTabs.length;
    }

    const nextTab = settingsTabs[nextIndex]?.key;
    if (!nextTab) {
      return;
    }

    selectSettingsTab(nextTab);
    focusSettingsTab(nextTab);
  }

  return {
    activeTab,
    settingsTabs,
    settingsTabKeyboardActions,
    settingsTabKeys,
    isSettingsTab,
    applySettingsTabFromHash,
    selectSettingsTab,
    onSettingsMobileTabChange,
    focusSettingsTab,
    handleSettingsTabKeydown,
  };
}
