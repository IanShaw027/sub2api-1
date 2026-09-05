<template>
  <AppLayout>
    <PageHeader :title="t('nav.buySubscription')" :description="t('purchase.description')" />
    <div class="purchase-container mx-auto space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-20">
        <LoadingSpinner size="xl" />
      </div>
      <template v-else>
        <!-- Tab Switcher (hide during payment and subscription confirm) -->
        <PurchaseTabSwitcher
          v-if="tabs.length > 1 && paymentPhase === 'select' && !selectedPlan"
          :tabs="tabs"
          :model-value="activeTab"
          @update:model-value="activeTab = $event"
        />
        <!-- Payment in progress (shared by recharge and subscription) -->
        <template v-if="paymentPhase === 'paying'">
          <PaymentStatusPanel
            :order-id="paymentState.orderId"
            :amount="paymentState.amount"
            :pay-amount="paymentState.payAmount"
            :qr-code="paymentState.qrCode"
            :expires-at="paymentState.expiresAt"
            :payment-type="paymentState.paymentType"
            :pay-url="paymentState.payUrl"
            :order-type="paymentState.orderType"
            :currency="paymentState.currency || selectedCurrency"
            :out-trade-no="paymentState.outTradeNo"
            :mobile-alipay-deep-link="paymentState.alipayMobilePrecreateDeepLink"
            @done="onPaymentDone"
            @success="onPaymentSuccess"
            @settled="onPaymentSettled"
          />
        </template>
        <!-- Tab content (select phase) -->
        <template v-else>
          <template v-if="activeTab === 'recharge'">
            <RechargePanel
              :username="user?.username || ''"
              :balance="user?.balance?.toFixed(2) || '0.00'"
              :enabled-methods="enabledMethods"
              :amount="amount"
              :global-min-amount="globalMinAmount"
              :global-max-amount="globalMaxAmount"
              :amount-error="amountError"
              :method-options="methodOptions"
              :selected-method="selectedMethod"
              :valid-amount="validAmount"
              :fee-rate="feeRate"
              :fee-amount="feeAmount"
              :total-amount="totalAmount"
              :balance-recharge-multiplier="balanceRechargeMultiplier"
              :credited-amount="creditedAmount"
              :selected-currency="selectedCurrency"
              :format-amount="formatSelectedPaymentAmount"
              :payment-button-class="paymentButtonClass"
              :can-submit="canSubmit"
              :submitting="submitting"
              @update:amount="amount = $event"
              @update:selected-method="selectedMethod = $event"
              @submit="handleSubmitRecharge"
            />
          </template>
          <template v-else-if="activeTab === 'subscription'">
            <template v-if="selectedPlan">
              <SubscriptionConfirmCard
                :plan="selectedPlan"
                :plan-badge-class="planBadgeClass"
                :plan-text-class="planTextClass"
                :validity-suffix="planValiditySuffix"
                :has-peak-rate="planHasPeakRate(selectedPlan)"
                :peak-rate-label="planPeakRateLabel(selectedPlan)"
                :format-subscription-amount="formatSelectedSubscriptionPaymentAmount"
                :format-amount="formatSelectedPaymentAmount"
                :enabled-methods="enabledMethods"
                :method-options="subMethodOptions"
                :selected-method="selectedMethod"
                :fee-rate="feeRate"
                :sub-payment-amount="subPaymentAmount"
                :sub-fee-amount="subFeeAmount"
                :sub-total-amount="subTotalAmount"
                :payment-button-class="paymentButtonClass"
                :can-submit="canSubmitSubscription"
                :submitting="submitting"
                @update:selected-method="selectedMethod = $event"
                @submit="confirmSubscribe"
                @cancel="selectedPlan = null"
              />
            </template>
            <template v-else>
              <div v-if="checkout.plans.length === 0" class="glass-card py-16 text-center">
                <Icon name="gift" size="xl" class="mx-auto mb-3 text-muted" />
                <p class="text-muted">{{ t('payment.noPlans') }}</p>
              </div>
              <div v-else :class="planGridClass">
                <SubscriptionPlanCard v-for="plan in checkout.plans" :key="plan.id" :plan="plan" :active-subscriptions="activeSubscriptions" @select="selectPlan" />
              </div>
              <ActiveSubscriptionsList
                :subscriptions="activeSubscriptions"
                :subscription-has-peak-rate="subscriptionHasPeakRate"
                :subscription-peak-rate-label="subscriptionPeakRateLabel"
                :get-days-remaining="getDaysRemaining"
              />
            </template>
          </template>
        </template>
        <CheckoutHelpCard
          v-if="paymentPhase === 'select' && !selectedPlan"
          :help-text="checkout.help_text"
          :help-image-url="checkout.help_image_url"
          @preview="previewImage = $event"
        />
      </template>
    </div>
    <RenewalPlanModal
      :open="showRenewalModal"
      :plans="renewalPlans"
      :active-subscriptions="activeSubscriptions"
      @close="closeRenewalModal"
      @select="selectPlanFromModal"
    />
    <!-- Image Preview Overlay -->
    <Teleport to="body">
      <Transition name="modal">
        <div v-if="previewImage" class="purchase-image-scrim" @click="previewImage = ''">
          <img :src="previewImage" alt="" class="purchase-image-preview" />
        </div>
      </Transition>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import SubscriptionPlanCard from '@/components/payment/SubscriptionPlanCard.vue'
