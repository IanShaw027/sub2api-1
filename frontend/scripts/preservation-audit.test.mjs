import test from 'node:test'
import assert from 'node:assert/strict'
import { inventorySource, compareInventories } from './preservation-audit.mjs'

test('extracts Vue interactions structurally and ignores commented-out controls', () => {
  const result = inventorySource('frontend/src/views/Example.vue', `<template>
    <!-- <button @click="removed">Old</button> -->
    <button id="save" @click="save" :aria-label="label" v-if="allowed">Save</button>
    <input v-model="name" class="hidden md:block" />
  </template><script setup lang="ts">const name = 'name'</script>`)
  assert.deepEqual(result.diagnostics, [])
  assert.deepEqual(result.inventory.actions.map(item => item.value), ['click:save'])
  assert.ok(result.inventory.anchors.some(item => item.value === 'id:save'))
  assert.ok(result.inventory.models.some(item => item.value === ':name'))
  assert.ok(result.inventory.visibility.some(item => item.value.includes('allowed')))
  assert.ok(result.inventory.visibility.some(item => item.value.includes('hidden md:block')))
})

test('keeps dynamic route expressions explicit and inventories navigation, columns and API calls', () => {
  const result = inventorySource('frontend/src/router/index.ts', `
    import { accountAPI } from '@/api/accounts'
    const routes = [{ path: '/keys', component: () => import('@/views/Keys.vue'), meta: { requiresAuth: true } }, { path: creationPath(mode), component: View }]
    const nav = [{ path: '/keys', label: t('nav.keys'), icon: Key, hideInSimpleMode: true }]
    const columns = [{ key: 'created_at', label: t('created'), sortable: true }]
    const DEFAULT_HIDDEN_COLUMNS = ['id']
    accountAPI.update(id, payload)
  `)
  assert.deepEqual(result.diagnostics, [])
  assert.ok(result.inventory.routes.some(item => item.value === '/keys' && item.meta.includes('requiresAuth')))
  assert.ok(result.inventory.routes.some(item => item.value === 'expression:creationPath(mode)' && item.expression))
  assert.equal(result.inventory.navigation[0].hideInSimpleMode, 'true')
  assert.equal(result.inventory.columns[0].value, 'created_at')
  assert.equal(result.inventory.apiCalls[0].value, 'accountAPI.update')
  assert.ok(result.inventory.imports.some(item => item.dynamic && item.value === '@/views/Keys.vue'))
  assert.ok(result.inventory.visibility.some(item => item.value.startsWith('DEFAULT_HIDDEN_COLUMNS=')))
})

test('surfaces removed candidates while allowing component moves without asserting behavior', () => {
  const before = inventorySource('old.vue', '<template><button @click="save"/><button @click="remove"/></template>').inventory
  const after = inventorySource('new.vue', '<template><button @click="save"/></template>').inventory
  const result = compareInventories(before, after)
  assert.deepEqual(result.actions.removedCandidates.map(item => item.value), ['click:remove'])
  assert.equal(result.actions.before, 2)
  assert.equal(result.actions.after, 1)
})

test('parse failures are diagnostics rather than silent empty inventories', () => {
  assert.ok(inventorySource('broken.ts', 'const value = {').diagnostics.length)
  assert.ok(inventorySource('broken.vue', '<template><button></template>').diagnostics.length)
})

test('inventories links, object menu handlers and imported store calls', () => {
  const result = inventorySource('page.vue', `<template><a href="/docs"/><RouterLink :to="destination" /></template><script setup>
    import { useAuthStore as useSession } from '@/stores/auth'
    const session = useSession()
    session.logout({ all: true })
    const items = [{ label: 'Exit', onSelect: logout }]
  </script>`)
  assert.deepEqual(result.inventory.links.map(item => item.value), ['href:/docs', ':to:destination'])
  assert.equal(result.inventory.menuActions[0].value, 'onSelect:logout')
  assert.equal(result.inventory.storeCalls[0].value, 'session.logout')
  assert.deepEqual(result.inventory.storeCalls[0].arguments, ['{ all: true }'])
})

test('per-file multiset diff does not hide moved, duplicated or argument-changed operations', () => {
  const before = inventorySource('a.vue', '<template><button @click="save"/><button @click="save"/></template>').inventory
  const after = inventorySource('a.vue', '<template><button @click="save"/></template>').inventory
  assert.equal(compareInventories(before, after).actions.removedCandidates.length, 0)
  assert.equal(compareInventories(before, after).actions.removedOccurrences.length, 1)
  const moved = inventorySource('b.vue', '<template><button @click="save"/></template>').inventory
  assert.equal(compareInventories(before, moved).actions.removedOccurrences.length, 2)
  const oldAPI = inventorySource('a.ts', 'fetch("/old")').inventory
  const newAPI = inventorySource('a.ts', 'fetch("/new")').inventory
  assert.equal(compareInventories(oldAPI, newAPI).apiCalls.removedOccurrences.length, 1)
})
