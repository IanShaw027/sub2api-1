<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { AudioLines, Upload, Download, Copy, Check, LoaderCircle, Square, RotateCcw, X, Phone, PhoneOff, Mic, MicOff } from '@lucide/vue'
import { userGroupsAPI } from '@/api/groups'
import { useAuthStore } from '@/stores/auth'
import type { Group } from '@/types'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import { generateSpeech, transcribeAudio, speechVoices, SPEECH_TEXT_LIMIT, VOICE_UPLOAD_LIMIT, type SpeechVoice, type SpeechLanguage } from '../voiceApi'
import { voiceMessages } from './voiceMessages'
import { VoiceCall, type VoiceCallStatus } from '../voiceCall'

const { locale } = useI18n()
const auth = useAuthStore()
const text = computed(() => voiceMessages[locale.value.startsWith('zh') ? 'zh' : 'en'])
const mode = ref<'call' | 'speech' | 'transcription'>('call')
const modes = computed(() => [{ value: 'call', label: text.value.call }, { value: 'speech', label: text.value.speech }, { value: 'transcription', label: text.value.transcription }])
const groups = ref<Group[]>([])
const groupOptions = computed(() => groups.value.map(group => ({ value: group.id, label: group.name })))
const voiceOptions = speechVoices.map(value => ({ value, label: value[0].toUpperCase() + value.slice(1) }))
const languageOptions = computed(() => [{ value: 'auto', label: text.value.auto }, { value: 'zh', label: text.value.chinese }, { value: 'en', label: text.value.english }])
const groupId = ref<number | null>(null)
const loading = ref(false)
const groupError = ref('')
const prompt = ref('')
const voice = ref<SpeechVoice>('eve')
const language = ref<SpeechLanguage>('auto')
const speed = ref(1)
const file = ref<File | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const audioURL = ref('')
const transcript = ref<string | null>(null)
const error = ref('')
const busy = ref(false)
const copied = ref(false)
let operation: AbortController | null = null
let generation = 0
let groupRequest = 0
let copyTimer: ReturnType<typeof setTimeout> | undefined
let call: VoiceCall | null = null
let callGeneration = 0
let callTimer: ReturnType<typeof setInterval> | undefined
const callStatus = ref<VoiceCallStatus | 'idle'>('idle')
const callMuted = ref(false)
const callLevel = ref(0)
const callSeconds = ref(0)
const callLines = ref<Array<{ role: 'user' | 'assistant'; text: string }>>([])
const callActive = computed(() => ['requesting', 'connecting', 'connected'].includes(callStatus.value))
const callDuration = computed(() => `${Math.floor(callSeconds.value / 60)}:${String(callSeconds.value % 60).padStart(2, '0')}`)
const characterCount = computed(() => Array.from(prompt.value.trim()).length)
const canSubmit = computed(() => mode.value !== 'call' && !busy.value && !loading.value && groupId.value !== null &&
  (mode.value === 'speech' ? characterCount.value > 0 && characterCount.value <= SPEECH_TEXT_LIMIT : file.value !== null))

function clearResult() {
  if (audioURL.value) URL.revokeObjectURL(audioURL.value)
  audioURL.value = ''
  transcript.value = null
  copied.value = false
  if (copyTimer) clearTimeout(copyTimer)
}

function cancel() {
  stopCall()
  generation++
  operation?.abort()
  operation = null
  busy.value = false
}

function stopCall() {
  callGeneration++
  call?.stop()
  call = null
  clearInterval(callTimer)
  callLevel.value = 0
  callMuted.value = false
  if (callStatus.value !== 'idle') callStatus.value = 'ended'
}

async function startCall() {
  if (groupId.value === null || callActive.value) return
  const current = ++callGeneration
  error.value = ''
  callLines.value = []
  callSeconds.value = 0
  callMuted.value = false
  call = new VoiceCall({
    groupId: groupId.value,
    voice: voice.value,
    onStatus(status) {
      if (current !== callGeneration) return
      callStatus.value = status
      if (status === 'connected') {
        const started = Date.now()
        callTimer = setInterval(() => { callSeconds.value = Math.floor((Date.now() - started) / 1000) }, 1000)
      } else if (status === 'ended') {
        clearInterval(callTimer)
        call = null
      }
    },
    onLevel(level) { if (current === callGeneration) callLevel.value = level },
    onTranscript(role, line) {
      if (current === callGeneration) callLines.value = [...callLines.value, { role, text: line }].slice(-20)
    },
    onError(cause) { if (current === callGeneration) error.value = cause.message },
  })
  await call.start().catch(() => undefined)
}

