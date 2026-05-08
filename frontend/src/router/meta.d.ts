/**
 * Type definitions for Vue Router meta fields
 * Extends the RouteMeta interface with custom properties
 */

import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    /**
     * Whether this route requires authentication
     * @default true
     */
    requiresAuth?: boolean

    /**
     * Whether this route requires admin role
     * @default false
     */
    requiresAdmin?: boolean

    /**
     * Page title for this route
     */
    title?: string

    /**
     * Optional breadcrumb items for navigation
     */
    breadcrumbs?: Array<{
      label: string
      to?: string
    }>

    /**
     * Icon name for this route (for sidebar navigation)
     */
    icon?: string

    /**
     * Whether to hide this route from navigation menu
     * @default false
     */
    hideInMenu?: boolean

    /**
     * Whether this route requires internal payment system to be enabled
     * @default false
     */
    requiresPayment?: boolean

    /**
     * Whether this route requires ticket module to be enabled
     * @default false
     */
    requiresTicket?: boolean

    /**
     * Whether this route requires affiliate rebates to be enabled
     * @default false
     */
    requiresAffiliate?: boolean

    /**
     * Whether this route requires AI Studio to be enabled
     * @default false
     */
    requiresAiStudio?: boolean

    /**
     * Whether this route requires channel monitor to be enabled
     * @default false
     */
    requiresChannelMonitor?: boolean

    /**
     * Whether this route requires available channels to be enabled
     * @default false
     */
    requiresAvailableChannels?: boolean

    /**
     * Whether this route requires risk control to be enabled
     * @default false
     */
    requiresRiskControl?: boolean

    /**
     * i18n key for the page title
     */
    titleKey?: string

    /**
     * i18n key for the page description
     */
    descriptionKey?: string
  }
}
