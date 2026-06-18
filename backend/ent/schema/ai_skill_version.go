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

type AISkillVersion struct {
	ent.Schema
}

func (AISkillVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "ai_skill_versions"},
	}
}

func (AISkillVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (AISkillVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("skill_id"),
		field.Int64("user_id"),
		field.Int("version"),
		field.String("review_status").MaxLen(32).Default(domain.AISkillVersionReviewStatusDraft),
		field.String("approved_artifact_digest").
			MaxLen(128).
			Optional().
			Nillable().
			Comment("sha256 digest of the script bundle captured at approval time; binds script execution to the reviewed artifact"),
		field.String("content_format").MaxLen(64).Optional().Nillable(),
		field.String("runtime").MaxLen(64).Optional().Nillable(),
		field.String("source_content").
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("config", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("input_schema", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("output_schema", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("change_note").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("submitted_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("reviewed_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("reviewer_user_id").Optional().Nillable(),
		field.String("review_note").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("metadata", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("request_id").MaxLen(64).Optional().Nillable(),
		field.Int64("usage_log_id").Optional().Nillable(),
		field.Int64("api_key_id").Optional().Nillable(),
		field.Int64("group_id").Optional().Nillable(),
	}
}

func (AISkillVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("skill", AISkill.Type).
			Ref("versions").
			Field("skill_id").
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Required().
			Unique(),
		edge.From("user", User.Type).
			Ref("ai_skill_versions").
			Field("user_id").
			Annotations(entsql.OnDelete(entsql.Cascade)).
			Required().
			Unique(),
		edge.To("runs", AISkillRun.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("reviews", AISkillReview.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("settlements", AISkillSettlement.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (AISkillVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("skill_id").
			StorageKey("idx_ai_skill_versions_skill_id").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("review_status").
			StorageKey("idx_ai_skill_versions_review_status").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("reviewed_at").
			StorageKey("idx_ai_skill_versions_reviewed_at").
			Annotations(entsql.Desc(), entsql.IndexWhere("deleted_at IS NULL")),
		index.Fields("skill_id", "version").
			Unique().
			StorageKey("idx_ai_skill_versions_skill_version"),
	}
}
