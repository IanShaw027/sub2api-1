import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import { createI18n } from 'vue-i18n'
import OrdersTabBar from '../OrdersTabBar.vue'

const dummy = { template: '<div />' }

function makeRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/orders', name: 'OrderList', component: dummy },
      { path: '/orders/invoices', name: 'MyInvoices', component: dummy },
    ],
  })
}

const i18n = createI18n({
  legacy: false,
  locale: 'zh',
  messages: {
    zh: { nav: { myOrders: '我的订单', myInvoices: '我的发票' } },
  },
})

describe('OrdersTabBar', () => {
  it('marks orders tab active on /orders', async () => {
    const router = makeRouter()
    await router.push('/orders')
    await router.isReady()
    const wrapper = mount(OrdersTabBar, { global: { plugins: [router, i18n] } })
    const tabs = wrapper.findAll('button')
    expect(tabs[0].classes()).toContain('tab-active')
    expect(tabs[1].classes()).not.toContain('tab-active')
  })

  it('marks invoices tab active on /orders/invoices', async () => {
    const router = makeRouter()
    await router.push('/orders/invoices')
    await router.isReady()
    const wrapper = mount(OrdersTabBar, { global: { plugins: [router, i18n] } })
    const tabs = wrapper.findAll('button')
    expect(tabs[1].classes()).toContain('tab-active')
    expect(tabs[0].classes()).not.toContain('tab-active')
  })
})
