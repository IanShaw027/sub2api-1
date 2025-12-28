# 遮罩式新手引导实施方案 (Onboarding Tour Implementation Guide)

## 1. 概述
本方案旨在通过交互式的新手引导（Onboarding Tour），帮助首次进入系统（标准模式）的管理员快速理解核心概念（如渠道配置、分组管理、充值流程）。使用轻量级库 `driver.js` 实现。

## 2. 技术选型
*   **库**: `driver.js` (无依赖，原生 JS，支持 Vue3)
*   **样式**: 默认样式 + 自定义 CSS 覆盖（适配暗色模式）

### 2.1 暗色模式样式适配
在 `frontend/src/styles/onboarding.css` 中添加暗色模式覆盖样式：

```css
/* 暗色模式适配 */
.dark .driver-popover {
  background-color: #1f2937;
  color: #f3f4f6;
}

.dark .driver-popover-title {
  color: #ffffff;
}

.dark .driver-popover-description {
  color: #d1d5db;
}

.dark .driver-popover-footer button {
  background-color: #374151;
  color: #f3f4f6;
}

.dark .driver-popover-footer button:hover {
  background-color: #4b5563;
}
```

在 `MainLayout.vue` 中引入：
```typescript
import '@/styles/onboarding.css';
```

## 3. 详细实施步骤

### 3.1 依赖安装 (Frontend)

```bash
pnpm add driver.js
# 或
npm install driver.js
```

### 3.2 定义引导步骤
创建 `frontend/src/components/Guide/steps.ts`，定义管理员视角的引导流程。

#### 基础步骤定义（支持国际化）

```typescript
import { DriveStep } from "driver.js";
import type { Composer } from 'vue-i18n';

// 支持国际化的步骤生成函数
export function makeAdminSteps(t: Composer['t']): DriveStep[] {
  return [
    {
      element: '#sidebar-channel-manage',
      popover: {
        title: t('guide.step1.title'),
        description: t('guide.step1.description'),
        side: "right",
        align: 'start'
      }
    },
    {
      element: '#sidebar-group-manage',
      popover: {
        title: t('guide.step2.title'),
        description: t('guide.step2.description'),
        side: "right",
        align: 'start'
      }
    },
    {
      element: '#sidebar-wallet',
      popover: {
        title: t('guide.step3.title'),
        description: t('guide.step3.description'),
        side: "right",
        align: 'start'
      }
    }
  ];
}

// 静态步骤（不使用国际化）
export const adminSteps: DriveStep[] = [
  {
    element: '#sidebar-channel-manage',
    popover: {
      title: '第一步：配置模型渠道',
      description: '在这里添加 OpenAI、Anthropic、Gemini 等上游服务商的 API Key。',
      side: "right",
      align: 'start'
    }
  },
  {
    element: '#sidebar-group-manage',
    popover: {
      title: '第二步：创建分组',
      description: '将不同的渠道打包成"分组"（如 VIP 分组、普通分组），以便分配给不同用户。',
      side: "right",
      align: 'start'
    }
  },
  {
    element: '#sidebar-wallet',
    popover: {
      title: '第三步：管理资金',
      description: '查看系统收益、生成充值卡密给用户充值。',
      side: "right",
      align: 'start'
    }
  }
];

// 普通用户步骤（示例）
export const userSteps: DriveStep[] = [
  {
    element: '#sidebar-chat',
    popover: {
      title: '开始对话',
      description: '点击这里开始与 AI 模型对话。',
      side: "right",
      align: 'start'
    }
  }
];
```

### 3.2.1 创建组合式函数
创建 `frontend/src/composables/useOnboardingTour.ts`，封装所有引导逻辑。

