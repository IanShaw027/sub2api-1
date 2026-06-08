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

// InvoiceOrder 是发票-订单的关联记录，pay_amount_snapshot 在写入时冻结，
// 保证发票金额不随订单状态后续变化而漂移（会计正确性）。
//
// 订单互斥占用约束在应用层 Create 事务里保证（SELECT FOR UPDATE + 反查活跃发票），
// 不引入 is_active 反范式列与 partial unique index。
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

		field.String("out_trade_no").MaxLen(64).Default(""),
		field.String("payment_type").MaxLen(30).Default(""),

		field.Time("created_at").Immutable().Default(time.Now).
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
	}
}
