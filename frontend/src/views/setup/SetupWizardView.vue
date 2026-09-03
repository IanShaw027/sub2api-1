<template>
  <div class="setup-page">
    <div class="setup-shell">
      <div class="setup-header">
        <div
          class="flex h-11 w-11 items-center justify-center rounded-full mb-1 bg-[color-mix(in_oklch,var(--accent)_18%,transparent)] text-[var(--accent)]"
        >
          <Icon name="cog" size="lg" :stroke-width="2" />
        </div>
        <h1 class="text-xl font-extrabold text-foreground">{{ t('setup.title') }}</h1>
        <p class="text-[13.5px] text-muted">{{ t('setup.description') }}</p>
      </div>

      <div class="setup-stepper" role="list">
        <template v-for="(step, index) in steps" :key="step.id">
          <div class="setup-step" role="listitem">
            <div
              class="flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold flex-none"
              :class="
                currentStep >= index
                  ? 'bg-[var(--accent)] text-[var(--background)]'
                  : 'bg-surface-2 text-muted'
              "
            >
              <Icon v-if="currentStep > index" name="check" size="sm" :stroke-width="2" />
              <span v-else>{{ index + 1 }}</span>
            </div>
            <span
              class="text-xs font-semibold whitespace-nowrap"
              :class="currentStep >= index ? 'text-foreground' : 'text-muted'"
            >
              {{ step.title }}
            </span>
          </div>
          <div
            v-if="index < steps.length - 1"
            class="setup-step-line"
            :class="currentStep > index ? 'bg-[var(--accent)]' : 'bg-surface-2'"
          />
        </template>
      </div>

      <GlassCard padding="lg" class="setup-card">
        <!-- Step 1: Database -->
        <div v-if="currentStep === 0" class="setup-step-body">
          <div class="setup-step-heading">
            <h2 class="text-[17px] font-bold text-foreground">{{ t('setup.database.title') }}</h2>
            <p class="mt-1 text-sm text-muted">{{ t('setup.database.description') }}</p>
          </div>

          <SettingRow :label="t('setup.database.host')">
            <TextInput v-model="formData.database.host" type="text" placeholder="localhost" />
          </SettingRow>
          <SettingRow :label="t('setup.database.port')">
            <TextInput v-model.number="formData.database.port" type="number" placeholder="5432" />
          </SettingRow>
          <SettingRow :label="t('setup.database.username')">
            <TextInput v-model="formData.database.user" type="text" placeholder="postgres" />
          </SettingRow>
          <SettingRow :label="t('setup.database.password')">
            <TextInput
              v-model="formData.database.password"
              type="password"
              :placeholder="t('setup.database.passwordPlaceholder')"
            />
          </SettingRow>
          <SettingRow :label="t('setup.database.databaseName')">
            <TextInput v-model="formData.database.dbname" type="text" placeholder="sub2api" />
          </SettingRow>
          <SettingRow :label="t('setup.database.sslMode')">
            <UiSelect
              v-model="formData.database.sslmode"
              :options="[
                { value: 'disable', label: t('setup.database.ssl.disable') },
                { value: 'require', label: t('setup.database.ssl.require') },
                { value: 'verify-ca', label: t('setup.database.ssl.verifyCa') },
                { value: 'verify-full', label: t('setup.database.ssl.verifyFull') }
              ]"
            />
          </SettingRow>
          <SettingRow :label="t('setup.redis.enableTls')" :description="t('setup.redis.enableTlsHint')">
            <ToggleSwitch v-model="formData.redis.enable_tls" />
          </SettingRow>

          <div class="setup-test-row">
            <Button variant="secondary" nativeType="button" :loading="testingDb" @click="testDatabaseConnection">
              {{ t('setup.status.testConnection') }}
            </Button>
            <StatusBadge
              v-if="dbConnected"
              tone="success"
              dot
              :label="t('setup.status.success')"
            />
          </div>
        </div>

        <!-- Step 2: Redis -->
        <div v-if="currentStep === 1" class="setup-step-body">
          <div class="setup-step-heading">
            <h2 class="text-[17px] font-bold text-foreground">{{ t('setup.redis.title') }}</h2>
            <p class="mt-1 text-sm text-muted">{{ t('setup.redis.description') }}</p>
          </div>

          <SettingRow :label="t('setup.redis.host')">
            <TextInput v-model="formData.redis.host" type="text" placeholder="localhost" />
          </SettingRow>
          <SettingRow :label="t('setup.redis.port')">
            <TextInput v-model.number="formData.redis.port" type="number" placeholder="6379" />
          </SettingRow>
          <SettingRow :label="t('setup.redis.username')">
            <TextInput
              v-model="formData.redis.username"
              type="text"
              :placeholder="t('setup.redis.usernamePlaceholder')"
            />
          </SettingRow>
          <SettingRow :label="t('setup.redis.password')">
            <TextInput
              v-model="formData.redis.password"
              type="password"
              :placeholder="t('setup.redis.passwordPlaceholder')"
            />
          </SettingRow>
          <SettingRow :label="t('setup.redis.database')">
            <TextInput v-model.number="formData.redis.db" type="number" placeholder="0" />
          </SettingRow>
          <SettingRow :label="t('setup.redis.enableTls')" :description="t('setup.redis.enableTlsHint')">
            <ToggleSwitch v-model="formData.redis.enable_tls" />
          </SettingRow>

          <div class="setup-test-row">
            <Button variant="secondary" nativeType="button" :loading="testingRedis" @click="testRedisConnection">
              {{ t('setup.status.testConnection') }}
            </Button>
            <StatusBadge
              v-if="redisConnected"
              tone="success"
              dot
              :label="t('setup.status.success')"
            />
          </div>
        </div>

        <!-- Step 3: Admin -->
        <div v-if="currentStep === 2" class="setup-step-body">
          <div class="setup-step-heading">
            <h2 class="text-[17px] font-bold text-foreground">{{ t('setup.admin.title') }}</h2>
            <p class="mt-1 text-sm text-muted">{{ t('setup.admin.description') }}</p>
          </div>

          <SettingRow :label="t('setup.admin.email')">
            <TextInput v-model="formData.admin.email" type="email" placeholder="admin@example.com" />
          </SettingRow>
          <SettingRow :label="t('setup.admin.password')">
            <TextInput
              v-model="formData.admin.password"
              type="password"
              :placeholder="t('setup.admin.passwordPlaceholder')"
            />
          </SettingRow>
          <SettingRow :label="t('setup.admin.confirmPassword')">
            <TextInput
              v-model="confirmPassword"
              type="password"
              :placeholder="t('setup.admin.confirmPasswordPlaceholder')"
              :error="confirmPassword && formData.admin.password !== confirmPassword ? t('setup.admin.passwordMismatch') : ''"
            />
          </SettingRow>
        </div>

        <!-- Step 4: Complete -->
        <div v-if="currentStep === 3" class="setup-step-body">
          <div class="setup-step-heading">
            <h2 class="text-[17px] font-bold text-foreground">{{ t('setup.ready.title') }}</h2>
            <p class="mt-1 text-sm text-muted">{{ t('setup.ready.description') }}</p>
          </div>

          <SettingRow :label="t('setup.ready.database')">
            <p class="text-sm text-foreground">
              {{ formData.database.user }}@{{ formData.database.host }}:{{ formData.database.port }}/{{
                formData.database.dbname
              }}
            </p>
          </SettingRow>
          <SettingRow :label="t('setup.ready.redis')">
            <p class="text-sm text-foreground">{{ formData.redis.host }}:{{ formData.redis.port }}</p>
          </SettingRow>
          <SettingRow :label="t('setup.ready.adminEmail')">
            <p class="text-sm text-foreground">{{ formData.admin.email }}</p>
          </SettingRow>
        </div>

        <!-- Error Message -->
        <p v-if="errorMessage" class="notice notice-danger mx-5 mt-[18px]">
          <Icon name="exclamationCircle" size="sm" :stroke-width="2" />
          <span>{{ errorMessage }}</span>
        </p>

        <!-- Success Message -->
        <p v-if="installSuccess" class="notice notice-success mx-5 mt-[18px]">
          <span
            v-if="!serviceReady"
            class="h-4 w-4 flex-none rounded-full border-2 animate-spin border-[color-mix(in_oklch,var(--success)_35%,transparent)] border-t-[var(--success)]"
            aria-hidden="true"
          />
          <Icon v-else name="checkCircle" size="sm" :stroke-width="2" />
          <span>
            <strong>{{ t('setup.status.completed') }}</strong>
            <br />
            {{ serviceReady ? t('setup.status.redirecting') : t('setup.status.restarting') }}
          </span>
        </p>

        <!-- Navigation Buttons -->
        <div class="setup-nav">
          <Button
            v-if="currentStep > 0 && !installSuccess"
            variant="secondary"
            nativeType="button"
            @click="currentStep--"
          >
            <Icon name="chevronLeft" size="sm" :stroke-width="2" />
            {{ t('common.back') }}
          </Button>
          <div v-else />

          <Button v-if="currentStep < 3" variant="primary" nativeType="button" :disabled="!canProceed" @click="nextStep">
            {{ t('common.next') }}
            <Icon name="chevronRight" size="sm" :stroke-width="2" />
          </Button>

          <Button
            v-else-if="!installSuccess"
            variant="primary"
            size="lg"
            nativeType="button"
            :loading="installing"
            @click="performInstall"
          >
            {{ installing ? t('setup.status.installing') : t('setup.status.completeInstallation') }}
          </Button>
        </div>
      </GlassCard>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  configureSetupBootstrapSecretFromLocation,
  testDatabase,
  testRedis,
  install,
  type InstallRequest
} from '@/api/setup'
import { buildGatewayUrl } from '@/api/client'
import GlassCard from '@/components/ui/GlassCard.vue'
import SettingRow from '@/components/ui/SettingRow.vue'
import TextInput from '@/components/ui/TextInput.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

