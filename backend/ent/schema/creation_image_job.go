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

// CreationImageJob tracks async image generation within the creation center.
type CreationImageJob struct {
	ent.Schema
}

func (CreationImageJob) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "creation_image_jobs"},
	}
}

func (CreationImageJob) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("session_id").
			Optional().
			Nillable(),
		field.Int64("user_id"),
		field.Int64("group_id"),
		field.String("status").
			MaxLen(32).
			Default("pending"),
		field.String("model").
			MaxLen(100).
			Default(""),
		field.Text("prompt").
			Default(""),
		field.Int64("media_asset_id").
			Optional().
			Nillable(),
		field.String("provider_task_id").
			MaxLen(128).
			Optional().
			Nillable(),
		field.Text("error").
			Optional().
			Nillable(),
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

func (CreationImageJob) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("session", CreationSession.Type).
			Ref("image_jobs").
			Field("session_id").
			Unique(),
	}
}

func (CreationImageJob) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id"),
		index.Fields("session_id"),
		index.Fields("user_id", "created_at"),
		index.Fields("provider_task_id"),
	}
}