function toggleMute() {
  callMuted.value = !callMuted.value
  call?.setMuted(callMuted.value)
}

async function loadGroups() {
  const current = ++groupRequest
  loading.value = true
  groupError.value = ''
  try {
    const available = await userGroupsAPI.getAvailable()
    if (current !== groupRequest) return
    groups.value = available.filter((group) => group.platform === 'grok')
    groupId.value = groups.value.some((group) => group.id === groupId.value) ? groupId.value : groups.value[0]?.id ?? null
  } catch {
    if (current === groupRequest) groupError.value = text.value.groupError
  } finally {
    if (current === groupRequest) loading.value = false
  }
}

function chooseFile(event: Event) {
  const input = event.target as HTMLInputElement
  const selected = input.files?.[0]
  input.value = ''
  if (!selected) return
  error.value = ''
  if (!selected.size || selected.size > VOICE_UPLOAD_LIMIT ||
    (!selected.type.startsWith('audio/') && !/\.(mp3|wav|m4a|ogg|flac|webm|aac|opus|mp4)$/i.test(selected.name))) {
    error.value = text.value.invalidFile
    return
  }
  file.value = selected
  clearResult()
}

async function submit() {
  if (!canSubmit.value || groupId.value === null) return
  const current = ++generation
  const controller = new AbortController()
  operation = controller
  busy.value = true
  error.value = ''
  clearResult()
  try {
    if (mode.value === 'speech') {
      const audio = await generateSpeech(groupId.value, {
        text: prompt.value.trim(), voice_id: voice.value, language: language.value, speed: speed.value,
      }, controller.signal)
      if (current !== generation || controller.signal.aborted) return
      audioURL.value = URL.createObjectURL(audio)
    } else if (file.value) {
      const result = await transcribeAudio(groupId.value, file.value, controller.signal)
      if (current !== generation || controller.signal.aborted) return
      transcript.value = result
    }
  } catch (cause) {
    if (current === generation && !controller.signal.aborted) error.value = cause instanceof Error ? cause.message : text.value.failed
  } finally {
    if (current === generation) {
      busy.value = false
      operation = null
    }
  }
}

async function copyTranscript() {
  try {
    await navigator.clipboard.writeText(transcript.value ?? '')
    copied.value = true
    copyTimer = setTimeout(() => { copied.value = false }, 1500)
  } catch { error.value = text.value.copyFailed }
}

