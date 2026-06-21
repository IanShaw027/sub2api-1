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

// TLSFingerprintCaptureSample stores one unique captured TLS fingerprint.
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
		field.String("platform").
			MaxLen(50).
			Default(""),
		field.Text("user_agent").
			Default(""),
		field.String("fingerprint_hash").
			MaxLen(64).
			NotEmpty(),
		field.JSON("profile", &model.TLSFingerprintProfile{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Text("raw_payload").
			Default(""),
	}
}

func (TLSFingerprintCaptureSample) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("task_id", "fingerprint_hash").Unique(),
		index.Fields("task_id", "platform"),
	}
}
