package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// MediaAsset tracks a file stored in the private media bucket.
type MediaAsset struct {
	ent.Schema
}

func (MediaAsset) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "media_assets"},
	}
}

func (MediaAsset) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("owner_user_id").
			Comment("拥有者用户 ID"),
		field.String("biz_type").
			MaxLen(32).
			NotEmpty().
			Comment("业务类型: invoice, ticket, avatar, site_logo, support_qr, announcement, payment_help, image_task"),
		field.String("biz_id").
			MaxLen(128).
			Default("").
			Comment("业务主键（发票/工单 ID 等）"),
		field.String("storage_key").
			MaxLen(512).
			NotEmpty().
			Comment("对象存储 key"),
		field.String("sha256").
			MaxLen(64).
			NotEmpty().
			Comment("内容 SHA-256 hex"),
		field.String("mime").
			MaxLen(128).
			NotEmpty().
			Comment("MIME 类型"),
		field.String("filename").
			MaxLen(255).
			Default("").
			Comment("原始文件名（已净化，用于 Content-Disposition）"),
		field.Int64("size").
			NonNegative().
			Comment("字节数"),
		field.String("visibility").
			MaxLen(16).
			Default(domain.MediaVisibilityPrivate).
			Comment("public 或 private"),
		field.String("status").
			MaxLen(16).
			Default(domain.MediaStatusReady).
			Comment("ready 或 deleted"),
		field.String("storage_profile_id").
			MaxLen(64).
			Default(domain.MediaStorageProfileBackup).
			Comment("绑定的 S3 profile，默认 backup"),
		field.String("public_base_url").
			SchemaType(map[string]string{dialect.Postgres: "text"}).
			Default("").
			Comment("可选 CDN 基址，仅 public 对象记录；客户端仍走网关"),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (MediaAsset) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("biz_type", "biz_id"),
		index.Fields("owner_user_id"),
		index.Fields("storage_key").Unique(),
	}
}
