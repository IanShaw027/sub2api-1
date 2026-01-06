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

// OpsMetricsHourly defines the schema for ops_metrics_hourly.
type OpsMetricsHourly struct {
	ent.Schema
}

func (OpsMetricsHourly) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ops_metrics_hourly"},
	}
}

func (OpsMetricsHourly) Fields() []ent.Field {
	return []ent.Field{
		field.Time("bucket_start").
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("platform").MaxLen(50).Default(""),

		field.Int64("request_count").Default(0),
		field.Int64("success_count").Default(0),
		field.Int64("error_count").Default(0),
		field.Int64("error_4xx_count").Default(0),
		field.Int64("error_5xx_count").Default(0),
		field.Int64("timeout_count").Default(0),

		field.Float("avg_latency_ms").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,2)"}),
		field.Float("p99_latency_ms").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,2)"}),

		field.Float("error_rate").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(5,2)"}),

		field.Time("computed_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OpsMetricsHourly) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("bucket_start", "platform").Unique(),
		index.Fields("bucket_start"),
		index.Fields("platform", "bucket_start"),
	}
}
