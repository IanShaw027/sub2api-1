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
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Required().
			Unique(),
		edge.From("user", User.Type).
			Ref("ai_skill_likes").
			Field("user_id").
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Required().
			Unique(),
	}
}

func (AISkillLike) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").
			StorageKey("idx_ai_skill_likes_user_id"),
		index.Fields("skill_id", "user_id").
			Unique().
			StorageKey("idx_ai_skill_likes_skill_user"),
	}
}
