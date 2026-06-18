package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AISkillRun struct {
	ent.Schema
}

func (AISkillRun) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_skill_runs"},
	}
}

func (AISkillRun) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (AISkillRun) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("skill_id"),
		field.Int64("version_id"),
		field.Int64("user_id"),
		field.String("run_mode").MaxLen(32).Default(domain.AISkillRunModeUse),
		field.String("status").MaxLen(32).Default(domain.AISkillRunStatusQueued),
		field.String("billing_mode").MaxLen(32).Default(domain.AISkillBillingModePerRequest),
		field.Float("price").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}),
		field.JSON("input_payload", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("output_payload", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("error_message").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("metadata", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("request_id").MaxLen(64).Optional().Nillable(),
		field.Int64("usage_log_id").Optional().Nillable(),
		field.Int64("api_key_id").Optional().Nillable(),
		field.Int64("group_id").Optional().Nillable(),
	}
}

func (AISkillRun) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("skill", AISkill.Type).
			Ref("runs").
			Field("skill_id").
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Required().
			Unique(),
		edge.From("version", AISkillVersion.Type).
			Ref("runs").
			Field("version_id").
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Required().
			Unique(),
		edge.From("user", User.Type).
			Ref("ai_skill_runs").
			Field("user_id").
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Required().
			Unique(),
		edge.To("settlements", AISkillSettlement.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (AISkillRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("skill_id").
			StorageKey("idx_ai_skill_runs_skill_id"),
		index.Fields("version_id").
			StorageKey("idx_ai_skill_runs_version_id"),
		index.Fields("status").
			StorageKey("idx_ai_skill_runs_status"),
		index.Fields("user_id", "created_at").
			StorageKey("idx_ai_skill_runs_user_created_at").
			Annotations(entsql.Desc()),
	}
}
