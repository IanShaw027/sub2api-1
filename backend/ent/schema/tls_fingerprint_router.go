// Package schema 定义 Ent ORM 的数据库 schema。
package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TLSFingerprintRouter 定义 TLS 指纹路由规则的 schema。
//
// Router 根据入站 User-Agent 匹配规则，并选择现有 TLSFingerprintProfile。
// 规则结构保存在 rules JSONB 中，由 service/model 层做强类型校验。
type TLSFingerprintRouter struct {
	ent.Schema
}

// Annotations 返回 schema 的注解配置。
func (TLSFingerprintRouter) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "tls_fingerprint_routers"},
	}
}

// Mixin 返回该 schema 使用的混入组件。
func (TLSFingerprintRouter) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

// Fields 定义 TLS 指纹路由实体的所有字段。
func (TLSFingerprintRouter) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			MaxLen(100).
			NotEmpty().
			Unique(),

		field.Text("description").
			Optional().
			Nillable(),

		field.Bool("enabled").
			Default(true),

		field.JSON("rules", []map[string]any{}).
			Default(func() []map[string]any { return []map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
	}
}

// Indexes 定义数据库索引。
func (TLSFingerprintRouter) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled").
			StorageKey("idx_tls_fingerprint_routers_enabled"),
	}
}
