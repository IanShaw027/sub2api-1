package admin

import (
	"encoding/json"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// TLSFingerprintProfileHandler 处理 TLS 指纹模板的 HTTP 请求
type TLSFingerprintProfileHandler struct {
	service        *service.TLSFingerprintProfileService
	captureService *service.TLSFingerprintCaptureService
}

// NewTLSFingerprintProfileHandler 创建 TLS 指纹模板处理器
func NewTLSFingerprintProfileHandler(profileService *service.TLSFingerprintProfileService, captureService *service.TLSFingerprintCaptureService) *TLSFingerprintProfileHandler {
	return &TLSFingerprintProfileHandler{service: profileService, captureService: captureService}
}

// CreateTLSFingerprintProfileRequest 创建模板请求
type CreateTLSFingerprintProfileRequest struct {
	Platform                       string   `json:"platform"`
	Transport                      string   `json:"transport"`
	OS                             string   `json:"os"`
	ClientType                     string   `json:"client_type"`
	Name                           string   `json:"name" binding:"required"`
	UserAgent                      string   `json:"user_agent"`
	Originator                     string   `json:"originator"`
	Description                    *string  `json:"description"`
	EnableGREASE                   *bool    `json:"enable_grease"`
	CipherSuites                   []uint16 `json:"cipher_suites"`
	Curves                         []uint16 `json:"curves"`
	PointFormats                   []uint16 `json:"point_formats"`
	SignatureAlgorithms            []uint16 `json:"signature_algorithms"`
	ALPNProtocols                  []string `json:"alpn_protocols"`
	SupportedVersions              []uint16 `json:"supported_versions"`
	KeyShareGroups                 []uint16 `json:"key_share_groups"`
	PSKModes                       []uint16 `json:"psk_modes"`
	Extensions                     []uint16 `json:"extensions"`
	CompressCertAlgos              []uint16 `json:"compress_cert_algos"`
	DelegatedCredentialsAlgorithms []uint16 `json:"delegated_credentials_algorithms"`
	ApplicationSettingsProtocols   []string `json:"application_settings_protocols"`
}

// UpdateTLSFingerprintProfileRequest 更新模板请求（部分更新）
type UpdateTLSFingerprintProfileRequest struct {
	Platform                       *string  `json:"platform"`
	Transport                      *string  `json:"transport"`
	OS                             *string  `json:"os"`
	ClientType                     *string  `json:"client_type"`
	Name                           *string  `json:"name"`
	UserAgent                      *string  `json:"user_agent"`
	Originator                     *string  `json:"originator"`
	Description                    *string  `json:"description"`
	EnableGREASE                   *bool    `json:"enable_grease"`
	CipherSuites                   []uint16 `json:"cipher_suites"`
	Curves                         []uint16 `json:"curves"`
	PointFormats                   []uint16 `json:"point_formats"`
	SignatureAlgorithms            []uint16 `json:"signature_algorithms"`
	ALPNProtocols                  []string `json:"alpn_protocols"`
	SupportedVersions              []uint16 `json:"supported_versions"`
	KeyShareGroups                 []uint16 `json:"key_share_groups"`
	PSKModes                       []uint16 `json:"psk_modes"`
	Extensions                     []uint16 `json:"extensions"`
	CompressCertAlgos              []uint16 `json:"compress_cert_algos"`
	DelegatedCredentialsAlgorithms []uint16 `json:"delegated_credentials_algorithms"`
	ApplicationSettingsProtocols   []string `json:"application_settings_protocols"`
}

// ImportTLSFingerprintCapturesRequest imports captured JSON/YAML fingerprints.
type ImportTLSFingerprintCapturesRequest struct {
	Platform string            `json:"platform"`
	Profiles []json.RawMessage `json:"profiles" binding:"required"`
}

type ImportCaptureTaskSamplesRequest struct {
	SampleIDs []int64 `json:"sample_ids"`
}

// List 获取所有模板
// GET /api/v1/admin/tls-fingerprint-profiles
func (h *TLSFingerprintProfileHandler) List(c *gin.Context) {
	profiles, err := h.service.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, profiles)
}

// GetByID 根据 ID 获取模板
// GET /api/v1/admin/tls-fingerprint-profiles/:id
func (h *TLSFingerprintProfileHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid profile ID")
		return
	}

	profile, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if profile == nil {
		response.NotFound(c, "Profile not found")
		return
	}

	response.Success(c, profile)
}

