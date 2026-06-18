package migrate

import (
	"testing"

	entschema "entgo.io/ent/dialect/sql/schema"
	"github.com/stretchr/testify/require"
)

func TestAISkillCenterForeignKeysAndIndexesMatchMigrations(t *testing.T) {
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillsTable, "ai_skills_users_ai_skills").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillVersionsTable, "ai_skill_versions_ai_skills_versions").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillVersionsTable, "ai_skill_versions_users_ai_skill_versions").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillRunsTable, "ai_skill_runs_ai_skills_runs").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillRunsTable, "ai_skill_runs_ai_skill_versions_runs").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillRunsTable, "ai_skill_runs_users_ai_skill_runs").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillLikesTable, "ai_skill_likes_ai_skills_likes").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillLikesTable, "ai_skill_likes_users_ai_skill_likes").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillReviewsTable, "ai_skill_reviews_ai_skills_reviews").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillReviewsTable, "ai_skill_reviews_ai_skill_versions_reviews").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillReviewsTable, "ai_skill_reviews_users_ai_skill_reviews_submitted").OnDelete)
	require.Equal(t, entschema.SetNull, findForeignKeyBySymbol(t, AiSkillReviewsTable, "ai_skill_reviews_users_ai_skill_reviews_reviewed").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillSettlementsTable, "ai_skill_settlements_ai_skills_settlements").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillSettlementsTable, "ai_skill_settlements_ai_skill_versions_settlements").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillSettlementsTable, "ai_skill_settlements_ai_skill_runs_settlements").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillSettlementsTable, "ai_skill_settlements_users_ai_skill_settlements_owned").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillSettlementsTable, "ai_skill_settlements_users_ai_skill_settlements_bought").OnDelete)

	requireIndex(t, AiSkillVersionsTable, "idx_ai_skill_versions_skill_version", true, []string{"skill_id", "version"}, "", false)
	requireIndex(t, AiSkillVersionsTable, "idx_ai_skill_versions_skill_id", false, []string{"skill_id"}, "deleted_at IS NULL", false)
	requireIndex(t, AiSkillVersionsTable, "idx_ai_skill_versions_review_status", false, []string{"review_status"}, "deleted_at IS NULL", false)
	requireIndex(t, AiSkillVersionsTable, "idx_ai_skill_versions_reviewed_at", false, []string{"reviewed_at"}, "deleted_at IS NULL", true)

	requireIndex(t, AiSkillReviewsTable, "idx_ai_skill_reviews_pending_version", true, []string{"version_id"}, "status = 'pending'", false)
	requireIndex(t, AiSkillReviewsTable, "idx_ai_skill_reviews_skill_id", false, []string{"skill_id"}, "", false)
	requireIndex(t, AiSkillReviewsTable, "idx_ai_skill_reviews_reviewer_status", false, []string{"reviewer_user_id", "status"}, "reviewer_user_id IS NOT NULL", false)

	requireIndex(t, AiSkillLikesTable, "idx_ai_skill_likes_skill_user", true, []string{"skill_id", "user_id"}, "", false)
	requireIndex(t, AiSkillLikesTable, "idx_ai_skill_likes_user_id", false, []string{"user_id"}, "", false)

	requireIndex(t, AiSkillRunsTable, "idx_ai_skill_runs_skill_id", false, []string{"skill_id"}, "", false)
	requireIndex(t, AiSkillRunsTable, "idx_ai_skill_runs_version_id", false, []string{"version_id"}, "", false)
	requireIndex(t, AiSkillRunsTable, "idx_ai_skill_runs_user_created_at", false, []string{"user_id", "created_at"}, "", true)
	requireIndex(t, AiSkillRunsTable, "idx_ai_skill_runs_status", false, []string{"status"}, "", false)

	requireIndex(t, AiSkillSettlementsTable, "idx_ai_skill_settlements_run_id", true, []string{"run_id"}, "", false)
	requireIndex(t, AiSkillSettlementsTable, "idx_ai_skill_settlements_skill_id", false, []string{"skill_id"}, "", false)
	requireIndex(t, AiSkillSettlementsTable, "idx_ai_skill_settlements_owner_created_at", false, []string{"owner_user_id", "created_at"}, "", true)
	requireIndex(t, AiSkillSettlementsTable, "idx_ai_skill_settlements_status", false, []string{"status"}, "", false)
}

func requireIndex(t *testing.T, table *entschema.Table, name string, unique bool, columns []string, where string, desc bool) {
	t.Helper()

	idx := findIndexByName(t, table, name)
	require.Equal(t, unique, idx.Unique, "index %s unique mismatch", name)
	require.Len(t, idx.Columns, len(columns), "index %s column count mismatch", name)
	for i, column := range columns {
		require.Equal(t, column, idx.Columns[i].Name, "index %s column %d mismatch", name, i)
	}
	if where == "" && !desc {
		return
	}
	require.NotNil(t, idx.Annotation, "index %s should have annotation", name)
	require.Equal(t, where, idx.Annotation.Where, "index %s where mismatch", name)
	require.Equal(t, desc, idx.Annotation.Desc, "index %s desc mismatch", name)
}
