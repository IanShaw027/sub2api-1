#!/usr/bin/env node
// Inventory is review evidence, not a proof of runtime behavior or visual accessibility.
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'
import { parse } from 'vue/compiler-sfc'

export const PRE_GLASS_BASE = 'f1c8ab7da6284179461613cec8a30a8f87f2c545'
export const HARDENING_BASE = 'c4efeec16'
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const KINDS = ['routes', 'navigation', 'columns', 'anchors', 'actions', 'links', 'menuActions', 'models', 'apiCalls', 'storeCalls', 'visibility', 'imports']
const normalize = value => value.trim().replace(/\s+/g, ' ')
const sourceFile = file => /\.(vue|ts|tsx|js|jsx)$/.test(file) && !/(?:^|\/)(?:__tests__|__mocks__)(?:\/|$)|\.(?:spec|test)\./.test(file)
const propertyName = node => ts.isIdentifier(node) || ts.isStringLiteral(node) ? node.text : node.getText()

export function inventorySource(file, text) {
  const inventory = Object.fromEntries(KINDS.map(kind => [kind, []]))
  const diagnostics = []
  const add = (kind, value, line, detail = {}) => inventory[kind].push({ value: normalize(value), file, line, ...detail })
  const script = (content, offset = 0) => {
    const ast = ts.createSourceFile(file + '.ts', content, ts.ScriptTarget.Latest, true, file.endsWith('tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS)
    for (const error of ast.parseDiagnostics) diagnostics.push({ file, line: offset + ast.getLineAndCharacterOfPosition(error.start ?? 0).line + 1, message: ts.flattenDiagnosticMessageText(error.messageText, '\n') })
    const apiBindings = new Set()
    const storeFactories = new Set()
    const storeBindings = new Set()
    for (const node of ast.statements) {
      if (!ts.isImportDeclaration(node) || !ts.isStringLiteral(node.moduleSpecifier)) continue
      const module = node.moduleSpecifier.text
      add('imports', module, offset + ast.getLineAndCharacterOfPosition(node.getStart(ast)).line + 1)
      const named = node.importClause?.namedBindings
      if (/(?:^|\/)stores(?:\/|$)/.test(module) && named && ts.isNamedImports(named)) {
        named.elements.forEach(element => storeFactories.add(element.name.text))
      }
      if (!/(?:^|\/)api(?:\/|$)|authenticatedFetch/.test(module)) continue
      const clause = node.importClause
      if (clause?.name) apiBindings.add(clause.name.text)
      const bindings = clause?.namedBindings
      if (bindings && ts.isNamespaceImport(bindings)) apiBindings.add(bindings.name.text)
      if (bindings && ts.isNamedImports(bindings)) bindings.elements.forEach(element => apiBindings.add(element.name.text))
    }
    const collectStores = node => {
      if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name) && node.initializer && ts.isCallExpression(node.initializer) && storeFactories.has(node.initializer.expression.getText(ast))) storeBindings.add(node.name.text)
      ts.forEachChild(node, collectStores)
    }
    collectStores(ast)
    const visit = node => {
      const line = offset + ast.getLineAndCharacterOfPosition(node.getStart(ast)).line + 1
      if (ts.isObjectLiteralExpression(node)) {
        const props = new Map(node.properties.filter(ts.isPropertyAssignment).map(prop => [propertyName(prop.name), prop.initializer]))
        const get = key => props.get(key)?.getText(ast)
        const literal = key => { const value = props.get(key); return value && ts.isStringLiteralLike(value) ? value.text : undefined }
        if (props.has('label') || props.has('icon')) {
          for (const key of ['action', 'onClick', 'onSelect', 'handler']) if (props.has(key)) add('menuActions', `${key}:${get(key)}`, line)
        }
        if (props.has('path') && (/\/router\//.test(file) || props.has('component') || props.has('redirect'))) {
          add('routes', literal('path') ?? `expression:${get('path')}`, line, { expression: !literal('path'), component: get('component'), redirect: get('redirect'), alias: get('alias'), meta: get('meta') })
        }
        if (props.has('path') && (props.has('label') || props.has('icon'))) {
          add('navigation', literal('path') ?? `expression:${get('path')}`, line, { label: get('label'), hideInSimpleMode: get('hideInSimpleMode'), featureFlag: get('featureFlag'), expandOnly: get('expandOnly') })
        }
        if (props.has('key') && (props.has('label') || props.has('title')) && (props.has('sortable') || props.has('width') || /columns/i.test(node.parent.getText(ast).slice(0, 180)))) {
          add('columns', literal('key') ?? `expression:${get('key')}`, line, { label: get('label') ?? get('title'), condition: normalize(node.parent.getText(ast)).slice(0, 240) })
        }
      }
      if (ts.isVariableDeclaration(node) && /(?:HIDDEN_COLUMNS|VISIBLE_COLUMNS|creationModes)/.test(node.name.getText(ast)) && node.initializer) {
        add('visibility', `${node.name.getText(ast)}=${node.initializer.getText(ast)}`, line)
      }
      if (ts.isCallExpression(node)) {
        const callee = node.expression.getText(ast)
        const root = callee.split(/[.[]/, 1)[0]
        if (apiBindings.has(root) || /^(?:fetch|authenticatedFetch)$/.test(callee) || /^(?:\w*API|apiClient|axios)\./.test(callee)) {
          add('apiCalls', callee, line, { arguments: node.arguments.map(argument => normalize(argument.getText(ast))) })
        }
        if (storeBindings.has(root)) add('storeCalls', callee, line, { arguments: node.arguments.map(argument => normalize(argument.getText(ast))) })
        if (node.expression.kind === ts.SyntaxKind.ImportKeyword && ts.isStringLiteralLike(node.arguments[0] ?? node)) {
          add('imports', node.arguments[0].text, line, { dynamic: true })
        }
      }
      ts.forEachChild(node, visit)
    }
    visit(ast)
  }
  if (!file.endsWith('.vue')) script(text)
  else {
    const result = parse(text, { filename: file })
    diagnostics.push(...result.errors.map(error => ({ file, message: typeof error === 'string' ? error : error.message })))
    const { descriptor } = result
    for (const block of [descriptor.script, descriptor.scriptSetup]) if (block) script(block.content, block.loc.start.line - 1)
    const template = descriptor.template
    const visit = node => {
      if (node.type === 1) {
        for (const prop of node.props) {
          const line = prop.loc.start.line
          if (prop.type === 6) {
            if (['id', 'data-tour', 'data-testid', 'aria-label'].includes(prop.name)) add('anchors', `${prop.name}:${prop.value?.content ?? ''}`, line)
            if (['href', 'to'].includes(prop.name)) add('links', `${prop.name}:${prop.value?.content ?? ''}`, line, { tag: node.tag })
            if (prop.name === 'class' && /(?:^|\s)(?:[\w-]+:)*hidden(?:\s|$)/.test(prop.value?.content ?? '')) add('visibility', `${node.tag}:class:${prop.value.content}`, line)
          } else if (prop.type === 7) {
            const arg = prop.arg?.content ?? ''
            const exp = prop.exp?.content ?? ''
            if (prop.name === 'on') add('actions', `${arg}:${exp}`, line, { tag: node.tag })
            if (prop.name === 'model') add('models', `${arg}:${exp}`, line, { tag: node.tag })
            if (prop.name === 'bind' && ['id', 'data-tour', 'data-testid', 'aria-label'].includes(arg)) add('anchors', `:${arg}:${exp}`, line)
            if (prop.name === 'bind' && ['href', 'to'].includes(arg)) add('links', `:${arg}:${exp}`, line, { tag: node.tag })
            if (['if', 'else-if', 'show'].includes(prop.name) || (prop.name === 'bind' && arg === 'class')) add('visibility', `${node.tag}:${prop.name}:${arg}:${exp}`, line)
          }
        }
      }
      for (const child of node.children ?? []) visit(child)
    }
    if (template?.ast) visit(template.ast)
    else if (template) diagnostics.push({ file, message: 'Vue compiler did not provide template AST; template inventory is incomplete.' })
  }
  return { inventory, diagnostics }
}

export function compareInventories(before, after) {
  return Object.fromEntries(KINDS.map(kind => {
    const beforeValues = new Set(before[kind].map(item => item.value))
    const afterValues = new Set(after[kind].map(item => item.value))
    const afterOccurrences = new Map()
    const signature = item => JSON.stringify([item.file, item.value, item.arguments ?? null])
    for (const item of after[kind]) afterOccurrences.set(signature(item), (afterOccurrences.get(signature(item)) ?? 0) + 1)
    const removedOccurrences = before[kind].filter(item => {
      const key = signature(item)
      const count = afterOccurrences.get(key) ?? 0
      if (!count) return true
      afterOccurrences.set(key, count - 1)
      return false
    })
    return [kind, {
      before: before[kind].length,
      after: after[kind].length,
      removedCandidates: before[kind].filter(item => !afterValues.has(item.value)),
      addedCandidates: after[kind].filter(item => !beforeValues.has(item.value)),
      removedOccurrences
    }]
  }))
}

function git(args) {
  return execFileSync('git', args, { cwd: ROOT, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 })
}
function gitBuffer(args) {
  return execFileSync('git', args, { cwd: ROOT, maxBuffer: 256 * 1024 * 1024 })
}

function snapshot(revision) {
  const files = (revision === 'worktree'
    ? git(['ls-files', '-z', '--cached', '--others', '--exclude-standard', '--', 'frontend/src'])
    : git(['ls-tree', '-rz', '--name-only', revision, '--', 'frontend/src']))
    .split('\0').filter(sourceFile).filter(Boolean)
  const inventory = Object.fromEntries(KINDS.map(kind => [kind, []]))
  const diagnostics = []
  const missingFiles = []
  let archived = null
  if (revision !== 'worktree') {
    archived = new Map()
    // One archive avoids spawning one git process per source file in large histories.
    const listing = gitBuffer(['archive', '--format=tar', revision, 'frontend/src'])
    const temp = fs.mkdtempSync(path.join('/tmp', 'sub2api-preservation-'))
    const archivePath = path.join(temp, 'snapshot.tar')
    fs.writeFileSync(archivePath, listing)
    execFileSync('tar', ['-xf', archivePath, '-C', temp])
    for (const file of files) {
      const absolute = path.join(temp, file)
      if (fs.existsSync(absolute)) archived.set(file, fs.readFileSync(absolute, 'utf8'))
    }
    fs.rmSync(temp, { recursive: true, force: true })
  }
  for (const file of new Set(files)) {
    if (revision === 'worktree' && !fs.existsSync(path.join(ROOT, file))) { missingFiles.push(file); continue }
    const text = revision === 'worktree' ? fs.readFileSync(path.join(ROOT, file), 'utf8') : archived.get(file)
    if (text == null) { missingFiles.push(file); continue }
    const result = inventorySource(file, text)
    for (const kind of KINDS) inventory[kind].push(...result.inventory[kind])
    diagnostics.push(...result.diagnostics)
  }
  return { revision, commit: revision === 'worktree' ? git(['rev-parse', 'HEAD']).trim() : git(['rev-parse', `${revision}^{commit}`]).trim(), files: files.filter(file => !missingFiles.includes(file)), inventory, diagnostics }
}

function main() {
  const args = process.argv.slice(2)
  const allowed = new Set(['--base', '--target', '--out'])
  if (args.includes('--help')) {
    console.log('node scripts/preservation-audit.mjs [--base c4efeec16|pre-glass|REV] [--target worktree|REV] [--out report.json]\nExit 0: inventory generated (NOT behavior passed). Exit 1: parse diagnostics. Exit 2: invalid invocation or unreadable revision.')
    return
  }
  const options = { '--base': HARDENING_BASE, '--target': 'worktree' }
  for (let i = 0; i < args.length; i += 2) {
    if (!allowed.has(args[i]) || !args[i + 1] || args[i + 1].startsWith('--')) throw new Error(`Invalid option: ${args[i]}`)
    options[args[i]] = args[i + 1]
  }
  const before = snapshot(options['--base'] === 'pre-glass' ? PRE_GLASS_BASE : options['--base'])
  const after = snapshot(options['--target'])
  const report = {
    schemaVersion: 2,
    status: 'manual-review-required',
    baseline: before.commit,
    target: { revision: after.revision, commit: after.commit, dirty: after.revision === 'worktree' ? Boolean(git(['status', '--porcelain', '--', 'frontend/src']).trim()) : false },
    limitations: [
      'AST extraction records syntax, not authorization, reachability, rendered visibility, asynchronous behavior or API success.',
      'Dynamic routes/navigation, spread/computed columns and inherited props require runtime checks; expression entries are not expanded.',
      'removedCandidates uses repository-wide values; removedOccurrences additionally compares file, value, API/store arguments and multiplicity. Moves and renames still need equivalence review.',
      'Removed handlers may have moved/renamed; API inventory covers imported API bindings and conventional client calls, not every indirect request.',
      'Responsive CSS, slot wiring, disabled/loading state and localStorage migrations require targeted tests and browser review.',
      'Empty candidate lists and exit code 0 do not mean functionality is preserved.'
    ],
    changes: compareInventories(before.inventory, after.inventory),
    files: { removed: before.files.filter(file => !after.files.includes(file)), added: after.files.filter(file => !before.files.includes(file)) },
    diagnostics: [...before.diagnostics.map(item => ({ ...item, revision: before.revision })), ...after.diagnostics.map(item => ({ ...item, revision: after.revision }))],
    snapshots: { before, after }
  }
  const output = JSON.stringify(report, null, 2) + '\n'
  if (options['--out']) fs.writeFileSync(path.resolve(options['--out']), output)
  else process.stdout.write(output)
  console.error(JSON.stringify({ status: report.status, baseline: report.baseline, target: report.target, diagnostics: report.diagnostics.length, changes: Object.fromEntries(Object.entries(report.changes).map(([kind, change]) => [kind, { before: change.before, after: change.after, removedCandidates: change.removedCandidates.length, addedCandidates: change.addedCandidates.length }])) }, null, 2))
  if (report.diagnostics.length) process.exitCode = 1
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try { main() } catch (error) { console.error(error.message); process.exitCode = 2 }
}
