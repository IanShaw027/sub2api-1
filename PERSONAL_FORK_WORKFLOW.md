# 个人 Fork 分支维护工作流

`personal-main` 的日常维护说明。同步上游、向 Wei-Shaw/sub2api 提 PR、以及历史再次打结时的压平，都只看这份文档。

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

```bash
git fetch upstream
git switch personal-main
git merge upstream/main
# 解决冲突后 commit
git push origin personal-main
```

建议：

```bash
git config rerere.enabled true          # 重复冲突套用上次解法
git config alias.plog "log --first-parent --oneline"
```

合并后至少验证：

```bash
cd backend && go build ./... && go test -tags=unit ./...
```

前端有冲突或依赖变化时再跑 `pnpm --dir frontend run typecheck`。

不要用 rebase 把整条 `personal-main` 接到上游上，也不要先切到 `upstream/main` 再 squash 个人分支——那是第七节的压平手术，不是日常同步。

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
