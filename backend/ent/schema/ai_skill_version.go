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

type AISkillVersion struct {
	ent.Schema
}

func (AISkillVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_skill_versions"},
	}
}

func (AISkillVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (AISkillVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("skill_id"),
		field.Int64("user_id"),
		field.Int("version"),
		field.String("review_status").MaxLen(32).Default(domain.AISkillVersionReviewStatusDraft),
		field.String("content_format").MaxLen(64).Optional().Nillable(),
		field.String("runtime").MaxLen(64).Optional().Nillable(),
		field.String("source_content").
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("config", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("input_schema", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("output_schema", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("change_note").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("submitted_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("reviewed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("reviewer_user_id").Optional().Nillable(),
		field.String("review_note").
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

func (AISkillVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("skill", AISkill.Type).
			Ref("versions").
			Field("skill_id").
			Required().
			Unique(),
		edge.To("runs", AISkillRun.Type),
		edge.To("reviews", AISkillReview.Type),
		edge.To("settlements", AISkillSettlement.Type),
	}
}

func (AISkillVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("skill_id"),
		index.Fields("user_id"),
		index.Fields("review_status"),
		index.Fields("reviewer_user_id"),
		index.Fields("submitted_at"),
		index.Fields("reviewed_at"),
		index.Fields("request_id"),
		index.Fields("usage_log_id"),
		index.Fields("api_key_id"),
		index.Fields("group_id"),
		index.Fields("skill_id", "version").Unique(),
	}
}