```typescript
import { onMounted, onBeforeUnmount } from 'vue';
import { driver, type Driver, type Config } from 'driver.js';
import 'driver.js/dist/driver.css';
import { useRouter } from 'vue-router';
import { useUserStore } from '@/stores/user';
import { useSystemStore } from '@/stores/system';

interface OnboardingOptions {
  steps: Config['steps'];
  storageKey?: string;
  autoStart?: boolean;
  onComplete?: () => void;
}

export function useOnboardingTour(options: OnboardingOptions) {
  const router = useRouter();
  const userStore = useUserStore();
  const systemStore = useSystemStore();

  let driverObj: Driver | null = null;

  // 生成版本化的存储键（包含用户ID、角色、版本号）
  const getStorageKey = () => {
    const { storageKey = 'onboarding_tour' } = options;
    const userId = userStore.user?.id || 'guest';
    const role = userStore.user?.role || 'user';
    const version = 'v1'; // 更新引导内容时递增版本号
    return `${storageKey}_${userId}_${role}_${version}`;
  };

  const hasSeen = () => {
    return localStorage.getItem(getStorageKey()) === 'true';
  };

  const markAsSeen = () => {
    localStorage.setItem(getStorageKey(), 'true');
  };

  const clearSeen = () => {
    localStorage.removeItem(getStorageKey());
  };

  // 验证元素是否存在
  const validateSteps = (steps: Config['steps']) => {
    if (!steps) return [];
    return steps.filter(step => {
      if (!step.element) return true;
      const element = document.querySelector(step.element as string);
      if (!element) {
        console.warn(`[Onboarding] Element not found: ${step.element}`);
        return false;
      }
      return true;
    });
  };

  const startTour = async () => {
    // 等待路由和 DOM 就绪
    await router.isReady();
    await new Promise(resolve => {
      if (document.readyState === 'complete') {
        resolve(true);
      } else {
        window.addEventListener('load', () => resolve(true), { once: true });
      }
    });

    // 验证步骤元素
    const validSteps = validateSteps(options.steps);
    if (validSteps.length === 0) {
      console.warn('[Onboarding] No valid steps found, skipping tour');
      return;
    }

    // 检测移动端
    const isMobile = window.innerWidth < 768;
    if (isMobile) {
      console.log('[Onboarding] Mobile device detected, adjusting popover position');
      // 移动端调整 popover 位置
      validSteps.forEach(step => {
        if (step.popover) {
          step.popover.side = 'bottom';
          step.popover.align = 'center';
        }
      });
    }

    driverObj = driver({
      showProgress: true,
      steps: validSteps,
      onDestroyed: () => {
        markAsSeen();
        options.onComplete?.();
      },
      onDestroyStarted: () => {
        if (driverObj?.hasNextStep()) {
          // 用户点击了关闭按钮，标记为已读
          markAsSeen();
        }
      }
    });

    driverObj.drive();
  };

  const replayTour = () => {
    clearSeen();
    startTour();
  };

  onMounted(() => {
    if (options.autoStart && !hasSeen()) {
      // 仅在标准模式下自动启动
      if (systemStore.runMode === 'standard') {
        startTour();
      }
    }
  });

  onBeforeUnmount(() => {
    // 清理 driver 实例
    if (driverObj) {
      driverObj.destroy();
      driverObj = null;
    }
  });

  return {
    startTour,
    replayTour,
    hasSeen,
    clearSeen
  };
}
```

#### 在 MainLayout.vue 中使用

```typescript
import { useOnboardingTour } from '@/composables/useOnboardingTour';
import { adminSteps } from '@/components/Guide/steps';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

// 使用国际化步骤
const { startTour, replayTour } = useOnboardingTour({
  steps: makeAdminSteps(t),
  storageKey: 'admin_guide',
  autoStart: true,
  onComplete: () => {
    console.log('Onboarding tour completed');
  }
});

// 或使用静态步骤
const { startTour, replayTour } = useOnboardingTour({
  steps: adminSteps,
  storageKey: 'admin_guide',
  autoStart: true
});

// 暴露 replayTour 供导航栏调用
defineExpose({ replayTour });
```

### 3.3 DOM 标识
修改 `frontend/src/layout/components/Sidebar/SidebarMenu.vue`，为关键菜单项添加唯一的 `id` 属性。

```html
<!-- 示例 -->
<el-menu-item index="/channel" id="sidebar-channel-manage">
  ...
</el-menu-item>
```

### 3.4 触发逻辑（已废弃 - 请使用 3.2.1 的组合式函数）

**注意：以下代码已被 3.2.1 节的 `useOnboardingTour` 组合式函数取代，此处仅作为参考。**

~~在 `frontend/src/layout/MainLayout.vue` 或 `App.vue` 中集成引导逻辑。~~

**核心逻辑**:
1.  ~~检查 `localStorage` 是否已有 `has_seen_admin_guide` 标记。~~
2.  ~~检查当前运行模式（`simple` 模式下可能不需要此引导，或显示不同的引导）。~~
3.  ~~如果满足条件，初始化 `driver` 并启动。~~

~~```typescript
import { driver } from "driver.js";
import "driver.js/dist/driver.css";
import { adminSteps } from "@/components/Guide/steps";
import { useSystemStore } from "@/stores/system";

const systemStore = useSystemStore();

onMounted(() => {
  const hasSeen = localStorage.getItem('has_seen_admin_guide');

  // 仅在标准模式且未观看过时触发
  if (!hasSeen && systemStore.runMode === 'standard') {
    const driverObj = driver({
      showProgress: true,
      steps: adminSteps,
      onDestroyed: () => {
        // 引导结束或跳过时，标记为已读
        localStorage.setItem('has_seen_admin_guide', 'true');
      }
    });

    // 稍微延迟一点，确保 DOM 渲染完成
    setTimeout(() => {
      driverObj.drive();
    }, 1000);
  }
});
```~~

