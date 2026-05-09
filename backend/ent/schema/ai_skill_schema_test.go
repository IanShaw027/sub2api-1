package schema

import (
	"testing"

	"entgo.io/ent/entc/load"
	"github.com/stretchr/testify/require"
)

func TestAISkillCenterSchemas_ModelUserEdges(t *testing.T) {
	spec, err := (&load.Config{Path: "."}).Load()
	require.NoError(t, err)

	schemas := map[string]*load.Schema{}
	for _, schema := range spec.Schemas {
		schemas[schema.Name] = schema
	}

	user := requireSchema(t, schemas, "User")
	requireSchemaEdge(t, user, "ai_skills")
	requireSchemaEdge(t, user, "ai_skill_versions")
	requireSchemaEdge(t, user, "ai_skill_runs")
	requireSchemaEdge(t, user, "ai_skill_likes")
	requireSchemaEdge(t, user, "ai_skill_reviews_submitted")
	requireSchemaEdge(t, user, "ai_skill_reviews_reviewed")
	requireSchemaEdge(t, user, "ai_skill_settlements_owned")
	requireSchemaEdge(t, user, "ai_skill_settlements_bought")

	skill := requireSchema(t, schemas, "AISkill")
	requireAISkillUserEdge(t, skill, "user", "user_id", true, true)

	version := requireSchema(t, schemas, "AISkillVersion")
	requireAISkillUserEdge(t, version, "user", "user_id", true, true)

	review := requireSchema(t, schemas, "AISkillReview")
	requireAISkillUserEdge(t, review, "submitter_user", "submitter_user_id", true, true)
	requireAISkillUserEdge(t, review, "reviewer_user", "reviewer_user_id", true, false)

	run := requireSchema(t, schemas, "AISkillRun")
	requireAISkillUserEdge(t, run, "user", "user_id", true, true)

	like := requireSchema(t, schemas, "AISkillLike")
	requireAISkillUserEdge(t, like, "user", "user_id", true, true)

	settlement := requireSchema(t, schemas, "AISkillSettlement")
	requireAISkillUserEdge(t, settlement, "owner_user", "owner_user_id", true, true)
	requireAISkillUserEdge(t, settlement, "buyer_user", "buyer_user_id", true, true)
}

func requireSchemaEdge(t *testing.T, schema *load.Schema, name string) *load.Edge {
	t.Helper()

	for _, edge := range schema.Edges {
		if edge.Name == name {
			return edge
		}
	}

	require.Failf(t, "missing edge", "schema %s should include edge %s", schema.Name, name)
	return nil
}

func requireAISkillUserEdge(t *testing.T, schema *load.Schema, edgeName, field string, unique, required bool) {
	t.Helper()

	edge := requireSchemaEdge(t, schema, edgeName)
	require.Equal(t, field, edge.Field, "schema %s edge %s should use field %s", schema.Name, edgeName, field)
	require.Equal(t, unique, edge.Unique, "schema %s edge %s unique mismatch", schema.Name, edgeName)
	require.Equal(t, required, edge.Required, "schema %s edge %s required mismatch", schema.Name, edgeName)
	require.Equal(t, "User", edge.Type, "schema %s edge %s should target User", schema.Name, edgeName)
}