// Create 创建模板
// POST /api/v1/admin/tls-fingerprint-profiles
func (h *TLSFingerprintProfileHandler) Create(c *gin.Context) {
	var req CreateTLSFingerprintProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	profile := &model.TLSFingerprintProfile{
		Platform:                       req.Platform,
		Transport:                      req.Transport,
		OS:                             req.OS,
		ClientType:                     req.ClientType,
		Name:                           req.Name,
		UserAgent:                      req.UserAgent,
		Originator:                     req.Originator,
		Description:                    req.Description,
		CipherSuites:                   req.CipherSuites,
		Curves:                         req.Curves,
		PointFormats:                   req.PointFormats,
		SignatureAlgorithms:            req.SignatureAlgorithms,
		ALPNProtocols:                  req.ALPNProtocols,
		SupportedVersions:              req.SupportedVersions,
		KeyShareGroups:                 req.KeyShareGroups,
		PSKModes:                       req.PSKModes,
		Extensions:                     req.Extensions,
		CompressCertAlgos:              req.CompressCertAlgos,
		DelegatedCredentialsAlgorithms: req.DelegatedCredentialsAlgorithms,
		ApplicationSettingsProtocols:   req.ApplicationSettingsProtocols,
	}

	if req.EnableGREASE != nil {
		profile.EnableGREASE = *req.EnableGREASE
	}

	created, err := h.service.Create(c.Request.Context(), profile)
	if err != nil {
		if _, ok := err.(*model.ValidationError); ok {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, created)
}

// ImportCaptures imports real collector payloads and deduplicates by TLS fields.
// POST /api/v1/admin/tls-fingerprint-profiles/import-captures
func (h *TLSFingerprintProfileHandler) ImportCaptures(c *gin.Context) {
	var req ImportTLSFingerprintCapturesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	payloads := make([]string, 0, len(req.Profiles))
	for i, raw := range req.Profiles {
		if len(raw) == 0 {
			response.BadRequest(c, "Invalid request: profiles["+strconv.Itoa(i)+"] is empty")
			return
		}
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			payloads = append(payloads, text)
			continue
		}
		payloads = append(payloads, string(raw))
	}

	result, err := h.service.ImportTLSFingerprintCaptures(c.Request.Context(), service.TLSFingerprintCaptureImportRequest{
		Platform: req.Platform,
		Profiles: payloads,
	})
	if err != nil {
		if _, ok := err.(*model.ValidationError); ok {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ListCaptureTasks lists TLS fingerprint capture tasks.
// GET /api/v1/admin/tls-fingerprint-profiles/capture-tasks
func (h *TLSFingerprintProfileHandler) ListCaptureTasks(c *gin.Context) {
	tasks, err := h.captureService.ListTasks(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, tasks)
}

// StartCaptureTask starts a token-protected live TLS fingerprint capture task.
// POST /api/v1/admin/tls-fingerprint-profiles/capture-tasks
func (h *TLSFingerprintProfileHandler) StartCaptureTask(c *gin.Context) {
	var req service.TLSFingerprintCaptureStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	task, err := h.captureService.StartTask(c.Request.Context(), req)
	if err != nil {
		if _, ok := err.(*model.ValidationError); ok {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, task)
}

// GetCaptureTask returns one capture task by ID.
// GET /api/v1/admin/tls-fingerprint-profiles/capture-tasks/:id
func (h *TLSFingerprintProfileHandler) GetCaptureTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid capture task ID")
		return
	}
	task, err := h.captureService.GetTaskByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if task == nil {
		response.NotFound(c, "Capture task not found")
		return
	}
	response.Success(c, task)
}

// StopCaptureTask stops a running capture task.
// POST /api/v1/admin/tls-fingerprint-profiles/capture-tasks/:id/stop
func (h *TLSFingerprintProfileHandler) StopCaptureTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid capture task ID")
		return
	}
	task, err := h.captureService.StopTask(c.Request.Context(), id)
	if err != nil {
		if _, ok := err.(*model.ValidationError); ok {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, task)
}

// DeleteCaptureTask deletes a stopped/completed capture task and its samples.
// DELETE /api/v1/admin/tls-fingerprint-profiles/capture-tasks/:id
func (h *TLSFingerprintProfileHandler) DeleteCaptureTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid capture task ID")
		return
	}
	err = h.captureService.DeleteTask(c.Request.Context(), id)
	if err != nil {
		if _, ok := err.(*model.ValidationError); ok {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "capture task deleted"})
}

// RestartCaptureTask restarts a stopped/completed capture task with a new token.
// POST /api/v1/admin/tls-fingerprint-profiles/capture-tasks/:id/restart
func (h *TLSFingerprintProfileHandler) RestartCaptureTask(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid capture task ID")
		return
	}
	task, err := h.captureService.RestartTask(c.Request.Context(), id)
	if err != nil {
		if _, ok := err.(*model.ValidationError); ok {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, task)
}

