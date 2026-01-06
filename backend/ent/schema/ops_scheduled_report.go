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

// OpsScheduledReport defines the schema for ops_scheduled_reports.
type OpsScheduledReport struct {
	ent.Schema
}

func (OpsScheduledReport) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ops_scheduled_reports"},
	}
}

func (OpsScheduledReport) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty().MaxLen(255),
		field.String("description").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("schedule_cron").NotEmpty().MaxLen(100),
		field.String("report_type").NotEmpty().MaxLen(50),

		field.JSON("report_config", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("notification_channels", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),

		field.Bool("enabled").Default(true),
		field.Time("last_run_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("next_run_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OpsScheduledReport) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("next_run_at"),
	}
}
