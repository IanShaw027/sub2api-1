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

// OpsGroupAvailabilityEvent defines the schema for ops_group_availability_events.
type OpsGroupAvailabilityEvent struct {
	ent.Schema
}

func (OpsGroupAvailabilityEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ops_group_availability_events"},
	}
}

func (OpsGroupAvailabilityEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("config_id"),
		field.Int64("group_id"),

		field.String("status").MaxLen(20).Default("firing"),
		field.String("severity").MaxLen(20).NotEmpty(),

		field.String("title").NotEmpty().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("description").NotEmpty().SchemaType(map[string]string{dialect.Postgres: "text"}),

		field.Int("available_accounts"),
		field.Int("threshold_accounts"),
		field.Int("total_accounts"),

		field.Bool("email_sent").Default(false),

		// The underlying SQL uses TIMESTAMP (without timezone).
		field.Time("fired_at").SchemaType(map[string]string{dialect.Postgres: "timestamp"}),
		field.Time("resolved_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamp"}),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamp"}),
	}
}

func (OpsGroupAvailabilityEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("config_id"),
		index.Fields("group_id"),
		index.Fields("status"),
		index.Fields("fired_at"),
	}
}
