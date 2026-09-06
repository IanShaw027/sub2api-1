<template>
  <UiModal :open="settingsOpen" :title="t('admin.riskControl.settingsTitle')" width="xl" @close="settingsOpen = false">
    <div class="space-y-6">
      <div class="flex gap-2 overflow-x-auto border-b border-line pb-3">
        <button
          v-for="tab in settingsTabs"
          :key="tab.id"
          type="button"
          class="inline-flex whitespace-nowrap rounded-lg px-3 py-2 text-sm font-medium transition-colors"
          :class="activeSettingsTab === tab.id ? 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent  ' : 'text-muted hover:bg-surface-2 hover:text-foreground '"
          @click="activeSettingsTab = tab.id"
        >
          {{ tab.label }}
        </button>
      </div>

      <div v-if="activeSettingsTab === 'basic'" class="space-y-5">
        <div class="grid grid-cols-1 gap-5 lg:grid-cols-2">
          <div class="flex items-center justify-between rounded-lg border border-line p-4">
            <div>
              <p class="text-sm font-medium text-foreground">{{ t('admin.riskControl.enabled') }}</p>
              <p class="mt-1 text-xs text-muted">{{ t('admin.riskControl.enabledHint') }}</p>
            </div>
            <Toggle v-model="configForm.enabled" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.riskControl.mode') }}</label>
            <Select v-model="configForm.mode" :options="modeOptions" />
            <p class="mt-2 text-xs leading-5 text-muted">{{ modeDescription(configForm.mode) }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.riskControl.baseUrl') }}</label>
            <input v-model.trim="configForm.base_url" type="url" class="input" placeholder="https://api.openai.com" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.riskControl.model') }}</label>
            <input v-model.trim="configForm.model" type="text" class="input" placeholder="omni-moderation-latest" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.riskControl.timeoutMs') }}</label>
            <input v-model.number="configForm.timeout_ms" type="number" min="500" max="30000" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.riskControl.retryCount') }}</label>
            <input v-model.number="configForm.retry_count" type="number" min="0" max="5" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.riskControl.sampleRate') }}</label>
            <div class="relative">
              <input v-model.number="configForm.sample_rate" type="number" min="0" max="100" step="1" class="input pr-8" />
              <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-muted">%</span>
            </div>
          </div>
          <div>
            <label class="input-label">{{ t('admin.riskControl.proxy') }}</label>
            <ProxySelector v-model="configForm.proxy_id" :proxies="proxies" />
            <p class="mt-2 text-xs leading-5 text-muted">{{ t('admin.riskControl.proxyHint') }}</p>
          </div>
        </div>

        <div class="overflow-hidden rounded-xl border border-line bg-surface shadow-sm">
          <div class="flex flex-col gap-4 border-b border-line bg-surface-2 px-4 py-4 lg:flex-row lg:items-center lg:justify-between">
            <div class="flex items-start gap-3">
              <span class="mt-0.5 flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent  ">
                <Icon name="key" size="md" />
              </span>
              <div>
                <label class="text-sm font-semibold text-foreground">{{ t('admin.riskControl.apiKeys') }}</label>
                <p class="mt-1 max-w-3xl text-xs leading-5 text-muted">
                  {{ t('admin.riskControl.apiKeysHint', { count: configForm.api_key_count }) }}
                </p>
              </div>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                class="btn-glass-secondary inline-flex items-center gap-2"
                :disabled="apiKeyTesting || inputApiKeyCount === 0 || configForm.clear_api_key"
                @click="testApiKeys(true)"
              >
                <Icon name="beaker" size="sm" :class="apiKeyTesting ? 'animate-pulse' : ''" />
                {{ apiKeyTesting ? t('admin.riskControl.testingApiKeys') : t('admin.riskControl.testInputApiKeys') }}
              </button>
              <button
                type="button"
                class="btn-glass-secondary inline-flex items-center gap-2"
                :disabled="apiKeyTesting || effectiveStoredApiKeyCount === 0 || pendingDeletedApiKeyCount > 0 || configForm.clear_api_key || configForm.api_keys_mode === 'replace'"
                @click="testApiKeys(false)"
              >
                <Icon name="shield" size="sm" />
                {{ storedApiKeyTestButtonText }}
              </button>
              <button
                v-if="configForm.api_key_configured"
                type="button"
                class="btn-glass-secondary inline-flex items-center gap-2"
                @click="toggleClearApiKey"
              >
                <Icon :name="configForm.clear_api_key ? 'x' : 'trash'" size="sm" />
                {{ configForm.clear_api_key ? t('admin.riskControl.keepApiKey') : t('admin.riskControl.clearApiKey') }}
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-4 p-4 xl:grid-cols-[minmax(0,1fr)_minmax(360px,440px)]">
            <div class="space-y-3">
              <div class="flex flex-col gap-2 rounded-lg border border-line bg-surface-2 p-2 sm:flex-row sm:items-center sm:justify-between">
                <div class="text-xs leading-5 text-muted">
                  <span class="font-medium text-foreground">{{ t('admin.riskControl.apiKeysWriteMode') }}</span>
                  <span class="ml-2">{{ apiKeysModeHint }}</span>
                </div>
                <div class="inline-flex rounded-lg bg-surface p-1 shadow-sm">
                  <button
                    type="button"
                    class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
                    :class="configForm.api_keys_mode === 'append' ? 'bg-accent text-white shadow-sm' : 'text-muted hover:bg-surface-2 '"
                    :disabled="configForm.clear_api_key"
                    @click="setAPIKeysMode('append')"
                  >
                    {{ t('admin.riskControl.apiKeysModeAppend') }}
                  </button>
                  <button
                    type="button"
                    class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
                    :class="configForm.api_keys_mode === 'replace' ? 'bg-warning-500 text-white shadow-sm' : 'text-muted hover:bg-surface-2 '"
                    :disabled="configForm.clear_api_key"
                    @click="setAPIKeysMode('replace')"
                  >
                    {{ t('admin.riskControl.apiKeysModeReplace') }}
                  </button>
                </div>
              </div>
              <textarea
                v-model="configForm.api_keys_text"
                class="input min-h-44 resize-y font-mono text-sm"
                :placeholder="apiKeysPlaceholder"
                autocomplete="new-password"
                :disabled="configForm.clear_api_key"
              ></textarea>
              <div class="flex flex-wrap items-center gap-2 text-xs text-muted">
                <span class="inline-flex rounded-md bg-surface-2 px-2 py-1">
                  {{ t('admin.riskControl.inputApiKeyCount', { count: inputApiKeyCount }) }}
                </span>
                <span v-if="configForm.api_key_configured" class="inline-flex rounded-md bg-surface-2 px-2 py-1">
                  {{ t('admin.riskControl.storedApiKeyCount', { count: configForm.api_key_count }) }}
                </span>
                <span v-if="configForm.clear_api_key" class="inline-flex rounded-md bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] px-2 py-1 text-danger-text  ">
                  {{ t('admin.riskControl.apiKeyWillClear') }}
                </span>
                <span v-else-if="pendingDeletedApiKeyCount > 0" class="inline-flex rounded-md bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] px-2 py-1 text-warning-text  ">
                  {{ t('admin.riskControl.apiKeyPendingDeleteCount', { count: pendingDeletedApiKeyCount }) }}
                </span>
                <span v-if="configForm.api_keys_mode === 'replace'" class="inline-flex rounded-md bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] px-2 py-1 text-warning-text  ">
                  {{ t('admin.riskControl.apiKeysReplaceWarning') }}
                </span>
              </div>

              <div class="rounded-lg border border-line bg-surface-2 p-3" @paste="handleModerationImagePaste">
                <div class="mb-3 flex items-center justify-between gap-3">
                  <div>
                    <p class="text-sm font-semibold text-foreground">{{ t('admin.riskControl.auditTestInput') }}</p>
                    <p class="mt-1 text-xs text-muted">{{ t('admin.riskControl.auditTestInputHint') }}</p>
                  </div>
                  <button
                    v-if="moderationTestPrompt || moderationTestImages.length > 0 || moderationTestResult"
                    type="button"
                    class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-muted hover:bg-surface hover:text-foreground "
                    @click="clearModerationTestInput"
                  >
                    <Icon name="x" size="xs" />
                    {{ t('admin.riskControl.clearAuditTest') }}
                  </button>
                </div>
                <textarea
                  v-model="moderationTestPrompt"
                  class="input min-h-24 resize-y text-sm"
                  :placeholder="t('admin.riskControl.auditTestPromptPlaceholder')"
                ></textarea>
                <div
                  class="mt-3 rounded-lg border border-dashed border-line bg-surface p-3"
                  @dragover.prevent
                  @drop.prevent="handleModerationImageDrop"
                >
                  <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div class="flex items-start gap-2">
                      <Icon name="upload" size="md" class="mt-0.5 text-muted" />
                      <div>
                        <p class="text-sm font-medium text-foreground">{{ t('admin.riskControl.auditTestImages') }}</p>
                        <p class="mt-1 text-xs text-muted">{{ t('admin.riskControl.auditTestImagesHint') }}</p>
                      </div>
                    </div>
                    <label class="btn-glass-secondary inline-flex cursor-pointer items-center gap-2">
                      <Icon name="plus" size="sm" />
                      {{ t('admin.riskControl.addAuditTestImage') }}
                      <input type="file" accept="image/*" multiple class="sr-only" @change="handleModerationImageUpload" />
                    </label>
                  </div>
                  <div v-if="moderationTestImages.length > 0" class="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
                    <div
                      v-for="(image, index) in moderationTestImages"
                      :key="image.slice(0, 64) + index"
                      class="group relative aspect-square overflow-hidden rounded-lg border border-line bg-surface-2"
                    >
                      <img :src="image" alt="" class="h-full w-full object-cover" />
                      <button
                        type="button"
                        class="absolute right-1.5 top-1.5 flex h-7 w-7 items-center justify-center rounded-full bg-black/60 text-white opacity-0 transition-opacity group-hover:opacity-100"
                        @click="removeModerationTestImage(index)"
                      >
                        <Icon name="x" size="xs" :stroke-width="2" />
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <div class="rounded-lg border border-line bg-surface-2 p-3">
              <div class="mb-3 flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <p class="text-sm font-semibold text-foreground">{{ t('admin.riskControl.apiKeyHealth') }}</p>
                  <p class="mt-1 text-xs text-muted">{{ t('admin.riskControl.apiKeyFreezeRule') }}</p>
                </div>
                <span class="inline-flex shrink-0 items-center whitespace-nowrap rounded-full bg-surface px-2 py-0.5 text-[11px] font-medium leading-5 text-muted shadow-sm">
                  {{ t('admin.riskControl.apiKeyRows', { count: apiKeyRows.length }) }}
                </span>
              </div>

              <div v-if="apiKeyRows.length === 0" class="flex min-h-32 flex-col items-center justify-center rounded-lg border border-dashed border-line bg-surface px-4 py-6 text-center">
                <Icon name="infoCircle" size="lg" class="text-muted" />
                <p class="mt-2 text-sm font-medium text-foreground">{{ t('admin.riskControl.apiKeyHealthEmpty') }}</p>
                <p class="mt-1 text-xs text-muted">{{ t('admin.riskControl.apiKeyHealthEmptyHint') }}</p>
              </div>
              <div v-else class="space-y-2">
                <div class="space-y-2" :class="apiKeyRowsExpanded ? 'max-h-72 overflow-y-auto pr-1' : ''">
                  <div
                    v-for="(row, index) in visibleApiKeyRows"
                    :key="apiKeyRowKey(row, index)"
                    class="rounded-lg border bg-surface p-2.5 shadow-sm"
                    :class="isStoredApiKeyPendingDelete(row) ? 'border-[color-mix(in_oklch,var(--warning)_35%,transparent)] opacity-70 ' : 'border-line'"
                  >
                    <div class="flex items-start justify-between gap-2">
                      <div class="min-w-0">
                        <div class="flex min-w-0 flex-wrap items-center gap-2">
                          <span class="truncate font-mono text-sm font-semibold text-foreground">{{ row.masked || '-' }}</span>
                          <span
                            class="inline-flex rounded-md px-1.5 py-0.5 text-[11px] font-medium"
                            :class="row.configured ? 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent  ' : 'bg-accent-500/15 text-accent-700  '"
                          >
                            {{ isStoredApiKeyPendingDelete(row) ? t('admin.riskControl.apiKeyPendingDelete') : row.configured ? t('admin.riskControl.apiKeyConfigured') : t('admin.riskControl.apiKeyTemporary') }}
                          </span>
                        </div>
                        <p class="mt-1 text-xs leading-5 text-muted">{{ apiKeyStatusMeta(row) }}</p>
                      </div>
                      <div class="flex flex-shrink-0 items-center gap-1.5">
                        <span class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2 py-0.5 text-xs font-medium" :class="apiKeyStatusBadgeClass(row.status)">
                          <span class="h-1.5 w-1.5 rounded-full" :class="apiKeyStatusDotClass(row.status)"></span>
                          {{ apiKeyStatusLabel(row.status) }}
                        </span>
                        <button
                          v-if="row.configured && !configForm.clear_api_key"
                          type="button"
                          class="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-2 hover:text-foreground "
                          :title="isStoredApiKeyPendingDelete(row) ? t('admin.riskControl.undoDeleteApiKey') : t('admin.riskControl.deleteApiKey')"
                          @click="toggleDeleteStoredApiKey(row)"
                        >
                          <Icon :name="isStoredApiKeyPendingDelete(row) ? 'refresh' : 'trash'" size="xs" />
                        </button>
                      </div>
                    </div>
                    <p v-if="row.last_error" class="mt-1.5 rounded-md bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] px-2 py-1.5 text-xs leading-5 text-warning-text  ">
                      {{ row.last_error }}
                    </p>
                  </div>
                </div>

                <div v-if="canToggleApiKeyRows" class="flex items-center justify-between gap-3 rounded-lg border border-dashed border-line bg-surface px-3 py-2 text-xs text-muted">
                  <span class="min-w-0 truncate">
                    {{ apiKeyRowsExpanded ? t('admin.riskControl.apiKeyRowsExpanded', { count: apiKeyRows.length }) : t('admin.riskControl.apiKeyRowsCollapsed', { count: hiddenApiKeyRowCount }) }}
                  </span>
                  <button
                    type="button"
                    class="inline-flex shrink-0 items-center gap-1 rounded-md px-2 py-1 font-medium text-accent transition-colors hover:bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] hover:text-accent "
                    @click="apiKeyRowsExpanded = !apiKeyRowsExpanded"
                  >
                    <Icon :name="apiKeyRowsExpanded ? 'chevronUp' : 'chevronDown'" size="xs" />
                    {{ apiKeyRowsExpanded ? t('admin.riskControl.collapseApiKeyRows') : t('admin.riskControl.expandApiKeyRows') }}
                  </button>
                </div>
              </div>

              <div v-if="moderationTestResult" class="mt-4 rounded-lg border border-line bg-surface p-3">
                <div class="flex items-start justify-between gap-3">
                  <div>
                    <p class="text-sm font-semibold text-foreground">{{ t('admin.riskControl.auditTestResult') }}</p>
                    <p class="mt-1 text-xs text-muted">
                      {{ t('admin.riskControl.auditTestHighest', { category: moderationTestResult.highest_category || '-', score: percent(moderationTestResult.highest_score) }) }}
                    </p>
                  </div>
                  <span class="inline-flex rounded-full px-2 py-1 text-xs font-medium" :class="moderationTestResult.flagged ? 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] text-danger-text  ' : 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text  '">
                    {{ moderationTestResult.flagged ? t('admin.riskControl.auditTestFlagged') : t('admin.riskControl.auditTestPassed') }}
                  </span>
                </div>
                <div class="mt-3">
                  <div class="mb-2 flex items-center justify-between text-xs text-muted">
                    <span>{{ t('admin.riskControl.auditTestComposite') }}</span>
                    <span class="font-semibold text-foreground">{{ percent(moderationTestResult.composite_score) }}</span>
                  </div>
                  <div class="h-2 overflow-hidden rounded-full bg-surface-2">
                    <div class="h-full rounded-full" :class="moderationTestResult.flagged ? 'bg-danger-500' : 'bg-success-500'" :style="{ width: percentWidth(moderationTestResult.composite_score) }"></div>
                  </div>
                </div>
                <div class="mt-3 max-h-52 space-y-2 overflow-y-auto pr-1">
                  <div v-for="score in moderationScoreRows" :key="score.category">
                    <div class="mb-1 flex items-center justify-between gap-3 text-xs">
                      <span class="truncate text-muted">{{ score.category }}</span>
                      <span class="font-mono text-muted">{{ percent(score.score) }} / {{ percent(score.threshold) }}</span>
                    </div>
                    <div class="h-1.5 overflow-hidden rounded-full bg-surface-2">
                      <div class="h-full rounded-full" :class="score.hit ? 'bg-danger-500' : 'bg-accent'" :style="{ width: percentWidth(score.score) }"></div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="activeSettingsTab === 'scope'" class="space-y-5">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h3 class="text-base font-semibold text-foreground">{{ t('admin.riskControl.groupScope') }}</h3>
            <p class="mt-1 text-sm text-muted">{{ t('admin.riskControl.groupScopeHint') }}</p>
          </div>
          <div class="inline-flex rounded-lg bg-surface-2 p-1">
            <button
              type="button"
              class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
              :class="configForm.all_groups ? 'bg-surface text-foreground shadow-sm ' : 'text-muted'"
              @click="configForm.all_groups = true"
            >
              {{ t('admin.riskControl.allGroups') }}
            </button>
            <button
              type="button"
              class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
              :class="!configForm.all_groups ? 'bg-surface text-foreground shadow-sm ' : 'text-muted'"
              @click="configForm.all_groups = false"
            >
              {{ t('admin.riskControl.selectedGroups') }}
            </button>
          </div>
        </div>

        <div v-if="!configForm.all_groups" class="space-y-4">
          <div class="relative">
            <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted" />
            <input v-model.trim="groupSearch" type="search" class="input pl-9" :placeholder="t('admin.riskControl.searchGroups')" />
          </div>
          <div class="grid max-h-[420px] grid-cols-1 gap-3 overflow-y-auto pr-1 md:grid-cols-2 xl:grid-cols-3">
            <button
              v-for="group in filteredGroups"
              :key="group.id"
              type="button"
              class="flex min-h-20 items-center justify-between rounded-lg border p-4 text-left transition-colors"
              :class="isGroupSelected(group.id) ? 'border-[color-mix(in_oklch,var(--accent)_40%,transparent)] bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]  ' : 'border-line hover:bg-surface-2'"
              @click="toggleGroup(group.id)"
            >
              <span class="min-w-0">
                <span class="block truncate text-sm font-semibold text-foreground">{{ group.name }}</span>
                <span class="mt-1 inline-flex rounded-md bg-surface-2 px-2 py-0.5 text-xs text-muted">{{ group.platform }}</span>
              </span>
              <span
                class="flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full border"
                :class="isGroupSelected(group.id) ? 'border-accent bg-accent text-white' : 'border-line text-transparent '"
              >
                <Icon name="check" size="xs" :stroke-width="2" />
              </span>
            </button>
            <p v-if="filteredGroups.length === 0" class="text-sm text-muted">{{ t('admin.riskControl.noGroups') }}</p>
          </div>
        </div>

        <div class="space-y-4 rounded-lg border border-line p-4">
          <div class="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h3 class="text-base font-semibold text-foreground">{{ t('admin.riskControl.modelFilter') }}</h3>
              <p class="mt-1 text-sm text-muted">{{ t('admin.riskControl.modelFilterHint') }}</p>
            </div>
            <span class="inline-flex w-fit rounded-md bg-surface-2 px-2.5 py-1 text-xs font-medium text-muted">
              {{ modelFilterSummary }}
            </span>
          </div>

          <div class="grid grid-cols-1 gap-2 md:grid-cols-3">
            <button
              v-for="option in modelFilterOptions"
              :key="option.value"
              type="button"
              class="rounded-lg border p-3 text-left transition-colors"
              :class="configForm.model_filter_type === option.value
 ? 'border-[color-mix(in_oklch,var(--accent)_40%,transparent)] bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent shadow-sm   '
 : 'border-line hover:bg-surface-2'"
              @click="setModelFilterType(option.value)"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="text-sm font-semibold">{{ option.label }}</span>
                <span
                  class="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full border"
                  :class="configForm.model_filter_type === option.value
 ? 'border-accent bg-accent text-white'
 : 'border-line text-transparent '"
                >
                  <Icon name="check" size="xs" :stroke-width="2" />
                </span>
              </div>
              <p class="mt-1 text-xs leading-5 text-muted">{{ option.description }}</p>
            </button>
          </div>

          <div v-if="configForm.model_filter_type !== 'all'" class="space-y-2">
            <label class="input-label">{{ t('admin.riskControl.modelFilterModels') }}</label>
            <ModelWhitelistSelector v-model="configForm.model_filter_models" />
            <p class="text-xs text-muted">
              {{ t('admin.riskControl.modelFilterModelCount', { count: modelFilterModelCount }) }}
            </p>
          </div>
        </div>
      </div>

      <div v-else-if="activeSettingsTab === 'runtime'" class="grid grid-cols-1 gap-5 lg:grid-cols-2">
        <div>
          <label class="input-label">{{ t('admin.riskControl.workerCount') }}</label>
          <input v-model.number="configForm.worker_count" type="number" min="1" max="32" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.riskControl.queueSize') }}</label>
          <input v-model.number="configForm.queue_size" type="number" min="100" max="100000" class="input" />
        </div>
        <div class="flex items-center justify-between rounded-lg border border-line p-4 lg:col-span-2">
          <div>
            <p class="text-sm font-medium text-foreground">{{ t('admin.riskControl.recordNonHits') }}</p>
            <p class="mt-1 text-xs text-muted">{{ t('admin.riskControl.recordNonHitsHint') }}</p>
          </div>
          <Toggle v-model="configForm.record_non_hits" />
        </div>
        <div class="space-y-4 rounded-lg border border-line p-4 lg:col-span-2">
          <div class="flex items-center justify-between gap-4">
            <div>
              <p class="text-sm font-medium text-foreground">{{ t('admin.riskControl.preHashCheck') }}</p>
              <p class="mt-1 text-xs text-muted">{{ t('admin.riskControl.preHashCheckHint') }}</p>
            </div>
            <Toggle v-model="configForm.pre_hash_check_enabled" />
          </div>
          <div class="rounded-lg bg-surface-2 p-3">
            <div class="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
              <div>
                <p class="text-sm font-medium text-foreground">
                  {{ t('admin.riskControl.flaggedHashCount', { count: formatNumber(status?.flagged_hash_count ?? 0) }) }}
                </p>
                <p class="mt-1 text-xs text-muted">{{ t('admin.riskControl.flaggedHashHint') }}</p>
              </div>
              <button
                type="button"
                class="btn-glass-secondary inline-flex items-center justify-center gap-2 text-danger-text hover:text-danger-text "
                :disabled="hashActionLoading || (status?.flagged_hash_count ?? 0) === 0"
                @click="emit('clear-flagged-hashes')"
              >
                <Icon name="trash" size="sm" :class="hashActionLoading ? 'animate-pulse' : ''" />
                {{ t('admin.riskControl.clearFlaggedHashes') }}
              </button>
            </div>
            <div class="mt-3 flex flex-col gap-2 sm:flex-row">
              <input
                v-model.trim="flaggedHashInput"
                type="text"
                class="input font-mono text-sm"
                :placeholder="t('admin.riskControl.flaggedHashPlaceholder')"
              />
              <button
                type="button"
                class="btn-glass-secondary inline-flex items-center justify-center gap-2"
                :disabled="hashActionLoading || !isFlaggedHashInputValid"
                @click="emit('delete-flagged-hash')"
              >
                <Icon name="trash" size="sm" />
                {{ t('admin.riskControl.deleteFlaggedHash') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="activeSettingsTab === 'response'" class="space-y-5">
        <div class="grid grid-cols-1 gap-5 lg:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.riskControl.blockStatus') }}</label>
            <input v-model.number="configForm.block_status" type="number" min="400" max="599" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.riskControl.blockMessage') }}</label>
            <input v-model.trim="configForm.block_message" type="text" class="input" />
          </div>
          <div class="flex items-center justify-between rounded-lg border border-line p-4">
            <div>
              <p class="text-sm font-medium text-foreground">{{ t('admin.riskControl.emailOnHit') }}</p>
              <p class="mt-1 text-xs text-muted">{{ t('admin.riskControl.emailOnHitHint') }}</p>
            </div>
            <Toggle v-model="configForm.email_on_hit" />
          </div>
          <div class="flex items-center justify-between rounded-lg border border-line p-4">
            <div>
              <p class="text-sm font-medium text-foreground">{{ t('admin.riskControl.autoBan') }}</p>
              <p class="mt-1 text-xs text-muted">{{ t('admin.riskControl.autoBanHint') }}</p>
            </div>
            <Toggle v-model="configForm.auto_ban_enabled" />
          </div>
          <div class="flex items-center justify-between rounded-lg border border-line p-4 lg:col-span-2">
            <div>
              <p class="text-sm font-medium text-foreground">{{ t('admin.riskControl.cyberPolicyExcludeBan') }}</p>
              <p class="mt-1 text-xs text-muted">{{ t('admin.riskControl.cyberPolicyExcludeBanHint') }}</p>
            </div>
            <Toggle v-model="configForm.cyber_policy_exclude_from_ban_count" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.riskControl.banThreshold') }}</label>
            <input v-model.number="configForm.ban_threshold" type="number" min="1" max="1000" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.riskControl.violationWindowHours') }}</label>
            <input v-model.number="configForm.violation_window_hours" type="number" min="1" max="8760" class="input" />
          </div>
        </div>
      </div>

      <div v-else-if="activeSettingsTab === 'riskThresholds'" class="space-y-5">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h3 class="text-base font-semibold text-foreground">{{ t('admin.riskControl.riskThresholds') }}</h3>
            <p class="mt-1 text-sm text-muted">{{ t('admin.riskControl.riskThresholdsHint') }}</p>
          </div>
          <button
            type="button"
            class="btn-glass-secondary inline-flex items-center justify-center gap-2"
            @click="resetRiskThresholds"
          >
            <Icon name="refresh" size="sm" />
            {{ t('admin.riskControl.riskThresholdReset') }}
          </button>
        </div>

        <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
          <div
            v-for="row in riskThresholdRows"
            :key="row.category"
            class="rounded-lg border border-line bg-surface-2 p-4"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <label class="block truncate text-sm font-semibold text-foreground" :for="`risk-threshold-${row.category}`">
                  {{ row.category }}
                </label>
                <p class="mt-1 text-xs text-muted">
                  {{ t('admin.riskControl.riskThresholdDefault', { value: formatThresholdPercent(row.defaultValue) }) }}
                </p>
              </div>
              <span class="inline-flex shrink-0 rounded-md bg-surface px-2 py-1 font-mono text-xs font-medium text-muted shadow-sm">
                {{ formatThresholdPercent(row.value) }}
              </span>
            </div>
            <div class="mt-3">
              <label class="sr-only" :for="`risk-threshold-${row.category}`">
                {{ t('admin.riskControl.riskThresholdPercent') }}
              </label>
              <div class="relative">
                <input
                  :id="`risk-threshold-${row.category}`"
                  v-model.number="configForm.thresholds[row.category]"
                  :data-test="`risk-threshold-${row.category}`"
                  type="number"
                  min="0"
                  max="100"
                  step="0.1"
                  class="input pr-8 font-mono"
                />
                <span class="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-muted">%</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="activeSettingsTab === 'keywords'" class="space-y-5">
        <div
          class="flex items-start gap-3 rounded-lg border p-4"
          :class="keywordNotice.toneClass"
        >
          <Icon
            :name="keywordNotice.icon"
            size="md"
            :class="keywordNotice.iconClass"
          />
          <div class="text-sm leading-6">
            <p class="font-medium" :class="keywordNotice.titleClass">{{ keywordNotice.title }}</p>
            <p class="mt-1 text-xs text-muted">{{ keywordNotice.description }}</p>
          </div>
        </div>

        <div class="space-y-2">
          <label class="input-label">{{ t('admin.riskControl.keywordBlockingMode') }}</label>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
            <button
              v-for="option in keywordBlockingModeOptions"
              :key="option.value"
              type="button"
              class="rounded-lg border p-3 text-left transition-colors"
              :class="configForm.keyword_blocking_mode === option.value
 ? 'border-[color-mix(in_oklch,var(--accent)_40%,transparent)] bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent shadow-sm   '
 : 'border-line hover:bg-surface-2'"
              @click="configForm.keyword_blocking_mode = option.value"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="text-sm font-semibold">{{ option.label }}</span>
                <span
                  class="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full border"
                  :class="configForm.keyword_blocking_mode === option.value
 ? 'border-accent bg-accent text-white'
 : 'border-line text-transparent '"
                >
                  <Icon name="check" size="xs" :stroke-width="2" />
                </span>
              </div>
              <p class="mt-1 text-xs leading-5 text-muted">{{ option.description }}</p>
            </button>
          </div>
        </div>

        <div>
          <div class="mb-2 flex items-center justify-between">
            <label class="input-label mb-0">{{ t('admin.riskControl.blockedKeywords') }}</label>
            <span class="inline-flex rounded-md bg-surface-2 px-2 py-1 text-xs text-muted">
              {{ t('admin.riskControl.blockedKeywordCount', { count: blockedKeywordCount }) }}
            </span>
          </div>
          <textarea
            v-model="configForm.blocked_keywords_text"
            class="input min-h-52 resize-y font-mono text-sm"
            :placeholder="t('admin.riskControl.blockedKeywordsPlaceholder')"
            :disabled="configForm.keyword_blocking_mode === 'api_only'"
          ></textarea>
          <p class="mt-2 text-xs text-muted">
            {{ t('admin.riskControl.blockedKeywordsLimit', { max: blockedKeywordMax }) }}
          </p>
        </div>
      </div>

      <div v-else class="grid grid-cols-1 gap-5 lg:grid-cols-2">
        <div>
          <label class="input-label">{{ t('admin.riskControl.hitRetentionDays') }}</label>
          <input v-model.number="configForm.hit_retention_days" type="number" min="1" max="3650" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.riskControl.nonHitRetentionDays') }}</label>
          <input v-model.number="configForm.non_hit_retention_days" type="number" min="1" max="3" class="input" />
        </div>
        <div class="rounded-lg border border-line p-4 text-sm text-muted lg:col-span-2">
          <div class="flex flex-wrap items-center gap-3">
            <Icon name="database" size="md" class="text-muted" />
            <span>{{ t('admin.riskControl.cleanupStats', { hit: status?.last_cleanup_deleted_hit ?? 0, nonHit: status?.last_cleanup_deleted_non_hit ?? 0 }) }}</span>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn-glass-secondary" @click="settingsOpen = false">{{ t('common.cancel') }}</button>
        <button type="button" class="btn-glass-primary inline-flex items-center gap-2" :disabled="saving" @click="saveConfig">
          <Icon v-if="saving" name="refresh" size="sm" class="animate-spin" />
          <Icon v-else name="check" size="sm" />
          {{ saving ? t('common.saving') : t('admin.riskControl.saveConfig') }}
        </button>
      </div>
    </template>
  </UiModal>
</template>

<script setup lang="ts">
import { toRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import UiModal from '@/components/ui/UiModal.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import ProxySelector from '@/components/common/ProxySelector.vue'
import type {
  ContentModerationConfig,
  ContentModerationRuntimeStatus,
} from '@/api/admin/riskControl'
import type { AdminGroup, Proxy } from '@/types'
import type { ConfigFormState } from './types'
import {
  apiKeyRowKey,
  apiKeyStatusBadgeClass,
  apiKeyStatusDotClass,
  blockedKeywordMax,
  formatNumber,
  formatThresholdPercent,
  percent,
  percentWidth,
} from './riskControlUtils'
import { useRiskControlSettings } from './useRiskControlSettings'

const { t } = useI18n()

const props = defineProps<{
  configForm: ConfigFormState
  proxies: Proxy[]
  groups: AdminGroup[]
  status: ContentModerationRuntimeStatus | null
  loadStatus: (silent?: boolean) => Promise<void>
  loadLogs: () => Promise<void>
  applyConfig: (config: ContentModerationConfig) => void
  hashActionLoading: boolean
  isFlaggedHashInputValid: boolean
}>()

const emit = defineEmits<{
  (e: 'delete-flagged-hash'): void
  (e: 'clear-flagged-hashes'): void
}>()

const settingsOpen = defineModel<boolean>('open', { required: true })
const flaggedHashInput = defineModel<string>('flaggedHashInput', { required: true })
const pendingDeleteApiKeyHashes = defineModel<string[]>('pendingDeleteApiKeyHashes', { required: true })

const statusRef = toRef(props, 'status')
const groupsRef = toRef(props, 'groups')

const {
  saving,
  apiKeyTesting,
  activeSettingsTab,
  groupSearch,
  apiKeyRowsExpanded,
  moderationTestPrompt,
  moderationTestImages,
  moderationTestResult,
  settingsTabs,
  modeOptions,
  keywordBlockingModeOptions,
  modelFilterOptions,
  keywordNotice,
  filteredGroups,
  inputApiKeyCount,
  blockedKeywordCount,
  pendingDeletedApiKeyCount,
  effectiveStoredApiKeyCount,
  apiKeysPlaceholder,
  apiKeysModeHint,
  storedApiKeyTestButtonText,
  apiKeyRows,
  visibleApiKeyRows,
  hiddenApiKeyRowCount,
  canToggleApiKeyRows,
  moderationScoreRows,
  riskThresholdRows,
  modelFilterModelCount,
  modelFilterSummary,
  modeDescription,
  apiKeyStatusLabel,
  apiKeyStatusMeta,
  toggleClearApiKey,
  setAPIKeysMode,
  setModelFilterType,
  testApiKeys,
  toggleDeleteStoredApiKey,
  isStoredApiKeyPendingDelete,
  clearModerationTestInput,
  removeModerationTestImage,
  handleModerationImageUpload,
  handleModerationImageDrop,
  handleModerationImagePaste,
  toggleGroup,
  isGroupSelected,
  resetRiskThresholds,
  saveConfig,
} = useRiskControlSettings(props.configForm, statusRef, groupsRef, pendingDeleteApiKeyHashes, {
  loadStatus: props.loadStatus,
  loadLogs: props.loadLogs,
  applyConfig: props.applyConfig,
  close: () => {
    settingsOpen.value = false
  },
})

const configForm = props.configForm

// 与原实现一致：每次打开设置弹层时重置到“基础”标签页
watch(settingsOpen, (value) => {
  if (value) activeSettingsTab.value = 'basic'
})
</script>