// Remote setup operators pass the bootstrap secret in the URL fragment. The
// fragment never reaches the server and is removed immediately after capture.
configureSetupBootstrapSecretFromLocation()

const steps = computed(() => [
  { id: 'database', title: t('setup.database.title') },
  { id: 'redis', title: t('setup.redis.title') },
  { id: 'admin', title: t('setup.admin.title') },
  { id: 'complete', title: t('setup.ready.title') }
])

const currentStep = ref(0)
const errorMessage = ref('')
const installSuccess = ref(false)

// Connection test states
const testingDb = ref(false)
const testingRedis = ref(false)
const dbConnected = ref(false)
const redisConnected = ref(false)
const installing = ref(false)
const confirmPassword = ref('')
const serviceReady = ref(false)

// Default server port
const getCurrentPort = (): number => {
  const port = window.location.port
  if (port) {
    return parseInt(port, 10)
  }

  return window.location.protocol === 'https:' ? 443 : 80
}

const formData = reactive<InstallRequest>({
  database: {
    host: 'localhost',
    port: 5432,
    user: 'postgres',
    password: '',
    dbname: 'sub2api',
    sslmode: 'disable'
  },
  redis: {
    host: 'localhost',
    port: 6379,
    username: '',
    password: '',
    db: 0,
    enable_tls: false
  },
  admin: {
    email: '',
    password: ''
  },
  server: {
    host: '0.0.0.0',
    port: getCurrentPort(), // Use current port from browser
    mode: 'release'
  }
})

