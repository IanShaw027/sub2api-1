<template>
  <BaseDialog :show="show" :title="dialogTitle" width="normal" @close="handleClose">
    <div class="space-y-4">
      <div class="rounded-2xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/60">
        <p class="text-xs font-medium uppercase tracking-[0.16em] text-gray-500 dark:text-gray-400">操作对象</p>
        <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ subject || '未选择技能' }}</p>
        <p class="mt-2 text-sm text-gray-600 dark:text-gray-400">{{ dialogDescription }}</p>
      </div>

      <TextArea
        v-model="note"
        :label="textareaLabel"
        :placeholder="textareaPlaceholder"
        :hint="textareaHint"
        :error="validationError"
        :rows="4"
      />
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="handleClose">取消</button>
        <button
          type="button"
          :class="confirmButtonClass"
          :disabled="loading"
          @click="handleSubmit"
        >
          {{ loading ? 'Submitting...' : confirmLabel }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TextArea from '@/components/common/TextArea.vue'
import type { SkillAdminAction } from '@/api/admin/skills'

const props = withDefaults(defineProps<{
  show: boolean
  action: SkillAdminAction | null
  subject?: string | null
  loading?: boolean
}>(), {
  action: null,
  subject: '',
  loading: false
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', payload: { note: string }): void
}>()

const note = ref('')
const validationError = ref('')

const isDestructive = computed(() => props.action === 'reject' || props.action === 'disable' || props.action === 'force-private')

const dialogTitle = computed(() => {
  return {
    approve: '通过技能版本审核',
    reject: '拒绝技能版本审核',
    disable: '下线技能',
    'force-private': '强制转私有'
  }[props.action || 'approve']
})

const dialogDescription = computed(() => {
  return {
    approve: '通过后该版本可进入后续发布或公开流程。',
    reject: '拒绝时请说明原因，便于提交方按意见重新提审。',
    disable: '下线后技能将不可继续对外运行，请保留治理依据。',
    'force-private': '强制转私有后技能将退出公开市场，仅保留私域可见。'
  }[props.action || 'approve']
})

const textareaLabel = computed(() => (isDestructive.value ? '处理说明' : '审核备注'))
const textareaPlaceholder = computed(() => {
  return {
    approve: '可选：记录审核结论、注意事项或上线要求',
    reject: '请输入拒绝原因、整改建议或风险说明',
    disable: '请输入下线原因、触发规则或处置依据',
    'force-private': '请输入强制转私有原因、风险点或投诉线索'
  }[props.action || 'approve']
})
const textareaHint = computed(() => (isDestructive.value ? '拒绝/下线/强制私有建议填写可追踪的治理依据。' : '备注会进入审核日志。'))

const confirmLabel = computed(() => {
  return {
    approve: '确认通过',
    reject: '确认拒绝',
    disable: '确认下线',
    'force-private': '确认转私有'
  }[props.action || 'approve']
})

const confirmButtonClass = computed(() => {
  if (props.action === 'approve') return 'btn btn-primary'
  return 'btn btn-danger'
})

watch(
  () => props.show,
  (visible) => {
    if (!visible) {
      note.value = ''
      validationError.value = ''
    }
  }
)

watch(
  () => props.action,
  () => {
    note.value = ''
    validationError.value = ''
  }
)

function handleClose() {
  emit('close')
}

function handleSubmit() {
  const trimmed = note.value.trim()
  if (isDestructive.value && !trimmed) {
    validationError.value = '请填写处理依据后再提交。'
    return
  }
  validationError.value = ''
  emit('submit', { note: trimmed })
}
</script>
