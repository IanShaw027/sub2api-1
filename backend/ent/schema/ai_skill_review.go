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

type AISkillReview struct {
	ent.Schema
}

func (AISkillReview) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_skill_reviews"},
	}
}

func (AISkillReview) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (AISkillReview) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("skill_id"),
		field.Int64("version_id"),
		field.Int64("submitter_user_id"),
		field.Int64("reviewer_user_id").Optional().Nillable(),
		field.String("status").MaxLen(32).Default(domain.AISkillReviewStatusPending),
		field.String("submit_note").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("review_note").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("snapshot", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("metadata", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("reviewed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("request_id").MaxLen(64).Optional().Nillable(),
		field.Int64("usage_log_id").Optional().Nillable(),
		field.Int64("api_key_id").Optional().Nillable(),
		field.Int64("group_id").Optional().Nillable(),
	}
}

func (AISkillReview) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("skill", AISkill.Type).
			Ref("reviews").
			Field("skill_id").
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Required().
			Unique(),
		edge.From("version", AISkillVersion.Type).
			Ref("reviews").
			Field("version_id").
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Required().
			Unique(),
		edge.From("submitter_user", User.Type).
			Ref("ai_skill_reviews_submitted").
			Field("submitter_user_id").
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Required().
			Unique(),
		edge.From("reviewer_user", User.Type).
			Ref("ai_skill_reviews_reviewed").
			Field("reviewer_user_id").
			Annotations(entsql.OnDelete(entsql.SetNull)).
			Unique(),
	}
}

func (AISkillReview) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("skill_id").
			StorageKey("idx_ai_skill_reviews_skill_id"),
		index.Fields("reviewer_user_id", "status").
			StorageKey("idx_ai_skill_reviews_reviewer_status").
			Annotations(entsql.IndexWhere("reviewer_user_id IS NOT NULL")),
		index.Fields("version_id").
			Unique().
			StorageKey("idx_ai_skill_reviews_pending_version").
			Annotations(entsql.IndexWhere("status = 'pending'")),
	}
}