const canProceed = computed(() => {
  switch (currentStep.value) {
    case 0:
      return dbConnected.value
    case 1:
      return redisConnected.value
    case 2:
      return (
        formData.admin.email &&
        formData.admin.password.length >= 8 &&
        formData.admin.password === confirmPassword.value
      )
    default:
      return true
  }
})

async function testDatabaseConnection() {
  testingDb.value = true
  errorMessage.value = ''
  dbConnected.value = false

  try {
    await testDatabase(formData.database)
    dbConnected.value = true
  } catch (error: unknown) {
    const err = error as { response?: { data?: { detail?: string; message?: string } }; message?: string }
    errorMessage.value =
      err.response?.data?.detail || err.response?.data?.message || err.message || 'Connection failed'
  } finally {
    testingDb.value = false
  }
}

async function testRedisConnection() {
  testingRedis.value = true
  errorMessage.value = ''
  redisConnected.value = false

  try {
    await testRedis(formData.redis)
    redisConnected.value = true
  } catch (error: unknown) {
    const err = error as { response?: { data?: { detail?: string; message?: string } }; message?: string }
    errorMessage.value =
      err.response?.data?.detail || err.response?.data?.message || err.message || 'Connection failed'
  } finally {
    testingRedis.value = false
  }
}

function nextStep() {
  if (canProceed.value) {
    errorMessage.value = ''
    currentStep.value++
  }
}

