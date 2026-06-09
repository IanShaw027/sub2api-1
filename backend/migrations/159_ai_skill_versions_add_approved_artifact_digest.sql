-- 为 ai_skill_versions 增加 approved_artifact_digest 列。
--
-- 背景：script 类型技能的归档（zip）在每次 run 时由 source_content + 技能元数据
-- 现场确定性地重新生成，digest = sha256(归档字节)。此前 ReviewGate.ArtifactDigest
-- 直接用 run 时的 bundle.Digest 自比，导致审核 digest 绑定永真、形同虚设。
--
-- 该列在审批通过时写入“被审批版本”的归档 digest（审批与 run 使用同一套确定性
-- 算法）。run 时用它校验：缺失或不匹配则拒绝执行，确保只执行被审核过的产物。
--
-- 兼容 ent/migrate/schema.go 中的定义：
--   {Name: "approved_artifact_digest", Type: field.TypeString, Nullable: true, Size: 128}
--
-- 幂等：列存在则跳过。审批前为空（NULL）。
-- 注意：本迁移不做存量回填——已存在的已审批 script 技能首次 run 会因缺失 digest
-- 被拒绝，需重新审批以落库 digest（digest 算法在 Go 层，无法在纯 SQL 中复算）。

ALTER TABLE ai_skill_versions
    ADD COLUMN IF NOT EXISTS approved_artifact_digest VARCHAR(128);
