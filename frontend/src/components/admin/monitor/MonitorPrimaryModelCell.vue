<template>
  <div class="flex flex-col gap-0.5">
    <div class="flex items-center gap-2">
      <!-- 纯配额模式主模型是占位符 "quota"（数据源是账号不是模型），展示层替换为本地化标签 -->
      <span class="text-sm text-foreground">{{ formatMonitorModel(row.primary_model) }}</span>
      <HelpTooltip>
      <template #trigger>
        <StatusCell
          :status="row.primary_status"
          :tone="statusTone(row.primary_status)"
          :label="statusLabel(row.primary_status)"
          dot
          :pulse="row.primary_status === 'operational'"
        />
      </template>
      <div class="space-y-2">
        <div class="text-xs font-semibold text-white">
          {{ formatMonitorModel(row.primary_model) }}
          <StatusCell
            class="ml-1"
            :status="row.primary_status"
            :tone="statusTone(row.primary_status)"
            :label="statusLabel(row.primary_status)"
            dot
            :pulse="row.primary_status === 'operational'"
          />
        </div>
        <div v-if="(row.extra_models?.length ?? 0) === 0" class="text-[11px] text-muted">
          {{ t('monitorCommon.extraModelsEmpty') }}
        </div>
        <div v-else class="space-y-1">
          <div class="text-[11px] font-semibold uppercase tracking-wide text-muted">
            {{ t('monitorCommon.extraModelsHeader') }}
          </div>
          <table class="w-full text-left text-[11px]">
            <thead>
              <tr class="text-muted">
                <th class="py-0.5 pr-2 font-medium">{{ t('admin.channelMonitor.columns.primaryModel') }}</th>
                <th class="py-0.5 pr-2 font-medium">{{ t('admin.channelMonitor.columns.actions') }}</th>
                <th class="py-0.5 font-medium">{{ t('admin.channelMonitor.columns.latency') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in (row.extra_models_status || [])" :key="m.model">
                <td class="py-0.5 pr-2 text-white">{{ m.model }}</td>
                <td class="py-0.5 pr-2">
                  <StatusCell
                    :status="m.status"
                    :tone="statusTone(m.status)"
                    :label="statusLabel(m.status)"
                    dot
                    :pulse="m.status === 'operational'"
                  />
                </td>
                <td class="py-0.5 text-white">{{ formatLatency(m.latency_ms) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      </HelpTooltip>
    </div>
    <!-- 配额模式监控：主模型行内联展示最新用量/余额快照（管理端不受用户端开关限制） -->
    <MonitorQuotaView :snapshot="row.latest_quota" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import MonitorQuotaView from '@/components/common/MonitorQuotaView.vue'
import { StatusCell } from '@/components/common/cells'
import { monitorStatusTone as statusTone } from './monitorStatusTone'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'

defineProps<{
  row: ChannelMonitor
}>()

const { t } = useI18n()
const { statusLabel, formatLatency, formatMonitorModel } = useChannelMonitorFormat()
</script>
