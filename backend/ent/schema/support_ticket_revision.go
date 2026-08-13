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

// SupportTicketRevision stores each submitted form version.
type SupportTicketRevision struct {
	ent.Schema
}

func (SupportTicketRevision) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "support_ticket_revisions"},
	}
}

func (SupportTicketRevision) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("ticket_id"),
		field.Int("revision_no"),
		field.String("title").
			MaxLen(200).
			NotEmpty(),
		field.JSON("form_payload", json.RawMessage{}).
			Default(json.RawMessage(`{}`)),
		field.Int64("submitted_by").
			Optional().
			Nillable(),
		field.Time("submitted_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicketRevision) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("ticket", SupportTicket.Type).
			Ref("revisions").
			Field("ticket_id").
			Unique().
			Required().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (SupportTicketRevision) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ticket_id"),
		index.Fields("ticket_id", "revision_no").
			Unique(),
	}
}
