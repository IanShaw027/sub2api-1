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

// SupportTicketMessage is one conversation or system event on a ticket.
type SupportTicketMessage struct {
	ent.Schema
}

func (SupportTicketMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "support_ticket_messages"},
	}
}

func (SupportTicketMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("ticket_id"),
		field.String("sender_role").
			MaxLen(16).
			NotEmpty(),
		field.Int64("sender_user_id").
			Optional().
			Nillable(),
		field.String("sender_name_snapshot").
			MaxLen(120).
			Default(""),
		field.String("sender_avatar_snapshot").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.String("message_type").
			MaxLen(16).
			Default("message"),
		field.String("content").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.JSON("attachments", json.RawMessage{}).
			Default(json.RawMessage(`[]`)),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicketMessage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("ticket", SupportTicket.Type).
			Ref("messages").
			Field("ticket_id").
			Unique().
			Required().
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (SupportTicketMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ticket_id"),
		index.Fields("ticket_id", "created_at"),
	}
}