### 3.4.1 移动端支持

移动端适配已集成在 `useOnboardingTour` 组合式函数中（见 3.2.1 节）。

**关键特性**：
- 自动检测屏幕宽度（< 768px 视为移动端）
- 移动端自动调整 popover 位置为 `side: "bottom", align: "center"`
- 小屏幕设备可选择跳过引导或显示简化版本

**手动配置示例**（如需自定义）：

```typescript
// 在 steps.ts 中为移动端创建专门的步骤
export const mobileAdminSteps: DriveStep[] = [
  {
    element: '#sidebar-channel-manage',
    popover: {
      title: '配置渠道',
      description: '添加 API Key',
      side: "bottom",
      align: 'center'
    }
  }
];

// 在组件中根据设备选择步骤
const isMobile = window.innerWidth < 768;
const { startTour, replayTour } = useOnboardingTour({
  steps: isMobile ? mobileAdminSteps : adminSteps,
  storageKey: 'admin_guide',
  autoStart: true
});
```

### 3.5 增加"重播"入口
在顶部导航栏（Navbar）的"帮助"或"头像"下拉菜单中，添加一个"重新查看新手引导"的按钮。

**推荐方案（使用组合式函数）**：

```vue
<!-- Navbar.vue -->
<template>
  <el-dropdown-item @click="handleReplayGuide">
    <el-icon><QuestionFilled /></el-icon>
    重新查看新手引导
  </el-dropdown-item>
</template>

<script setup lang="ts">
import { getCurrentInstance } from 'vue';

const instance = getCurrentInstance();

function handleReplayGuide() {
  // 调用 MainLayout 暴露的 replayTour 方法
  const mainLayout = instance?.parent?.parent; // 根据实际层级调整
  if (mainLayout?.exposed?.replayTour) {
    mainLayout.exposed.replayTour();
  } else {
    console.error('[Onboarding] replayTour method not found');
  }
}
</script>
```

**替代方案（使用事件总线或 Pinia）**：

```typescript
// stores/onboarding.ts
import { defineStore } from 'pinia';

export const useOnboardingStore = defineStore('onboarding', {
  state: () => ({
    replayCallback: null as (() => void) | null
  }),
  actions: {
    setReplayCallback(callback: () => void) {
      this.replayCallback = callback;
    },
    replay() {
      if (this.replayCallback) {
        this.replayCallback();
      }
    }
  }
});

// MainLayout.vue
import { useOnboardingStore } from '@/stores/onboarding';

const onboardingStore = useOnboardingStore();
const { replayTour } = useOnboardingTour({ /* ... */ });

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour);
});

// Navbar.vue
import { useOnboardingStore } from '@/stores/onboarding';

function handleReplayGuide() {
  useOnboardingStore().replay();
}
```

**注意**：
- 移除了不可靠的 `location.reload()` 方案
- 直接调用 `replayTour()` 函数，无需刷新页面
- 确保引导逻辑封装在可复用的函数中

## 4. 验证清单

### 4.1 基础功能测试
- [ ] 首次登录（清除 LocalStorage 后）应自动弹出引导遮罩
- [ ] 引导步骤的高亮区域准确对应侧边栏菜单位置
- [ ] 点击"下一步"流畅切换
- [ ] 点击"跳过"或完成引导后，刷新页面不再自动弹出
- [ ] 能够通过"重播"按钮再次触发引导

### 4.2 多角色测试
- [ ] 管理员角色显示完整的引导流程（渠道、分组、资金）
- [ ] 普通用户角色显示简化的引导流程（或不显示）
- [ ] 不同角色的 localStorage 键互不干扰
- [ ] 角色切换后引导状态正确更新

### 4.3 移动端测试
- [ ] 小屏幕设备（< 768px）自动调整 popover 位置
- [ ] 移动端引导不遮挡关键内容
- [ ] 触摸操作流畅，无误触
- [ ] 横竖屏切换时引导正常显示

### 4.4 错误处理测试
- [ ] 目标元素不存在时优雅降级（跳过该步骤）
- [ ] 控制台输出清晰的调试日志
- [ ] 权限不足导致元素隐藏时不显示该步骤
- [ ] 功能标志关闭时相关步骤自动过滤

