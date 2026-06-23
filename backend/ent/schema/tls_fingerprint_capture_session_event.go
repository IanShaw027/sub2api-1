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

// TLSFingerprintCaptureSessionEvent stores one observation event for a capture session.
type TLSFingerprintCaptureSessionEvent struct {
	ent.Schema
}

func (TLSFingerprintCaptureSessionEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "tls_fingerprint_capture_session_events"},
	}
}

func (TLSFingerprintCaptureSessionEvent) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (TLSFingerprintCaptureSessionEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("task_id"),
		field.Int64("session_ref").
			Optional().
			Nillable(),
		field.String("session_id").
			MaxLen(128).
			Default(""),
		field.String("event_id").
			MaxLen(128).
			Default(""),
		field.String("platform").
			MaxLen(50).
			Default(""),
		field.String("transport").
			MaxLen(32).
			Default(""),
		field.String("event_type").
			MaxLen(64).
			NotEmpty(),
		field.Int("request_sequence").
			Default(0),
		field.String("stream_id").
			MaxLen(128).
			Default(""),
		field.Text("request_path").
			Default(""),
		field.String("http_method").
			MaxLen(16).
			Default(""),
		field.Bool("is_websocket").
			Default(false),
		field.String("websocket_protocol").
			MaxLen(64).
			Default(""),
		field.String("client_type").
			MaxLen(64).
			Default(""),
		field.String("model").
			MaxLen(255).
			Default(""),
		field.String("request_kind").
			MaxLen(64).
			Default(""),
		field.Bool("streaming").
			Default(false),
		field.String("response_mode").
			MaxLen(64).
			Default(""),
		field.Text("user_agent").
			Default(""),
		field.String("originator").
			MaxLen(50).
			Default(""),
		field.JSON("stainless_metadata", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("headers_snapshot", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Text("body_summary").
			Default(""),
		field.String("event_status").
			MaxLen(64).
			Default("observed"),
		field.Text("event_error").
			Default(""),
		field.Bool("replayable").
			Default(true),
		field.Int64("sample_id").
			Optional().
			Nillable(),
		field.String("replay_hash").
			MaxLen(64).
			Default(""),
	}
}

func (TLSFingerprintCaptureSessionEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("task_id", "session_ref", "created_at"),
		index.Fields("task_id", "session_id", "created_at"),
		index.Fields("task_id", "sample_id"),
	}
}
