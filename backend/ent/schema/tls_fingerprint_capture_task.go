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

// TLSFingerprintCaptureTask stores a live TLS fingerprint capture run.
type TLSFingerprintCaptureTask struct {
	ent.Schema
}

func (TLSFingerprintCaptureTask) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "tls_fingerprint_capture_tasks"},
	}
}

func (TLSFingerprintCaptureTask) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (TLSFingerprintCaptureTask) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			MaxLen(120).
			Default("TLS fingerprint capture"),
		field.String("status").
			MaxLen(20).
			Default("running"),
		field.String("token").
			MaxLen(96).
			NotEmpty().
			Unique(),
		field.JSON("targets", map[string]int{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("counts", map[string]int{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("ua_keywords", []string{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("completed_at").
			Optional().
			Nillable(),
	}
}

func (TLSFingerprintCaptureTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "created_at"),
	}
}
