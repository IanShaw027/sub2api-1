<template>
  <BaseDialog
    :show="show"
    :title="t('admin.tlsFingerprintProfiles.title')"
    width="wide"
    @close="$emit('close')"
  >
    <div class="space-y-5">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <p class="text-sm text-ink-soft">
          {{ t('admin.tlsFingerprintProfiles.description') }}
        </p>
        <div class="flex flex-wrap items-center gap-2">
          <button @click="refreshAll" :disabled="loading || captureLoading" class="btn btn-secondary btn-sm">
            <Icon
              name="refresh"
              size="sm"
              :class="['mr-1', (loading || captureLoading) ? 'animate-spin' : '']"
            />
            {{ t('common.refresh') }}
          </button>
          <button @click="showCreateModal = true" class="btn btn-primary btn-sm">
            <Icon name="plus" size="sm" class="mr-1" />
            {{ t('admin.tlsFingerprintProfiles.createProfile') }}
          </button>
        </div>
      </div>

      <div class="border-b border-line dark:border-dark-600">
        <nav class="-mb-px flex gap-6">
          <button
            type="button"
            class="border-b-2 px-1 pb-2 text-sm font-medium transition-colors"
            :class="activeTab === 'profiles'
              ? 'border-brand-500 text-brand-600 dark:text-brand-400'
              : 'border-transparent text-ink-soft hover:border-line hover:text-ink-body dark:hover:text-ink-body'"
            @click="activeTab = 'profiles'"
          >
            {{ t('admin.tlsFingerprintProfiles.tabs.profiles') }}
          </button>
          <button
            type="button"
            class="border-b-2 px-1 pb-2 text-sm font-medium transition-colors"
            :class="activeTab === 'capture'
              ? 'border-brand-500 text-brand-600 dark:text-brand-400'
              : 'border-transparent text-ink-soft hover:border-line hover:text-ink-body dark:hover:text-ink-body'"
            @click="activeTab = 'capture'"
          >
            {{ t('admin.tlsFingerprintProfiles.tabs.capture') }}
          </button>
        </nav>
      </div>

      <section v-show="activeTab === 'capture'" class="rounded-card border border-accent-200 bg-accent-50/70 p-4 shadow-xs dark:border-accent-900/60 dark:bg-accent-950/20">
        <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h4 class="text-sm font-semibold text-accent-900 dark:text-accent-100">
              {{ t('admin.tlsFingerprintProfiles.capture.title') }}
            </h4>
            <p class="mt-1 text-xs text-accent-700 dark:text-accent-300">
              {{ t('admin.tlsFingerprintProfiles.capture.description') }}
            </p>
          </div>
          <div class="flex flex-shrink-0 flex-wrap gap-2">
            <template v-if="captureView === 'form'">
              <button
                v-if="captureTasks.length > 0"
                @click="captureView = 'detail'"
                class="btn btn-secondary btn-sm"
              >
                {{ t('admin.tlsFingerprintProfiles.capture.backToTasks') }}
              </button>
              <button @click="startCaptureTask" :disabled="captureSubmitting" class="btn btn-primary btn-sm">
                <Icon v-if="captureSubmitting" name="refresh" size="sm" class="mr-1 animate-spin" />
                <Icon v-else name="play" size="sm" class="mr-1" />
                {{ t('admin.tlsFingerprintProfiles.capture.start') }}
              </button>
            </template>
            <template v-else>
              <button @click="captureView = 'form'" class="btn btn-secondary btn-sm">
                <Icon name="plus" size="sm" class="mr-1" />
                {{ t('admin.tlsFingerprintProfiles.capture.newTask') }}
              </button>
              <button
                v-if="selectedTask?.status === 'running'"
                @click="stopSelectedTask"
                :disabled="captureSubmitting"
                class="btn btn-secondary btn-sm"
              >
                {{ t('admin.tlsFingerprintProfiles.capture.stop') }}
              </button>
              <button
                v-if="selectedTask && selectedTask.status !== 'running'"
                @click="restartSelectedTask"
                :disabled="captureSubmitting"
                class="btn btn-secondary btn-sm"
              >
                {{ t('admin.tlsFingerprintProfiles.capture.restart') }}
              </button>
              <button
                v-if="selectedTask && selectedTask.status !== 'running'"
                @click="showDeleteTaskDialog = true"
                :disabled="captureSubmitting"
                class="btn btn-danger btn-sm"
              >
                {{ t('admin.tlsFingerprintProfiles.capture.delete') }}
              </button>
            </template>
          </div>
        </div>

        <!-- Form view: shown when creating a task -->
        <div v-if="captureView === 'form'" class="space-y-3">
          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.capture.taskName') }}</label>
            <input
              v-model="captureForm.name"
              type="text"
              class="input"
              :placeholder="t('admin.tlsFingerprintProfiles.capture.taskNamePlaceholder')"
            />
          </div>

          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.capture.uaKeywords') }}</label>
            <textarea
              v-model="captureForm.uaKeywords"
              rows="2"
              class="input font-mono text-xs"
              :placeholder="t('admin.tlsFingerprintProfiles.capture.uaKeywordsPlaceholder')"
            />
            <p class="input-hint text-xs">{{ t('admin.tlsFingerprintProfiles.capture.uaKeywordsHint') }}</p>
          </div>

          <label class="flex items-start gap-3 rounded-control border border-line bg-card px-3 py-2 dark:border-dark-600 dark:bg-dark-800">
            <input
              v-model="captureForm.storeBody"
              type="checkbox"
              class="mt-0.5 h-4 w-4 rounded border-line text-brand-600 focus:ring-accent/25"
              data-testid="capture-store-body"
            />
            <span>
              <span class="block text-xs font-medium text-ink-body">
                {{ t('admin.tlsFingerprintProfiles.capture.storeBody') }}
              </span>
              <span class="mt-0.5 block text-xs text-ink-soft">
                {{ t('admin.tlsFingerprintProfiles.capture.storeBodyHint') }}
              </span>
            </span>
          </label>

          <div>
            <div class="mb-2 flex items-center justify-between">
              <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.capture.targets') }}</label>
              <span class="text-xs text-ink-soft">
                {{ t('admin.tlsFingerprintProfiles.capture.targetsHint') }}
              </span>
            </div>
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
              <label
                v-for="target in captureTargets"
                :key="target.platform"
                class="flex items-center justify-between gap-2 rounded-control border border-line bg-card px-3 py-2 dark:border-dark-600 dark:bg-dark-800"
              >
                <span class="min-w-0 truncate text-xs font-medium text-ink-body">
                  {{ target.label }}
                </span>
                <input
                  v-model.number="target.count"
                  type="number"
                  min="0"
                  max="500"
                  class="input w-16 flex-shrink-0 px-2 py-1 text-center text-sm"
                />
              </label>
              <label class="flex items-center gap-2 rounded-control border border-dashed border-line bg-card px-3 py-2 dark:border-dark-600 dark:bg-dark-800">
                <input
                  v-model="customCaptureTarget.platform"
                  type="text"
                  class="input min-w-0 flex-1 px-2 py-1 text-sm"
                  :placeholder="t('admin.tlsFingerprintProfiles.capture.customPlatform')"
                />
                <input
                  v-model.number="customCaptureTarget.count"
                  type="number"
                  min="0"
                  max="500"
                  class="input w-16 flex-shrink-0 px-2 py-1 text-center text-sm"
                />
              </label>
            </div>
          </div>
        </div>

        <!-- Detail view: shown after a capture task is started/selected -->
        <div v-else class="space-y-3">
          <div v-if="captureLoading" class="flex items-center justify-center rounded-control bg-card py-8 dark:bg-dark-800">
            <Icon name="refresh" size="lg" class="animate-spin text-ink-faint" />
          </div>

          <div v-else-if="captureTasks.length === 0" class="rounded-control bg-card p-6 text-center text-sm text-ink-soft dark:bg-dark-800">
            {{ t('admin.tlsFingerprintProfiles.capture.noTasks') }}
          </div>

          <template v-else>
            <div class="flex gap-2 overflow-x-auto pb-1">
              <button
                v-for="task in captureTasks"
                :key="task.id"
                type="button"
                :class="[
                  'w-56 flex-shrink-0 rounded-control border p-3 text-left transition',
                  selectedTask?.id === task.id
                    ? 'border-brand-500 bg-brand-50 dark:border-brand-500 dark:bg-brand-900/20'
                    : 'border-line bg-card hover:border-brand-300 dark:border-dark-600 dark:bg-dark-800'
                ]"
                @click="selectTask(task.id)"
              >
                <div class="flex items-start justify-between gap-2">
                  <div class="min-w-0">
                    <div class="truncate text-sm font-semibold text-ink dark:text-white">{{ task.name }}</div>
                    <div class="mt-1 text-xs text-ink-soft">{{ formatDateTime(task.created_at) }}</div>
                  </div>
                  <span :class="['badge flex-shrink-0 whitespace-nowrap text-xs', captureStatusClass(task.status)]">
                    {{ t(`admin.tlsFingerprintProfiles.capture.status.${task.status}`) }}
                  </span>
                </div>
                <div class="mt-2 space-y-1">
                  <div
                    v-for="platform in Object.keys(task.targets || {})"
                    :key="platform"
                    class="flex items-center justify-between gap-3 text-xs text-ink-body"
                  >
                    <span>{{ platform }}</span>
                    <span>{{ task.counts?.[platform] || 0 }} / {{ task.targets?.[platform] || 0 }}</span>
                  </div>
                </div>
              </button>
            </div>

            <div v-if="selectedTask" class="rounded-control border border-line bg-card p-3 dark:border-dark-600 dark:bg-dark-800">
              <div class="mb-3">
                <div class="text-sm font-semibold text-ink dark:text-white">{{ selectedTask.name }}</div>
                <div class="text-xs text-ink-soft">
                  {{ t('admin.tlsFingerprintProfiles.capture.selectedTaskHint') }}
                </div>
              </div>

              <div class="mb-3 flex flex-wrap gap-2">
                <button @click="loadSelectedTaskSamples()" :disabled="samplesLoading" class="btn btn-secondary btn-sm">
                  <Icon
                    name="refresh"
                    size="sm"
                    :class="['mr-1', samplesLoading ? 'animate-spin' : '']"
                  />
                  {{ t('common.refresh') }}
                </button>
                <button @click="importSelectedSamples" :disabled="importingSamples || selectedSamples.length === 0" class="btn btn-primary btn-sm">
                  <Icon v-if="importingSamples" name="refresh" size="sm" class="mr-1 animate-spin" />
                  {{ t('admin.tlsFingerprintProfiles.capture.importSelected', { count: selectedSamples.length }) }}
                </button>
                <button @click="importAllSamples" :disabled="importingSamples || captureSamples.length === 0" class="btn btn-secondary btn-sm">
                  {{ t('admin.tlsFingerprintProfiles.capture.importAll') }}
                </button>
                <a
                  :href="collectorURL"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="btn btn-secondary btn-sm"
                >
                  {{ t('admin.tlsFingerprintProfiles.form.openCollector') }}
                </a>
                <button @click="copyCaptureConfig" class="btn btn-secondary btn-sm">
                  {{ t('admin.tlsFingerprintProfiles.capture.copyConfig') }}
                </button>
              </div>

              <div class="mb-3 space-y-1.5 rounded-control bg-page p-3 text-xs dark:bg-dark-700">
                <div
                  v-for="row in captureInfoRows"
                  :key="row.label"
                  class="flex items-center gap-3"
                >
                  <span class="w-24 flex-shrink-0 font-medium text-ink-soft">{{ row.label }}</span>
                  <span class="min-w-0 flex-1 truncate font-mono text-ink-body dark:text-ink-body">{{ row.value }}</span>
                  <button
                    type="button"
                    @click="copyText(row.value)"
                    class="flex-shrink-0 rounded p-1 text-ink-faint transition hover:bg-page hover:text-brand-600 dark:hover:bg-dark-600 dark:hover:text-brand-400"
                    :title="t('common.copy')"
                  >
                    <Icon name="copy" size="sm" />
                  </button>
                </div>
              </div>

              <div v-if="samplesLoading" class="flex items-center justify-center py-6">
                <Icon name="refresh" size="lg" class="animate-spin text-ink-faint" />
              </div>

              <div v-else-if="captureSamples.length === 0" class="py-4 text-center text-sm text-ink-soft">
                {{ t('admin.tlsFingerprintProfiles.capture.noSamples') }}
              </div>

              <div v-else class="max-h-[28rem] overflow-auto rounded-card border border-line dark:border-dark-600">
                <table class="min-w-full divide-y divide-line dark:divide-dark-700">
                  <thead class="sticky top-0 bg-page dark:bg-dark-700">
                    <tr>
                      <th class="w-8 px-2 py-2">
                        <input
                          type="checkbox"
                          :checked="allSamplesSelected"
                          @change="toggleAllSamples(($event.target as HTMLInputElement).checked)"
                        />
                      </th>
                      <th class="px-2 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                        {{ t('admin.tlsFingerprintProfiles.columns.platform') }}
                      </th>
                      <th class="px-2 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                        {{ t('admin.tlsFingerprintProfiles.capture.userAgent') }}
                      </th>
                      <th class="px-2 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                        {{ t('admin.tlsFingerprintProfiles.capture.originator') }}
                      </th>
                      <th class="px-2 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                        {{ t('admin.tlsFingerprintProfiles.capture.hash') }}
                      </th>
                      <th class="px-2 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                        {{ t('admin.tlsFingerprintProfiles.capture.details') }}
                      </th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-line bg-card dark:divide-dark-700 dark:bg-dark-800">
                    <tr v-for="sample in captureSamples" :key="sample.id">
                      <td class="px-2 py-2">
                        <input v-model="selectedSampleIDs" type="checkbox" :value="sample.id" />
                      </td>
                      <td class="px-2 py-2 text-xs text-ink-body">{{ sample.platform || 'shared' }}</td>
                      <td class="px-2 py-2">
                        <div class="max-w-sm truncate text-xs text-ink-body">{{ sample.user_agent || '—' }}</div>
                      </td>
                      <td class="px-2 py-2">
                        <div class="max-w-xs truncate text-xs text-ink-body">{{ sample.originator || '—' }}</div>
                      </td>
                      <td class="px-2 py-2">
                        <code class="text-xs text-ink-soft">{{ sample.fingerprint_hash.slice(0, 12) }}</code>
                      </td>
                      <td class="px-2 py-2">
                        <details class="text-xs">
                          <summary class="cursor-pointer text-brand-600 dark:text-brand-400">
                            {{ t('admin.tlsFingerprintProfiles.capture.viewDetails') }}
                          </summary>
                          <pre class="mt-2 max-h-56 overflow-auto rounded bg-ink dark:bg-dark-700 p-2 text-[11px] text-white">{{ formatSampleDetail(sample) }}</pre>
                        </details>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </template>
        </div>
      </section>

      <div v-show="activeTab === 'profiles'" class="space-y-5">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div class="flex flex-wrap items-center gap-2">
          <label class="text-xs font-medium text-ink-body">
            {{ t('admin.tlsFingerprintProfiles.filterPlatform') }}
          </label>
          <select v-model="profilePlatformFilter" class="input w-36 text-sm">
            <option value="">{{ t('admin.tlsFingerprintProfiles.allPlatforms') }}</option>
            <option v-for="platform in profilePlatforms" :key="platform" :value="platform">
              {{ platform || 'shared' }}
            </option>
          </select>
          <select v-model="profileOSFilter" class="input w-32 text-sm">
            <option value="">{{ t('admin.tlsFingerprintProfiles.allOS') }}</option>
            <option v-for="os in profileOSOptions" :key="os" :value="os">
              {{ os || 'shared' }}
            </option>
          </select>
          <select v-model="profileClientTypeFilter" class="input w-40 text-sm">
            <option value="">{{ t('admin.tlsFingerprintProfiles.allClientTypes') }}</option>
            <option v-for="ct in profileClientTypeOptions" :key="ct" :value="ct">
              {{ ct || 'shared' }}
            </option>
          </select>
          <input
            v-model="profileNameFilter"
            type="text"
            class="input w-40 text-sm"
            :placeholder="t('admin.tlsFingerprintProfiles.filterNamePlaceholder')"
          />
        </div>
        <div class="text-xs text-ink-soft">
          {{ t('admin.tlsFingerprintProfiles.profileCount', { count: filteredProfiles.length }) }}
        </div>
      </div>

      <div v-if="loading" class="flex items-center justify-center py-8">
        <Icon name="refresh" size="lg" class="animate-spin text-ink-faint" />
      </div>

      <div v-else-if="filteredProfiles.length === 0" class="py-8 text-center">
        <div class="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-line dark:bg-dark-700">
          <Icon name="shield" size="lg" class="text-ink-faint" />
        </div>
        <h4 class="mb-1 text-sm font-medium text-ink dark:text-white">
          {{ t('admin.tlsFingerprintProfiles.noProfiles') }}
        </h4>
        <p class="text-sm text-ink-soft">
          {{ t('admin.tlsFingerprintProfiles.createFirstProfile') }}
        </p>
      </div>

      <div v-else class="max-h-96 overflow-auto rounded-card border border-line dark:border-dark-600">
        <table class="min-w-full divide-y divide-line dark:divide-dark-700">
          <thead class="sticky top-0 bg-page dark:bg-dark-700">
            <tr>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                {{ t('admin.tlsFingerprintProfiles.columns.platform') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                {{ t('admin.tlsFingerprintProfiles.columns.transport') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                {{ t('admin.tlsFingerprintProfiles.columns.os') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                {{ t('admin.tlsFingerprintProfiles.columns.clientType') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                {{ t('admin.tlsFingerprintProfiles.columns.name') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                {{ t('admin.tlsFingerprintProfiles.columns.description') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                {{ t('admin.tlsFingerprintProfiles.columns.grease') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                {{ t('admin.tlsFingerprintProfiles.columns.alpn') }}
              </th>
              <th class="px-3 py-2 text-left text-xs font-medium uppercase text-ink-soft">
                {{ t('admin.tlsFingerprintProfiles.columns.actions') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-line bg-card dark:divide-dark-700 dark:bg-dark-800">
            <tr v-for="profile in filteredProfiles" :key="profile.id" class="hover:bg-page dark:hover:bg-dark-700">
              <td class="px-3 py-2">
                <span class="badge badge-gray text-xs">{{ profile.platform || 'shared' }}</span>
              </td>
              <td class="px-3 py-2">
                <span v-if="profile.transport" class="badge badge-primary text-xs">{{ profile.transport }}</span>
                <span v-else class="text-xs text-ink-faint dark:text-ink-soft">—</span>
              </td>
              <td class="px-3 py-2">
                <span v-if="profile.os" class="badge badge-gray text-xs">{{ profile.os }}</span>
                <span v-else class="text-xs text-ink-faint dark:text-ink-soft">—</span>
              </td>
              <td class="px-3 py-2">
                <span v-if="profile.client_type" class="badge badge-gray text-xs">{{ profile.client_type }}</span>
                <span v-else class="text-xs text-ink-faint dark:text-ink-soft">—</span>
              </td>
              <td class="px-3 py-2">
                <div class="text-sm font-medium text-ink dark:text-white">{{ profile.name }}</div>
              </td>
              <td class="px-3 py-2">
                <div v-if="profile.description" class="max-w-xs truncate text-sm text-ink-soft">
                  {{ profile.description }}
                </div>
                <div v-else class="text-xs text-ink-faint dark:text-ink-soft">—</div>
              </td>
              <td class="px-3 py-2">
                <Icon
                  :name="profile.enable_grease ? 'check' : 'lock'"
                  size="sm"
                  :class="profile.enable_grease ? 'text-success' : 'text-ink-faint'"
                />
              </td>
              <td class="px-3 py-2">
                <div v-if="profile.alpn_protocols?.length" class="flex flex-wrap gap-1">
                  <span
                    v-for="proto in profile.alpn_protocols.slice(0, 3)"
                    :key="proto"
                    class="badge badge-primary text-xs"
                  >
                    {{ proto }}
                  </span>
                  <span v-if="profile.alpn_protocols.length > 3" class="text-xs text-ink-soft">
                    +{{ profile.alpn_protocols.length - 3 }}
                  </span>
                </div>
                <div v-else class="text-xs text-ink-faint dark:text-ink-soft">—</div>
              </td>
              <td class="px-3 py-2">
                <div class="flex items-center gap-1">
                  <button
                    @click="handleEdit(profile)"
                    class="p-1 text-ink-soft hover:text-brand-600 dark:hover:text-brand-400"
                    :title="t('common.edit')"
                  >
                    <Icon name="edit" size="sm" />
                  </button>
                  <button
                    @click="handleDelete(profile)"
                    class="p-1 text-ink-soft hover:text-danger dark:hover:text-danger"
                    :title="t('common.delete')"
                  >
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button @click="$emit('close')" class="btn btn-secondary">
          {{ t('common.close') }}
        </button>
      </div>
    </template>

    <BaseDialog
      :show="showCreateModal || showEditModal"
      :title="showEditModal ? t('admin.tlsFingerprintProfiles.editProfile') : t('admin.tlsFingerprintProfiles.createProfile')"
      width="wide"
      :z-index="60"
      @close="closeFormModal"
    >
      <form @submit.prevent="handleSubmit" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.tlsFingerprintProfiles.form.pasteYaml') }}</label>
          <textarea
            v-model="yamlInput"
            rows="4"
            class="input font-mono text-xs"
            :placeholder="t('admin.tlsFingerprintProfiles.form.pasteYamlPlaceholder')"
            @paste="handleYamlPaste"
          />
          <div class="mt-1 flex items-center gap-2">
            <button type="button" @click="parseYamlInput" class="btn btn-secondary btn-sm">
              {{ t('admin.tlsFingerprintProfiles.form.parseYaml') }}
            </button>
            <p class="text-xs text-ink-soft">
              {{ t('admin.tlsFingerprintProfiles.form.pasteYamlHint') }}
              <a :href="collectorURL" target="_blank" rel="noopener noreferrer" class="text-brand-600 underline hover:text-brand-700 dark:text-brand-400 dark:hover:text-brand-300">{{ t('admin.tlsFingerprintProfiles.form.openCollector') }}</a>
            </p>
          </div>
        </div>

        <hr class="border-line dark:border-dark-600" />

        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="input-label">{{ t('admin.tlsFingerprintProfiles.form.platform') }}</label>
            <input
              v-model="form.platform"
              type="text"
              class="input"
              :placeholder="t('admin.tlsFingerprintProfiles.form.platformPlaceholder')"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.tlsFingerprintProfiles.form.transport') }}</label>
            <select v-model="form.transport" class="input">
              <option value="">{{ t('admin.tlsFingerprintProfiles.form.transportAny') }}</option>
              <option value="http1">HTTP/1.1</option>
              <option value="h2">{{ t('admin.tlsFingerprintProfiles.form.transportH2') }}</option>
              <option value="websocket-http1">WebSocket HTTP/1.1</option>
              <option value="websocket-h2">{{ t('admin.tlsFingerprintProfiles.form.transportWebsocketH2') }}</option>
            </select>
            <p class="input-hint text-xs">{{ t('admin.tlsFingerprintProfiles.form.transportReplayHint') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.tlsFingerprintProfiles.form.os') }}</label>
            <select v-model="form.os" class="input">
              <option value="">{{ t('admin.tlsFingerprintProfiles.form.osAny') }}</option>
              <option value="windows">Windows</option>
              <option value="macos">macOS</option>
              <option value="linux">Linux</option>
            </select>
          </div>
          <div>
            <label class="input-label">{{ t('admin.tlsFingerprintProfiles.form.clientType') }}</label>
            <input
              v-model="form.client_type"
              type="text"
              class="input"
              :placeholder="t('admin.tlsFingerprintProfiles.form.clientTypePlaceholder')"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.tlsFingerprintProfiles.form.name') }}</label>
            <input
              v-model="form.name"
              type="text"
              required
              class="input"
              :placeholder="t('admin.tlsFingerprintProfiles.form.namePlaceholder')"
            />
          </div>
          <div class="col-span-2">
            <label class="input-label">{{ t('admin.tlsFingerprintProfiles.form.description') }}</label>
            <input
              v-model="form.description"
              type="text"
              class="input"
              :placeholder="t('admin.tlsFingerprintProfiles.form.descriptionPlaceholder')"
            />
          </div>
          <div class="col-span-2">
            <label class="input-label">{{ t('admin.tlsFingerprintProfiles.form.userAgent') }}</label>
            <input
              v-model="form.user_agent"
              type="text"
              class="input font-mono text-xs"
              :placeholder="t('admin.tlsFingerprintProfiles.form.userAgentPlaceholder')"
            />
            <p class="input-hint text-xs">{{ t('admin.tlsFingerprintProfiles.form.userAgentHint') }}</p>
          </div>
          <div class="col-span-2">
            <label class="input-label">{{ t('admin.tlsFingerprintProfiles.form.originator') }}</label>
            <input
              v-model="form.originator"
              type="text"
              class="input font-mono text-xs"
              :placeholder="t('admin.tlsFingerprintProfiles.form.originatorPlaceholder')"
            />
            <p class="input-hint text-xs">{{ t('admin.tlsFingerprintProfiles.form.originatorHint') }}</p>
          </div>
          <div v-if="form.transport === 'h2' || form.transport === 'websocket-h2'" class="col-span-2">
            <label class="input-label">{{ t('admin.tlsFingerprintProfiles.form.http2Fingerprint') }}</label>
            <textarea
              v-model="form.http2_fingerprint"
              rows="3"
              required
              class="input font-mono text-xs"
              data-testid="http2-fingerprint"
              :placeholder="t('admin.tlsFingerprintProfiles.form.http2FingerprintPlaceholder')"
            />
            <p class="input-hint text-xs">{{ t('admin.tlsFingerprintProfiles.form.http2FingerprintHint') }}</p>
          </div>
        </div>

        <div class="flex items-center gap-3">
          <button
            type="button"
            @click="form.enable_grease = !form.enable_grease"
            :class="[
              'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-150 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent/25 focus:ring-offset-2',
              form.enable_grease ? 'bg-brand-600' : 'bg-line dark:bg-dark-600'
            ]"
          >
            <span
              :class="[
                'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-150 ease-in-out',
                form.enable_grease ? 'translate-x-4' : 'translate-x-0'
              ]"
            />
          </button>
          <div>
            <span class="text-sm font-medium text-ink-body">
              {{ t('admin.tlsFingerprintProfiles.form.enableGrease') }}
            </span>
            <p class="text-xs text-ink-soft">
              {{ t('admin.tlsFingerprintProfiles.form.enableGreaseHint') }}
            </p>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.cipherSuites') }}</label>
            <textarea v-model="fieldInputs.cipher_suites" rows="2" class="input font-mono text-xs" placeholder="0x1301, 0x1302, 0xc02c" />
            <p class="input-hint text-xs">{{ t('admin.tlsFingerprintProfiles.form.cipherSuitesHint') }}</p>
          </div>

          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.curves') }}</label>
            <textarea v-model="fieldInputs.curves" rows="2" class="input font-mono text-xs" placeholder="29, 23, 24" />
            <p class="input-hint text-xs">{{ t('admin.tlsFingerprintProfiles.form.curvesHint') }}</p>
          </div>

          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.signatureAlgorithms') }}</label>
            <textarea v-model="fieldInputs.signature_algorithms" rows="2" class="input font-mono text-xs" placeholder="0x0403, 0x0804, 0x0401" />
          </div>

          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.signatureAlgorithmsCert') }}</label>
            <textarea v-model="fieldInputs.signature_algorithms_cert" rows="2" class="input font-mono text-xs" placeholder="0x0403, 0x0804" />
          </div>

          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.supportedVersions') }}</label>
            <textarea v-model="fieldInputs.supported_versions" rows="2" class="input font-mono text-xs" placeholder="0x0304, 0x0303" />
          </div>

          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.keyShareGroups') }}</label>
            <textarea v-model="fieldInputs.key_share_groups" rows="2" class="input font-mono text-xs" placeholder="29, 23" />
          </div>

          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.extensions') }}</label>
            <textarea v-model="fieldInputs.extensions" rows="2" class="input font-mono text-xs" placeholder="0x0000, 0x0005, 0x000a" />
          </div>

          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.extensionPayloads') }}</label>
            <textarea v-model="fieldInputs.extension_payloads" rows="2" class="input font-mono text-xs" placeholder='{"65037":"AQID"}' />
          </div>

          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.pointFormats') }}</label>
            <textarea v-model="fieldInputs.point_formats" rows="2" class="input font-mono text-xs" placeholder="0" />
          </div>

          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.pskModes') }}</label>
            <textarea v-model="fieldInputs.psk_modes" rows="2" class="input font-mono text-xs" placeholder="1" />
          </div>

          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.compressCertAlgos') }}</label>
            <textarea v-model="fieldInputs.compress_cert_algos" rows="2" class="input font-mono text-xs" placeholder="2, 1" />
          </div>

          <div>
            <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.delegatedCredentialsAlgorithms') }}</label>
            <textarea v-model="fieldInputs.delegated_credentials_algorithms" rows="2" class="input font-mono text-xs" placeholder="0x0403, 0x0804" />
          </div>
        </div>

        <div>
          <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.alpnProtocols') }}</label>
          <textarea v-model="fieldInputs.alpn_protocols" rows="2" class="input font-mono text-xs" placeholder="h2, http/1.1" />
        </div>

        <div>
          <label class="input-label text-xs">{{ t('admin.tlsFingerprintProfiles.form.applicationSettingsProtocols') }}</label>
          <textarea v-model="fieldInputs.application_settings_protocols" rows="2" class="input font-mono text-xs" placeholder="h2" />
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeFormModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button @click="handleSubmit" :disabled="submitting" class="btn btn-primary">
            <Icon v-if="submitting" name="refresh" size="sm" class="mr-1 animate-spin" />
            {{ showEditModal ? t('common.update') : t('common.create') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.tlsFingerprintProfiles.deleteProfile')"
      :message="t('admin.tlsFingerprintProfiles.deleteConfirmMessage', { name: deletingProfile?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <ConfirmDialog
      :show="showDeleteTaskDialog"
      :title="t('admin.tlsFingerprintProfiles.capture.delete')"
      :message="t('admin.tlsFingerprintProfiles.capture.deleteConfirmMessage', { name: selectedTask?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDeleteTask"
      @cancel="showDeleteTaskDialog = false"
    />
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type {
  TLSFingerprintCaptureSample,
  TLSFingerprintCaptureTask,
  TLSFingerprintProfileTransport,
  TLSFingerprintProfile
} from '@/api/admin/tlsFingerprintProfile'
import { formatDateTime } from '@/utils/format'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  show: boolean
}>()

defineEmits<{
  close: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const profiles = ref<TLSFingerprintProfile[]>([])
const loading = ref(false)
const submitting = ref(false)
const showCreateModal = ref(false)
const showEditModal = ref(false)
const showDeleteDialog = ref(false)
const editingProfile = ref<TLSFingerprintProfile | null>(null)
const deletingProfile = ref<TLSFingerprintProfile | null>(null)
const yamlInput = ref('')
const profilePlatformFilter = ref('')
const profileOSFilter = ref('')
const profileClientTypeFilter = ref('')
const profileNameFilter = ref('')

const captureTasks = ref<TLSFingerprintCaptureTask[]>([])
const selectedTaskID = ref<number | null>(null)
const captureSamples = ref<TLSFingerprintCaptureSample[]>([])
const selectedSampleIDs = ref<number[]>([])
const captureLoading = ref(false)
const captureSubmitting = ref(false)
const samplesLoading = ref(false)
const importingSamples = ref(false)
const activeTab = ref<'profiles' | 'capture'>('profiles')
const captureView = ref<'form' | 'detail'>('form')
let capturePollTimer: ReturnType<typeof setInterval> | null = null

const captureForm = reactive({
  name: 'Codex TLS fingerprint capture',
  uaKeywords: 'codex, Codex Desktop, codex-tui, codex_exec',
  storeBody: false
})

const captureTargets = reactive([
  { platform: 'openai', label: 'OpenAI / Codex', count: 100 },
  { platform: 'anthropic', label: 'Anthropic / Claude', count: 0 },
  { platform: 'gemini', label: 'Gemini', count: 0 },
  { platform: 'kiro', label: 'Kiro', count: 0 },
  { platform: 'grok', label: 'Grok / xAI', count: 0 },
  { platform: 'antigravity', label: 'Antigravity', count: 0 }
])

const customCaptureTarget = reactive({
  platform: '',
  count: 0
})

const fieldInputs = reactive({
  cipher_suites: '',
  curves: '',
  point_formats: '',
  signature_algorithms: '',
  signature_algorithms_cert: '',
  alpn_protocols: '',
  supported_versions: '',
  key_share_groups: '',
  psk_modes: '',
  extensions: '',
  extension_payloads: '',
  compress_cert_algos: '',
  delegated_credentials_algorithms: '',
  application_settings_protocols: ''
})

const form = reactive({
  platform: 'openai',
  transport: '' as TLSFingerprintProfileTransport,
  os: '',
  client_type: '',
  name: '',
  user_agent: '',
  originator: '',
  http2_fingerprint: '',
  description: null as string | null,
  enable_grease: false
})

const selectedTask = computed(() => {
  return captureTasks.value.find(task => task.id === selectedTaskID.value) || null
})

const selectedCapturePlatform = computed(() => {
  const firstTargetPlatform = Object.keys(selectedTask.value?.targets || {})[0]
  return firstTargetPlatform || form.platform || 'openai'
})

const selectedCaptureURL = computed(() => {
  const task = selectedTask.value
  if (task?.capture_url) {
    return trimTrailingSlash(task.capture_url)
  }
  if (task?.capture_base_url) {
    return normalizeCaptureURL(task.capture_base_url)
  }
  if (typeof window === 'undefined') {
    return 'https://localhost:8444/capture'
  }
  return `https://${window.location.hostname}:8444/capture`
})

const selectedPlatformBaseURL = computed(() => {
  return `${selectedCaptureURL.value}/${encodeURIComponent(selectedCapturePlatform.value)}/v1`
})

const collectorURL = computed(() => {
  const base = typeof window === 'undefined' ? '' : window.location.origin
  const params = new URLSearchParams()
  params.set('capture_url', selectedCaptureURL.value)

  if (selectedTask.value?.token) {
    params.set('token', selectedTask.value.token)
  }

  params.set('platform', selectedCapturePlatform.value)

  return `${base}/tls-fingerprint-collector?${params.toString()}`
})

const captureInfoRows = computed(() => {
  if (!selectedTask.value) return []
  return [
    { label: t('admin.tlsFingerprintProfiles.capture.captureUrl'), value: selectedCaptureURL.value },
    { label: t('admin.tlsFingerprintProfiles.capture.platformBaseUrl'), value: selectedPlatformBaseURL.value },
    { label: t('admin.tlsFingerprintProfiles.capture.token'), value: selectedTask.value.token || '' },
    { label: t('admin.tlsFingerprintProfiles.capture.collectorUrl'), value: collectorURL.value }
  ]
})

const selectedSamples = computed(() => {
  const selected = new Set(selectedSampleIDs.value)
  return captureSamples.value.filter(sample => selected.has(sample.id))
})

const allSamplesSelected = computed(() => {
  return captureSamples.value.length > 0 && selectedSampleIDs.value.length === captureSamples.value.length
})

const profilePlatforms = computed(() => {
  return Array.from(new Set(profiles.value.map(profile => profile.platform || '').filter(Boolean))).sort()
})

const profileOSOptions = computed(() => {
  return Array.from(new Set(profiles.value.map(profile => profile.os || '').filter(Boolean))).sort()
})

const profileClientTypeOptions = computed(() => {
  return Array.from(new Set(profiles.value.map(profile => profile.client_type || '').filter(Boolean))).sort()
})

const filteredProfiles = computed(() => {
  const nameKeyword = profileNameFilter.value.trim().toLowerCase()
  return profiles.value.filter(profile => {
    if (profilePlatformFilter.value && (profile.platform || '') !== profilePlatformFilter.value) {
      return false
    }
    if (profileOSFilter.value && (profile.os || '') !== profileOSFilter.value) {
      return false
    }
    if (profileClientTypeFilter.value && (profile.client_type || '') !== profileClientTypeFilter.value) {
      return false
    }
    if (nameKeyword && !(profile.name || '').toLowerCase().includes(nameKeyword)) {
      return false
    }
    return true
  })
})

const refreshAll = async () => {
  await Promise.all([loadProfiles(), loadCaptureTasks()])
}

const loadProfiles = async () => {
  loading.value = true
  try {
    profiles.value = await adminAPI.tlsFingerprintProfiles.list()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.tlsFingerprintProfiles.loadFailed'))
    console.error('Error loading TLS fingerprint profiles:', error)
  } finally {
    loading.value = false
  }
}

const loadCaptureTasks = async (options: { silent?: boolean } = {}) => {
  const { silent = false } = options
  // 轮询时静默刷新：不切换 captureLoading，避免采集详情区整体被销毁重建
  // （否则任务卡片、样本表格每 3 秒整块闪烁，视觉上像整个弹窗刷新）。
  if (!silent) {
    captureLoading.value = true
  }
  try {
    const previousTasks = new Map(captureTasks.value.map(task => [task.id, task]))
    const listedTasks = await adminAPI.tlsFingerprintProfiles.listCaptureTasks()
    captureTasks.value = listedTasks.map(task => {
      const previousToken = previousTasks.get(task.id)?.token
      return task.token || !previousToken ? task : { ...task, token: previousToken }
    })
    if (!selectedTaskID.value && captureTasks.value.length > 0) {
      selectedTaskID.value = captureTasks.value[0].id
      await loadSelectedTaskSamples({ silent })
    }
    if (selectedTaskID.value && !captureTasks.value.some(task => task.id === selectedTaskID.value)) {
      selectedTaskID.value = captureTasks.value[0]?.id || null
      await loadSelectedTaskSamples({ silent })
    }
    if (selectedTaskID.value) {
      await loadSelectedTaskDetail(selectedTaskID.value)
    }
    updateCapturePollingState()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.tlsFingerprintProfiles.capture.loadFailed'))
    console.error('Error loading TLS fingerprint capture tasks:', error)
  } finally {
    if (!silent) {
      captureLoading.value = false
    }
  }
}

const loadSelectedTaskDetail = async (taskID: number) => {
  try {
    const detail = await adminAPI.tlsFingerprintProfiles.getCaptureTask(taskID)
    captureTasks.value = captureTasks.value.map(task => {
      if (task.id !== detail.id) return task
      const token = detail.token || task.token
      return token ? { ...task, ...detail, token } : { ...task, ...detail }
    })
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.tlsFingerprintProfiles.capture.loadFailed'))
    console.error('Error loading TLS fingerprint capture task details:', error)
  }
}

const loadSelectedTaskSamples = async (options: { silent?: boolean } = {}) => {
  if (!selectedTaskID.value) {
    captureSamples.value = []
    selectedSampleIDs.value = []
    return
  }
  const { silent = false } = options
  // 轮询时静默刷新：不切换 samplesLoading，避免样本表格容器被销毁重建
  // （否则滚动位置、展开的详情会被重置）。样本行有稳定 :key，整表赋值时 DOM 会复用。
  if (!silent) {
    samplesLoading.value = true
  }
  try {
    captureSamples.value = await adminAPI.tlsFingerprintProfiles.listCaptureSamples(selectedTaskID.value)
    const available = new Set(captureSamples.value.map(sample => sample.id))
    selectedSampleIDs.value = selectedSampleIDs.value.filter(id => available.has(id))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.tlsFingerprintProfiles.capture.samplesLoadFailed'))
    console.error('Error loading TLS fingerprint capture samples:', error)
  } finally {
    if (!silent) {
      samplesLoading.value = false
    }
  }
}

const selectTask = async (taskID: number) => {
  selectedTaskID.value = taskID
  selectedSampleIDs.value = []
  await Promise.all([loadSelectedTaskDetail(taskID), loadSelectedTaskSamples()])
}

const startCapturePolling = () => {
  if (capturePollTimer) {
    return
  }
  capturePollTimer = setInterval(async () => {
    if (!props.show) {
      return
    }
    await loadCaptureTasks({ silent: true })
    if (selectedTaskID.value) {
      await loadSelectedTaskSamples({ silent: true })
    }
  }, 3000)
}

const stopCapturePolling = () => {
  if (capturePollTimer) {
    clearInterval(capturePollTimer)
    capturePollTimer = null
  }
}

const updateCapturePollingState = () => {
  if (captureTasks.value.some(task => task.status === 'running')) {
    startCapturePolling()
    return
  }
  stopCapturePolling()
}

watch(
  () => props.show,
  (newVal) => {
    if (newVal) {
      refreshAll()
      startCapturePolling()
    } else {
      stopCapturePolling()
    }
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  stopCapturePolling()
})

watch(activeTab, (tab) => {
  if (tab === 'capture') {
    captureView.value = captureTasks.value.length > 0 ? 'detail' : 'form'
  }
})

const buildCaptureTargets = (): Record<string, number> => {
  const targets: Record<string, number> = {}
  for (const target of captureTargets) {
    const count = Number(target.count || 0)
    if (count > 0) {
      targets[target.platform] = count
    }
  }
  const customPlatform = customCaptureTarget.platform.trim()
  const customCount = Number(customCaptureTarget.count || 0)
  if (customPlatform && customCount > 0) {
    targets[customPlatform] = customCount
  }
  return targets
}

const startCaptureTask = async () => {
  const targets = buildCaptureTargets()
  if (Object.keys(targets).length === 0) {
    appStore.showError(t('admin.tlsFingerprintProfiles.capture.targetRequired'))
    return
  }
  captureSubmitting.value = true
  try {
    const task = await adminAPI.tlsFingerprintProfiles.startCaptureTask({
      name: captureForm.name.trim() || undefined,
      targets,
      ua_keywords: parseStringArray(captureForm.uaKeywords),
      capture_filters: captureForm.storeBody ? { store_body: true } : undefined
    })
    captureTasks.value = [task, ...captureTasks.value.filter(item => item.id !== task.id)]
    selectedTaskID.value = task.id
    captureSamples.value = []
    selectedSampleIDs.value = []
    captureView.value = 'detail'
    startCapturePolling()
    appStore.showSuccess(t('admin.tlsFingerprintProfiles.capture.startSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.tlsFingerprintProfiles.capture.startFailed'))
    console.error('Error starting TLS fingerprint capture task:', error)
  } finally {
    captureSubmitting.value = false
  }
}

const stopSelectedTask = async () => {
  if (!selectedTask.value) return
  captureSubmitting.value = true
  try {
    const task = await adminAPI.tlsFingerprintProfiles.stopCaptureTask(selectedTask.value.id)
    captureTasks.value = captureTasks.value.map(item => item.id === task.id ? task : item)
    updateCapturePollingState()
    appStore.showSuccess(t('admin.tlsFingerprintProfiles.capture.stopSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.tlsFingerprintProfiles.capture.stopFailed'))
    console.error('Error stopping TLS fingerprint capture task:', error)
  } finally {
    captureSubmitting.value = false
  }
}

const restartSelectedTask = async () => {
  if (!selectedTask.value) return
  captureSubmitting.value = true
  try {
    const task = await adminAPI.tlsFingerprintProfiles.restartCaptureTask(selectedTask.value.id)
    captureTasks.value = captureTasks.value.map(item => item.id === task.id ? task : item)
    startCapturePolling()
    appStore.showSuccess(t('admin.tlsFingerprintProfiles.capture.restartSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.tlsFingerprintProfiles.capture.restartFailed'))
    console.error('Error restarting TLS fingerprint capture task:', error)
  } finally {
    captureSubmitting.value = false
  }
}

const showDeleteTaskDialog = ref(false)

const confirmDeleteTask = async () => {
  if (!selectedTask.value) return
  captureSubmitting.value = true
  try {
    await adminAPI.tlsFingerprintProfiles.deleteCaptureTask(selectedTask.value.id)
    captureTasks.value = captureTasks.value.filter(item => item.id !== selectedTask.value!.id)
    selectedTaskID.value = captureTasks.value[0]?.id || null
    captureSamples.value = []
    selectedSampleIDs.value = []
    if (captureTasks.value.length === 0) {
      captureView.value = 'form'
    }
    updateCapturePollingState()
    appStore.showSuccess(t('admin.tlsFingerprintProfiles.capture.deleteSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.tlsFingerprintProfiles.capture.deleteFailed'))
    console.error('Error deleting TLS fingerprint capture task:', error)
  } finally {
    showDeleteTaskDialog.value = false
    captureSubmitting.value = false
  }
}

const importSelectedSamples = async () => {
  if (!selectedTaskID.value || selectedSamples.value.length === 0) {
    return
  }
  await importSamples(selectedSampleIDs.value)
}

const importAllSamples = async () => {
  if (!selectedTaskID.value || captureSamples.value.length === 0) {
    return
  }
  await importSamples([])
}

const importSamples = async (sampleIDs: number[]) => {
  if (!selectedTaskID.value) return
  importingSamples.value = true
  try {
    const result = await adminAPI.tlsFingerprintProfiles.importCaptureTaskSamples(selectedTaskID.value, {
      sample_ids: sampleIDs
    })
    appStore.showSuccess(t('admin.tlsFingerprintProfiles.capture.importSuccess', {
      imported: result.imported,
      duplicates: result.duplicates
    }))
    selectedSampleIDs.value = []
    await loadProfiles()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.tlsFingerprintProfiles.capture.importFailed'))
    console.error('Error importing TLS fingerprint capture samples:', error)
  } finally {
    importingSamples.value = false
  }
}

const toggleAllSamples = (checked: boolean) => {
  selectedSampleIDs.value = checked ? captureSamples.value.map(sample => sample.id) : []
}

const copyCaptureConfig = async () => {
  if (!selectedTask.value) return
  const config = JSON.stringify({
    collector_url: collectorURL.value,
    capture_url: selectedCaptureURL.value,
    platform_base_url: selectedPlatformBaseURL.value,
    api_key: selectedTask.value.token,
    authorization: `Bearer ${selectedTask.value.token || ''}`,
    token: selectedTask.value.token,
    targets: selectedTask.value.targets,
    ua_keywords: selectedTask.value.ua_keywords
  }, null, 2)
  try {
    await navigator.clipboard.writeText(config)
    appStore.showSuccess(t('admin.tlsFingerprintProfiles.capture.copySuccess'))
  } catch {
    appStore.showError(t('admin.tlsFingerprintProfiles.capture.copyFailed'))
  }
}

const copyText = async (value: string) => {
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
    appStore.showSuccess(t('admin.tlsFingerprintProfiles.capture.copyValueSuccess'))
  } catch {
    appStore.showError(t('admin.tlsFingerprintProfiles.capture.copyValueFailed'))
  }
}

const captureStatusClass = (status: string): string => {
  if (status === 'running') return 'badge-primary'
  if (status === 'completed') return 'badge-success'
  if (status === 'stopped') return 'badge-gray'
  return 'badge-gray'
}

const formatSampleDetail = (sample: TLSFingerprintCaptureSample): string => {
  return JSON.stringify({
    platform: sample.platform,
    user_agent: sample.user_agent,
    originator: sample.originator,
    fingerprint_hash: sample.fingerprint_hash,
    profile: sample.profile,
    raw_payload: sample.raw_payload,
    raw_client_hello: sample.raw_client_hello
  }, null, 2)
}

const trimTrailingSlash = (value: string): string => {
  return value.trim().replace(/\/+$/, '')
}

const normalizeCaptureURL = (value: string): string => {
  const normalized = trimTrailingSlash(value)
  if (!normalized) {
    return ''
  }
  return normalized.endsWith('/capture') ? normalized : `${normalized}/capture`
}

const resetForm = () => {
  form.platform = 'openai'
  form.transport = ''
  form.os = ''
  form.client_type = ''
  form.name = ''
  form.user_agent = ''
  form.originator = ''
  form.http2_fingerprint = ''
  form.description = null
  form.enable_grease = false
  fieldInputs.cipher_suites = ''
  fieldInputs.curves = ''
  fieldInputs.point_formats = ''
  fieldInputs.signature_algorithms = ''
  fieldInputs.signature_algorithms_cert = ''
  fieldInputs.alpn_protocols = ''
  fieldInputs.supported_versions = ''
  fieldInputs.key_share_groups = ''
  fieldInputs.psk_modes = ''
  fieldInputs.extensions = ''
  fieldInputs.extension_payloads = ''
  fieldInputs.compress_cert_algos = ''
  fieldInputs.delegated_credentials_algorithms = ''
  fieldInputs.application_settings_protocols = ''
  yamlInput.value = ''
}

const parseYamlInput = () => {
  const text = yamlInput.value.trim()
  if (!text) return

  const lines = text.split('\n')
  let foundName = false

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) continue

    const match = trimmed.match(/^(\w+):\s*(.+)$/)
    if (!match) continue

    const [, key, rawValue] = match
    const value = rawValue.trim()

    switch (key) {
      case 'platform':
        form.platform = value.replace(/^["']|["']$/g, '')
        break
      case 'transport':
        form.transport = value.replace(/^["']|["']$/g, '') as TLSFingerprintProfileTransport
        break
      case 'os':
        form.os = value.replace(/^["']|["']$/g, '')
        break
      case 'client_type':
        form.client_type = value.replace(/^["']|["']$/g, '')
        break
      case 'user_agent':
        form.user_agent = value.replace(/^["']|["']$/g, '')
        break
      case 'originator':
        form.originator = value.replace(/^["']|["']$/g, '')
        break
      case 'http2_fingerprint':
        form.http2_fingerprint = value.replace(/^["']|["']$/g, '')
        break
      case 'name': {
        const unquoted = value.replace(/^["']|["']$/g, '')
        if (unquoted) {
          form.name = unquoted
          foundName = true
        }
        break
      }
      case 'description': {
        const unquoted = value.replace(/^["']|["']$/g, '')
        form.description = unquoted || null
        break
      }
      case 'enable_grease':
        form.enable_grease = value === 'true'
        break
      case 'cipher_suites':
      case 'curves':
      case 'point_formats':
      case 'signature_algorithms':
      case 'signature_algorithms_cert':
      case 'supported_versions':
      case 'key_share_groups':
      case 'psk_modes':
      case 'compress_cert_algos':
      case 'delegated_credentials_algorithms':
      case 'extensions': {
        const arrMatch = value.match(/^\[(.*)?\]$/)
        if (arrMatch) {
          const inner = arrMatch[1] || ''
          fieldInputs[key as keyof typeof fieldInputs] = inner
            .split(',')
            .map(s => s.trim())
            .filter(s => s.length > 0)
            .join(', ')
        }
        break
      }
      case 'extension_payloads':
        fieldInputs.extension_payloads = value
        break
      case 'alpn_protocols':
      case 'application_settings_protocols': {
        const arrMatch = value.match(/^\[(.*)?\]$/)
        if (arrMatch) {
          const inner = arrMatch[1] || ''
          fieldInputs[key as keyof typeof fieldInputs] = inner
            .split(',')
            .map(s => s.trim().replace(/^["']|["']$/g, ''))
            .filter(s => s.length > 0)
            .join(', ')
        }
        break
      }
    }
  }

  if (foundName) {
    appStore.showSuccess(t('admin.tlsFingerprintProfiles.form.yamlParsed'))
  } else {
    appStore.showError(t('admin.tlsFingerprintProfiles.form.yamlParseFailed'))
  }
}

const handleYamlPaste = () => {
  setTimeout(() => parseYamlInput(), 50)
}

const closeFormModal = () => {
  showCreateModal.value = false
  showEditModal.value = false
  editingProfile.value = null
  resetForm()
}

const parseNumericArray = (input: string): number[] => {
  if (!input.trim()) return []
  return input
    .split(',')
    .map(s => s.trim())
    .filter(s => s.length > 0)
    .map(s => s.startsWith('0x') || s.startsWith('0X') ? parseInt(s, 16) : parseInt(s, 10))
    .filter(n => !isNaN(n))
}

const parseStringArray = (input: string): string[] => {
  if (!input.trim()) return []
  return input
    .split(',')
    .map(s => s.trim())
    .filter(s => s.length > 0)
}

const parseExtensionPayloads = (input: string): Record<string, string> => {
  const trimmed = input.trim()
  if (!trimmed) return {}
  const parsed = JSON.parse(trimmed) as unknown
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('extension_payloads must be a JSON object')
  }
  return Object.fromEntries(
    Object.entries(parsed as Record<string, unknown>)
      .map(([key, value]) => [key.trim(), typeof value === 'string' ? value.trim() : String(value)])
      .filter(([key, value]) => key.length > 0 && value.length > 0)
  )
}

const formatExtensionPayloads = (payloads: Record<string, string> | null | undefined): string => {
  if (!payloads || Object.keys(payloads).length === 0) return ''
  return JSON.stringify(payloads, null, 2)
}

const formatHex = (n: number): string => '0x' + n.toString(16).padStart(4, '0')
const formatNumericArray = (arr: number[] | null | undefined): string => (arr ?? []).map(formatHex).join(', ')
const formatPlainNumericArray = (arr: number[] | null | undefined): string => (arr ?? []).join(', ')

const handleEdit = (profile: TLSFingerprintProfile) => {
  editingProfile.value = profile
  form.platform = profile.platform || ''
  form.transport = (profile.transport || '') as TLSFingerprintProfileTransport
  form.os = profile.os || ''
  form.client_type = profile.client_type || ''
  form.name = profile.name
  form.user_agent = profile.user_agent || ''
  form.originator = profile.originator || ''
  form.http2_fingerprint = profile.http2_fingerprint || ''
  form.description = profile.description
  form.enable_grease = profile.enable_grease
  fieldInputs.cipher_suites = formatNumericArray(profile.cipher_suites)
  fieldInputs.curves = formatPlainNumericArray(profile.curves)
  fieldInputs.point_formats = formatPlainNumericArray(profile.point_formats)
  fieldInputs.signature_algorithms = formatNumericArray(profile.signature_algorithms)
  fieldInputs.signature_algorithms_cert = formatNumericArray(profile.signature_algorithms_cert)
  fieldInputs.alpn_protocols = (profile.alpn_protocols ?? []).join(', ')
  fieldInputs.supported_versions = formatNumericArray(profile.supported_versions)
  fieldInputs.key_share_groups = formatPlainNumericArray(profile.key_share_groups)
  fieldInputs.psk_modes = formatPlainNumericArray(profile.psk_modes)
  fieldInputs.extensions = formatNumericArray(profile.extensions)
  fieldInputs.extension_payloads = formatExtensionPayloads(profile.extension_payloads)
  fieldInputs.compress_cert_algos = formatPlainNumericArray(profile.compress_cert_algos)
  fieldInputs.delegated_credentials_algorithms = formatNumericArray(profile.delegated_credentials_algorithms)
  fieldInputs.application_settings_protocols = (profile.application_settings_protocols ?? []).join(', ')
  showEditModal.value = true
}

const handleDelete = (profile: TLSFingerprintProfile) => {
  deletingProfile.value = profile
  showDeleteDialog.value = true
}

const handleSubmit = async () => {
  if (!form.name.trim()) {
    appStore.showError(t('admin.tlsFingerprintProfiles.form.name') + ' ' + t('common.required'))
    return
  }

  submitting.value = true
  try {
    const data = {
      platform: form.platform.trim(),
      transport: form.transport,
      os: form.os,
      client_type: form.client_type.trim(),
      name: form.name.trim(),
      user_agent: form.user_agent.trim(),
      originator: form.originator.trim(),
      http2_fingerprint: form.transport === 'h2' || form.transport === 'websocket-h2'
        ? form.http2_fingerprint.trim()
        : '',
      description: form.description?.trim() || null,
      enable_grease: form.enable_grease,
      cipher_suites: parseNumericArray(fieldInputs.cipher_suites),
      curves: parseNumericArray(fieldInputs.curves),
      point_formats: parseNumericArray(fieldInputs.point_formats),
      signature_algorithms: parseNumericArray(fieldInputs.signature_algorithms),
      signature_algorithms_cert: parseNumericArray(fieldInputs.signature_algorithms_cert),
      alpn_protocols: parseStringArray(fieldInputs.alpn_protocols),
      supported_versions: parseNumericArray(fieldInputs.supported_versions),
      key_share_groups: parseNumericArray(fieldInputs.key_share_groups),
      psk_modes: parseNumericArray(fieldInputs.psk_modes),
      extensions: parseNumericArray(fieldInputs.extensions),
      extension_payloads: parseExtensionPayloads(fieldInputs.extension_payloads),
      compress_cert_algos: parseNumericArray(fieldInputs.compress_cert_algos),
      delegated_credentials_algorithms: parseNumericArray(fieldInputs.delegated_credentials_algorithms),
      application_settings_protocols: parseStringArray(fieldInputs.application_settings_protocols)
    }

    if (showEditModal.value && editingProfile.value) {
      await adminAPI.tlsFingerprintProfiles.update(editingProfile.value.id, data)
      appStore.showSuccess(t('admin.tlsFingerprintProfiles.updateSuccess'))
    } else {
      await adminAPI.tlsFingerprintProfiles.create(data)
      appStore.showSuccess(t('admin.tlsFingerprintProfiles.createSuccess'))
    }

    closeFormModal()
    await loadProfiles()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.tlsFingerprintProfiles.saveFailed'))
    console.error('Error saving TLS fingerprint profile:', error)
  } finally {
    submitting.value = false
  }
}

const confirmDelete = async () => {
  if (!deletingProfile.value) return

  try {
    await adminAPI.tlsFingerprintProfiles.delete(deletingProfile.value.id)
    appStore.showSuccess(t('admin.tlsFingerprintProfiles.deleteSuccess'))
    showDeleteDialog.value = false
    deletingProfile.value = null
    await loadProfiles()
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.tlsFingerprintProfiles.deleteFailed'))
    console.error('Error deleting TLS fingerprint profile:', error)
  }
}
</script>
