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

// SupportTicketReplyTemplate is an admin-managed canned reply.
type SupportTicketReplyTemplate struct {
	ent.Schema
}

func (SupportTicketReplyTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "support_ticket_reply_templates"},
	}
}

func (SupportTicketReplyTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").
			MaxLen(120).
			NotEmpty(),
		field.String("content").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			NotEmpty(),
		field.Int("sort_order").
			Default(0),
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

func (SupportTicketReplyTemplate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sort_order"),
	}
}
