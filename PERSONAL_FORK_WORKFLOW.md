# 个人 Fork 分支维护工作流

`personal-main` 的日常维护说明。同步上游、冲突取舍、向 Wei-Shaw/sub2api 提 PR、以及历史再次打结时的压平，都只看这份文档。

## 一、分支与远程

| 分支 / 引用 | 角色 |
|---|---|
| `personal-main` | 个人主线：上游历史 + 个人定制。日常开发只在这里 |
| `personal-dev` | 冻结存档，只读对照。不要在上面开发，也不要在同步上游时从它往主线合 |
| `main` | 保留。不作开发，也不用它同步上游 |
| `upstream/main` | 上游只读镜像（Wei-Shaw/sub2api）。只 fetch，不往上推 |
| `archive/personal-dev-20260813` | 旧 `personal-dev` 历史 tag，考古用 |

远程：

| 远程 | 仓库 | 用途 |
|---|---|---|
| `origin` | IanShaw027/sub2api | 个人仓库。只保留 `personal-main` / `personal-dev` / `main` |
| `fork` | IanShaw027/sub2api-1 | 专门给上游开 PR，不要从这里往 `personal-main` 合 |
| `upstream` | Wei-Shaw/sub2api | 上游，只读 |

## 二、日常开发

```bash
git switch personal-main
git pull --ff-only origin personal-main
# ...开发...
git commit -m "feat(xxx): ..."
git push origin personal-main
```

多机或 worktree：**先 pull --ff-only 再开发**。如果已经分叉，只用 `git pull --rebase` 收敛**尚未推送**的少量本地提交。

## 三、同步上游

每次上游有更新，只做这一步。停在 `personal-main` 上，把 `upstream/main` merge 进来，产生**一个** merge commit。

工作区必须干净。有未提交改动先 stash 或提交，再同步。

不要用 `merge -X theirs` / `-X ours`。那是整侧覆盖，不是审查。

```bash
git fetch upstream
git switch personal-main
git log --first-parent --oneline personal-main..upstream/main   # 先看上游进来了什么
git merge upstream/main
# 按下面取舍，不要解完红字就 commit
git push origin personal-main
```

建议：

```bash
git config rerere.enabled true          # 只记住同一处文本冲突的旧解法
git config alias.plog "log --first-parent --oneline"
```

`rerere` 不管自动合并，也不能代替热区复查。

不要用 rebase 把整条 `personal-main` 接到上游上，也不要先切到 `upstream/main` 再 squash 个人分支——那是第六节的压平手术，不是日常同步。

### 取舍政策：两边都留

默认并集，不是选边：

- 留下 `personal-main` 的定制：Grok、Kiro、device TLS pin、identity pinning、429/529 重试与冷却、个人管理端多出来的字段和菜单。
- 同时收上游新功能：新供应商、渠道监控、协议修复、新路由 / setting / i18n。

判断：

1. **加法对加法**（新平台、新路由、新 i18n、新 setting key）→ 并集。
2. **上游修 bug / 改协议，个人侧没改同一语义** → 收上游，再确认个人调用点还成立。
3. **同一段行为两边都改了**（重试、TLS 选择、调度）→ 停下来对，禁止整文件 `checkout --ours` / `--theirs`。
4. **上游用另一种实现覆盖了个人特性** → 默认留个人侧，除非这次明确弃掉。

生成代码（`backend/ent/`、`wire_gen.go`）不手解。手写 schema / `wire.go` 解完再 `go generate`。

### 三种冲突

| 种类 | 表现 | 怎么处理 |
|---|---|---|
| 文本冲突 | `<<<<<<<` | 按上面四条解，列表/枚举/路由/DI 用并集 |
| 静默自动合并 | 同一文件两边改了不同函数，Git 直接拼上 | 热区必须打开看，不能当已审过 |
| 跨文件语义冲突 | 上游改了约定，个人补丁还按旧假设工作 | 靠热区 diff + 单测，Git 看不见 |

没冲突 ≠ 没有取舍。只表示两边没改同一段文字。

### 合入后先看三份 diff

有冲突或已经自动合完都要看，不要只看 `git diff --diff-filter=U`。

```bash
# 1. 相对合入前的 personal-main，工作区变了什么
git diff --stat HEAD

# 2. 合完以后相对上游还多出哪些个人补丁
git diff --stat MERGE_HEAD

# 3. 上游相对分叉点改过哪些个人热区（即使没有 <<<<<<<）
git diff HEAD...MERGE_HEAD -- \
  backend/internal/domain/constants.go \
  backend/internal/service/domain_constants.go \
  backend/internal/service/gateway_upstream_request.go \
  backend/internal/service/ratelimit_service.go \
  backend/internal/repository/http_upstream.go \
  backend/internal/handler/handler.go \
  backend/internal/handler/wire.go \
  backend/internal/server/routes/admin.go \
  backend/ent/schema
```

第 3 份里出现的热区文件，即使没有冲突标记，也要对照自动拼出来的结果。热区还包括 Grok / Kiro 网关、device TLS / identity pinning、个人管理端和对应前端（账号弹窗、平台 badge、admin i18n）。

### 合完必跑

解完或自动合完都要跑，不能只靠「没有冲突」：

```bash
cd backend && go build ./... && go test -tags=unit ./...
```

前端有冲突、依赖变化，或热区前端文件出现在上面的 diff 里时，再跑 `pnpm --dir frontend run typecheck`。单测失败先修再推；文本冲突按并集解完仍可能把静默拼接打爆。

## 四、给上游提 PR

不要从 `personal-main` 直接开 PR。上游是 squash 合并，同一改动两边 SHA 不同，再合回来会把历史打结。

```bash
git fetch upstream
git switch -c fix/xxx upstream/main
git cherry-pick <personal-main 上的提交>   # 或手工摘取改动
git push fork fix/xxx
```

PR 被上游合并后不用再把 PR 分支合回 `personal-main`。下次执行第三节时，改动会随 `upstream/main` 回来。

## 五、考古

```bash
git log archive/personal-dev-20260813 -S"关键字"
git blame archive/personal-dev-20260813 -- <文件>
```

## 六、压平手术

只在 `personal-main` 历史再次打结时使用。日常同步不要走这条。

```bash
git tag archive/personal-main-YYYYMMDD personal-main
git push origin archive/personal-main-YYYYMMDD
git switch -c tmp upstream/main
git merge --squash personal-main
git commit -m "chore: flatten personal patches onto upstream vX.Y.Z"
git diff tmp personal-main            # 必须为空
git branch -f personal-main tmp && git branch -d tmp
git push --force-with-lease origin personal-main
```

## 七、禁止事项

1. 从 `personal-main` 直接向上游开 PR。
2. 把 `fork` 上的 PR 分支 merge 回 `personal-main`。
3. 对 `personal-main` 做日常 rebase / force push。
4. 多机器不 pull 就开发，然后互相 merge。
5. 同步上游时从 `personal-dev` 往 `personal-main` 合代码。
6. 手工编辑 `backend/ent/` 生成代码。
7. 批量改账号时跨平台混合（不同平台的 model mapping 会互相污染）。
8. `merge -X theirs` / `-X ours`，或对冲突文件整份 `checkout --ours` / `--theirs`。
9. 只解 `<<<<<<<` 就 commit，不看热区自动合并、不跑第三节的合完验证。
