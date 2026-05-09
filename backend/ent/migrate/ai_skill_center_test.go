package migrate

import (
	"testing"

	"entgo.io/ent/dialect/entsql"
	entschema "entgo.io/ent/dialect/sql/schema"
	"github.com/stretchr/testify/require"
)

func TestAISkillCenterForeignKeysAndPendingReviewIndex(t *testing.T) {
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillsTable, "ai_skills_users_ai_skills").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillVersionsTable, "ai_skill_versions_users_ai_skill_versions").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillRunsTable, "ai_skill_runs_users_ai_skill_runs").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillLikesTable, "ai_skill_likes_users_ai_skill_likes").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillReviewsTable, "ai_skill_reviews_users_ai_skill_reviews_submitted").OnDelete)
	require.Equal(t, entschema.SetNull, findForeignKeyBySymbol(t, AiSkillReviewsTable, "ai_skill_reviews_users_ai_skill_reviews_reviewed").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillSettlementsTable, "ai_skill_settlements_users_ai_skill_settlements_owned").OnDelete)
	require.Equal(t, entschema.Cascade, findForeignKeyBySymbol(t, AiSkillSettlementsTable, "ai_skill_settlements_users_ai_skill_settlements_bought").OnDelete)

	idx := findIndexByName(t, AiSkillReviewsTable, "aiskillreview_version_id")
	require.True(t, idx.Unique)
	require.Len(t, idx.Columns, 1)
	require.Equal(t, "version_id", idx.Columns[0].Name)
	require.NotNil(t, idx.Annotation)
	require.Equal(t, (&entsql.IndexAnnotation{Where: "status = 'pending'"}).Where, idx.Annotation.Where)
}
