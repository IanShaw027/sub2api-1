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

type AIAsset struct {
	ent.Schema
}

func (AIAsset) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_assets"},
	}
}

func (AIAsset) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (AIAsset) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("generation_job_id").Optional().Nillable(),
		field.Int64("session_id").Optional().Nillable(),
		field.Int64("prompt_template_id").Optional().Nillable(),
		field.String("asset_type").MaxLen(32).Default(domain.AIAssetTypeImage),
		field.String("status").MaxLen(32).Default(domain.AIAssetStatusPending),
		field.String("visibility").MaxLen(32).Default(domain.AIVisibilityPrivate),
		field.String("moderation_state").MaxLen(32).Default(domain.AIModerationStateNormal),
		field.String("storage_kind").MaxLen(32).Optional().Nillable(),
		field.String("storage_path").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("source_url").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("mime_type").MaxLen(100).Optional().Nillable(),
		field.Int("width").Optional().Nillable(),
		field.Int("height").Optional().Nillable(),
		field.Int64("byte_size").Optional().Nillable(),
		field.String("checksum").MaxLen(128).Optional().Nillable(),
		field.JSON("metadata", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("request_id").MaxLen(64).Optional().Nillable(),
		field.Int64("usage_log_id").Optional().Nillable(),
		field.Int64("api_key_id").Optional().Nillable(),
		field.Int64("group_id").Optional().Nillable(),
	}
}

func (AIAsset) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("generation_job", AIGenerationJob.Type).
			Ref("assets").
			Field("generation_job_id").
			Unique(),
		edge.From("session", AISession.Type).
			Ref("assets").
			Field("session_id").
			Unique(),
		edge.From("prompt_template", AIPromptTemplate.Type).
			Ref("assets").
			Field("prompt_template_id").
			Unique(),
	}
}

func (AIAsset) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("generation_job_id"),
		index.Fields("session_id"),
		index.Fields("prompt_template_id"),
		index.Fields("status"),
		index.Fields("visibility"),
		index.Fields("moderation_state"),
		index.Fields("request_id"),
		index.Fields("usage_log_id"),
		index.Fields("api_key_id"),
		index.Fields("group_id"),
		index.Fields("user_id", "created_at"),
	}
}
