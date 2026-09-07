import { test } from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const root = fileURLToPath(new URL('..', import.meta.url))

test('legacy stat classes are rejected in views but shared component names are allowed', () => {
  const dir = fs.mkdtempSync(path.join(root, 'src/views/ui-lint-fixture-'))
  const file = path.join(dir, 'Fixture.vue')
  const lint = source => {
    fs.writeFileSync(file, source)
    const result = spawnSync(process.execPath, [path.join(root, 'scripts/ui-lint.mjs'), '--json', file], { encoding: 'utf8' })
    return { status: result.status, ...JSON.parse(result.stdout) }
  }
  try {
    const rejected = lint('<template><div class="stat-card"><i :class="[\'stat-icon\', \'stat-icon-primary\']" /></div></template>')
    assert.equal(rejected.status, 1)
    assert.equal(rejected.legacy.length, 3)
    const allowed = lint('<template><StatCard class="ui-stat-card" /><MiniStatCard /></template>')
    assert.equal(allowed.status, 0)
    assert.equal(allowed.total, 0)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})
