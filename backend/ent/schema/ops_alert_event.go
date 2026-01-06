// Package schema 定义 Ent ORM 的数据库 schema。
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

// OpsAlertEvent defines the schema for ops_alert_events.
type OpsAlertEvent struct {
	ent.Schema
}

func (OpsAlertEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ops_alert_events"},
	}
}

func (OpsAlertEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("rule_id"),

		field.String("severity").MaxLen(20).NotEmpty(),
		field.String("status").MaxLen(20).Default("firing"),
		field.String("title").Optional().Nillable().MaxLen(200),
		field.String("description").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Float("metric_value").Optional().Nillable(),
		field.Float("threshold_value").Optional().Nillable(),

		field.Time("fired_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("resolved_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.Bool("email_sent").Default(false),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OpsAlertEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("rule_id", "status"),
		index.Fields("fired_at"),
	}
}
