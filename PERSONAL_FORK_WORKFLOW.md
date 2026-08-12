# 个人 Fork 分支维护工作流

> 本文档是 personal-main 分支体系的指导文件。2026-08-13 起生效，用于替代旧 personal-dev 的混乱历史模式。
> 背景：旧 personal-dev 因多次上游合并 + PR 往返，积累了 1340 个交错提交（个人增量约 2894 个文件 / +75 万行），
> 历史图谱无法维护，于是重建分支体系。

## 一、分支模型

| 分支 / 引用 | 角色 | 规则 |
|---|---|---|
| `personal-main` | **新的个人主线**。上游历史 + 个人定制提交 | 日常开发直接提交在这里；定期 merge 上游；**永不 rebase / force push**（除非做"压平手术"） |
| `upstream/main` | 上游只读镜像（Wei-Shaw/sub2api） | 只 fetch，永不直接提交 |
| `personal-dev`（旧） | 遗留分支，作为迁移期的代码来源 | 冻结：不再新增开发；分批迁移完成后归档删除 |
| `archive/personal-dev-YYYYMMDD`（tag） | 旧历史完整存档 | 已推送 origin，考古用，永久保留 |
| `feat/*`、`fix/*`（fork 远程） | 给上游提 PR 的专用分支 | **一律从 upstream/main 拉出**，cherry-pick 需要的改动 |

远程约定：`origin` = 个人仓库（IanShaw027/sub2api），`fork` = PR 用仓库（IanShaw027/sub2api-1），`upstream` = 上游。

## 二、日常开发

直接在 `personal-main` 上正常提交，保持原子提交和清晰的 commit message。个人定制的开发细节
从此永久留在主线历史里，不会再被压掉。

```bash
git switch personal-main
# ...开发...
git commit -m "feat(xxx): ..."
git push origin personal-main
```

多台机器 / worktree 协作时：**先 pull --ff-only 再开发**，避免产生"merge 自己"的提交。
如果出现分叉，用 `git pull --rebase` 收敛（本地未推送的少量提交 rebase 是安全的）。

## 三、同步上游（重复流程，每次只有一步）

```bash
git fetch upstream
git switch personal-main
git merge upstream/main        # 解决冲突后 commit，产生一个 merge 节点
git push origin personal-main
```

- 每次同步只产生**一个** merge commit，不需要备份、不需要重写历史。
- 首次使用前开启冲突记忆，重复冲突自动套用上次的解法：
  `git config rerere.enabled true`
- 查看主线历史用 first-parent 视图（已配置别名）：
  `git plog` = `git log --first-parent --oneline`

## 四、给上游提 PR（防止历史再打结的关键纪律）

**绝对不要从 personal-main 直接开 PR。** 上游是 squash 合并，直接开 PR 会导致同一改动
两边 SHA 不同，再合并回来时历史打结——这就是旧 personal-dev 变乱的根源。

正确流程：

```bash
git fetch upstream
git switch -c fix/xxx upstream/main     # 从上游最新拉分支
git cherry-pick <personal-main上的提交>  # 或手工摘取改动
git push fork fix/xxx                    # 用 fork 仓库开 PR
```

PR 被上游合并后什么都不用做：下次 `git merge upstream/main` 时该改动自然回来，
与本地已有的同内容提交自动收敛（顶多出现一次平凡冲突）。

## 五、迁移期：旧 personal-dev 分批合入（一次性阶段）

旧 personal-dev 的个人增量按模块分批迁入 personal-main，清单见
`PERSONAL_MIGRATION_INVENTORY.md`。每批的操作模式：

```bash
git switch personal-main
# 方式 A：按路径整体迁移（适合独立模块）
git checkout personal-dev -- <该模块的路径...>
git commit -m "feat(migrate): <模块名> from personal-dev"

# 方式 B：按提交摘取（适合改动散落在共享文件里的功能）
git cherry-pick <相关提交...>
```

每批迁移后必须验证：`cd backend && go build ./... && go test -tags=unit ./...`，
前端相关批次跑 `pnpm run typecheck`。ent 生成代码不手工迁移，
迁移 `ent/schema/*.go` 后运行 `go generate ./ent` 重新生成。

全部批次完成后，用下面命令确认没有遗漏（输出应只剩确认放弃的内容）：

```bash
git diff personal-main personal-dev --stat
```

确认后删除旧分支（tag 存档仍在）：

```bash
git branch -D personal-dev
git push origin --delete personal-dev   # 谨慎：确认所有内容已迁移或放弃
```

## 六、考古：查旧历史的开发细节

旧历史完整保存在 `archive/personal-dev-20260813` tag 中：

```bash
git log archive/personal-dev-20260813 -S"关键字"     # 搜索历史改动
git blame archive/personal-dev-20260813 -- <文件>    # 查某行的原始提交
```

可选增强：用 graft 把旧历史缝到迁移提交上，让主线 blame 透明穿透到原始提交：

```bash
git replace --graft <迁移提交SHA> archive/personal-dev-20260813
# 仅本地生效；多机共享需 git push origin 'refs/replace/*'
```

## 七、压平手术（罕见，仅当历史再次变乱时）

只要守住第二、三、四节的纪律，历史只会"变长"不会"变乱"，本节应该永远用不上。
如果哪天又打结了（预期一年以上），重复一次性手术：

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

## 八、禁止事项清单

1. ❌ 从 personal-main 直接向上游开 PR（用第四节流程）。
2. ❌ 把上游 PR 分支 merge 回 personal-main（等上游合并后随 sync 自然回来）。
3. ❌ 对 personal-main 做日常 rebase / force push（只有第七节手术例外）。
4. ❌ 多机器不 pull 就开发，然后互相 merge。
5. ❌ 手工编辑 `backend/ent/` 生成代码或把它跟 schema 分开迁移。
6. ❌ 批量迁移时跨平台混合改动（见 CLAUDE.md：不同平台的 model mapping 会互相污染）。
