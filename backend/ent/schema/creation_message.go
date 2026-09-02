package schema

import (
	"encoding/json"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CreationMessage stores a single message within a creation session.
type CreationMessage struct {
	ent.Schema
}

func (CreationMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "creation_messages"},
	}
}

func (CreationMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("session_id"),
		field.String("role").
			MaxLen(32).
			NotEmpty(),
		field.JSON("content", json.RawMessage{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("model").
			MaxLen(100).
			Optional().
			Nillable(),
		field.Int("input_tokens").
			Optional().
			Nillable(),
		field.Int("output_tokens").
			Optional().
			Nillable(),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (CreationMessage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", CreationSession.Type).
			Ref("messages").
			Field("session_id").
			Unique().
			Required(),
	}
}

func (CreationMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("session_id"),
		index.Fields("session_id", "created_at"),
	}
}
