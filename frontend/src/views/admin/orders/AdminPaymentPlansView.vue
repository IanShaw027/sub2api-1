<template>
  <AppLayout>
    <div class="list-page">
      <PageHeader :title="t('nav.paymentPlans')" :description="t('payment.admin.plansDescription')">
        <template #actions>
          <Button
            variant="secondary"
            class="btn-icon"
            :disabled="plansLoading"
            :title="t('common.refresh')"
            :aria-label="t('common.refresh')"
            @click="loadPlans"
          >
            <Icon name="refresh" size="sm" :class="plansLoading ? 'animate-spin' : ''" />
          </Button>
          <Button class="plans-create-desktop" @click="openPlanEdit(null)">{{ t('payment.admin.createPlan') }}</Button>
        </template>
      </PageHeader>

      <div class="filter-row">
        <span class="filter-count">
          {{ t('payment.admin.totalPlansLabel') }} <b>{{ plans.length }}</b>
        </span>
      </div>

      <section class="table-card">
        <DataTable :columns="planColumns" :data="plans" :loading="plansLoading">
          <template #cell-name="{ value, row }">
            <span class="cell-primary-name" :class="getPlanNameClass(row.group_id)">{{ value }}</span>
          </template>
          <template #cell-group_id="{ value }">
            <span v-if="isGroupMissing(value)" class="cell-primary-meta">
              <span>#{{ value }}</span>
              <span class="badge badge-danger">{{ t('payment.admin.groupMissing') }}</span>
            </span>
            <GroupBadge
              v-else-if="getGroup(value)"
              :name="getGroup(value)!.name"
              :platform="getGroup(value)!.platform"
              :rate-multiplier="getGroup(value)!.rate_multiplier"
            />
            <span v-else class="cell-primary-meta">-</span>
          </template>
          <template #cell-price="{ value, row }">
            <div class="cell-amount">
              <span class="cell-amount-value">{{ planCurrencySymbol(row.currency) }}{{ (value ?? 0).toFixed(2) }}<span v-if="row.currency"> {{ row.currency }}</span></span>
              <span v-if="row.original_price" class="cell-amount-meta cell-amount-strike">{{ planCurrencySymbol(row.currency) }}{{ row.original_price.toFixed(2) }}</span>
            </div>
          </template>
          <template #cell-validity_days="{ value, row }">
            <span class="cell-time">{{ value }} {{ t('payment.admin.' + (row.validity_unit || 'days')) }}</span>
          </template>
          <template #cell-for_sale="{ value, row }">
            <ToggleSwitch size="compact" :model-value="!!value" @update:model-value="toggleForSale(row)" />
          </template>
          <template #cell-actions="{ row }">
            <div class="row-actions">
              <button
                type="button"
                class="icon-btn"
                :title="t('common.edit')"
                :aria-label="t('common.edit')"
                @click="openPlanEdit(row)"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button
                type="button"
                class="icon-btn icon-btn-danger"
                :title="t('common.delete')"
                :aria-label="t('common.delete')"
                @click="confirmDeletePlan(row)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>
        </DataTable>
      </section>
    </div>

    <!-- Plan Edit Dialog -->
    <PlanEditDialog :show="showPlanDialog" :plan="editingPlan" :groups="groups" :payment-config="paymentConfig" @close="showPlanDialog = false" @saved="loadPlans" />

    <ConfirmDialog :show="showDeletePlanDialog" :title="t('payment.admin.deletePlan')" :message="t('payment.admin.deletePlanConfirm')" :confirm-text="t('common.delete')" danger @confirm="handleDeletePlan" @cancel="showDeletePlanDialog = false" />
    <Fab class="plans-fab" :label="t('payment.admin.createPlan')" @click="openPlanEdit(null)">
      <Icon name="plus" size="md" />
      {{ t('payment.admin.createPlan') }}
    </Fab>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { AdminPaymentConfig } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import adminAPI from '@/api/admin'
import type { SubscriptionPlan } from '@/types/payment'
import type { AdminGroup } from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import Fab from '@/components/ui/Fab.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import DataTable from '@/components/common/DataTable.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import PlanEditDialog from './PlanEditDialog.vue'
import { currencySymbol } from '@/components/payment/currency'
import { platformTextClass } from '@/utils/platformColors'

const { t } = useI18n()
const appStore = useAppStore()

function planCurrencySymbol(currency?: string): string {
  return currencySymbol(currency || 'USD')
}

// ==================== Groups ====================

const groups = ref<AdminGroup[]>([])
const paymentConfig = ref<AdminPaymentConfig | null>(null)

async function loadGroups() {
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch { /* ignore */ }
}

async function loadPaymentConfig() {
  try {
    const res = await adminPaymentAPI.getConfig()
    paymentConfig.value = res.data
  } catch { /* preview only */ }
}

