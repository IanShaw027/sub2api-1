<script setup lang="ts">
import type {
  ProviderHealthData,
  LatencyHistogramResponse,
  ErrorDistributionResponse
} from '@/api/admin/ops'
import OpsLatencyChart from './OpsLatencyChart.vue'
import OpsErrorChart from './OpsErrorChart.vue'
import OpsProviderChart from './OpsProviderChart.vue'

interface Props {
  hasLoadedOnce: boolean
  loading: boolean
  providers: ProviderHealthData[]
  latencyData: LatencyHistogramResponse | null
  errorDistribution: ErrorDistributionResponse | null
}

defineProps<Props>()
</script>

<template>
  <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
    <OpsLatencyChart :latency-data="latencyData" :loading="loading || !hasLoadedOnce" />
    <OpsErrorChart :error-distribution="errorDistribution" :loading="loading || !hasLoadedOnce" />
    <OpsProviderChart :providers="providers" :loading="loading || !hasLoadedOnce" />
  </div>
</template>
