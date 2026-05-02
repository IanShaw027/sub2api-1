package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type AISkillLike struct {
	ent.Schema
}

func (AISkillLike) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_skill_likes"},
	}
}

func (AISkillLike) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (AISkillLike) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("skill_id"),
		field.Int64("user_id"),
		field.String("request_id").MaxLen(64).Optional().Nillable(),
		field.Int64("usage_log_id").Optional().Nillable(),
		field.Int64("api_key_id").Optional().Nillable(),
		field.Int64("group_id").Optional().Nillable(),
	}
}

func (AISkillLike) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("skill", AISkill.Type).
			Ref("likes").
			Field("skill_id").
			Required().
			Unique(),
	}
}

func (AISkillLike) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("skill_id"),
		index.Fields("user_id"),
		index.Fields("request_id"),
		index.Fields("usage_log_id"),
		index.Fields("api_key_id"),
		index.Fields("group_id"),
		index.Fields("skill_id", "user_id").Unique(),
	}
}
