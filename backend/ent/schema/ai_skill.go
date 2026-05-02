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

type AISkill struct {
	ent.Schema
}

func (AISkill) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_skills"},
	}
}

func (AISkill) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (AISkill) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("skill_type").MaxLen(32).NotEmpty(),
		field.String("title").MaxLen(200).NotEmpty(),
		field.String("summary").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("description").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("category").MaxLen(100).Optional().Nillable(),
		field.JSON("tags", []string{}).
			Default(func() []string { return []string{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("visibility").MaxLen(32).Default(domain.AIVisibilityPrivate),
		field.String("source_visibility").MaxLen(32).Default(domain.AISkillSourceVisibilityPublic),
		field.String("billing_mode").MaxLen(32).Default(domain.AISkillBillingModePerRequest),
		field.Float("price").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}),
		field.Int64("current_version_id").Optional().Nillable(),
		field.Int64("published_version_id").Optional().Nillable(),
		field.Int64("latest_approved_version_id").Optional().Nillable(),
		field.Int("latest_version").Default(0),
		field.Int("like_count").Default(0),
		field.Int("run_count").Default(0),
		field.Float("total_income").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}),
		field.Int64("cover_asset_id").Optional().Nillable(),
		field.JSON("metadata", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("request_id").MaxLen(64).Optional().Nillable(),
		field.Int64("usage_log_id").Optional().Nillable(),
		field.Int64("api_key_id").Optional().Nillable(),
		field.Int64("group_id").Optional().Nillable(),
	}
}

func (AISkill) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("versions", AISkillVersion.Type),
		edge.To("runs", AISkillRun.Type),
		edge.To("reviews", AISkillReview.Type),
		edge.To("likes", AISkillLike.Type),
		edge.To("settlements", AISkillSettlement.Type),
	}
}

func (AISkill) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("skill_type"),
		index.Fields("visibility"),
		index.Fields("source_visibility"),
		index.Fields("billing_mode"),
		index.Fields("current_version_id"),
		index.Fields("published_version_id"),
		index.Fields("latest_approved_version_id"),
		index.Fields("cover_asset_id"),
		index.Fields("request_id"),
		index.Fields("usage_log_id"),
		index.Fields("api_key_id"),
		index.Fields("group_id"),
		index.Fields("user_id", "updated_at"),
		index.Fields("visibility", "published_version_id"),
		index.Fields("skill_type", "visibility"),
	}
}