async function performInstall() {
  installing.value = true
  errorMessage.value = ''

  try {
    await install(formData)
    installSuccess.value = true
    // Start polling for service restart
    waitForServiceRestart()
  } catch (error: unknown) {
    const err = error as { response?: { data?: { detail?: string; message?: string } }; message?: string }
    errorMessage.value =
      err.response?.data?.detail || err.response?.data?.message || err.message || 'Installation failed'
  } finally {
    installing.value = false
  }
}

// Wait for service to restart and become available
async function waitForServiceRestart() {
  const maxAttempts = 60 // Increase to 60 attempts, ~60 seconds max
  const interval = 1000 // 1 second between attempts

  // Wait a moment for the service to start restarting
  await new Promise((resolve) => setTimeout(resolve, 3000))

  for (let attempt = 0; attempt < maxAttempts; attempt++) {
    try {
      // Use setup status endpoint as it tells us the real mode
      // Service might return 404 or connection refused while restarting
      const response = await fetch(buildGatewayUrl('/setup/status'), {
        method: 'GET',
        cache: 'no-store'
      })

      if (response.ok) {
        const data = await response.json()
        // If needs_setup is false, service has restarted in normal mode
        if (data.data && !data.data.needs_setup) {
          serviceReady.value = true
          // Redirect to login page after a short delay
          setTimeout(() => {
            window.location.href = '/login'
          }, 1500)
          return
        }
      }
    } catch {
      // Service not ready or network error during restart, continue polling
    }

    await new Promise((resolve) => setTimeout(resolve, interval))
  }

  // If we reach here, service didn't restart in time
  // Show a message to refresh manually
  errorMessage.value = t('setup.status.timeout')
}
</script>

<style scoped>
.setup-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
}

.setup-shell {
  width: 100%;
  max-width: 640px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.setup-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  text-align: center;
}

.setup-stepper {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0;
  flex-wrap: wrap;
}

.setup-step {
  display: flex;
  align-items: center;
  gap: 8px;
}

.setup-step-line {
  width: 24px;
  height: 2px;
  margin: 0 8px;
  flex: none;
}

.setup-step-body {
  display: flex;
  flex-direction: column;
}

.setup-step-heading {
  text-align: center;
  padding: 4px 0 18px;
}

.setup-test-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 20px 4px;
}

.setup-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 20px 0;
}

@media (max-width: 480px) {
  .setup-step span:last-child {
    display: none;
  }

  .setup-nav {
    flex-wrap: wrap;
    gap: 12px;
  }
}
</style>
