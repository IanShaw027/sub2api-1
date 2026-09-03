{"total_files": 931, "nontest": 622, "reachable": 597, "unreachable": 25, "unreach_lines": 3697}

## referenced by nobody (14, 1709 lines)
- src/views/admin/ops/components/OpsRuntimeSettingsCard.vue (536)
- src/views/admin/ops/components/OpsEmailNotificationCard.vue (441)
- src/components/payment/StripePaymentInline.vue (212)
- src/components/user/PlatformUsageBreakdown.vue (103)
- src/components/admin/payment/PaymentMethodChart.vue (91)
- src/components/ticket/TicketCategoryForm.vue (74)
- src/components/admin/payment/TopUsersLeaderboard.vue (68)
- src/components/common/Skeleton.vue (50)
- src/components/user/profile/ProfileAccountBindingsCard.vue (39)
- src/components/common/StatusBadge.vue (36)
- src/components/ui/index.ts (25)
- src/components/common/index.ts (14)
- src/components/tickets/TicketInfoItem.vue (13)
- src/views/auth/index.ts (7)

## referenced only by tests (6, 1485 lines)
- src/components/admin/account/OpenAIOAuthCapacityDialog.vue (587) <- src/components/admin/account/__tests__/OpenAIOAuthCapacityDialog.spec.ts
- src/components/payment/PaymentQRDialog.vue (311) <- src/components/payment/__tests__/PaymentQRDialog.spec.ts
- src/components/admin/payment/AdminOrderTable.vue (238) <- src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts
- src/components/admin/payment/AdminOrderDetail.vue (165) <- src/components/admin/payment/__tests__/orderCurrencyDisplay.spec.ts
- src/components/keys/EndpointPopover.vue (141) <- src/components/keys/__tests__/EndpointPopover.spec.ts
- src/composables/useForm.ts (43) <- src/composables/__tests__/useForm.spec.ts

## unreachable but imported by other unreachable files (5)
- src/components/ui/UiDrawer.vue (226) <- src/components/ui/__tests__/UiDrawer.spec.ts, src/components/ui/index.ts
- src/components/common/StatCard.vue (81) <- src/components/common/index.ts
- src/api/admin/oauthCapacity.ts (72) <- src/components/admin/account/OpenAIOAuthCapacityDialog.vue
- src/components/ui/SettingRow.vue (65) <- src/components/ui/__tests__/SettingRow.spec.ts, src/components/ui/index.ts
- src/components/ui/SettingsSection.vue (59) <- src/components/ui/__tests__/SettingsSection.spec.ts, src/components/ui/index.ts

## importer counts for suspects
- src/components/common/StatCard.vue: 1 non-test importers
- src/components/ui/StatCard.vue: 5 non-test importers
- src/components/common/StatusBadge.vue: 0 non-test importers
- src/components/ui/StatusBadge.vue: 14 non-test importers
- src/components/common/Toggle.vue: 15 non-test importers
- src/components/ui/ToggleSwitch.vue: 3 non-test importers
- src/components/common/Input.vue: 2 non-test importers
- src/components/common/TextArea.vue: 2 non-test importers
- src/components/ui/TextInput.vue: 3 non-test importers
- src/components/common/Select.vue: 61 non-test importers
- src/components/ui/UiSelect.vue: 4 non-test importers
- src/components/common/Pagination.vue: 31 non-test importers
- src/components/ui/UiPagination.vue: 6 non-test importers
- src/components/common/BaseDialog.vue: 82 non-test importers
- src/components/ui/UiModal.vue: 7 non-test importers
- src/components/common/EmptyState.vue: 22 non-test importers
- src/views/user/ChannelStatusView.vue: 1 non-test importers
- src/views/user/ChannelStatusV1View.vue: 1 non-test importers
- src/views/user/ChannelStatusV2View.vue: 1 non-test importers
- src/views/admin/ops/components/OpsErrorDetailModal.vue: 3 non-test importers
- src/views/admin/ops/components/OpsErrorDetailsModal.vue: 1 non-test importers
- src/components/layout/TablePageLayout.vue: 16 non-test importers
- src/components/common/ConfirmDialog.vue: 28 non-test importers
- src/components/common/LoadingSpinner.vue: 18 non-test importers
- src/components/common/Skeleton.vue: 0 non-test importers

## ticket vs tickets dirs
- src/components/ticket/TicketCategoryForm.vue: 0 importers
- src/components/tickets/TicketCategoryForm.vue: 2 importers
- src/components/tickets/TicketConversationPane.vue: 3 importers
- src/components/tickets/TicketCreateDialog.vue: 1 importers
- src/components/tickets/TicketDetailPane.vue: 2 importers
- src/components/tickets/TicketEditorCard.vue: 3 importers
- src/components/tickets/TicketInfoItem.vue: 0 importers
- src/components/tickets/TicketReplyTemplatesDialog.vue: 1 importers

## package usage (import sites / files)
- @airwallex/components-sdk: 1 sites, 1 files
- @lobehub/icons: 0 sites, 0 files
- @stripe/stripe-js: 6 sites, 4 files
- @tanstack/vue-virtual: 2 sites, 2 files
- @vueuse/core: 10 sites, 10 files
- axios: 6 sites, 5 files
- chart.js: 16 sites, 16 files
- dompurify: 8 sites, 8 files
- driver.js: 5 sites, 4 files
- file-saver: 2 sites, 2 files
- marked: 7 sites, 7 files
- pinia: 38 sites, 38 files
- qrcode: 4 sites, 4 files
- vue: 409 sites, 405 files
- vue-chartjs: 17 sites, 16 files
- vue-draggable-plus: 2 sites, 2 files
- vue-i18n: 431 sites, 428 files
- vue-router: 63 sites, 63 files
- xlsx: 1 sites, 1 files

## largest files
- src/views/admin/SettingsView.vue (14077)
- src/components/account/CreateAccountModal.vue (7464)
- src/views/admin/GroupsView.vue (7015)
- src/components/account/EditAccountModal.vue (5932)
- src/views/admin/AccountsView.vue (3498)
- src/views/user/BatchImageGuideView.vue (2695)
- src/types/index.ts (2534)
- src/components/account/BulkEditAccountModal.vue (2443)
- src/views/user/KeysView.vue (2443)
- src/views/admin/RiskControlView.vue (2372)
- src/views/admin/ProxiesView.vue (2140)
- src/views/admin/DashboardView.vue (2123)
- src/views/admin/UsersView.vue (1909)
- src/components/keys/UseKeyModal.vue (1869)
- src/i18n/locales/en/admin/accounts.ts (1856)
- src/api/admin/settings.ts (1834)
- src/i18n/locales/zh/admin/accounts.ts (1833)
- src/components/account/AccountUsageCell.vue (1759)
- src/views/admin/ChannelsView.vue (1721)
- src/views/admin/RedeemView.vue (1684)
- src/views/admin/ops/components/OpsDashboardHeader.vue (1628)
- src/i18n/locales/en/admin/settings.ts (1503)
- src/i18n/locales/zh/admin/settings.ts (1497)
- src/views/admin/SubscriptionsView.vue (1466)
- src/api/admin/ops.ts (1363)

## markers
- legacy: 132  e.g. src/api/admin/settings.ts:383; src/api/admin/settings.ts:397; src/api/admin/settings.ts:411; src/api/admin/settings.ts:416
