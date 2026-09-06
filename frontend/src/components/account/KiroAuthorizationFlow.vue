<template>
  <div class="rounded-lg border border-[color-mix(in_oklch,var(--accent)_35%,transparent)] bg-[color-mix(in_oklch,var(--accent)_8%,transparent)] p-4">
    <div class="mb-4 flex items-start gap-3">
      <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-accent text-white">
        <Icon name="link" size="md" />
      </div>
      <div>
        <h4 class="font-semibold text-foreground">
          {{ title }}
        </h4>
        <p class="mt-1 text-sm text-accent">
          {{ description }}
        </p>
      </div>
    </div>

    <div class="space-y-4">
      <p class="text-sm text-accent">
        {{ t('admin.accounts.kiro.followSteps') }}
      </p>

      <div class="rounded-lg border border-line bg-surface/80 p-4">
        <label class="mb-3 block text-sm font-medium text-foreground">
          {{ t('admin.accounts.inputMethod') }}
        </label>
        <div class="flex flex-wrap gap-4">
          <label class="flex cursor-pointer items-center gap-2">
            <input
              v-model="inputMode"
              type="radio"
              value="oauth"
              class="text-accent focus:ring-accent-500"
            />
            <span class="text-sm text-foreground">
              {{ t('admin.accounts.oauth.manualAuth') }}
            </span>
          </label>
          <label class="flex cursor-pointer items-center gap-2">
            <input
              v-model="inputMode"
              type="radio"
              value="refresh_token"
              class="text-accent focus:ring-accent-500"
            />
            <span class="text-sm text-foreground">
              {{ t('admin.accounts.kiro.manualRefreshTokenAuth') }}
            </span>
          </label>
        </div>
      </div>

      <div
        v-if="inputMode === 'oauth'"
        class="rounded-lg border border-line bg-surface/80 p-4"
      >
        <div class="flex items-start gap-3">
          <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-accent text-xs font-bold text-white">
            1
          </div>
          <div class="flex-1">
            <p class="mb-2 font-medium text-foreground">
              {{ t('admin.accounts.kiro.step1GenerateUrl') }}
            </p>
            <button
              v-if="!authUrl"
              type="button"
              :disabled="loading"
              class="btn btn-primary text-sm"
              @click="emit('generate-url')"
            >
              <svg
                v-if="loading"
                class="-ml-1 mr-2 h-4 w-4 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <Icon v-else name="link" size="sm" class="mr-2" />
              {{ loading ? t('admin.accounts.oauth.generating') : t('admin.accounts.oauth.generateAuthUrl') }}
            </button>
            <div v-else class="space-y-3">
              <div class="flex items-center gap-2">
                <input
                  :value="authUrl"
                  readonly
                  type="text"
                  class="input flex-1 bg-surface-2 font-mono text-xs"
                />
                <button
                  type="button"
                  class="btn btn-secondary p-2"
                  :title="t('common.copy')"
                  @click="copyToClipboard(authUrl, t('common.copiedToClipboard'))"
                >
                  <svg
                    v-if="!copied"
                    class="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    stroke-width="1.5"
                  >
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      d="M15.666 3.888A2.25 2.25 0 0013.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 01-.75.75H9a.75.75 0 01-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 01-2.25 2.25H6.75A2.25 2.25 0 014.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 011.927-.184"
                    />
                  </svg>
                  <Icon
                    v-else
                    name="check"
                    size="sm"
                    class="text-success-text"
                    :stroke-width="2"
                  />
                </button>
              </div>
              <p class="text-xs text-accent">
                {{ t('admin.accounts.kiro.callbackBaseUrlHint', { value: callbackBaseUrl || t('admin.accounts.kiro.optionalPlaceholder') }) }}
              </p>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="inputMode === 'oauth'"
        class="rounded-lg border border-line bg-surface/80 p-4"
      >
        <div class="flex items-start gap-3">
          <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-accent text-xs font-bold text-white">
            2
          </div>
          <div class="flex-1">
            <p class="mb-2 font-medium text-foreground">
              {{ t('admin.accounts.kiro.step2Authorize') }}
            </p>
            <p class="text-sm text-accent">
              {{ t('admin.accounts.kiro.step2AuthorizeHint') }}
            </p>
          </div>
        </div>
      </div>

      <div
        v-if="inputMode === 'oauth'"
        class="rounded-lg border border-line bg-surface/80 p-4"
      >
        <div class="flex items-start gap-3">
          <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-accent text-xs font-bold text-white">
            3
          </div>
          <div class="flex-1 space-y-3">
            <div>
              <p class="mb-2 font-medium text-foreground">
                {{ t('admin.accounts.kiro.step3PasteCallback') }}
              </p>
              <p class="mb-3 text-sm text-accent">
                {{ t('admin.accounts.kiro.callbackUrlHint') }}
              </p>
              <textarea
                v-model="callbackUrl"
                rows="4"
                class="input w-full resize-y font-mono text-sm"
                :placeholder="t('admin.accounts.kiro.callbackUrlPlaceholder')"
              />
            </div>

            <details class="rounded-lg border border-[color-mix(in_oklch,var(--accent)_35%,transparent)] bg-surface/70 p-3">
              <summary class="cursor-pointer text-sm font-medium text-foreground">
                {{ t('admin.accounts.kiro.advancedFieldsTitle') }}
              </summary>
              <div class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2">
                <p class="md:col-span-2 text-sm text-accent">
                  {{ t('admin.accounts.kiro.runtimeManagedHint') }}
                </p>
                <div>
                  <label class="input-label">{{ t('admin.accounts.kiro.regionLabel') }}</label>
                  <input
                    v-model="region"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.accounts.kiro.regionPlaceholder')"
                  />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.accounts.kiro.authRegionLabel') }}</label>
                  <input
                    v-model="authRegion"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
                  />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.accounts.kiro.apiRegionLabel') }}</label>
                  <input
                    v-model="apiRegion"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
                  />
                </div>
                <div>
                  <label class="input-label">{{ t('admin.accounts.kiro.profileArnLabel') }}</label>
                  <div v-if="canDiscoverProfiles" class="mb-2 flex items-center gap-2">
                    <button
                      type="button"
                      class="btn btn-secondary text-xs"
                      :disabled="discoveringProfiles"
                      @click="discoverProfiles"
                    >
                      {{ discoveringProfiles ? t('admin.accounts.oauth.generating') : t('admin.accounts.kiro.discoverProfiles') }}
                    </button>
                    <select
                      v-if="discoveredProfiles.length"
                      v-model="selectedDiscoveredProfile"
                      class="input text-sm"
                    >
                      <option
                        v-for="profile in discoveredProfiles"
                        :key="profile.arn || profile.profileArn"
                        :value="(profile.arn || profile.profileArn || '').trim()"
                      >
                        {{ profile.profileName || profile.arn || profile.profileArn }}
                      </option>
                    </select>
                  </div>
                  <input
                    v-model="profileARN"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
                  />
                  <div
                    v-if="mode === 'reauth'"
                    class="mt-2 flex flex-wrap items-center gap-2 text-xs"
                  >
                    <span
                      class="inline-flex rounded px-1.5 py-0.5 font-medium"
                      :class="kiroProfileStatusBadgeClass"
                    >
                      {{ kiroProfileStatusLabel }}
                    </span>
                    <span
                      v-if="kiroProfilePendingHint"
                      class="text-accent"
                    >
                      {{ kiroProfilePendingHint }}
                    </span>
                  </div>
                </div>
                <div>
                  <label class="input-label">{{ t('admin.accounts.kiro.machineIdLabel') }}</label>
                  <input
                    v-model="machineID"
                    type="text"
                    class="input font-mono text-sm"
                    :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
                  />
                </div>
              </div>
            </details>

            <div
              v-if="localError || error"
              class="notice notice-danger"
            >
              {{ localError || error }}
            </div>

            <button
              type="button"
              class="btn btn-primary"
              :disabled="loading"
              @click="handleSubmit"
            >
              <svg
                v-if="loading"
                class="-ml-1 mr-2 h-4 w-4 animate-spin"
                fill="none"
                viewBox="0 0 24 24"
              >
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              {{ loading ? t('admin.accounts.oauth.verifying') : submitLabel }}
            </button>
          </div>
        </div>
      </div>

      <div
        v-if="inputMode === 'oauth' && continuation"
        class="rounded-lg border border-[color-mix(in_oklch,var(--warning)_45%,transparent)] bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] p-4"
      >
        <div class="flex items-start gap-3">
          <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-warning text-xs font-bold text-white">
            4
          </div>
          <div class="flex-1 space-y-3">
            <div>
              <p class="font-medium text-warning-text">
                {{ t('admin.accounts.kiro.idcContinuationTitle') }}
              </p>
              <p class="mt-1 text-sm text-warning-text">
                {{ t('admin.accounts.kiro.idcContinuationDesc') }}
              </p>
            </div>

            <div class="rounded-md border border-line bg-surface/80 p-3">
              <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-warning-text">
                {{ t('admin.accounts.kiro.idcUserCodeLabel') }}
              </p>
              <div class="flex flex-wrap items-center gap-3">
                <span class="select-all font-mono text-2xl font-bold tracking-[0.3em] text-warning-text">
                  {{ continuation.user_code || '—' }}
                </span>
                <button
                  v-if="continuation.user_code"
                  type="button"
                  class="btn btn-secondary text-xs"
                  @click="copyToClipboard(continuation.user_code, t('common.copiedToClipboard'))"
                >
                  <Icon v-if="!copied" name="copy" size="sm" class="mr-1" />
                  <Icon v-else name="check" size="sm" class="mr-1 text-success-text" :stroke-width="2" />
                  {{ t('common.copy') }}
                </button>
              </div>
              <p class="mt-2 text-xs text-warning-text">
                {{ t('admin.accounts.kiro.idcUserCodeHint') }}
              </p>
            </div>

            <div class="rounded-md border border-line bg-surface/80 p-3">
              <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-warning-text">
                {{ t('admin.accounts.kiro.idcVerificationUrlLabel') }}
              </p>
              <a
                v-if="verificationUrl"
                :href="verificationUrl"
                target="_blank"
                rel="noopener"
                class="btn btn-primary text-xs"
              >
                <Icon name="link" size="sm" class="mr-2" />
                {{ t('admin.accounts.kiro.idcVerificationUrlOpen') }}
              </a>
              <p v-else class="text-xs text-warning-text">
                {{ t('admin.accounts.kiro.idcVerificationUrlMissing') }}
              </p>
              <p v-if="verificationUrl" class="mt-2 break-all font-mono text-xs text-warning-text">
                {{ verificationUrl }}
              </p>
            </div>

            <div class="grid grid-cols-1 gap-2 text-xs text-warning-text md:grid-cols-2">
              <div>
                <span class="font-semibold">{{ t('admin.accounts.kiro.idcExpiresLabel') }}:</span>
                <span class="ml-1">
                  {{ remainingSeconds === null
                    ? '—'
                    : remainingSeconds <= 0
                      ? t('admin.accounts.kiro.idcExpiresExpired')
                      : t('admin.accounts.kiro.idcExpiresValue', { seconds: remainingSeconds })
                  }}
                </span>
              </div>
              <div v-if="continuation.idc_region">
                {{ t('admin.accounts.kiro.idcMetaRegion', { value: continuation.idc_region }) }}
              </div>
              <div v-if="continuation.start_url" class="break-all">
                {{ t('admin.accounts.kiro.idcMetaStartUrl', { value: continuation.start_url }) }}
              </div>
              <div v-if="continuation.issuer_url" class="break-all">
                {{ t('admin.accounts.kiro.idcMetaIssuerUrl', { value: continuation.issuer_url }) }}
              </div>
            </div>

            <div class="flex items-center gap-2 text-xs text-warning-text">
              <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <span>{{ t('admin.accounts.kiro.idcStatusPending') }}</span>
            </div>

            <div class="flex flex-col gap-2">
              <button
                type="button"
                class="btn btn-secondary self-start text-xs"
                @click="emit('cancel-continuation')"
              >
                {{ t('admin.accounts.kiro.idcCancelAction') }}
              </button>
              <p class="text-xs text-warning-text">
                {{ t('admin.accounts.kiro.idcCancelHint') }}
              </p>
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="inputMode === 'oauth' && externalIDPAuthorization"
        class="rounded-lg border border-[color-mix(in_oklch,var(--accent)_45%,transparent)] bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] p-4"
      >
        <div class="flex items-start gap-3">
          <div class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-accent text-xs font-bold text-white">
            4
          </div>
          <div class="flex-1 space-y-3">
            <div>
              <p class="font-medium text-foreground">
                {{ t('admin.accounts.kiro.externalIdpAuthorizationTitle') }}
              </p>
              <p class="mt-1 text-sm text-accent">
                {{ t('admin.accounts.kiro.externalIdpAuthorizationDesc') }}
              </p>
            </div>

            <div class="rounded-md border border-line bg-surface/80 p-3">
              <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-accent">
                {{ t('admin.accounts.kiro.externalIdpAuthUrlLabel') }}
              </p>
              <div class="flex flex-wrap items-center gap-2">
                <a
                  :href="externalIDPAuthorization.auth_url"
                  target="_blank"
                  rel="noopener"
                  class="btn btn-primary text-xs"
                >
                  <Icon name="link" size="sm" class="mr-2" />
                  {{ t('admin.accounts.kiro.externalIdpOpenMicrosoft') }}
                </a>
                <button
                  type="button"
                  class="btn btn-secondary text-xs"
                  @click="copyToClipboard(externalIDPAuthorization.auth_url, t('common.copiedToClipboard'))"
                >
                  <Icon v-if="!copied" name="copy" size="sm" class="mr-1" />
                  <Icon v-else name="check" size="sm" class="mr-1 text-success-text" :stroke-width="2" />
                  {{ t('common.copy') }}
                </button>
              </div>
              <p class="mt-2 break-all font-mono text-xs text-accent">
                {{ externalIDPAuthorization.auth_url }}
              </p>
            </div>

            <div class="grid grid-cols-1 gap-2 text-xs text-foreground md:grid-cols-2">
              <div v-if="externalIDPAuthorization.client_id" class="break-all">
                {{ t('admin.accounts.kiro.externalIdpClientId', { value: externalIDPAuthorization.client_id }) }}
              </div>
              <div v-if="externalIDPAuthorization.redirect_uri" class="break-all">
                {{ t('admin.accounts.kiro.externalIdpRedirectUri', { value: externalIDPAuthorization.redirect_uri }) }}
              </div>
              <div v-if="externalIDPAuthorization.issuer_url" class="break-all">
                {{ t('admin.accounts.kiro.externalIdpIssuerUrl', { value: externalIDPAuthorization.issuer_url }) }}
              </div>
              <div v-if="externalIDPAuthorization.login_hint" class="break-all">
                {{ t('admin.accounts.kiro.externalIdpLoginHint', { value: externalIDPAuthorization.login_hint }) }}
              </div>
              <div v-if="externalIDPAuthorization.scopes?.length" class="break-all md:col-span-2">
                {{ t('admin.accounts.kiro.externalIdpScopes', { value: externalIDPAuthorization.scopes.join(' ') }) }}
              </div>
            </div>

            <div class="rounded-md border border-[color-mix(in_oklch,var(--accent)_35%,transparent)] bg-surface/70 p-3 text-xs text-foreground">
              {{ t('admin.accounts.kiro.externalIdpFinalCallbackHint') }}
            </div>
          </div>
        </div>
      </div>

      <div
        v-if="inputMode === 'refresh_token'"
        class="rounded-lg border border-line bg-surface/80 p-4"
      >
        <p class="mb-3 text-sm text-accent">
          {{ t('admin.accounts.kiro.manualRefreshTokenDesc') }}
        </p>

        <div class="mb-4">
          <label class="input-label">{{ t('admin.accounts.kiro.authMethodLabel') }}</label>
          <select v-model="manualAuthMethod" class="input">
            <option value="social">{{ t('admin.accounts.kiro.authMethodSocial') }}</option>
            <option value="idc">{{ t('admin.accounts.kiro.authMethodIDC') }}</option>
            <option value="external_idp">{{ t('admin.accounts.kiro.authMethodExternalIdp') }}</option>
          </select>
          <p class="input-hint">{{ t('admin.accounts.kiro.authMethodHint') }}</p>
        </div>

        <div class="mb-4">
          <label class="mb-2 flex items-center gap-2 text-sm font-semibold text-foreground">
            <Icon name="key" size="sm" class="text-accent" />
            {{ t('admin.accounts.kiro.refreshTokenLabel') }}
            <span
              v-if="parsedRefreshTokenCount > 1"
              class="rounded-full bg-accent px-2 py-0.5 text-xs text-white"
            >
              {{ t('admin.accounts.oauth.keysCount', { count: parsedRefreshTokenCount }) }}
            </span>
          </label>
          <textarea
            v-model="manualRefreshToken"
            rows="4"
            class="input w-full resize-y font-mono text-sm"
            :placeholder="t('admin.accounts.kiro.refreshTokenPlaceholderBatch')"
          />
          <p v-if="parsedRefreshTokenCount > 1" class="mt-1 text-xs text-accent">
            {{ t('admin.accounts.oauth.batchCreateAccounts', { count: parsedRefreshTokenCount }) }}
          </p>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.accounts.kiro.accessTokenLabel') }}</label>
            <input
              v-model="manualAccessToken"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.accounts.kiro.accessTokenPlaceholder')"
            />
            <p class="input-hint">{{ t('admin.accounts.kiro.accessTokenHintCreate') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.kiro.expiresAtLabel') }}</label>
            <input v-model="manualExpiresAtInput" type="datetime-local" class="input" />
            <p class="input-hint">{{ t('admin.accounts.kiro.expiresAtHintCreate') }}</p>
          </div>
        </div>

        <div v-if="manualAuthMethod === 'idc'" class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.accounts.kiro.clientIdLabel') }}</label>
            <input
              v-model="manualClientID"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.accounts.kiro.clientIdPlaceholder')"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.kiro.clientSecretLabel') }}</label>
            <input
              v-model="manualClientSecret"
              type="password"
              class="input font-mono text-sm"
              :placeholder="t('admin.accounts.kiro.clientSecretPlaceholder')"
            />
          </div>
        </div>

        <div v-if="manualAuthMethod === 'external_idp'" class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.accounts.kiro.clientIdLabel') }}</label>
            <input
              v-model="manualClientID"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.accounts.kiro.clientIdPlaceholder')"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.kiro.tokenEndpointLabel') }}</label>
            <input
              v-model="manualTokenEndpoint"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.accounts.kiro.tokenEndpointPlaceholder')"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.kiro.issuerUrlLabel') }}</label>
            <input
              v-model="manualIssuerURL"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.accounts.kiro.issuerUrlPlaceholder')"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.kiro.scopesLabel') }}</label>
            <input
              v-model="manualScopes"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.accounts.kiro.scopesPlaceholder')"
            />
          </div>
          <div class="md:col-span-2">
            <label class="input-label">{{ t('admin.accounts.kiro.loginHintLabel') }}</label>
            <input
              v-model="manualLoginHint"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.accounts.kiro.loginHintPlaceholder')"
            />
          </div>
        </div>

        <details class="mt-4 rounded-lg border border-[color-mix(in_oklch,var(--accent)_35%,transparent)] bg-surface/70 p-3">
          <summary class="cursor-pointer text-sm font-medium text-foreground">
            {{ t('admin.accounts.kiro.advancedFieldsTitle') }}
          </summary>
          <div class="mt-4 grid grid-cols-1 gap-4 md:grid-cols-2">
            <p class="md:col-span-2 text-sm text-accent">
              {{ t('admin.accounts.kiro.runtimeManagedHint') }}
            </p>
            <div>
              <label class="input-label">{{ t('admin.accounts.kiro.regionLabel') }}</label>
              <input
                v-model="region"
                type="text"
                class="input font-mono text-sm"
                :placeholder="t('admin.accounts.kiro.regionPlaceholder')"
              />
            </div>
            <div>
              <label class="input-label">{{ t('admin.accounts.kiro.authRegionLabel') }}</label>
              <input
                v-model="authRegion"
                type="text"
                class="input font-mono text-sm"
                :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
              />
            </div>
            <div>
              <label class="input-label">{{ t('admin.accounts.kiro.apiRegionLabel') }}</label>
              <input
                v-model="apiRegion"
                type="text"
                class="input font-mono text-sm"
                :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
              />
            </div>
            <div>
              <label class="input-label">{{ t('admin.accounts.kiro.profileArnLabel') }}</label>
              <div v-if="canDiscoverProfiles" class="mb-2 flex items-center gap-2">
                <button
                  type="button"
                  class="btn btn-secondary text-xs"
                  :disabled="discoveringProfiles"
                  @click="discoverProfiles"
                >
                  {{ discoveringProfiles ? t('admin.accounts.oauth.generating') : t('admin.accounts.kiro.discoverProfiles') }}
                </button>
                <select
                  v-if="discoveredProfiles.length"
                  v-model="selectedDiscoveredProfile"
                  class="input text-sm"
                >
                  <option
                    v-for="profile in discoveredProfiles"
                    :key="profile.arn || profile.profileArn"
                    :value="(profile.arn || profile.profileArn || '').trim()"
                  >
                    {{ profile.profileName || profile.arn || profile.profileArn }}
                  </option>
                </select>
              </div>
              <input
                v-model="profileARN"
                type="text"
                class="input font-mono text-sm"
                :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
              />
              <div
                v-if="mode === 'reauth'"
                class="mt-2 flex flex-wrap items-center gap-2 text-xs"
              >
                <span
                  class="inline-flex rounded px-1.5 py-0.5 font-medium"
                  :class="kiroProfileStatusBadgeClass"
                >
                  {{ kiroProfileStatusLabel }}
                </span>
                <span
                  v-if="kiroProfilePendingHint"
                  class="text-accent"
                >
                  {{ kiroProfilePendingHint }}
                </span>
              </div>
            </div>
            <div>
              <label class="input-label">{{ t('admin.accounts.kiro.machineIdLabel') }}</label>
              <input
                v-model="machineID"
                type="text"
                class="input font-mono text-sm"
                :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
              />
            </div>
          </div>
        </details>

        <div
          v-if="localError || error"
          class="mt-4 notice notice-danger"
        >
          {{ localError || error }}
        </div>

        <button
          type="button"
          class="btn btn-primary mt-4 w-full"
          :disabled="loading"
          @click="handleSubmitRefreshToken"
        >
          <svg
            v-if="loading"
            class="-ml-1 mr-2 h-4 w-4 animate-spin"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          {{ loading ? t('admin.accounts.kiro.validating') : submitRefreshTokenLabel }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { getKiroProfiles } from '@/api/admin/accounts'
import type { KiroAccountExtra, KiroCredentials } from '@/types'
import { discoverProfiles as discoverKiroProfiles } from '@/api/admin/kiro'
import type { KiroDiscoveredProfile, KiroExternalIDPAuthorizationInfo, KiroIDCContinuationInfo } from '@/api/admin/kiro'

interface Props {
  mode?: 'create' | 'reauth'
  loading?: boolean
  error?: string
  authUrl?: string
  callbackBaseUrl?: string
  accountId?: number | null
  proxyId?: number | null
  initialCredentials?: KiroCredentials | null
  initialExtra?: KiroAccountExtra | null
  continuation?: KiroIDCContinuationInfo | null
  externalIDPAuthorization?: KiroExternalIDPAuthorizationInfo | null
}

const props = withDefaults(defineProps<Props>(), {
  mode: 'create',
  loading: false,
  error: '',
  authUrl: '',
  callbackBaseUrl: '',
  accountId: null,
  proxyId: null,
  initialCredentials: null,
  initialExtra: null,
  continuation: null,
  externalIDPAuthorization: null
})

const emit = defineEmits<{
  'generate-url': []
  'cancel-continuation': []
  submit: [payload: {
    callbackUrl: string
    credentials: KiroCredentials & Record<string, unknown>
    extra: KiroAccountExtra & Record<string, unknown>
  }]
  'submit-refresh-token': [payload: {
    credentials: KiroCredentials & Record<string, unknown>
    extra: KiroAccountExtra & Record<string, unknown>
  }]
}>()

const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()

const callbackUrl = ref('')
const profileARN = ref('')
const region = ref('us-east-1')
const authRegion = ref('')
const apiRegion = ref('')
const machineID = ref('')
const localError = ref('')
const inputMode = ref<'oauth' | 'refresh_token'>('oauth')
const manualAuthMethod = ref<'social' | 'idc' | 'external_idp'>('social')
const manualRefreshToken = ref('')
const manualAccessToken = ref('')
const manualExpiresAtInput = ref('')
const manualClientID = ref('')
const manualClientSecret = ref('')
const manualTokenEndpoint = ref('')
const manualIssuerURL = ref('')
const manualScopes = ref('')
const manualLoginHint = ref('')
const discoveringProfiles = ref(false)
const discoveredProfiles = ref<KiroDiscoveredProfile[]>([])

const title = computed(() => (
  props.mode === 'reauth'
    ? t('admin.accounts.kiro.reauthorizeTitle')
    : t('admin.accounts.kiro.authorizationTitle')
))

const description = computed(() => (
  props.mode === 'reauth'
    ? t('admin.accounts.kiro.reauthorizeDesc')
    : t('admin.accounts.kiro.authorizationDesc')
))

const submitLabel = computed(() => (
  props.mode === 'reauth'
    ? t('admin.accounts.reAuthorize')
    : t('admin.accounts.oauth.completeAuth')
))

const submitRefreshTokenLabel = computed(() => (
  props.mode === 'reauth'
    ? t('admin.accounts.reAuthorize')
    : t('admin.accounts.kiro.validateAndCreate')
))
const parsedRefreshTokenCount = computed(() => (
  manualRefreshToken.value
    .split('\n')
    .map((rt) => rt.trim())
    .filter((rt) => rt).length
))

const canDiscoverProfiles = computed(() => {
  if (props.accountId && props.mode === 'reauth') return true
  return inputMode.value === 'refresh_token' && manualRefreshToken.value.trim().length > 0
})
const currentProfileArn = computed(() => {
  const raw = props.initialCredentials?.profile_arn
  return typeof raw === 'string' ? raw.trim() : ''
})
const effectiveProfileArn = computed(() => profileARN.value.trim())
const kiroProfileStatusLabel = computed(() => {
  return effectiveProfileArn.value
    ? t('admin.accounts.kiro.profileStateManual')
    : t('admin.accounts.kiro.profileStateAuto')
})
const kiroProfileStatusBadgeClass = computed(() => {
  return effectiveProfileArn.value
    ? 'bg-[color-mix(in_oklch,var(--accent)_16%,transparent)] text-accent'
    : 'bg-surface-2 text-muted'
})
const kiroProfilePendingHint = computed(() => {
  if (props.mode !== 'reauth') return ''
  if (effectiveProfileArn.value === currentProfileArn.value) return ''
  return effectiveProfileArn.value
    ? t('admin.accounts.kiro.profileStatePendingManual')
    : t('admin.accounts.kiro.profileStatePendingAuto')
})
const selectedDiscoveredProfile = computed({
  get: () => profileARN.value,
  set: (value: string) => {
    profileARN.value = value
  }
})

const resetForm = () => {
  const credentials = props.initialCredentials || {}
  callbackUrl.value = ''
  profileARN.value = credentials.profile_arn || ''
  region.value = credentials.region || 'us-east-1'
  authRegion.value = credentials.auth_region || ''
  apiRegion.value = credentials.api_region || ''
  machineID.value = credentials.machine_id || ''
  inputMode.value = 'oauth'
  manualAuthMethod.value = (credentials.auth_method || 'social') as 'social' | 'idc' | 'external_idp'
  manualRefreshToken.value = ''
  manualAccessToken.value = ''
  manualExpiresAtInput.value = ''
  manualClientID.value = credentials.client_id || ''
  manualClientSecret.value = ''
  manualTokenEndpoint.value = credentials.token_endpoint || ''
  manualIssuerURL.value = credentials.issuer_url || ''
  manualScopes.value = credentials.scopes || ''
  manualLoginHint.value = credentials.login_hint || ''
  discoveredProfiles.value = []
  localError.value = ''
}

watch(
  () => [props.initialCredentials, props.mode],
  () => {
    resetForm()
  },
  { immediate: true, deep: true }
)

const handleSubmit = () => {
  localError.value = ''

  if (!props.authUrl.trim()) {
    localError.value = t('admin.accounts.kiro.generateUrlFirst')
    return
  }

  if (!callbackUrl.value.trim()) {
    localError.value = t('admin.accounts.kiro.callbackUrlRequired')
    return
  }

  const credentials: KiroCredentials & Record<string, unknown> = {}
  if (region.value.trim()) credentials.region = region.value.trim()
  if (authRegion.value.trim()) credentials.auth_region = authRegion.value.trim()
  if (apiRegion.value.trim()) credentials.api_region = apiRegion.value.trim()
  if (profileARN.value.trim()) credentials.profile_arn = profileARN.value.trim()
  if (machineID.value.trim()) credentials.machine_id = machineID.value.trim()

  emit('submit', {
    callbackUrl: callbackUrl.value.trim(),
    credentials,
    extra: {}
  })
}

const handleSubmitRefreshToken = () => {
  localError.value = ''

  if (!manualRefreshToken.value.trim()) {
    localError.value = t('admin.accounts.kiro.refreshTokenRequired')
    return
  }

  if (
    manualAuthMethod.value === 'idc' &&
    (!manualClientID.value.trim() || !manualClientSecret.value.trim())
  ) {
    localError.value = t('admin.accounts.kiro.idcClientRequired')
    return
  }

  if (
    manualAuthMethod.value === 'external_idp' &&
    (!manualClientID.value.trim() || (!manualTokenEndpoint.value.trim() && !manualIssuerURL.value.trim()))
  ) {
    localError.value = t('admin.accounts.kiro.externalIdpRequired')
    return
  }

  const expiresAt = manualExpiresAtInput.value.trim()
    ? new Date(manualExpiresAtInput.value)
    : null
  if (expiresAt && Number.isNaN(expiresAt.getTime())) {
    localError.value = t('admin.accounts.kiro.expiresAtInvalid')
    return
  }

  const credentials: KiroCredentials & Record<string, unknown> = {
    refresh_token: manualRefreshToken.value.trim(),
    auth_method: manualAuthMethod.value,
    region: region.value.trim() || 'us-east-1'
  }
  if (manualAccessToken.value.trim()) credentials.access_token = manualAccessToken.value.trim()
  if (expiresAt) credentials.expires_at = expiresAt.toISOString()
  if (manualAuthMethod.value === 'idc') {
    credentials.client_id = manualClientID.value.trim()
    credentials.client_secret = manualClientSecret.value.trim()
  }
  if (manualAuthMethod.value === 'external_idp') {
    credentials.client_id = manualClientID.value.trim()
    if (manualTokenEndpoint.value.trim()) credentials.token_endpoint = manualTokenEndpoint.value.trim()
    if (manualIssuerURL.value.trim()) credentials.issuer_url = manualIssuerURL.value.trim()
    if (manualScopes.value.trim()) credentials.scopes = manualScopes.value.trim()
    if (manualLoginHint.value.trim()) credentials.login_hint = manualLoginHint.value.trim()
  }
  if (authRegion.value.trim()) credentials.auth_region = authRegion.value.trim()
  if (apiRegion.value.trim()) credentials.api_region = apiRegion.value.trim()
  if (profileARN.value.trim()) credentials.profile_arn = profileARN.value.trim()
  if (machineID.value.trim()) credentials.machine_id = machineID.value.trim()

  emit('submit-refresh-token', {
    credentials,
    extra: {}
  })
}

const discoverProfiles = async () => {
  if (discoveringProfiles.value) return
  localError.value = ''
  discoveringProfiles.value = true
  try {
    let profiles: KiroDiscoveredProfile[] = []
    if (props.accountId && props.mode === 'reauth') {
      profiles = await getKiroProfiles(props.accountId)
    } else {
      const credentials: Record<string, unknown> = {
        refresh_token: manualRefreshToken.value.trim(),
        auth_method: manualAuthMethod.value,
        region: region.value.trim() || 'us-east-1'
      }
      if (manualAuthMethod.value === 'idc') {
        if (manualClientID.value.trim()) credentials.client_id = manualClientID.value.trim()
        if (manualClientSecret.value.trim()) credentials.client_secret = manualClientSecret.value.trim()
      }
      if (manualAuthMethod.value === 'external_idp') {
        if (manualClientID.value.trim()) credentials.client_id = manualClientID.value.trim()
        if (manualTokenEndpoint.value.trim()) credentials.token_endpoint = manualTokenEndpoint.value.trim()
        if (manualIssuerURL.value.trim()) credentials.issuer_url = manualIssuerURL.value.trim()
        if (manualScopes.value.trim()) credentials.scopes = manualScopes.value.trim()
        if (manualLoginHint.value.trim()) credentials.login_hint = manualLoginHint.value.trim()
      }
      if (authRegion.value.trim()) credentials.auth_region = authRegion.value.trim()
      if (apiRegion.value.trim()) credentials.api_region = apiRegion.value.trim()
      if (profileARN.value.trim()) credentials.profile_arn = profileARN.value.trim()
      if (machineID.value.trim()) credentials.machine_id = machineID.value.trim()
      profiles = await discoverKiroProfiles({
        credentials,
        extra: {},
        proxy_id: props.proxyId || undefined
      })
    }
    discoveredProfiles.value = profiles
    if (!profileARN.value.trim() && profiles.length > 0) {
      selectedDiscoveredProfile.value = String(profiles[0].arn || profiles[0].profileArn || '').trim()
    }
  } catch (err: any) {
    localError.value = err?.response?.data?.message || err?.message || t('admin.accounts.kiro.discoverProfilesFailed')
  } finally {
    discoveringProfiles.value = false
  }
}

const verificationUrl = computed(() => (
  props.continuation?.verification_uri_complete || props.continuation?.verification_uri || ''
))

const now = ref(Date.now())
let nowTimer: ReturnType<typeof setInterval> | null = null

watch(
  () => props.continuation?.expires_at,
  (expiresAt) => {
    if (nowTimer) {
      clearInterval(nowTimer)
      nowTimer = null
    }
    if (!expiresAt) return
    now.value = Date.now()
    nowTimer = setInterval(() => {
      now.value = Date.now()
    }, 1000)
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  if (nowTimer) {
    clearInterval(nowTimer)
    nowTimer = null
  }
})

const remainingSeconds = computed<number | null>(() => {
  const expiresAt = props.continuation?.expires_at
  if (!expiresAt) return null
  const ts = Date.parse(expiresAt)
  if (Number.isNaN(ts)) return null
  return Math.max(0, Math.floor((ts - now.value) / 1000))
})
</script>
