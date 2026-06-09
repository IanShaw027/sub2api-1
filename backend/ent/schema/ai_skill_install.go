package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AISkillInstall mirrors the ai_skill_installs join table created by SQL
// migration 143. The runtime repository still uses raw SQL for toggle/check
// operations, but keeping the table in Ent closes the schema/migration drift.
type AISkillInstall struct {
	ent.Schema
}

func (AISkillInstall) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_skill_installs"},
	}
}

func (AISkillInstall) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (AISkillInstall) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("skill_id"),
		field.Int64("user_id"),
	}
}

func (AISkillInstall) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("skill", AISkill.Type).
			Ref("installs").
			Field("skill_id").
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Required().
			Unique(),
		edge.From("user", User.Type).
			Ref("ai_skill_installs").
			Field("user_id").
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Required().
			Unique(),
	}
}

func (AISkillInstall) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("skill_id").StorageKey("idx_ai_skill_installs_skill_id"),
		index.Fields("user_id").StorageKey("idx_ai_skill_installs_user_id"),
		index.Fields("skill_id", "user_id").Unique(),
	}
}