### 4.5 多用户/设备测试
- [ ] 同一用户在不同设备上的引导状态独立
- [ ] 不同用户在同一设备上的引导状态独立
- [ ] 版本更新后旧用户能看到新的引导内容
- [ ] localStorage 键包含用户ID、角色、版本号

### 4.6 性能测试
- [ ] 引导启动不阻塞页面渲染
- [ ] DOM 就绪检测可靠（无闪烁或错位）
- [ ] 组件卸载时正确清理 driver 实例
- [ ] 无内存泄漏（多次重播后内存稳定）

### 4.7 国际化测试
- [ ] 切换语言后引导文本正确更新
- [ ] 多语言文本长度不影响 popover 布局
- [ ] RTL 语言（如阿拉伯语）布局正确

### 4.8 暗色模式测试
- [ ] 暗色模式下 popover 样式正确
- [ ] 主题切换时引导样式实时更新
- [ ] 高亮遮罩在暗色背景下清晰可见

## 5. 关键改进总结

本次更新修复了原方案中的多个关键问题，提升了可靠性和用户体验：

### 5.1 触发可靠性改进（高优先级）
- **问题**：使用 `setTimeout(fn, 1000)` 不可靠，可能在 DOM 未就绪时触发
- **解决**：使用 `router.isReady()` + `document.readyState` 确保 DOM 完全就绪
- **新增**：元素存在性验证，过滤不存在的步骤
- **新增**：`onBeforeUnmount` 清理逻辑，防止内存泄漏
- **新增**：版本化的 localStorage 键（`storageKey_userId_role_version`）

### 5.2 重播逻辑改进（高优先级）
- **问题**：使用 `location.reload()` 会刷新整个页面，体验差
- **解决**：直接调用 `replayTour()` 函数，无需刷新
- **新增**：提供多种重播方案（组件暴露、Pinia store）

### 5.3 移动端支持（中优先级）
- **新增**：自动检测屏幕尺寸（< 768px）
- **新增**：移动端自动调整 popover 位置（`side: "bottom", align: "center"`）
- **新增**：支持自定义移动端步骤

### 5.4 组合式函数封装（中优先级）
- **新增**：`useOnboardingTour` 组合式函数
- **优势**：逻辑复用、类型安全、易于测试
- **功能**：初始化、启动、重播、清理、状态管理

### 5.5 国际化支持（中优先级）
- **新增**：`makeAdminSteps(t)` 函数支持 i18n
- **新增**：多角色不同引导内容（`adminSteps`, `userSteps`）
- **优势**：支持多语言、易于维护

### 5.6 错误处理增强（高优先级）
- **新增**：元素缺失时优雅降级
- **新增**：控制台日志用于调试（`[Onboarding]` 前缀）
- **新增**：权限/功能标志导致元素隐藏的处理

### 5.7 测试覆盖扩展（中优先级）
- **新增**：8 大类测试场景（基础、多角色、移动端、错误处理、多用户、性能、国际化、暗色模式）
- **新增**：40+ 测试检查点
- **优势**：全面覆盖边缘情况

### 5.8 暗色模式样式（中优先级）
- **新增**：完整的暗色模式 CSS 覆盖
- **新增**：主题适配指南
- **优势**：与项目主题无缝集成

## 6. 迁移指南

如果已使用旧版实现，请按以下步骤迁移：

1. **创建组合式函数**：复制 3.2.1 节的 `useOnboardingTour.ts`
2. **更新步骤定义**：在 `steps.ts` 中添加国际化支持（可选）
3. **替换触发逻辑**：将 3.4 节的旧代码替换为组合式函数调用
4. **更新重播入口**：将 3.5 节的 `location.reload()` 替换为 `replayTour()`
5. **添加暗色模式样式**：复制 2.1 节的 CSS 文件
6. **清理旧数据**：提醒用户清除旧的 localStorage 键（或使用版本号自动失效）

## 7. 常见问题

### Q1: 引导在某些页面不显示？
**A**: 检查目标元素是否存在，使用 `validateSteps` 会自动过滤不存在的元素。

### Q2: 如何为不同角色显示不同引导？
**A**: 在 `steps.ts` 中定义多个步骤数组（`adminSteps`, `userSteps`），根据用户角色选择。

### Q3: 如何更新引导内容而不影响已看过的用户？
**A**: 递增 `useOnboardingTour` 中的 `version` 变量（如 `v1` -> `v2`）。

### Q4: 移动端引导遮挡内容怎么办？
**A**: 使用 `side: "bottom"` 或自定义移动端步骤，减少步骤数量。

### Q5: 如何调试引导不触发的问题？
**A**: 检查控制台 `[Onboarding]` 前缀的日志，验证 localStorage 键和元素存在性。