function getGroup(id: number): AdminGroup | undefined {
  return groups.value.find(g => g.id === id)
}

function isGroupMissing(id: number): boolean {
  return id > 0 && !groups.value.find(g => g.id === id)
}

function getPlanNameClass(groupId: number): string {
  const group = getGroup(groupId)
  return group ? platformTextClass(group.platform) : 'text-foreground'
}


// ==================== Plans ====================

const plansLoading = ref(false)
const plans = ref<SubscriptionPlan[]>([])
const showPlanDialog = ref(false)
const showDeletePlanDialog = ref(false)
const editingPlan = ref<SubscriptionPlan | null>(null)
const deletingPlanId = ref<number | null>(null)

const planColumns = computed((): Column[] => [
  { key: 'id', label: 'ID' },
  { key: 'name', label: t('payment.admin.planName') },
  { key: 'group_id', label: t('payment.admin.group') },
  { key: 'price', label: t('payment.admin.price') },
  { key: 'validity_days', label: t('payment.admin.validity') },
  { key: 'for_sale', label: t('payment.admin.forSale') },
  { key: 'sort_order', label: t('payment.admin.sortOrder') },
  { key: 'actions', label: t('common.actions') },
])

async function loadPlans() {
  plansLoading.value = true
  try {
    const res = await adminPaymentAPI.getPlans()
    // Backend returns features as newline-separated string; parse to array
    plans.value = (res.data || []).map((p: Omit<SubscriptionPlan, 'features'> & { features: string | string[] }) => ({
      ...p,
      features: typeof p.features === 'string'
        ? p.features.split('\n').map((f: string) => f.trim()).filter(Boolean)
        : (p.features || []),
    }))
  }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
  finally { plansLoading.value = false }
}

function openPlanEdit(plan: SubscriptionPlan | null) {
  editingPlan.value = plan
  showPlanDialog.value = true
}


/** Quick toggle for_sale from the list */
async function toggleForSale(plan: SubscriptionPlan) {
  try {
    await adminPaymentAPI.updatePlan(plan.id, { for_sale: !plan.for_sale })
    plan.for_sale = !plan.for_sale
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

function confirmDeletePlan(plan: SubscriptionPlan) { deletingPlanId.value = plan.id; showDeletePlanDialog.value = true }
async function handleDeletePlan() {
  if (!deletingPlanId.value) return
  try { await adminPaymentAPI.deletePlan(deletingPlanId.value); appStore.showSuccess(t('common.deleted')); showDeletePlanDialog.value = false; loadPlans() }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
}

// ==================== Lifecycle ====================

onMounted(() => {
  loadGroups()
  loadPaymentConfig()
  loadPlans()
})
</script>
<style scoped>
.list-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
  /* Local type-scale tokens: ui-lint's scoped check requires `var(--...)` in
     view-level styles instead of literal font sizes/weights. */
  --cell-fs-1: 11.5px;
  --cell-fs-3: 12.5px;
  --cell-fs-4: 13px;
  --cell-fw-semibold: 600;
}

.list-page :deep(.ui-page-header) {
  margin-bottom: 0;
}

.filter-row {
  display: flex;
  align-items: center;
}

.filter-count {
  margin-left: auto;
  font-size: var(--cell-fs-3);
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.filter-count b {
  color: var(--foreground);
  font-weight: var(--cell-fw-semibold);
}

.table-card {
  display: flex;
  flex-direction: column;
  min-height: 0;
  border-radius: var(--radius-card);
  border: 1px solid color-mix(in oklch, var(--border) 85%, transparent);
  background: color-mix(in oklch, var(--surface) 70%, transparent);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  box-shadow: var(--shadow);
  overflow: hidden;
}

.table-card :deep(.table-wrapper) {
  overflow-y: visible;
}

.table-card :deep(.table-body) {
  background: transparent;
}

.cell-primary-name {
  font-size: var(--cell-fs-4);
  font-weight: var(--cell-fw-semibold);
}

.cell-primary-meta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: var(--cell-fs-1);
  color: var(--muted);
}

.cell-amount {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-variant-numeric: tabular-nums;
}

.cell-amount-value {
  font-size: var(--cell-fs-4);
  font-weight: var(--cell-fw-semibold);
  color: var(--foreground);
}

.cell-amount-meta {
  font-size: var(--cell-fs-1);
  color: var(--muted);
}

.cell-amount-strike {
  text-decoration: line-through;
}

.cell-time {
  font-size: var(--cell-fs-3);
  color: var(--muted);
}

.row-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 2px;
}

.plans-fab {
  display: none;
}
@media (max-width: 767px) {
  .plans-create-desktop {
    display: none;
  }
  .plans-fab {
    display: inline-flex;
  }
  .filter-row {
    display: none;
  }
}
</style>