function downloadTranscript() {
  const url = URL.createObjectURL(new Blob([transcript.value ?? ''], { type: 'text/plain;charset=utf-8' }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = 'transcript.txt'
  anchor.click()
  setTimeout(() => URL.revokeObjectURL(url), 0)
}

watch([groupId, mode], () => { cancel(); clearResult(); error.value = '' }, { flush: 'sync' })
watch(() => auth.user?.id, () => {
  cancel()
  clearResult()
  groupRequest++
  groups.value = []
  groupId.value = null
  prompt.value = ''
  file.value = null
  callLines.value = []
  callStatus.value = 'idle'
  callSeconds.value = 0
  if (auth.user) void loadGroups()
}, { immediate: true, flush: 'sync' })
onBeforeUnmount(() => { groupRequest++; cancel(); clearResult() })
</script>

<template>
  <section class="voice-workspace" :aria-label="text.title">
    <header class="voice-heading">
      <AudioLines :size="20" :stroke-width="1.5" aria-hidden="true" />
      <h2>{{ text.title }}</h2>
      <SegmentedControl v-model="mode" :options="modes" class="voice-modes" :aria-label="text.title" />
    </header>
    <div class="voice-content">
      <div v-if="loading" class="voice-status" role="status"><LoaderCircle class="animate-spin" :size="18" />{{ text.loading }}</div>
      <div v-else-if="groupError" class="voice-status" role="alert">
        {{ groupError }}<button type="button" class="icon-btn" :title="text.retry" :aria-label="text.retry" @click="loadGroups"><RotateCcw :size="18" /></button>
      </div>
      <p v-else-if="!groups.length" class="voice-status">{{ text.noGroups }}</p>
      <form v-else class="voice-form" @submit.prevent="submit">
        <label class="voice-field voice-group">
          <span>{{ text.group }}</span>
          <UiSelect :model-value="groupId" :options="groupOptions" :aria-label="text.group" :disabled="busy || callActive" @update:model-value="groupId = $event === null ? null : Number($event)" />
        </label>
        <div v-if="mode === 'call'" class="voice-call">
          <div class="voice-wave" aria-hidden="true">
            <span v-for="bar in 13" :key="bar" :style="{ height: `${8 + callLevel * (20 + (bar % 4) * 12)}px` }" />
          </div>
          <div class="voice-call-state" role="status">{{ callMuted ? text.muted : text[callStatus] }}<span v-if="callStatus === 'connected'">{{ callDuration }}</span></div>
          <label class="voice-field voice-call-voice"><span>{{ text.voice }}</span><UiSelect :model-value="voice" :options="voiceOptions" :aria-label="text.voice" :disabled="callActive" @update:model-value="voice = $event as SpeechVoice" /></label>
          <div class="voice-call-actions">
            <button v-if="callStatus === 'connected'" type="button" class="voice-call-secondary" :title="callMuted ? text.unmute : text.mute" :aria-label="callMuted ? text.unmute : text.mute" :aria-pressed="callMuted" @click="toggleMute"><MicOff v-if="callMuted" :size="22" /><Mic v-else :size="22" /></button>
            <button v-if="callActive" type="button" class="voice-call-button is-active" :title="text.hangUp" :aria-label="text.hangUp" @click="stopCall"><PhoneOff :size="28" /></button>
            <button v-else type="button" class="voice-call-button" :title="text.startCall" :aria-label="text.startCall" @click="startCall"><Phone :size="28" /></button>
          </div>
          <p class="voice-call-action-label">{{ callActive ? text.hangUp : text.startCall }}</p>
          <div v-if="callLines.length" class="voice-call-transcript" aria-live="polite"><p v-for="(line, index) in callLines" :key="index"><strong>{{ line.role === 'user' ? text.you : text.assistant }}</strong>{{ line.text }}</p></div>
        </div>
        <template v-else-if="mode === 'speech'">
          <label class="voice-field">
            <span>{{ text.prompt }}</span>
            <textarea v-model="prompt" class="field voice-prompt" :aria-label="text.prompt" :disabled="busy" required />
            <span class="voice-count">{{ characterCount.toLocaleString() }} / {{ SPEECH_TEXT_LIMIT.toLocaleString() }}</span>
          </label>
          <div class="voice-settings">
            <label class="voice-field"><span>{{ text.voice }}</span><UiSelect :model-value="voice" :options="voiceOptions" :aria-label="text.voice" :disabled="busy" @update:model-value="voice = $event as SpeechVoice" /></label>
            <label class="voice-field"><span>{{ text.language }}</span><UiSelect :model-value="language" :options="languageOptions" :aria-label="text.language" :disabled="busy" @update:model-value="language = $event as SpeechLanguage" /></label>
            <label class="voice-field"><span>{{ text.speed }} <output>{{ speed.toFixed(1) }}x</output></span><input v-model.number="speed" type="range" min="0.7" max="1.5" step="0.1" :disabled="busy" :aria-label="text.speed"></label>
          </div>
        </template>
        <div v-else class="voice-upload">
          <input ref="fileInput" type="file" accept="audio/*,.mp4,.webm" hidden @change="chooseFile">
          <button type="button" class="btn btn-secondary" :disabled="busy" @click="fileInput?.click()"><Upload :size="18" />{{ text.upload }}</button>
          <span class="voice-file-name">{{ file?.name ?? text.fileLimit }}</span>
          <button v-if="file" type="button" class="icon-btn" :disabled="busy" :title="text.remove" :aria-label="text.remove" @click="file = null; clearResult()"><X :size="18" /></button>
        </div>
        <p v-if="error" class="voice-error" role="alert">{{ error }}</p>
        <div v-if="mode !== 'call'" class="voice-submit">
          <button v-if="busy" type="button" class="btn btn-secondary" @click="cancel"><Square :size="16" />{{ text.cancel }}</button>
          <button type="submit" class="btn btn-primary" :disabled="!canSubmit"><LoaderCircle v-if="busy" class="animate-spin" :size="18" /><AudioLines v-else :size="18" />{{ busy ? text.working : mode === 'speech' ? text.generate : text.transcribe }}</button>
        </div>
      </form>
      <div v-if="audioURL" class="voice-result">
        <audio :src="audioURL" controls :aria-label="text.audio" />
        <a :href="audioURL" download="speech.mp3" class="icon-btn" :title="text.download" :aria-label="text.download"><Download :size="18" /></a>
      </div>
      <div v-if="transcript !== null" class="voice-transcript">
        <div class="voice-result-heading"><h2>{{ text.result }}</h2><button type="button" class="icon-btn" :title="copied ? text.copied : text.copy" :aria-label="copied ? text.copied : text.copy" @click="copyTranscript"><Check v-if="copied" :size="18" /><Copy v-else :size="18" /></button><button type="button" class="icon-btn" :title="text.download" :aria-label="text.download" @click="downloadTranscript"><Download :size="18" /></button></div>
        <p>{{ transcript }}</p>
      </div>
    </div>

  </section>
</template>

<style scoped>
.voice-workspace { display: flex; flex: 1; flex-direction: column; align-items: stretch; gap: 20px; min-height: 0; overflow-y: auto; padding: 0 0 16px; }
.voice-heading { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; color: var(--foreground); padding-bottom: 16px; border-bottom: 1px solid var(--border); }
.voice-heading h2 { margin: 0 auto 0 0; font-size: var(--fs-15); font-weight: var(--fw-semibold); }
.voice-content { width: min(100%, 760px); margin: 0 auto; flex: 1; }
.voice-form { display: flex; flex-direction: column; gap: 18px; }
.voice-field { display: flex; flex-direction: column; gap: 7px; min-width: 0; font-size: 13px; color: var(--muted); }
.voice-field .field { width: 100%; min-width: 0; }
.voice-field output { float: right; }
.voice-group { max-width: 320px; }
.voice-prompt { height: 160px; min-height: 120px; padding: 14px; resize: vertical; line-height: 1.7; }
.voice-count { align-self: flex-end; font-size: 12px; font-variant-numeric: tabular-nums; }
.voice-settings { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 16px; align-items: center; }
.voice-field input[type=range] { accent-color: var(--accent); height: 36px; width: 100%; }
.voice-submit { display: flex; justify-content: flex-end; flex-wrap: wrap; gap: 8px; }
.voice-upload { display: flex; align-items: center; flex-wrap: wrap; gap: 12px; min-height: 160px; padding: 20px 0; border-block: 1px solid var(--border); }
.voice-file-name { min-width: 0; flex: 1; overflow-wrap: anywhere; font-size: 13px; color: var(--muted); }
.voice-error { margin: 0; color: var(--danger-text); font-size: 13px; overflow-wrap: anywhere; }
.voice-status { display: flex; gap: 12px; align-items: center; justify-content: center; color: var(--muted); min-height: 180px; }
.voice-result { display: flex; align-items: center; gap: 12px; padding-top: 24px; }
.voice-result audio { width: 100%; min-width: 0; }
.voice-transcript { margin-top: 24px; border-top: 1px solid var(--border); padding-top: 16px; }
.voice-result-heading { display: flex; gap: 8px; align-items: center; }
.voice-result-heading h2 { flex: 1; font-size: 14px; font-weight: 600; }
.voice-transcript p { white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.7; font-size: 14px; }
.voice-modes { flex-shrink: 0; }
.voice-call { display: flex; flex-direction: column; align-items: center; padding: 32px 0; }
.voice-wave { height: 84px; display: flex; align-items: center; justify-content: center; gap: 4px; }
.voice-wave span { width: 4px; min-height: 8px; border-radius: 2px; background: var(--foreground); transition: height 80ms linear; }
.voice-call-state { display: flex; align-items: center; gap: 12px; color: var(--muted); font-size: 14px; min-height: 24px; font-variant-numeric: tabular-nums; }
.voice-call-voice { width: 180px; margin: 24px 0 32px; }
.voice-call-actions { display: flex; align-items: center; justify-content: center; gap: 24px; min-height: 68px; }
.voice-call-button { display: inline-flex; align-items: center; justify-content: center; width: 68px; height: 68px; border-radius: var(--radius-circle); background: var(--accent); color: var(--on-tone); box-shadow: var(--shadow); }
.voice-call-button.is-active { background: var(--danger); color: var(--on-tone); }
.voice-call-secondary { display: inline-flex; align-items: center; justify-content: center; width: 44px; height: 44px; border-radius: 50%; background: var(--surface-secondary); color: var(--foreground); }
.voice-call-action-label { color: var(--muted); font-size: 13px; margin-top: 12px; }
.voice-call-transcript { width: 100%; max-height: 200px; overflow-y: auto; margin-top: 24px; font-size: 13px; line-height: 1.6; }
.voice-call-transcript p { display: flex; gap: 10px; overflow-wrap: anywhere; margin: 8px 0; }
.voice-call-transcript strong { flex-shrink: 0; min-width: 40px; color: var(--muted); }
@media (max-width: 540px) { .voice-workspace { padding: 0 0 16px; gap: 16px; } .voice-settings { grid-template-columns: 1fr 1fr; } .voice-settings > :last-child { grid-column: 1 / -1; } .voice-group { max-width: none; } .voice-prompt { height: 180px; } }
</style>
