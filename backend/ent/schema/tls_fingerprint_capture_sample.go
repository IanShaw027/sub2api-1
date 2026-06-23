package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/model"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TLSFingerprintCaptureSample stores one unique replayable capture request observation.
type TLSFingerprintCaptureSample struct {
	ent.Schema
}

func (TLSFingerprintCaptureSample) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "tls_fingerprint_capture_samples"},
	}
}

func (TLSFingerprintCaptureSample) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (TLSFingerprintCaptureSample) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("task_id"),
		field.String("session_id").
			MaxLen(128).
			Default(""),
		field.String("platform").
			MaxLen(50).
			Default(""),
		field.String("transport").
			MaxLen(32).
			Default(""),
		field.Text("user_agent").
			Default(""),
		field.String("originator").
			MaxLen(50).
			Default(""),
		field.String("fingerprint_hash").
			MaxLen(128).
			NotEmpty(),
		field.String("replay_hash").
			MaxLen(64).
			Default(""),
		field.Text("ja3_raw").
			Default(""),
		field.String("ja3_hash").
			MaxLen(64).
			Default(""),
		field.String("ja4").
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
		field.Text("http2_fingerprint").
			Default(""),
		field.JSON("stainless_metadata", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("replay_profile", &model.TLSFingerprintProfile{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Text("raw_payload").
			Default(""),
		field.Bytes("raw_client_hello").
			Optional().
			Nillable(),
		field.Time("captured_at"),
	}
}

func (TLSFingerprintCaptureSample) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("task_id", "fingerprint_hash").Unique(),
		index.Fields("task_id", "replay_hash", "transport", "platform"),
		index.Fields("task_id", "session_id", "transport"),
		index.Fields("task_id", "platform"),
	}
}
