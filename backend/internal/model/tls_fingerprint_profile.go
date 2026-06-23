// Package model 定义服务层使用的数据模型。
package model

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	tlsfpTransport "github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint/transport"
)

// TLSFingerprintProfile TLS 指纹配置模板
// 包含完整的 ClientHello 参数，用于模拟特定客户端的 TLS 握手特征
type TLSFingerprintProfile struct {
	ID                             int64             `json:"id"`
	Platform                       string            `json:"platform"`
	Transport                      string            `json:"transport"`
	Name                           string            `json:"name"`
	UserAgent                      string            `json:"user_agent"`
	Originator                     string            `json:"originator"`
	Description                    *string           `json:"description"`
	EnableGREASE                   bool              `json:"enable_grease"`
	CipherSuites                   []uint16          `json:"cipher_suites"`
	Curves                         []uint16          `json:"curves"`
	PointFormats                   []uint16          `json:"point_formats"`
	SignatureAlgorithms            []uint16          `json:"signature_algorithms"`
	SignatureAlgorithmsCert        []uint16          `json:"signature_algorithms_cert"`
	ALPNProtocols                  []string          `json:"alpn_protocols"`
	SupportedVersions              []uint16          `json:"supported_versions"`
	KeyShareGroups                 []uint16          `json:"key_share_groups"`
	PSKModes                       []uint16          `json:"psk_modes"`
	Extensions                     []uint16          `json:"extensions"`
	ExtensionPayloads              map[uint16][]byte `json:"extension_payloads"`
	CompressCertAlgos              []uint16          `json:"compress_cert_algos"`
	DelegatedCredentialsAlgorithms []uint16          `json:"delegated_credentials_algorithms"`
	ApplicationSettingsProtocols   []string          `json:"application_settings_protocols"`
	CreatedAt                      time.Time         `json:"created_at"`
	UpdatedAt                      time.Time         `json:"updated_at"`
}

// Validate 验证模板配置的有效性
func (p *TLSFingerprintProfile) Validate() error {
	if p.Name == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}
	if len(p.Platform) > 50 {
		return &ValidationError{Field: "platform", Message: "platform is too long"}
	}
	switch p.Transport {
	case "",
		string(tlsfpTransport.HTTP1),
		string(tlsfpTransport.H2),
		string(tlsfpTransport.WebSocketH1),
		string(tlsfpTransport.WebSocketH2):
	default:
		return &ValidationError{Field: "transport", Message: "transport must be empty, http1, h2, websocket-http1, or websocket-h2"}
	}
	if len(p.SupportedVersions) > 0 && !tlsFingerprintProfileHasExtension(p.Extensions, 43) {
		return &ValidationError{Field: "supported_versions", Message: "supported_versions requires extension 43"}
	}
	if len(p.KeyShareGroups) > 0 && !tlsFingerprintProfileHasExtension(p.Extensions, 51) {
		return &ValidationError{Field: "key_share_groups", Message: "key_share_groups requires extension 51"}
	}
	if len(p.PSKModes) > 0 && !tlsFingerprintProfileHasExtension(p.Extensions, 45) {
		return &ValidationError{Field: "psk_modes", Message: "psk_modes requires extension 45"}
	}
	if len(p.SignatureAlgorithmsCert) > 0 && !tlsFingerprintProfileHasExtension(p.Extensions, 50) {
		return &ValidationError{Field: "signature_algorithms_cert", Message: "signature_algorithms_cert requires extension 50"}
	}
	if len(p.CompressCertAlgos) > 0 && !tlsFingerprintProfileHasExtension(p.Extensions, 27) {
		return &ValidationError{Field: "compress_cert_algos", Message: "compress_cert_algos requires extension 27"}
	}
	if len(p.DelegatedCredentialsAlgorithms) > 0 && !tlsFingerprintProfileHasExtension(p.Extensions, 34) {
		return &ValidationError{Field: "delegated_credentials_algorithms", Message: "delegated_credentials_algorithms requires extension 34"}
	}
	if len(p.ApplicationSettingsProtocols) > 0 &&
		!tlsFingerprintProfileHasExtension(p.Extensions, 17513) &&
		!tlsFingerprintProfileHasExtension(p.Extensions, 17613) {
		return &ValidationError{Field: "application_settings_protocols", Message: "application_settings_protocols requires extension 17513 or 17613"}
	}
	return nil
}

func tlsFingerprintProfileHasExtension(extensions []uint16, target uint16) bool {
	for _, ext := range extensions {
		if ext == target {
			return true
		}
	}
	return false
}

// ToTLSProfile 将领域模型转换为运行时使用的 tlsfingerprint.Profile
// 空切片字段会在 dialer 中 fallback 到内置默认值
func (p *TLSFingerprintProfile) ToTLSProfile() *tlsfingerprint.Profile {
	return &tlsfingerprint.Profile{
		Name:                           p.Name,
		UserAgent:                      p.UserAgent,
		Originator:                     p.Originator,
		EnableGREASE:                   p.EnableGREASE,
		CipherSuites:                   p.CipherSuites,
		Curves:                         p.Curves,
		PointFormats:                   p.PointFormats,
		SignatureAlgorithms:            p.SignatureAlgorithms,
		SignatureAlgorithmsCert:        p.SignatureAlgorithmsCert,
		ALPNProtocols:                  p.ALPNProtocols,
		SupportedVersions:              p.SupportedVersions,
		KeyShareGroups:                 p.KeyShareGroups,
		PSKModes:                       p.PSKModes,
		Extensions:                     p.Extensions,
		ExtensionPayloads:              p.ExtensionPayloads,
		CompressCertAlgos:              p.CompressCertAlgos,
		DelegatedCredentialsAlgorithms: p.DelegatedCredentialsAlgorithms,
		ApplicationSettingsProtocols:   p.ApplicationSettingsProtocols,
	}
}
