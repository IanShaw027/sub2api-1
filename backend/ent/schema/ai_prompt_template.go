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

type AIPromptTemplate struct {
	ent.Schema
}

func (AIPromptTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_prompt_templates"},
	}
}

func (AIPromptTemplate) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (AIPromptTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("title").MaxLen(200).NotEmpty(),
		field.String("description").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("category").MaxLen(100).Optional().Nillable(),
		field.JSON("tags", []string{}).
			Default(func() []string { return []string{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("visibility").MaxLen(32).Default(domain.AIVisibilityPrivate),
		field.String("moderation_state").MaxLen(32).Default(domain.AIModerationStateNormal),
		field.Int("current_version").Default(1),
		field.String("content").
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("model_hint").MaxLen(100).Optional().Nillable(),
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

func (AIPromptTemplate) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("versions", AIPromptTemplateVersion.Type),
		edge.To("generation_jobs", AIGenerationJob.Type),
		edge.To("assets", AIAsset.Type),
	}
}

func (AIPromptTemplate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("visibility"),
		index.Fields("moderation_state"),
		index.Fields("request_id"),
		index.Fields("usage_log_id"),
		index.Fields("api_key_id"),
		index.Fields("group_id"),
		index.Fields("user_id", "updated_at"),
		index.Fields("visibility", "moderation_state"),
	}
}
