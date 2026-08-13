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

// SupportTicket is a user support request with a category-specific form.
type SupportTicket struct {
	ent.Schema
}

func (SupportTicket) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "support_tickets"},
	}
}

func (SupportTicket) Fields() []ent.Field {
	return []ent.Field{
		field.String("ticket_no").
			MaxLen(32).
			Unique().
			NotEmpty(),
		field.Int64("user_id"),
		field.String("category").
			MaxLen(32).
			NotEmpty(),
		field.String("title").
			MaxLen(200).
			NotEmpty(),
		field.String("status").
			MaxLen(32).
			Default("submitted"),
		field.JSON("current_form_payload", json.RawMessage{}).
			Default(json.RawMessage(`{}`)),
		field.Int("current_revision_no").
			Default(1),
		field.Time("latest_message_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("last_reply_role").
			MaxLen(16).
			Default("system"),
		field.Bool("unread_by_user").
			Default(false),
		field.Bool("unread_by_admin").
			Default(true),
		field.Time("submitted_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("closed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("withdrawn_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (SupportTicket) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("messages", SupportTicketMessage.Type),
		edge.To("revisions", SupportTicketRevision.Type),
	}
}

func (SupportTicket) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("status"),
		index.Fields("category"),
		index.Fields("unread_by_admin"),
		index.Fields("unread_by_user"),
		index.Fields("latest_message_at"),
		index.Fields("created_at"),
	}
}
