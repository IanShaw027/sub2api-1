package schema

import (
	"fmt"
	"regexp"

	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

var (
	accountDevicePlatforms = map[string]struct{}{
		"anthropic":   {},
		"openai":      {},
		"gemini":      {},
		"antigravity": {},
		"grok":        {},
		"kiro":        {},
	}
	accountDeviceClientFamilies = map[string]struct{}{
		"claude-code": {},
		"codex-cli":   {},
		"grok-cli":    {},
		"kiro-ide":    {},
		"gemini-cli":  {},
		"antigravity": {},
	}
	accountDeviceTransportFamilies = map[string]struct{}{
		"h1": {},
		"h2": {},
	}
	accountDeviceLearnedFrom = map[string]struct{}{
		"baseline":         {},
		"official_traffic": {},
		"baseline_floor":   {},
	}
	sessionNamespacePattern = regexp.MustCompile(`^[0-9a-f]{32,64}$`)
)

func validateAccountDeviceEnum(value, fieldName string, allowed map[string]struct{}) error {
	if _, ok := allowed[value]; ok {
		return nil
	}
	return fmt.Errorf("invalid %s %q", fieldName, value)
}

func validateSessionNamespace(value string) error {
	if sessionNamespacePattern.MatchString(value) {
		return nil
	}
	return fmt.Errorf("session_namespace must be 32-64 lowercase hex")
}

// AccountDeviceProfile is the durable outbound identity for one canonical account.
type AccountDeviceProfile struct {
	ent.Schema
}

func (AccountDeviceProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table: "account_device_profiles",
			Checks: map[string]string{
				"account_device_profiles_revision_check":         "revision >= 1",
				"account_device_profiles_schema_version_check":   "schema_version BETWEEN 1 AND 100",
				"account_device_profiles_platform_check":         "platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kiro')",
				"account_device_profiles_client_family_check":    "client_family IN ('claude-code', 'codex-cli', 'grok-cli', 'kiro-ide', 'gemini-cli', 'antigravity')",
				"account_device_profiles_os_family_check":        "length(os_family) BETWEEN 1 AND 32",
				"account_device_profiles_arch_check":             "length(arch) BETWEEN 1 AND 32",
				"account_device_profiles_runtime_check":          "length(runtime) BETWEEN 1 AND 32",
				"account_device_profiles_tls_profile_id_check":   "tls_profile_id IS NULL OR tls_profile_id > 0",
				"account_device_profiles_transport_family_check": "transport_family IN ('h1', 'h2')",
				"account_device_profiles_learned_from_check":     "learned_from IN ('baseline', 'official_traffic', 'baseline_floor')",
			},
		},
	}
}

func (AccountDeviceProfile) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (AccountDeviceProfile) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("account_id").
			Comment("Canonical account id; one profile row per account."),
		field.Int64("revision").
			Default(1).
			Comment("CAS token. schema_version and client_version are not CAS tokens."),
		field.Int("schema_version").
			Default(1).
			Range(1, 100),
		field.String("platform").
			MaxLen(32).
			NotEmpty().
			Validate(func(s string) error {
				return validateAccountDeviceEnum(s, "platform", accountDevicePlatforms)
			}),
		field.String("client_family").
			MaxLen(32).
			NotEmpty().
			Validate(func(s string) error {
				return validateAccountDeviceEnum(s, "client_family", accountDeviceClientFamilies)
			}),
		field.String("installation_id").
			MaxLen(64).
			NotEmpty(),
		field.String("device_id").
			MaxLen(64).
			NotEmpty(),
		field.String("client_id").
			MaxLen(64).
			Default(""),
		field.String("machine_id").
			MaxLen(64).
			NotEmpty(),
		field.String("gateway_account_uuid").
			MaxLen(64).
			NotEmpty().
			Comment("Gateway-generated UUID. Never copy extra.account_uuid."),
		field.String("session_namespace").
			MaxLen(64).
			NotEmpty().
			Immutable().
			Validate(validateSessionNamespace).
			Comment("32-64 lowercase hex. Immutable after insert."),
		field.String("os_family").
			MaxLen(32).
			NotEmpty(),
		field.String("arch").
			MaxLen(32).
			NotEmpty(),
		field.String("runtime").
			MaxLen(32).
			NotEmpty(),
		field.String("runtime_version").
			MaxLen(64).
			NotEmpty(),
		field.String("client_version").
			MaxLen(64).
			NotEmpty(),
		field.Int64("tls_profile_id").
			Optional().
			Nillable().
			Min(1).
			Comment("NULL or a positive FK to tls_fingerprint_profiles. -1 is illegal."),
		field.String("transport_family").
			MaxLen(8).
			NotEmpty().
			Validate(func(s string) error {
				return validateAccountDeviceEnum(s, "transport_family", accountDeviceTransportFamilies)
			}),
		field.JSON("profile_payload", map[string]any{}).
			Default(func() map[string]any { return map[string]any{} }).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("learned_from").
			MaxLen(32).
			NotEmpty().
			Validate(func(s string) error {
				return validateAccountDeviceEnum(s, "learned_from", accountDeviceLearnedFrom)
			}),
		field.Bool("learning_enabled").
			Default(false),
		field.Time("version_upgraded_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (AccountDeviceProfile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("account", Account.Type).
			Ref("device_profile").
			Field("account_id").
			Required().
			Unique().
			Annotations(entsql.OnDelete(entsql.Cascade)),
		edge.To("tls_profile", TLSFingerprintProfile.Type).
			Field("tls_profile_id").
			Unique().
			Annotations(entsql.OnDelete(entsql.SetNull)),
	}
}
