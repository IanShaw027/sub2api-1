package schema

import (
	"encoding/json"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
)

// CreationSession stores a user's creation-center conversation or image workspace.
type CreationSession struct {
	ent.Schema
}

func (CreationSession) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "creation_sessions"},
	}
}

func (CreationSession) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (CreationSession) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("group_id"),
		field.String("title").
			MaxLen(200).
			Default(""),
		field.String("model").
			MaxLen(100).
			Default(""),
		field.String("mode").
			MaxLen(16).
			Default("chat").
			Comment("chat or image"),
		field.String("status").
			MaxLen(32).
			Default("active"),
		field.JSON("metadata", json.RawMessage{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
	}
}

func (CreationSession) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("messages", CreationMessage.Type),
		edge.To("image_jobs", CreationImageJob.Type),
	}
}

func (CreationSession) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("group_id"),
		index.Fields("user_id", "status"),
		index.Fields("user_id", "updated_at"),
	}
}
