<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Bar } from 'vue-chartjs'
import {
  Chart as ChartJS,
  Title,
  Tooltip,
  Legend,
  BarElement,
  CategoryScale,
  LinearScale
} from 'chart.js'
import { onKeyStroke } from '@vueuse/core'
import { opsAPI, type OpsErrorDetail } from '@/api/admin/ops'

const { t } = useI18n()

ChartJS.register(Title, Tooltip, Legend, BarElement, CategoryScale, LinearScale)

interface Props {
  errorId: number
  modelValue: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

const loading = ref(false)
const retrying = ref(false)
const errorDetail = ref<OpsErrorDetail | null>(null)

const close = () => {
  emit('update:modelValue', false)
}

// ESC键关闭
onKeyStroke('Escape', () => {
  if (props.modelValue) {
    close()
  }
})

// 遮罩层点击关闭
const handleOverlayClick = (e: MouseEvent) => {
  if (e.target === e.currentTarget) {
    close()
  }
}

// 监听errorId变化,获取详情
watch(() => props.errorId, async (newId) => {
  if (newId && props.modelValue) {
    loading.value = true
    try {
      errorDetail.value = await opsAPI.getErrorDetail(newId)
    } catch (err) {
      console.error(t('admin.ops.details.failedToLoad'), err)
    } finally {
      loading.value = false
    }
  }
}, { immediate: true })

// 监听modelValue变化,当打开时立即获取数据
watch(() => props.modelValue, async (isOpen) => {
  if (isOpen && props.errorId) {
    loading.value = true
    try {
      errorDetail.value = await opsAPI.getErrorDetail(props.errorId)
    } catch (err) {
      console.error(t('admin.ops.details.failedToLoad'), err)
    } finally {
      loading.value = false
    }
  }
})

// 严重级别样式
const severityClass = computed(() => {
  const severity = errorDetail.value?.severity
  if (!severity) return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'

  const classMap = {
    P0: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
    P1: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
    P2: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400',
    P3: 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300'
  }
  return classMap[severity as keyof typeof classMap] || classMap.P3
})

// 延迟瀑布图数据
const latencyChartData = computed(() => {
  if (!errorDetail.value) return null

  const authLatency = errorDetail.value.auth_latency_ms ?? 0
  const routingLatency = errorDetail.value.routing_latency_ms ?? 0
  const upstreamLatency = errorDetail.value.upstream_latency_ms ?? 0
  const responseLatency = errorDetail.value.response_latency_ms ?? 0

  // 如果所有延迟都为0,不显示图表
  if (authLatency === 0 && routingLatency === 0 && upstreamLatency === 0 && responseLatency === 0) {
    return null
  }

  return {
    labels: [t('admin.ops.details.latencyWaterfall')],
    datasets: [
      {
        label: 'Auth',
        data: [authLatency],
        backgroundColor: '#3b82f6'
      },
      {
        label: 'Routing',
        data: [routingLatency],
        backgroundColor: '#10b981'
      },
      {
        label: 'Upstream',
        data: [upstreamLatency],
        backgroundColor: '#f59e0b'
      },
      {
        label: 'Response',
        data: [responseLatency],
        backgroundColor: '#8b5cf6'
      }
    ]
  }
})

const latencyChartOptions = computed(() => ({
  indexAxis: 'y' as const,
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      display: true,
      position: 'top' as const
    },
    tooltip: {
      enabled: true,
      callbacks: {
        label: (context: any) => {
          const label = context.dataset.label || ''
          const value = context.parsed.x || 0
          return `${label}: ${value.toFixed(2)}ms`
        }
      }
    }
  },
  scales: {
    x: {
      stacked: true,
      title: {
        display: true,
        text: `${t('admin.ops.details.totalLatency')} (ms)`
      },
      grid: {
        display: true
      }
    },
    y: {
      stacked: true,
      grid: {
        display: false
      }
    }
  }
}))

// 格式化JSON
const formatJSON = (jsonStr?: string) => {
  if (!jsonStr) return 'N/A'
  try {
    return JSON.stringify(JSON.parse(jsonStr), null, 2)
  } catch {
    return jsonStr
  }
}

