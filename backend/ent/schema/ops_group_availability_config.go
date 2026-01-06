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

// OpsGroupAvailabilityConfig defines the schema for ops_group_availability_configs.
type OpsGroupAvailabilityConfig struct {
	ent.Schema
}

func (OpsGroupAvailabilityConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ops_group_availability_configs"},
	}
}

func (OpsGroupAvailabilityConfig) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id"),

		field.Bool("enabled").Default(false),
		field.Int("min_available_accounts").Default(1),
		field.String("threshold_mode").Default("count").MaxLen(20),
		field.Float("min_available_percentage").Default(0),

		field.Bool("notify_email").Default(true),
		field.String("severity").Default("warning").MaxLen(20),
		field.Int("cooldown_minutes").Default(30),

		// The underlying SQL uses TIMESTAMP (without timezone).
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamp"}),
		field.Time("updated_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamp"}),
	}
}

func (OpsGroupAvailabilityConfig) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id").Unique(),
		index.Fields("enabled"),
	}
}
