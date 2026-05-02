package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AIAuditLog struct {
	ent.Schema
}

func (AIAuditLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_audit_logs"},
	}
}

func (AIAuditLog) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("operator_user_id").Optional().Nillable(),
		field.Int64("owner_user_id").Optional().Nillable(),
		field.String("entity_type").MaxLen(64).NotEmpty(),
		field.Int64("entity_id").Optional().Nillable(),
		field.String("action").MaxLen(64).NotEmpty(),
		field.String("reason").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("before_state", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("after_state", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("request_id").MaxLen(64).Optional().Nillable(),
		field.Int64("usage_log_id").Optional().Nillable(),
		field.Int64("api_key_id").Optional().Nillable(),
		field.Int64("group_id").Optional().Nillable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (AIAuditLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("operator_user_id"),
		index.Fields("owner_user_id"),
		index.Fields("entity_type"),
		index.Fields("entity_id"),
		index.Fields("action"),
		index.Fields("request_id"),
		index.Fields("usage_log_id"),
		index.Fields("api_key_id"),
		index.Fields("group_id"),
		index.Fields("created_at"),
		index.Fields("entity_type", "entity_id", "created_at"),
	}
}
