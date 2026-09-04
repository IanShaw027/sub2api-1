<template>
  <div v-show="activeTab === 'gateway'" class="settings-stack">
    <OverloadCooldownSection />
    <RateLimit429CooldownSection />
    <StreamTimeoutSection />
    <RequestRectifierSection />
    <BetaPolicySection />
    <OpenAIFastPolicySection />
    <ClaudeCodeSection />
    <KiroRuntimeDefaultsSection />
    <CodexSettingsSection />
    <UpstreamBillingProbeSection />
    <OllamaCloudUsageSection />
    <GatewaySchedulingSection />
    <GatewayForwardingBehaviorSection />
    <WebSearchEmulationSection />
    <UsageRecordsSection />
  </div>
</template>

<script setup lang="ts">
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import OverloadCooldownSection from "../gateway/OverloadCooldownSection.vue";
import RateLimit429CooldownSection from "../gateway/RateLimit429CooldownSection.vue";
import StreamTimeoutSection from "../gateway/StreamTimeoutSection.vue";
import RequestRectifierSection from "../gateway/RequestRectifierSection.vue";
import BetaPolicySection from "../gateway/BetaPolicySection.vue";
import OpenAIFastPolicySection from "../gateway/OpenAIFastPolicySection.vue";
import ClaudeCodeSection from "../gateway/ClaudeCodeSection.vue";
import KiroRuntimeDefaultsSection from "../gateway/KiroRuntimeDefaultsSection.vue";
import CodexSettingsSection from "../gateway/CodexSettingsSection.vue";
import UpstreamBillingProbeSection from "../gateway/UpstreamBillingProbeSection.vue";
import OllamaCloudUsageSection from "../gateway/OllamaCloudUsageSection.vue";
import GatewaySchedulingSection from "../gateway/GatewaySchedulingSection.vue";
import GatewayForwardingBehaviorSection from "../gateway/GatewayForwardingBehaviorSection.vue";
import WebSearchEmulationSection from "../gateway/WebSearchEmulationSection.vue";
import UsageRecordsSection from "../gateway/UsageRecordsSection.vue";

const settingsForm = inject(SettingsFormKey)!;
const { activeTab } = settingsForm;
</script>
