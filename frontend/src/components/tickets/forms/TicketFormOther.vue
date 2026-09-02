<template>
 <div class="space-y-4">
 <div>
 <label class="input-label">{{ t('tickets.fields.details') }}</label>
 <textarea
 :value="stringValue('details')"
 :readonly="readonly"
 class="input min-h-[140px]"
 @input="updateField('details', ($event.target as HTMLTextAreaElement).value)"
 />
 </div>
 </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const props = defineProps<{ modelValue: Record<string, unknown>; readonly?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: Record<string, unknown>] }>()
const { t } = useI18n()

function stringValue(key: string) {
 return String(props.modelValue?.[key] ?? '')
}

function updateField(key: string, value: string) {
 emit('update:modelValue', { ...props.modelValue, [key]: value })
}
</script>
