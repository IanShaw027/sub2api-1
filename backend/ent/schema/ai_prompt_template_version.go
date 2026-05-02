package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AIPromptTemplateVersion struct {
	ent.Schema
}

func (AIPromptTemplateVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_prompt_template_versions"},
	}
}

func (AIPromptTemplateVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (AIPromptTemplateVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("template_id"),
		field.Int64("user_id"),
		field.Int("version"),
		field.String("title").MaxLen(200).NotEmpty(),
		field.String("content").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.String("model_hint").MaxLen(100).Optional().Nillable(),
		field.JSON("variables", []map[string]any{}).
			Default(func() []map[string]any { return []map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("change_note").
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

func (AIPromptTemplateVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("template", AIPromptTemplate.Type).
			Ref("versions").
			Field("template_id").
			Required().
			Unique(),
	}
}

func (AIPromptTemplateVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("template_id"),
		index.Fields("user_id"),
		index.Fields("request_id"),
		index.Fields("usage_log_id"),
		index.Fields("api_key_id"),
		index.Fields("group_id"),
		index.Fields("template_id", "version").Unique(),
	}
}
