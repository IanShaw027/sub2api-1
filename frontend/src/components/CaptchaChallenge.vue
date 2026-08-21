<template>
  <TurnstileWidget
    v-if="turnstileEnabled && turnstileSiteKey"
    ref="turnstileRef"
    :site-key="turnstileSiteKey"
    @verify="(token) => emit('verify', token, '')"
    @expire="emit('expire')"
    @error="emit('error')"
  />
  <TencentCaptchaGate
    v-else-if="tencentEnabled && tencentAppId"
    ref="tencentRef"
    :app-id="tencentAppId"
    :region="tencentRegion"
  />
  <AliyunCaptchaWidget
    v-else-if="aliyunEnabled && aliyunSceneId && aliyunPrefix"
    ref="aliyunRef"
    :scene-id="aliyunSceneId"
    :prefix="aliyunPrefix"
    :region="aliyunRegion === 'sgp' ? 'sgp' : 'cn'"
    @verify="(param: string) => emit('verify', param, '')"
    @expire="emit('expire')"
    @error="emit('error')"
  />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import TurnstileWidget from '@/components/TurnstileWidget.vue'
import TencentCaptchaGate from '@/components/TencentCaptchaGate.vue'
import AliyunCaptchaWidget, {
  type AliyunCaptchaBizResult
} from '@/components/AliyunCaptchaWidget.vue'

// ActionCaptchaResult 动作触发式验证的结果：
// 腾讯 token=ticket、randstr 非空；阿里云 token=captchaVerifyParam、randstr 恒为空。
export interface ActionCaptchaResult {
  token: string
  randstr: string
}

const props = defineProps<{
  siteKey?: string
  turnstileEnabled: boolean
  turnstileSiteKey: string
  tencentEnabled: boolean
  tencentAppId: string
  tencentRegion?: string
  aliyunEnabled?: boolean
  aliyunSceneId?: string
  aliyunPrefix?: string
  aliyunRegion?: string
}>()

const emit = defineEmits<{
  verify: [tokenOrTicket: string, randstr: string]
  expire: []
  error: []
}>()

const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)
const tencentRef = ref<InstanceType<typeof TencentCaptchaGate> | null>(null)
const aliyunRef = ref<InstanceType<typeof AliyunCaptchaWidget> | null>(null)

function reset(): void {
  turnstileRef.value?.reset()
  tencentRef.value?.reset()
  aliyunRef.value?.reset()
}

/**
 * 阿里云官方 V2：把业务请求放进 captchaVerifyCallback。
 * fn 收到 captchaVerifyParam，返回给 SDK 的 captchaResult/bizResult。
 */
async function runAliyunVerify(
  fn: (captchaVerifyParam: string) => Promise<AliyunCaptchaBizResult>
): Promise<AliyunCaptchaBizResult> {
  if (!(props.aliyunEnabled && props.aliyunSceneId && props.aliyunPrefix)) {
    return { captchaResult: false, bizResult: false }
  }
  return (
    (await aliyunRef.value?.run(fn)) ?? {
      captchaResult: false,
      bizResult: false
    }
  )
}

// verifyAction 等待当前启用的动作触发式验证码结果。
// 腾讯仍走弹窗；阿里云走官方 button 触发回调并取出 captchaVerifyParam。
async function verifyAction(): Promise<ActionCaptchaResult | null> {
  if (props.tencentEnabled && props.tencentAppId) {
    try {
      const proof = (await tencentRef.value?.verify()) ?? null
      if (!proof) return null
      return { token: proof.ticket, randstr: proof.randstr }
    } catch {
      emit('error')
      return null
    }
  }
  if (props.aliyunEnabled && props.aliyunSceneId && props.aliyunPrefix) {
    try {
      let token: string | null = null
      const result = await runAliyunVerify(async (param) => {
        token = param
        // 仅取参；真正的 VerifyIntelligentCaptcha 仍由后续业务接口完成
        return { captchaResult: true, bizResult: true }
      })
      if (!result.captchaResult || !token) return null
      return { token, randstr: '' }
    } catch {
      emit('error')
      return null
    }
  }
  return null
}

defineExpose({ reset, verifyAction, runAliyunVerify })
</script>
