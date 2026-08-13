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

// InvoiceOrder links one payment order to an invoice. An order may only have
// one active (is_active=true) invoice at a time; cancelled links are retained.
type InvoiceOrder struct {
	ent.Schema
}

func (InvoiceOrder) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "invoice_orders"},
	}
}

func (InvoiceOrder) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("invoice_id"),
		field.Int64("order_id"),
		field.Float("pay_amount_snapshot").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).
			Default(0),
		field.String("currency").
			MaxLen(8).
			Default(""),
		field.String("out_trade_no").
			MaxLen(64).
			Default(""),
		field.String("payment_type").
			MaxLen(30).
			Default(""),
		field.Bool("is_active").
			Default(true),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (InvoiceOrder) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("invoice", Invoice.Type).
			Ref("orders").
			Field("invoice_id").
			Unique().
			Required(),
	}
}

func (InvoiceOrder) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("invoice_id"),
		index.Fields("order_id"),
		index.Fields("order_id").
			Unique().
			StorageKey("invoice_orders_order_id_active_unique").
			Annotations(entsql.IndexWhere("is_active = TRUE")),
	}
}
