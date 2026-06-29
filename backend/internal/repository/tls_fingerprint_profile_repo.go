package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/tlsfingerprintprofile"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type tlsFingerprintProfileRepository struct {
	client *ent.Client
}

// NewTLSFingerprintProfileRepository 创建 TLS 指纹模板仓库
func NewTLSFingerprintProfileRepository(client *ent.Client) service.TLSFingerprintProfileRepository {
	return &tlsFingerprintProfileRepository{client: client}
}

// List 获取所有模板
func (r *tlsFingerprintProfileRepository) List(ctx context.Context) ([]*model.TLSFingerprintProfile, error) {
	profiles, err := r.client.TLSFingerprintProfile.Query().
		Order(ent.Asc(tlsfingerprintprofile.FieldName)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*model.TLSFingerprintProfile, len(profiles))
	for i, p := range profiles {
		result[i] = r.toModel(p)
	}
	return result, nil
}

// GetByID 根据 ID 获取模板
func (r *tlsFingerprintProfileRepository) GetByID(ctx context.Context, id int64) (*model.TLSFingerprintProfile, error) {
	p, err := r.client.TLSFingerprintProfile.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return r.toModel(p), nil
}

// Create 创建模板
func (r *tlsFingerprintProfileRepository) Create(ctx context.Context, p *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	builder := r.client.TLSFingerprintProfile.Create().
		SetName(p.Name).
		SetPlatform(p.Platform).
		SetTransport(p.Transport).
		SetOs(p.OS).
		SetClientType(p.ClientType).
		SetUserAgent(p.UserAgent).
		SetOriginator(p.Originator).
		SetEnableGrease(p.EnableGREASE)

	if p.Description != nil {
		builder.SetDescription(*p.Description)
	}
	if len(p.CipherSuites) > 0 {
		builder.SetCipherSuites(p.CipherSuites)
	}
	if len(p.Curves) > 0 {
		builder.SetCurves(p.Curves)
	}
	if len(p.PointFormats) > 0 {
		builder.SetPointFormats(p.PointFormats)
	}
	if len(p.SignatureAlgorithms) > 0 {
		builder.SetSignatureAlgorithms(p.SignatureAlgorithms)
	}
	if len(p.SignatureAlgorithmsCert) > 0 {
		builder.SetSignatureAlgorithmsCert(p.SignatureAlgorithmsCert)
	}
	if len(p.ALPNProtocols) > 0 {
		builder.SetAlpnProtocols(p.ALPNProtocols)
	}
	if len(p.SupportedVersions) > 0 {
		builder.SetSupportedVersions(p.SupportedVersions)
	}
	if len(p.KeyShareGroups) > 0 {
		builder.SetKeyShareGroups(p.KeyShareGroups)
	}
	if len(p.PSKModes) > 0 {
		builder.SetPskModes(p.PSKModes)
	}
	if len(p.Extensions) > 0 {
		builder.SetExtensions(p.Extensions)
	}
	if len(p.ExtensionPayloads) > 0 {
		builder.SetExtensionPayloads(p.ExtensionPayloads)
	}
	if len(p.CompressCertAlgos) > 0 {
		builder.SetCompressCertAlgos(p.CompressCertAlgos)
	}
	if len(p.DelegatedCredentialsAlgorithms) > 0 {
		builder.SetDelegatedCredentialsAlgorithms(p.DelegatedCredentialsAlgorithms)
	}
	if len(p.ApplicationSettingsProtocols) > 0 {
		builder.SetApplicationSettingsProtocols(p.ApplicationSettingsProtocols)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModel(created), nil
}

// Update 更新模板
func (r *tlsFingerprintProfileRepository) Update(ctx context.Context, p *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	builder := r.client.TLSFingerprintProfile.UpdateOneID(p.ID).
		SetName(p.Name).
		SetPlatform(p.Platform).
		SetTransport(p.Transport).
		SetOs(p.OS).
		SetClientType(p.ClientType).
		SetUserAgent(p.UserAgent).
		SetOriginator(p.Originator).
		SetEnableGrease(p.EnableGREASE)

	if p.Description != nil {
		builder.SetDescription(*p.Description)
	} else {
		builder.ClearDescription()
	}

	if len(p.CipherSuites) > 0 {
		builder.SetCipherSuites(p.CipherSuites)
	} else {
		builder.SetCipherSuites([]uint16{})
	}
	if len(p.Curves) > 0 {
		builder.SetCurves(p.Curves)
	} else {
		builder.SetCurves([]uint16{})
	}
	if len(p.PointFormats) > 0 {
		builder.SetPointFormats(p.PointFormats)
	} else {
		builder.SetPointFormats([]uint16{})
	}
	if len(p.SignatureAlgorithms) > 0 {
		builder.SetSignatureAlgorithms(p.SignatureAlgorithms)
	} else {
		builder.SetSignatureAlgorithms([]uint16{})
	}
	if len(p.SignatureAlgorithmsCert) > 0 {
		builder.SetSignatureAlgorithmsCert(p.SignatureAlgorithmsCert)
	} else {
		builder.SetSignatureAlgorithmsCert([]uint16{})
	}
	if len(p.ALPNProtocols) > 0 {
		builder.SetAlpnProtocols(p.ALPNProtocols)
	} else {
		builder.SetAlpnProtocols([]string{})
	}
	if len(p.SupportedVersions) > 0 {
		builder.SetSupportedVersions(p.SupportedVersions)
	} else {
		builder.SetSupportedVersions([]uint16{})
	}
	if len(p.KeyShareGroups) > 0 {
		builder.SetKeyShareGroups(p.KeyShareGroups)
	} else {
		builder.SetKeyShareGroups([]uint16{})
	}
	if len(p.PSKModes) > 0 {
		builder.SetPskModes(p.PSKModes)
	} else {
		builder.SetPskModes([]uint16{})
	}
	if len(p.Extensions) > 0 {
		builder.SetExtensions(p.Extensions)
	} else {
		builder.SetExtensions([]uint16{})
	}
	if len(p.ExtensionPayloads) > 0 {
		builder.SetExtensionPayloads(p.ExtensionPayloads)
	} else {
		builder.SetExtensionPayloads(map[uint16][]byte{})
	}
	if len(p.CompressCertAlgos) > 0 {
		builder.SetCompressCertAlgos(p.CompressCertAlgos)
	} else {
		builder.SetCompressCertAlgos([]uint16{})
	}
	if len(p.DelegatedCredentialsAlgorithms) > 0 {
		builder.SetDelegatedCredentialsAlgorithms(p.DelegatedCredentialsAlgorithms)
	} else {
		builder.SetDelegatedCredentialsAlgorithms([]uint16{})
	}
	if len(p.ApplicationSettingsProtocols) > 0 {
		builder.SetApplicationSettingsProtocols(p.ApplicationSettingsProtocols)
	} else {
		builder.SetApplicationSettingsProtocols([]string{})
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModel(updated), nil
}

// Delete 删除模板
func (r *tlsFingerprintProfileRepository) Delete(ctx context.Context, id int64) error {
	return r.client.TLSFingerprintProfile.DeleteOneID(id).Exec(ctx)
}

// toModel 将 Ent 实体转换为服务模型
func (r *tlsFingerprintProfileRepository) toModel(e *ent.TLSFingerprintProfile) *model.TLSFingerprintProfile {
	p := &model.TLSFingerprintProfile{
		ID:                             e.ID,
		Platform:                       e.Platform,
		Transport:                      e.Transport,
		OS:                             e.Os,
		ClientType:                     e.ClientType,
		Name:                           e.Name,
		UserAgent:                      e.UserAgent,
		Originator:                     e.Originator,
		Description:                    e.Description,
		EnableGREASE:                   e.EnableGrease,
		CipherSuites:                   e.CipherSuites,
		Curves:                         e.Curves,
		PointFormats:                   e.PointFormats,
		SignatureAlgorithms:            e.SignatureAlgorithms,
		SignatureAlgorithmsCert:        e.SignatureAlgorithmsCert,
		ALPNProtocols:                  e.AlpnProtocols,
		SupportedVersions:              e.SupportedVersions,
		KeyShareGroups:                 e.KeyShareGroups,
		PSKModes:                       e.PskModes,
		Extensions:                     e.Extensions,
		ExtensionPayloads:              e.ExtensionPayloads,
		CompressCertAlgos:              e.CompressCertAlgos,
		DelegatedCredentialsAlgorithms: e.DelegatedCredentialsAlgorithms,
		ApplicationSettingsProtocols:   e.ApplicationSettingsProtocols,
		CreatedAt:                      e.CreatedAt,
		UpdatedAt:                      e.UpdatedAt,
	}

	// 确保切片不为 nil
	if p.CipherSuites == nil {
		p.CipherSuites = []uint16{}
	}
	if p.Curves == nil {
		p.Curves = []uint16{}
	}
	if p.PointFormats == nil {
		p.PointFormats = []uint16{}
	}
	if p.SignatureAlgorithms == nil {
		p.SignatureAlgorithms = []uint16{}
	}
	if p.SignatureAlgorithmsCert == nil {
		p.SignatureAlgorithmsCert = []uint16{}
	}
	if p.ALPNProtocols == nil {
		p.ALPNProtocols = []string{}
	}
	if p.SupportedVersions == nil {
		p.SupportedVersions = []uint16{}
	}
	if p.KeyShareGroups == nil {
		p.KeyShareGroups = []uint16{}
	}
	if p.PSKModes == nil {
		p.PSKModes = []uint16{}
	}
	if p.Extensions == nil {
		p.Extensions = []uint16{}
	}
	if p.ExtensionPayloads == nil {
		p.ExtensionPayloads = map[uint16][]byte{}
	}
	if p.CompressCertAlgos == nil {
		p.CompressCertAlgos = []uint16{}
	}
	if p.DelegatedCredentialsAlgorithms == nil {
		p.DelegatedCredentialsAlgorithms = []uint16{}
	}
	if p.ApplicationSettingsProtocols == nil {
		p.ApplicationSettingsProtocols = []string{}
	}

	return p
}
