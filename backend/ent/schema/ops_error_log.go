// Package schema 定义 Ent ORM 的数据库 schema。
package schema

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// OpsErrorLog defines the schema for ops_error_logs.
//
// This table is created/maintained via SQL migrations (backend/migrations/*_ops_*.sql)
// and intentionally mirrors those columns for type-safe querying in Ent.
type OpsErrorLog struct {
	ent.Schema
}

func (OpsErrorLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ops_error_logs"},
	}
}

func (OpsErrorLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("request_id").MaxLen(64).Optional().Nillable(),
		field.Int64("user_id").Optional().Nillable(),
		field.Int64("api_key_id").Optional().Nillable(),
		field.Int64("account_id").Optional().Nillable(),
		field.Int64("group_id").Optional().Nillable(),
		field.String("client_ip").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "inet"}),

		field.String("error_phase").MaxLen(32).NotEmpty(),
		field.String("error_type").MaxLen(64).NotEmpty(),
		field.String("severity").MaxLen(4).NotEmpty(),
		field.Int("status_code").Optional().Nillable(),
		field.String("platform").MaxLen(32).Optional().Nillable(),
		field.String("model").MaxLen(100).Optional().Nillable(),
		field.String("request_path").MaxLen(256).Optional().Nillable(),
		field.Bool("stream").Default(false),

		field.String("error_message").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("error_body").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("provider_error_code").MaxLen(64).Optional().Nillable(),
		field.String("provider_error_type").MaxLen(64).Optional().Nillable(),
		field.Bool("is_retryable").Default(false),
		field.Bool("is_user_actionable").Default(false),
		field.Int("retry_count").Default(0),
		field.String("completion_status").MaxLen(16).Optional().Nillable(),
		field.Int("duration_ms").Optional().Nillable(),

		// Error classification (ops v2)
		field.String("error_source").MaxLen(50).Optional().Nillable(),
		field.String("error_owner").MaxLen(50).Optional().Nillable(),
		field.String("account_status").MaxLen(50).Optional().Nillable(),
		field.Int("upstream_status_code").Optional().Nillable(),
		field.String("upstream_error_message").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("upstream_error_detail").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("network_error_type").MaxLen(50).Optional().Nillable(),
		field.Int("retry_after_seconds").Optional().Nillable(),

		// Deep monitoring timings (stored as BIGINT ms in SQL).
		field.Int64("time_to_first_token_ms").Optional().Nillable(),
		field.Int64("auth_latency_ms").Optional().Nillable(),
		field.Int64("routing_latency_ms").Optional().Nillable(),
		field.Int64("upstream_latency_ms").Optional().Nillable(),
		field.Int64("response_latency_ms").Optional().Nillable(),

		// Request context.
		field.JSON("request_body", json.RawMessage{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("user_agent").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OpsErrorLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at"),
		index.Fields("error_phase"),
		index.Fields("platform"),
		index.Fields("severity"),
		index.Fields("status_code"),
		index.Fields("client_ip"),
		index.Fields("account_id"),
		index.Fields("group_id"),
		index.Fields("request_id"),
		// Common time-series queries
		index.Fields("platform", "created_at"),
		index.Fields("severity", "created_at"),
		index.Fields("account_id", "created_at"),
		index.Fields("group_id", "created_at"),
	}
}
