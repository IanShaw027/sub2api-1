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

// OpsSystemMetric defines the schema for ops_system_metrics.
//
// This table is populated by OpsMetricsCollector and queried frequently by the ops dashboard.
// It is created/extended via SQL migrations and mirrored here for Ent discoverability and type safety.
type OpsSystemMetric struct {
	ent.Schema
}

func (OpsSystemMetric) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ops_system_metrics"},
	}
}

func (OpsSystemMetric) Fields() []ent.Field {
	return []ent.Field{
		field.Int("window_minutes").Default(1),

		// Requests
		field.Int64("request_count").Default(0),
		field.Int64("success_count").Default(0),
		field.Int64("error_count").Default(0),
		field.Float("qps").Optional().Nillable(),
		field.Float("tps").Optional().Nillable(),

		// Error breakdown
		field.Int64("error_4xx_count").Default(0),
		field.Int64("error_5xx_count").Default(0),
		field.Int64("error_timeout_count").Default(0),

		// Latency (ms)
		field.Float("latency_p50").Optional().Nillable(),
		field.Float("latency_p95").Optional().Nillable(),
		field.Float("latency_p99").Optional().Nillable(),
		field.Float("latency_avg").Optional().Nillable(),
		field.Float("latency_max").Optional().Nillable(),
		field.Float("upstream_latency_avg").Optional().Nillable(),

		// Rates
		field.Float("success_rate").Default(0),
		field.Float("error_rate").Default(0),

		// System stats
		field.Float("cpu_usage_percent").Optional().Nillable(),
		field.Int64("memory_used_mb").Optional().Nillable(),
		field.Int64("memory_total_mb").Optional().Nillable(),
		field.Float("memory_usage_percent").Optional().Nillable(),

		// DB connection pool stats
		field.Int("db_conn_active").Optional().Nillable(),
		field.Int("db_conn_idle").Optional().Nillable(),
		field.Int("db_conn_waiting").Optional().Nillable(),

		field.Int("goroutine_count").Optional().Nillable(),

		// Business stats
		field.Int64("token_consumed").Default(0),
		field.Float("token_rate").Optional().Nillable(),
		field.Int("active_subscriptions").Optional().Nillable(),

		// Alerts
		field.Int("active_alerts").Default(0),

		// Queue depth
		field.Int("concurrency_queue_depth").Optional().Nillable(),

		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (OpsSystemMetric) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("created_at"),
		index.Fields("window_minutes", "created_at"),
	}
}
