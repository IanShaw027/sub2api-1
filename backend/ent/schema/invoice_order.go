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
// 订单互斥占用约束双层保证：
//   - 应用层 Create 事务仍用 SELECT FOR UPDATE + 反查活跃发票做友好报错；
//   - DB 层用 is_active + partial unique index 兜底，防止旁路写入或未来锁路径漂移。
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
		field.Bool("is_active").Default(true),

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
		index.Fields("order_id").
			Unique().
			Annotations(entsql.IndexWhere("is_active = true")),
	}
}
