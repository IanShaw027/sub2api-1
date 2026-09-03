import os,re,json,collections
ROOT='src'
files=[]
for dp,dn,fs in os.walk(ROOT):
    for f in fs:
        if f.endswith(('.ts','.vue','.js')) and not f.endswith('.d.ts'):
            files.append(os.path.join(dp,f))
files=sorted(files)
is_test=lambda p: '__tests__' in p or p.endswith('.spec.ts') or p.endswith('.test.ts') or '/test/' in p or p.startswith('src/__tests__')
imp_re=re.compile(r"""(?:from\s*|import\s*\(?\s*|require\s*\(\s*)['"]([^'"]+)['"]""")
def resolve(spec, frm):
    if spec.startswith('@/'): base=os.path.join(ROOT,spec[2:])
    elif spec.startswith('.'): base=os.path.normpath(os.path.join(os.path.dirname(frm),spec))
    else: return None
    for c in [base, base+'.ts', base+'.vue', base+'.js', os.path.join(base,'index.ts'), os.path.join(base,'index.vue')]:
        if os.path.isfile(c): return c
    return None
graph={}; importers=collections.defaultdict(set); pkg_use=collections.Counter(); pkg_files=collections.defaultdict(set)
for f in files:
    s=open(f,encoding='utf-8',errors='ignore').read()
    deps=set()
    for m in imp_re.finditer(s):
        spec=m.group(1)
        r=resolve(spec,f)
        if r: deps.add(r); importers[r].add(f)
        elif not spec.startswith('.') and not spec.startswith('@/'):
            pk=spec if not spec.startswith('@') else '/'.join(spec.split('/')[:2])
            pk=pk.split('/')[0] if not pk.startswith('@') else pk
            pkg_use[pk]+=1; pkg_files[pk].add(f)
    graph[f]=deps
entries=[p for p in ['src/main.ts','src/App.vue','src/router/index.ts'] if os.path.exists(p)]
seen=set(entries); st=list(entries)
while st:
    x=st.pop()
    for d in graph.get(x,()):
        if d not in seen: seen.add(d); st.append(d)
nontest=[f for f in files if not is_test(f)]
unreach=[f for f in nontest if f not in seen and '/i18n/locales/' not in f]
def lines(p): return sum(1 for _ in open(p,encoding='utf-8',errors='ignore'))
only_tests=[f for f in unreach if importers[f] and all(is_test(i) for i in importers[f])]
nobody=[f for f in unreach if not importers[f]]
other=[f for f in unreach if f not in only_tests and f not in nobody]
out={'total_files':len(files),'nontest':len(nontest),'reachable':len([f for f in nontest if f in seen]),
     'unreachable':len(unreach),'unreach_lines':sum(lines(f) for f in unreach)}
print(json.dumps(out))
print('\n## referenced by nobody (%d, %d lines)'%(len(nobody),sum(lines(f) for f in nobody)))
for f in sorted(nobody,key=lambda f:-lines(f)): print(f'- {f} ({lines(f)})')
print('\n## referenced only by tests (%d, %d lines)'%(len(only_tests),sum(lines(f) for f in only_tests)))
for f in sorted(only_tests,key=lambda f:-lines(f)): print(f'- {f} ({lines(f)}) <- {", ".join(sorted(importers[f]))[:120]}')
print('\n## unreachable but imported by other unreachable files (%d)'%len(other))
for f in sorted(other,key=lambda f:-lines(f)): print(f'- {f} ({lines(f)}) <- {", ".join(sorted(importers[f]))[:120]}')
# duplicates suspects: importer counts
print('\n## importer counts for suspects')
sus=['src/components/common/StatCard.vue','src/components/ui/StatCard.vue','src/components/common/StatusBadge.vue','src/components/ui/StatusBadge.vue','src/components/common/Toggle.vue','src/components/ui/ToggleSwitch.vue','src/components/common/Input.vue','src/components/common/TextArea.vue','src/components/ui/TextInput.vue','src/components/common/Select.vue','src/components/ui/UiSelect.vue','src/components/common/Pagination.vue','src/components/ui/UiPagination.vue','src/components/common/BaseDialog.vue','src/components/ui/UiModal.vue','src/components/common/EmptyState.vue','src/views/user/ChannelStatusView.vue','src/views/user/ChannelStatusV1View.vue','src/views/user/ChannelStatusV2View.vue','src/views/admin/ops/components/OpsErrorDetailModal.vue','src/views/admin/ops/components/OpsErrorDetailsModal.vue','src/components/layout/TablePageLayout.vue','src/components/common/ConfirmDialog.vue','src/components/common/LoadingSpinner.vue','src/components/common/Skeleton.vue']
for s_ in sus:
    imps=[i for i in importers.get(s_,()) if not is_test(i)]
    print(f'- {s_}: {len(imps)} non-test importers' + ('' if os.path.exists(s_) else ' (MISSING)'))
print('\n## ticket vs tickets dirs')
for d in ['src/components/ticket','src/components/tickets']:
    for f in sorted(os.listdir(d)) if os.path.isdir(d) else []:
        p=os.path.join(d,f)
        if p.endswith('.vue'): print(f'- {p}: {len([i for i in importers.get(p,()) if not is_test(i)])} importers')
print('\n## package usage (import sites / files)')
pkg=json.load(open('package.json'))
for k in sorted(pkg['dependencies']): print(f'- {k}: {pkg_use.get(k,0)} sites, {len(pkg_files.get(k,()))} files')
print('\n## largest files')
big=sorted(nontest,key=lambda f:-lines(f))[:25]
for f in big: print(f'- {f} ({lines(f)})')
print('\n## markers')
mk=collections.Counter(); ex=collections.defaultdict(list)
for f in nontest:
    for i,l in enumerate(open(f,encoding='utf-8',errors='ignore'),1):
        for w in ['@deprecated','deprecated','legacy','TODO','FIXME','HACK','XXX','TEMP(']:
            if w in l: mk[w]+=1; ex[w].append(f'{f}:{i}')
for w,c in mk.most_common(): print(f'- {w}: {c}  e.g. ' + '; '.join(ex[w][:4]))