// ListCaptureSamples lists samples captured by one task.
// GET /api/v1/admin/tls-fingerprint-profiles/capture-tasks/:id/samples
func (h *TLSFingerprintProfileHandler) ListCaptureSamples(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid capture task ID")
		return
	}
	samples, err := h.captureService.ListSamplesByTask(c.Request.Context(), id)
	if err != nil {
		if _, ok := err.(*model.ValidationError); ok {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, samples)
}

// ImportCaptureTaskSamples imports selected/all captured task samples to profiles.
// POST /api/v1/admin/tls-fingerprint-profiles/capture-tasks/:id/import
func (h *TLSFingerprintProfileHandler) ImportCaptureTaskSamples(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid capture task ID")
		return
	}
	var req ImportCaptureTaskSamplesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.captureService.ImportTaskSamples(c.Request.Context(), service.TLSFingerprintCaptureTaskImportRequest{
		TaskID:    id,
		SampleIDs: req.SampleIDs,
	})
	if err != nil {
		if _, ok := err.(*model.ValidationError); ok {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// Update 更新模板（支持部分更新）
// PUT /api/v1/admin/tls-fingerprint-profiles/:id
func (h *TLSFingerprintProfileHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid profile ID")
		return
	}

	var req UpdateTLSFingerprintProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	existing, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if existing == nil {
		response.NotFound(c, "Profile not found")
		return
	}

	// 部分更新
	profile := &model.TLSFingerprintProfile{
		ID:                             id,
		Platform:                       existing.Platform,
		Transport:                      existing.Transport,
		Name:                           existing.Name,
		UserAgent:                      existing.UserAgent,
		Originator:                     existing.Originator,
		Description:                    existing.Description,
		EnableGREASE:                   existing.EnableGREASE,
		CipherSuites:                   existing.CipherSuites,
		Curves:                         existing.Curves,
		PointFormats:                   existing.PointFormats,
		SignatureAlgorithms:            existing.SignatureAlgorithms,
		ALPNProtocols:                  existing.ALPNProtocols,
		SupportedVersions:              existing.SupportedVersions,
		KeyShareGroups:                 existing.KeyShareGroups,
		PSKModes:                       existing.PSKModes,
		Extensions:                     existing.Extensions,
		CompressCertAlgos:              existing.CompressCertAlgos,
		DelegatedCredentialsAlgorithms: existing.DelegatedCredentialsAlgorithms,
		ApplicationSettingsProtocols:   existing.ApplicationSettingsProtocols,
	}

	if req.Name != nil {
		profile.Name = *req.Name
	}
	if req.Platform != nil {
		profile.Platform = *req.Platform
	}
	if req.Transport != nil {
		profile.Transport = *req.Transport
	}
	if req.OS != nil {
		profile.OS = *req.OS
	}
	if req.ClientType != nil {
		profile.ClientType = *req.ClientType
	}
	if req.UserAgent != nil {
		profile.UserAgent = *req.UserAgent
	}
	if req.Originator != nil {
		profile.Originator = *req.Originator
	}
	if req.Description != nil {
		profile.Description = req.Description
	}
	if req.EnableGREASE != nil {
		profile.EnableGREASE = *req.EnableGREASE
	}
	if req.CipherSuites != nil {
		profile.CipherSuites = req.CipherSuites
	}
	if req.Curves != nil {
		profile.Curves = req.Curves
	}
	if req.PointFormats != nil {
		profile.PointFormats = req.PointFormats
	}
	if req.SignatureAlgorithms != nil {
		profile.SignatureAlgorithms = req.SignatureAlgorithms
	}
	if req.ALPNProtocols != nil {
		profile.ALPNProtocols = req.ALPNProtocols
	}
	if req.SupportedVersions != nil {
		profile.SupportedVersions = req.SupportedVersions
	}
	if req.KeyShareGroups != nil {
		profile.KeyShareGroups = req.KeyShareGroups
	}
	if req.PSKModes != nil {
		profile.PSKModes = req.PSKModes
	}
	if req.Extensions != nil {
		profile.Extensions = req.Extensions
	}
	if req.CompressCertAlgos != nil {
		profile.CompressCertAlgos = req.CompressCertAlgos
	}
	if req.DelegatedCredentialsAlgorithms != nil {
		profile.DelegatedCredentialsAlgorithms = req.DelegatedCredentialsAlgorithms
	}
	if req.ApplicationSettingsProtocols != nil {
		profile.ApplicationSettingsProtocols = req.ApplicationSettingsProtocols
	}

	updated, err := h.service.Update(c.Request.Context(), profile)
	if err != nil {
		if _, ok := err.(*model.ValidationError); ok {
			response.BadRequest(c, err.Error())
			return
		}
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, updated)
}

// Delete 删除模板
// DELETE /api/v1/admin/tls-fingerprint-profiles/:id
func (h *TLSFingerprintProfileHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid profile ID")
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Profile deleted successfully"})
}
