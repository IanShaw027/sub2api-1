package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Invoice is a user invoice application that may cover one or more completed orders.
type Invoice struct {
	ent.Schema
}

func (Invoice) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "invoices"},
	}
}

func (Invoice) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("user_email").
			MaxLen(255).
			Default(""),
		field.String("status").
			MaxLen(16).
			Default("APPLIED").
			Comment("APPLIED | ISSUED | CANCELLED"),
		field.Bool("unread_by_admin").
			Default(true),
		field.Float("invoice_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).
			Default(0),
		field.String("currency").
			MaxLen(8).
			Default(""),
		field.Int("order_count").
			Default(0),
		field.String("title").
			MaxLen(200).
			NotEmpty(),
		field.String("tax_number").
			MaxLen(64).
			Default(""),
		field.String("email").
			MaxLen(255).
			NotEmpty(),
		field.String("contact_name").
			MaxLen(100).
			Default(""),
		field.String("contact_phone").
			MaxLen(40).
			Default(""),
		field.String("request_note").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default(""),
		field.Int64("file_media_id").
			Optional().
			Nillable(),
		field.String("file_name").
			MaxLen(255).
			Default(""),
		field.String("file_mime_type").
			MaxLen(128).
			Default(""),
		field.Int64("file_size_bytes").
			Default(0),
		field.Time("applied_at").
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("cancelled_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("issued_at").
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

func (Invoice) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("orders", InvoiceOrder.Type),
	}
}

func (Invoice) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "status"),
		index.Fields("status", "unread_by_admin"),
		index.Fields("created_at"),
	}
}
