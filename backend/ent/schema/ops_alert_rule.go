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

// OpsAlertRule defines the schema for ops_alert_rules.
//
// This table is created/maintained via SQL migrations and mirrored here to make
// the ops module discoverable in Ent.
type OpsAlertRule struct {
	ent.Schema
}

func (OpsAlertRule) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ops_alert_rules"},
	}
}

func (OpsAlertRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(128).NotEmpty(),
		field.String("description").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Bool("enabled").Default(true),

		field.String("metric_type").MaxLen(64).NotEmpty(),
		field.String("operator").MaxLen(8).NotEmpty(),
		field.Float("threshold"),
		field.Int("window_minutes").Default(1),
		field.Int("sustained_minutes").Default(1),

		field.String("severity").MaxLen(20).Default("P1"),
		field.Bool("notify_email").Default(false),
		field.Int("cooldown_minutes").Default(10),

		// v2+ extensions (some fields may be unused by current runtime code but exist in DB schema).
		field.JSON("dimension_filters", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("notify_channels", []string{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("notify_config", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("created_by").Optional().Nillable().MaxLen(100),
		field.Time("last_triggered_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.String("alert_category").Optional().Nillable().MaxLen(50),
		field.JSON("filter_conditions", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Strings("aggregation_dimensions").Optional(),
		field.JSON("notification_channels", map[string]any{}).Optional().SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("notification_frequency").Optional().Nillable().MaxLen(50),
		field.String("notification_template").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "text"}),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OpsAlertRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled"),
		index.Fields("metric_type", "window_minutes"),
	}
}
