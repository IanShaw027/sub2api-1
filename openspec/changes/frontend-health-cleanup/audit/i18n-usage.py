import re,os,json,subprocess,sys
root='src'
# 1. flatten zh keys via node (evaluate TS by stripping types is hard) -> use regex parse of nested object literals
def parse_keys(path):
    src=open(path,encoding='utf-8').read()
    # crude: track nesting of { } and keys `name:` or `'name':`
    keys=[];stack=[];i=0;n=len(src)
    tok=re.compile(r"(?P<key>[A-Za-z0-9_$]+|'[^']+'|\"[^\"]+\")\s*:|(?P<open>\{)|(?P<close>\})|(?P<str>'(?:\\.|[^'\\])*'|\"(?:\\.|[^\"\\])*\"|`(?:\\.|[^`\\])*`)|(?P<cmt>//[^\n]*|/\*.*?\*/)")
    pending=None
    for m in tok.finditer(src):
        if m.group('cmt') or m.group('str'):
            if m.group('str') and pending is not None:
                keys.append('.'.join([x for x in stack+[pending] if x])); pending=None
            continue
        if m.group('key'):
            pending=m.group('key').strip('\'"')
        elif m.group('open'):
            if pending is not None: stack.append(pending); pending=None
            else: stack.append(None)
        elif m.group('close'):
            if stack: stack.pop()
    return keys
def all_keys(locale):
    out=set()
    for dp,_,fs in os.walk(f'{root}/i18n/locales/{locale}'):
        for f in fs:
            if not f.endswith('.ts'): continue
            p=os.path.join(dp,f)
            rel=os.path.relpath(p,f'{root}/i18n/locales/{locale}')[:-3]
            ks=[k for k in parse_keys(p) if k]
            # namespace: index.ts merges modules; assume file exports default object whose top-level keys are namespaces already
            for k in ks:
                if rel.startswith('admin/'): k='admin.'+k
                out.add(k)
    return out
zh=all_keys('zh'); en=all_keys('en')
print('zh keys',len(zh),'en keys',len(en),'zh-only',len(zh-en),'en-only',len(en-zh))
# 2. collect referenced keys in src (non-locale, non-test)
code=''
files=[]
for dp,_,fs in os.walk(root):
    if '/i18n/locales' in dp or '__tests__' in dp: continue
    for f in fs:
        if f.endswith(('.vue','.ts')) and not f.endswith('.spec.ts'):
            p=os.path.join(dp,f); files.append(p); code+=open(p,encoding='utf-8',errors='ignore').read()+'\n'
refs=set(re.findall(r"""(?<![\w.])(?:\$?t|te|tm|rt|tc)\(\s*['"`]([A-Za-z0-9_.$-]+)['"`]""",code))
strs=set(re.findall(r"""['"`]([a-z][A-Za-z0-9_$-]*(?:\.[A-Za-z0-9_$-]+)+)['"`]""",code))
prefixes=set(re.findall(r"""['"`]([a-z][A-Za-z0-9_$.-]*)\.\$\{""",code))|set(re.findall(r"""`([a-z][A-Za-z0-9_$.-]*\.)\$\{""",code))
used=set()
for k in zh:
    if k in refs or k in strs: used.add(k); continue
    for p in prefixes:
        if k.startswith(p.rstrip('.')+'.'): used.add(k); break
unused=sorted(zh-used)
print('referenced (exact or by dynamic prefix)',len(used),'unused',len(unused), 'files scanned',len(files))
# group unused by top-level namespace
from collections import Counter
c=Counter(k.split('.')[0] for k in unused)
print('unused by namespace', c.most_common(25))
c2=Counter('.'.join(k.split('.')[:2]) for k in unused)
print('unused by 2-level', c2.most_common(30))
open('/tmp/claude-1000/-home-box-code-sub2api/42a7ff81-4e53-45f5-b619-8dc39e8731a3/scratchpad/audit/i18n-unused.txt','w').write('\n'.join(unused))
open('/tmp/claude-1000/-home-box-code-sub2api/42a7ff81-4e53-45f5-b619-8dc39e8731a3/scratchpad/audit/i18n-zh-only.txt','w').write('\n'.join(sorted(zh-en)))
open('/tmp/claude-1000/-home-box-code-sub2api/42a7ff81-4e53-45f5-b619-8dc39e8731a3/scratchpad/audit/i18n-en-only.txt','w').write('\n'.join(sorted(en-zh)))
