package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TLSFingerprintCaptureSession stores one observed capture session per task.
type TLSFingerprintCaptureSession struct {
	ent.Schema
}

func (TLSFingerprintCaptureSession) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "tls_fingerprint_capture_sessions"},
	}
}

func (TLSFingerprintCaptureSession) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (TLSFingerprintCaptureSession) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("task_id"),
		field.String("session_id").
			MaxLen(128).
			NotEmpty(),
		field.String("client_ip").
			MaxLen(128).
			Default(""),
		field.String("platform").
			MaxLen(50).
			Default(""),
		field.Text("user_agent").
			Default(""),
		field.String("originator").
			MaxLen(50).
			Default(""),
		field.String("alpn_negotiated").
			MaxLen(32).
			Default(""),
		field.Bytes("raw_client_hello").
			Optional().
			Nillable(),
		field.JSON("observed_client_hello", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("replay_profile", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("derived_fingerprint", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("session_status").
			MaxLen(64).
			Default("observed"),
		field.Text("error_summary").
			Default(""),
		field.Time("opened_at"),
		field.Time("closed_at").
			Optional().
			Nillable(),
	}
}

func (TLSFingerprintCaptureSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("task_id", "session_id").Unique(),
		index.Fields("task_id", "platform"),
	}
}
