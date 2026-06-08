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

// Invoice 表示一张合并发票，可关联 1..N 张已完成订单。
//
// 状态机：APPLIED → ISSUED（终态）/ CANCELLED（终态，订单释放）。
// 不在订单表上维护发票状态——订单的发票链路通过 invoice_orders + invoices.status 计算。
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
		field.String("user_email").MaxLen(255).Default(""),

		field.String("status").MaxLen(20).Default("APPLIED"),
		field.Float("invoice_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).
			Default(0),
		field.Int("order_count").Default(0),

		field.String("title").MaxLen(255).Default(""),
		field.String("tax_number").MaxLen(64).Default(""),
		field.String("email").MaxLen(255).Default(""),
		field.String("contact_name").MaxLen(100).Default(""),
		field.String("contact_phone").MaxLen(32).Default(""),
		field.String("request_note").
			Optional().Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),

		field.Int64("file_media_id").Optional().Nillable(),
		field.String("file_name").MaxLen(255).Default(""),
		field.String("file_mime_type").MaxLen(255).Default(""),
		field.Int64("file_size_bytes").Default(0),

		field.Time("applied_at").Optional().Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("cancelled_at").Optional().Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("issued_at").Optional().Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		field.Time("created_at").Immutable().Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).
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
		index.Fields("status", "created_at"),
		index.Fields("created_at"),
	}
}