import PaymentStatusPanel from '@/components/payment/PaymentStatusPanel.vue'
import RechargePanel from '@/components/payment/RechargePanel.vue'
import SubscriptionConfirmCard from '@/components/payment/SubscriptionConfirmCard.vue'
import ActiveSubscriptionsList from '@/components/payment/ActiveSubscriptionsList.vue'
import CheckoutHelpCard from '@/components/payment/CheckoutHelpCard.vue'
import RenewalPlanModal from '@/components/payment/RenewalPlanModal.vue'
import PurchaseTabSwitcher from '@/components/payment/PurchaseTabSwitcher.vue'
import { usePurchaseFlow } from './payment/usePurchaseFlow'

const {
  t,
  user,
  activeSubscriptions,
  getDaysRemaining,
  subscriptionHasPeakRate,
  subscriptionPeakRateLabel,
  loading,
  submitting,
  activeTab,
  amount,
  selectedMethod,
  selectedPlan,
  previewImage,
  paymentPhase,
  paymentState,
  onPaymentDone,
  onPaymentSuccess,
  onPaymentSettled,
  checkout,
  tabs,
  enabledMethods,
  validAmount,
  balanceRechargeMultiplier,
  creditedAmount,
  planGridClass,
  globalMinAmount,
  globalMaxAmount,
  selectedCurrency,
  formatSelectedPaymentAmount,
  formatSelectedSubscriptionPaymentAmount,
  methodOptions,
  feeRate,
  feeAmount,
  totalAmount,
  amountError,
  canSubmit,
  subPaymentAmount,
  subFeeAmount,
  subTotalAmount,
  subMethodOptions,
  canSubmitSubscription,
  paymentButtonClass,
  planBadgeClass,
  planTextClass,
  showRenewalModal,
  renewalPlans,
  planValiditySuffix,
  planHasPeakRate,
  planPeakRateLabel,
  selectPlan,
  selectPlanFromModal,
  closeRenewalModal,
  handleSubmitRecharge,
  confirmSubscribe,
} = usePurchaseFlow()
</script>

<style scoped>
.purchase-container {
  max-width: 56rem;
}

.purchase-image-scrim {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  background: color-mix(in oklch, black 70%, transparent);
  backdrop-filter: blur(4px);
}

.purchase-image-preview {
  max-height: 85vh;
  max-width: 90vw;
  border-radius: var(--radius-hero);
  object-fit: contain;
  box-shadow: var(--shadow);
}
</style>
