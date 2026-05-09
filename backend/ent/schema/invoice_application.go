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

type InvoiceApplication struct {
	ent.Schema
}

func (InvoiceApplication) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "invoice_applications"},
	}
}

func (InvoiceApplication) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("order_id"),
		field.Int64("user_id"),
		field.String("user_email").
			MaxLen(255).
			Default(""),
		field.String("order_out_trade_no").
			MaxLen(64).
			Default(""),
		field.String("payment_type").
			MaxLen(30).
			Default(""),
		field.String("provider_instance_id").
			MaxLen(64).
			Default(""),
		field.String("provider_key").
			MaxLen(30).
			Default(""),
		field.String("invoice_status").
			MaxLen(20).
			Default("APPLIED"),
		field.Float("invoice_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).
			Default(0),
		field.String("invoice_title").
			MaxLen(255).
			Default(""),
		field.String("tax_number").
			MaxLen(64).
			Default(""),
		field.String("email").
			MaxLen(255).
			Default(""),
		field.String("contact_name").
			MaxLen(100).
			Default(""),
		field.String("contact_phone").
			MaxLen(32).
			Default(""),
		field.String("request_note").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int64("file_media_id").
			Optional().
			Nillable(),
		field.String("file_name").
			MaxLen(255).
			Default(""),
		field.String("file_mime_type").
			MaxLen(255).
			Default(""),
		field.Int64("file_size_bytes").
			Default(0),
		field.Time("applied_at").
			Optional().
			Nillable().
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

func (InvoiceApplication) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("order_id").Unique(),
		index.Fields("user_id"),
		index.Fields("invoice_status"),
		index.Fields("created_at"),
	}
}
