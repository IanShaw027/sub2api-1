<script setup lang="ts">
import { ref, watch, computed } from 'vue'
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
      console.error('Failed to fetch error detail:', err)
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
      console.error('Failed to fetch error detail:', err)
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
  return classMap[severity] || classMap.P3
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
    labels: ['延迟分解'],
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

const latencyChartOptions = {
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
        text: '延迟 (ms)'
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
}

// 格式化JSON
const formatJSON = (jsonStr?: string) => {
  if (!jsonStr) return 'N/A'
  try {
    return JSON.stringify(JSON.parse(jsonStr), null, 2)
  } catch {
    return jsonStr
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
            <h2 class="text-lg font-black text-gray-900 dark:text-white">错误详情</h2>
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
            <span class="text-sm font-medium text-gray-500 dark:text-gray-400">加载中...</span>
          </div>
        </div>

        <!-- Content -->
        <div v-else-if="errorDetail" class="space-y-6 p-6">
          <!-- Top Info -->
          <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
              <div class="text-xs font-bold text-gray-400 uppercase tracking-wider">请求ID</div>
              <div class="mt-1 font-mono text-sm font-medium text-gray-900 dark:text-white break-all">
                {{ errorDetail.request_id }}
              </div>
            </div>
            <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
              <div class="text-xs font-bold text-gray-400 uppercase tracking-wider">发生时间</div>
              <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
                {{ new Date(errorDetail.created_at).toLocaleString() }}
              </div>
            </div>
            <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
              <div class="text-xs font-bold text-gray-400 uppercase tracking-wider">错误阶段</div>
              <div class="mt-1 text-sm font-bold text-gray-900 dark:text-white uppercase">
                {{ errorDetail.phase }}
              </div>
            </div>
          </div>

          <!-- Latency Waterfall Chart -->
          <div v-if="latencyChartData" class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
            <h3 class="mb-4 text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">延迟瀑布图</h3>
            <div class="h-32">
              <Bar :data="latencyChartData" :options="latencyChartOptions" />
            </div>
          </div>

          <!-- Basic Info Card -->
          <div class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
            <h3 class="mb-4 text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">基本信息</h3>
            <div class="grid grid-cols-2 gap-4">
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">平台</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ errorDetail.platform }}</div>
              </div>
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">模型</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ errorDetail.model }}</div>
              </div>
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">错误码</div>
                <div class="mt-1 text-sm font-medium text-red-600 dark:text-red-400">{{ errorDetail.status_code }}</div>
              </div>
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">总延迟</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
                  {{ errorDetail.latency_ms ? `${errorDetail.latency_ms}ms` : 'N/A' }}
                </div>
              </div>
              <div v-if="errorDetail.time_to_first_token_ms" class="col-span-2">
                <div class="text-xs font-bold text-gray-400 uppercase">首Token延迟 (TTFT)</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
                  {{ errorDetail.time_to_first_token_ms }}ms
                </div>
              </div>
              <div class="col-span-2">
                <div class="text-xs font-bold text-gray-400 uppercase">错误信息</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white break-words">
                  {{ errorDetail.message }}
                </div>
              </div>
            </div>
          </div>

          <!-- Request Body Card -->
          <div v-if="errorDetail.request_body" class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
            <h3 class="mb-4 text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">请求体</h3>
            <pre class="overflow-x-auto rounded-lg bg-white p-4 text-xs font-mono text-gray-800 dark:bg-dark-800 dark:text-gray-200">{{ formatJSON(errorDetail.request_body) }}</pre>
          </div>

          <!-- Upstream Info Card -->
          <div class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
            <h3 class="mb-4 text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">上游信息</h3>
            <div class="space-y-4">
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">账号ID</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
                  {{ errorDetail.account_id ?? 'N/A' }}
                </div>
              </div>
              <div v-if="errorDetail.response_body">
                <div class="text-xs font-bold text-gray-400 uppercase mb-2">错误响应</div>
                <pre class="overflow-x-auto rounded-lg bg-white p-4 text-xs font-mono text-gray-800 dark:bg-dark-800 dark:text-gray-200">{{ formatJSON(errorDetail.response_body) }}</pre>
              </div>
            </div>
          </div>

          <!-- Client Info Card -->
          <div class="rounded-xl bg-gray-50 p-6 dark:bg-dark-900">
            <h3 class="mb-4 text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">客户信息</h3>
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">用户ID</div>
                <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
                  {{ errorDetail.user_id ?? 'N/A' }}
                </div>
              </div>
              <div>
                <div class="text-xs font-bold text-gray-400 uppercase">IP地址</div>
                <div class="mt-1 font-mono text-sm font-medium text-gray-900 dark:text-white">
                  {{ errorDetail.client_ip ?? 'N/A' }}
                </div>
              </div>
              <div class="col-span-full">
                <div class="text-xs font-bold text-gray-400 uppercase">User-Agent</div>
                <div class="mt-1 text-xs font-mono text-gray-700 dark:text-gray-300 break-all">
                  {{ errorDetail.user_agent ?? 'N/A' }}
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Footer -->
        <div class="sticky bottom-0 border-t border-gray-200 bg-white px-6 py-4 dark:border-dark-700 dark:bg-dark-800">
          <div class="flex justify-end">
            <button
              @click="close"
              class="rounded-lg bg-gray-100 px-4 py-2 text-sm font-bold text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
            >
              关闭
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
