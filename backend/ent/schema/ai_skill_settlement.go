package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AISkillSettlement struct {
	ent.Schema
}

func (AISkillSettlement) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_skill_settlements"},
	}
}

func (AISkillSettlement) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (AISkillSettlement) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("skill_id"),
		field.Int64("version_id"),
		field.Int64("run_id"),
		field.Int64("owner_user_id"),
		field.Int64("buyer_user_id"),
		field.String("status").MaxLen(32).Default(domain.AISkillSettlementStatusPending),
		field.String("billing_mode").MaxLen(32).Default(domain.AISkillBillingModePerRequest),
		field.Float("amount").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}),
		field.Float("quota_amount").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "numeric(20,8)"}),
		field.Time("quota_applied_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("balance_transferred_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.JSON("metadata", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("request_id").MaxLen(64).Optional().Nillable(),
		field.Int64("usage_log_id").Optional().Nillable(),
		field.Int64("api_key_id").Optional().Nillable(),
		field.Int64("group_id").Optional().Nillable(),
	}
}

func (AISkillSettlement) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("skill", AISkill.Type).
			Ref("settlements").
			Field("skill_id").
			Required().
			Unique(),
		edge.From("version", AISkillVersion.Type).
			Ref("settlements").
			Field("version_id").
			Required().
			Unique(),
		edge.From("run", AISkillRun.Type).
			Ref("settlements").
			Field("run_id").
			Required().
			Unique(),
		edge.From("owner_user", User.Type).
			Ref("ai_skill_settlements_owned").
			Field("owner_user_id").
			Required().
			Unique(),
		edge.From("buyer_user", User.Type).
			Ref("ai_skill_settlements_bought").
			Field("buyer_user_id").
			Required().
			Unique(),
	}
}

func (AISkillSettlement) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("skill_id"),
		index.Fields("version_id"),
		index.Fields("run_id").Unique(),
		index.Fields("owner_user_id"),
		index.Fields("buyer_user_id"),
		index.Fields("status"),
		index.Fields("request_id"),
		index.Fields("usage_log_id"),
		index.Fields("api_key_id"),
		index.Fields("group_id"),
		index.Fields("owner_user_id", "created_at"),
	}
}
