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

type AIGenerationJob struct {
	ent.Schema
}

func (AIGenerationJob) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_generation_jobs"},
	}
}

func (AIGenerationJob) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (AIGenerationJob) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("session_id").Optional().Nillable(),
		field.Int64("prompt_template_id").Optional().Nillable(),
		field.String("status").MaxLen(32).Default(domain.AIGenerationJobStatusQueued),
		field.String("model").MaxLen(100).NotEmpty(),
		field.String("prompt").
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("negative_prompt").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("size").MaxLen(32).Optional().Nillable(),
		field.Int("image_count").Default(1),
		field.Int64("seed").Optional().Nillable(),
		field.String("error_message").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("parameters", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("request_id").MaxLen(64).Optional().Nillable(),
		field.Int64("usage_log_id").Optional().Nillable(),
		field.Int64("api_key_id").Optional().Nillable(),
		field.Int64("group_id").Optional().Nillable(),
	}
}

func (AIGenerationJob) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", AISession.Type).
			Ref("generation_jobs").
			Field("session_id").
			Unique(),
		edge.From("prompt_template", AIPromptTemplate.Type).
			Ref("generation_jobs").
			Field("prompt_template_id").
			Unique(),
		edge.To("assets", AIAsset.Type),
	}
}

func (AIGenerationJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("session_id"),
		index.Fields("prompt_template_id"),
		index.Fields("status"),
		index.Fields("request_id"),
		index.Fields("usage_log_id"),
		index.Fields("api_key_id"),
		index.Fields("group_id"),
		index.Fields("user_id", "created_at"),
	}
}