// 重试错误请求
const handleRetry = async () => {
  if (!props.errorId) return

  retrying.value = true
  try {
    const result = await opsAPI.retryErrorRequest(props.errorId)

    if (result.can_retry) {
      ElMessage.success({
        message: t('admin.ops.details.retryInfo'),
        duration: 5000
      })

      // 可以在这里显示重试信息的对话框
      console.log('Retry info:', result)
    } else {
      ElMessage.warning(t('admin.ops.details.cannotRetry'))
    }
  } catch (err: any) {
    console.error('Retry failed:', err)
    ElMessage.error(err?.message || t('admin.ops.details.retryFailed'))
  } finally {
    retrying.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="modelValue"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      @click="handleOverlayClick"
    >
      <div
        class="relative max-h-[90vh] w-full max-w-4xl overflow-y-auto rounded-2xl bg-white shadow-2xl dark:bg-dark-800"
        @click.stop
      >
        <!-- Header -->
        <div class="sticky top-0 z-10 flex items-center justify-between border-b border-gray-200 bg-white px-6 py-4 dark:border-dark-700 dark:bg-dark-800">
          <div class="flex items-center gap-3">
            <h2 class="text-lg font-black text-gray-900 dark:text-white">{{ t('admin.ops.details.title') }}</h2>
            <span
              v-if="errorDetail"
              class="rounded-full px-3 py-1 text-xs font-bold"
              :class="severityClass"
            >
              {{ errorDetail.severity }}
            </span>
          </div>
          <button
            @click="close"
            class="flex h-8 w-8 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300"
          >
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <!-- Loading State -->
        <div v-if="loading" class="flex items-center justify-center py-20">
          <div class="flex flex-col items-center gap-3">
            <svg class="h-8 w-8 animate-spin text-blue-500" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <span class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</span>
          </div>
        </div>

        <!-- Content -->
        <div v-else-if="errorDetail" class="space-y-6 p-6">
          <!-- Top Info -->
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
              <div class="text-xs font-bold text-gray-400 uppercase tracking-wider">{{ t('admin.ops.details.requestId') }}</div>
              <div class="mt-1 font-mono text-sm font-medium text-gray-900 dark:text-white break-all">
                {{ errorDetail.request_id }}
              </div>
            </div>
            <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
              <div class="text-xs font-bold text-gray-400 uppercase tracking-wider">{{ t('admin.ops.details.occurrenceTime') }}</div>
              <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
                {{ new Date(errorDetail.created_at).toLocaleString() }}
              </div>
            </div>
            <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
              <div class="text-xs font-bold text-gray-400 uppercase tracking-wider">{{ t('admin.ops.details.errorPhase') }}</div>
              <div class="mt-1 text-sm font-bold text-gray-900 dark:text-white uppercase">
                {{ errorDetail.phase }}
              </div>
            </div>
          </div>

          <!-- Latency Waterfall Chart -->
          <div v-if="latencyChartData" class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
            <h3 class="mb-4 text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">{{ t('admin.ops.details.latencyWaterfall') }}</h3>
            <div class="h-32">
              <Bar :data="latencyChartData" :options="latencyChartOptions" />
            </div>
          </div>

          <!-- Basic Info Card -->
          <div class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
            <h3 class="mb-4 text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">{{ t('admin.ops.details.basicInfo') }}</h3>
            <div class="grid grid-cols-2 gap-4">
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">{{ t('admin.accounts.platform') }}</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ errorDetail.platform }}</div>
              </div>
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">{{ t('admin.accounts.testModel') }}</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ errorDetail.model }}</div>
              </div>
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">{{ t('admin.ops.errors.table.statusCode') }}</div>
                <div class="mt-1 text-sm font-medium text-red-600 dark:text-red-400">{{ errorDetail.status_code }}</div>
              </div>
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">{{ t('admin.ops.details.totalLatency') }}</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
                  {{ errorDetail.latency_ms ? `${errorDetail.latency_ms}ms` : 'N/A' }}
                </div>
              </div>
              <div v-if="errorDetail.time_to_first_token_ms" class="col-span-2">
                <div class="text-xs font-bold text-gray-400 uppercase">{{ t('admin.ops.details.ttft') }}</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
                  {{ errorDetail.time_to_first_token_ms }}ms
                </div>
              </div>
              <div class="col-span-2">
                <div class="text-xs font-bold text-gray-400 uppercase">{{ t('admin.ops.details.errorMessage') }}</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white break-words">
                  {{ errorDetail.message }}
                </div>
              </div>
            </div>
          </div>

          <!-- Request Body Card -->
          <div v-if="errorDetail.request_body" class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
            <h3 class="mb-4 text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">{{ t('admin.ops.details.requestBody') }}</h3>
            <pre class="overflow-x-auto rounded-lg bg-white p-4 text-xs font-mono text-gray-800 dark:bg-dark-800 dark:text-gray-200">{{ formatJSON(errorDetail.request_body) }}</pre>
          </div>

          <!-- Upstream Info Card -->
          <div class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
            <h3 class="mb-4 text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">{{ t('admin.ops.details.upstreamInfo') }}</h3>
            <div class="space-y-4">
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">{{ t('admin.ops.details.accountId') }}</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
                  {{ errorDetail.account_id ?? 'N/A' }}
                </div>
              </div>
              <div v-if="errorDetail.error_body">
                <div class="text-xs font-bold text-gray-400 uppercase mb-2">{{ t('admin.ops.details.errorResponse') }}</div>
                <pre class="overflow-x-auto rounded-lg bg-white p-4 text-xs font-mono text-gray-800 dark:bg-dark-800 dark:text-gray-200">{{ formatJSON(errorDetail.error_body) }}</pre>
              </div>
            </div>
          </div>

          <!-- Client Info Card -->
          <div class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
            <h3 class="mb-4 text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">{{ t('admin.ops.details.clientInfo') }}</h3>
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">{{ t('admin.users.email') }}</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
                  {{ errorDetail.user_id ?? 'N/A' }}
                </div>
              </div>
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">{{ t('admin.ops.details.clientIp') }}</div>
                <div class="mt-1 font-mono text-sm font-medium text-gray-900 dark:text-white">
                  {{ errorDetail.client_ip ?? 'N/A' }}
                </div>
              </div>
              <div class="col-span-full">
                <div class="text-xs font-bold text-gray-400 uppercase">{{ t('admin.ops.details.userAgent') }}</div>
                <div class="mt-1 text-xs font-mono text-gray-700 dark:text-gray-300 break-all">
                  {{ errorDetail.user_agent ?? 'N/A' }}
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Footer -->
        <div class="sticky bottom-0 border-t border-gray-200 bg-white px-6 py-4 dark:border-dark-700 dark:bg-dark-800">
          <div class="flex items-center justify-between">
            <button
              @click="handleRetry"
              :disabled="retrying || !errorDetail?.request_body"
              class="flex items-center gap-2 rounded-lg bg-blue-500 px-4 py-2 text-sm font-bold text-white hover:bg-blue-600 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-blue-600 dark:hover:bg-blue-700"
            >
              <svg v-if="retrying" class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <svg v-else class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
              {{ retrying ? t('admin.ops.details.retrying') : t('admin.ops.details.retry') }}
            </button>
            <button
              @click="close"
              class="rounded-lg bg-gray-100 px-4 py-2 text-sm font-bold text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
            >
              {{ t('common.close') }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
/* Pre标签滚动条样式 */
pre {
  scrollbar-width: thin;
  scrollbar-color: #cbd5e1 #f1f5f9;
}

.dark pre {
  scrollbar-color: #4b5563 #1f2937;
}

pre::-webkit-scrollbar {
  height: 8px;
}

pre::-webkit-scrollbar-track {
  background: #f1f5f9;
  border-radius: 4px;
}

.dark pre::-webkit-scrollbar-track {
  background: #1f2937;
}

pre::-webkit-scrollbar-thumb {
  background: #cbd5e1;
  border-radius: 4px;
}

.dark pre::-webkit-scrollbar-thumb {
  background: #4b5563;
}

pre::-webkit-scrollbar-thumb:hover {
  background: #94a3b8;
}

.dark pre::-webkit-scrollbar-thumb:hover {
  background: #6b7280;
}
</style>
