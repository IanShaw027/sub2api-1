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

type AISessionMessage struct {
	ent.Schema
}

func (AISessionMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_session_messages"},
	}
}

func (AISessionMessage) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (AISessionMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("session_id"),
		field.Int64("user_id"),
		field.Int64("reply_to_message_id").Optional().Nillable(),
		field.String("role").MaxLen(32).Default(domain.AIMessageRoleUser),
		field.String("status").MaxLen(32).Default(domain.AIMessageStatusAccepted),
		field.String("content").
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("content_parts", []map[string]any{}).
			Default(func() []map[string]any { return []map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("model").MaxLen(100).Optional().Nillable(),
		field.String("provider").MaxLen(50).Optional().Nillable(),
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

func (AISessionMessage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", AISession.Type).
			Ref("messages").
			Field("session_id").
			Required().
			Unique(),
	}
}

func (AISessionMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id"),
		index.Fields("user_id"),
		index.Fields("role"),
		index.Fields("status"),
		index.Fields("request_id"),
		index.Fields("usage_log_id"),
		index.Fields("api_key_id"),
		index.Fields("group_id"),
		index.Fields("session_id", "created_at"),
	}
}
