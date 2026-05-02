package dto

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type AIChatMessage struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	LineID    *int64    `json:"line_id,omitempty"`
	GroupID   *int64    `json:"group_id,omitempty"`
	LineName  string    `json:"line_name,omitempty"`
	Model     string    `json:"model,omitempty"`
}

type AIChatResponse struct {
	Message *AIChatMessage `json:"message"`
}

type AIPromptTemplate struct {
	ID              int64            `json:"id"`
	Title           string           `json:"title"`
	Content         string           `json:"content"`
	Description     string           `json:"description,omitempty"`
	Tags            []string         `json:"tags,omitempty"`
	Visibility      string           `json:"visibility"`
	Status          string           `json:"status"`
	ModerationState string           `json:"moderation_state"`
	LineID          *int64           `json:"line_id,omitempty"`
	GroupID         *int64           `json:"group_id,omitempty"`
	LineName        string           `json:"line_name,omitempty"`
	OwnerID         *int64           `json:"owner_id,omitempty"`
	UserID          *int64           `json:"user_id,omitempty"`
	OwnerName       string           `json:"owner_name,omitempty"`
	IsMine          bool             `json:"is_mine"`
	ClonedFromID    *int64           `json:"cloned_from_id,omitempty"`
	UsageCount      int64            `json:"usage_count,omitempty"`
	Featured        bool             `json:"featured"`
	Category        string           `json:"category,omitempty"`
	ModelHint       string           `json:"model_hint,omitempty"`
	Versions        []map[string]any `json:"versions,omitempty"`
	Metadata        map[string]any   `json:"metadata,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type AIArtwork struct {
	ID               int64          `json:"id"`
	Title            string         `json:"title"`
	Prompt           string         `json:"prompt"`
	NegativePrompt   string         `json:"negative_prompt,omitempty"`
	Visibility       string         `json:"visibility"`
	Status           string         `json:"status"`
	ImageURL         string         `json:"image_url"`
	ThumbnailURL     string         `json:"thumbnail_url,omitempty"`
	LineID           *int64         `json:"line_id,omitempty"`
	GroupID          *int64         `json:"group_id,omitempty"`
	LineName         string         `json:"line_name,omitempty"`
	OwnerID          *int64         `json:"owner_id,omitempty"`
	UserID           *int64         `json:"user_id,omitempty"`
	OwnerName        string         `json:"owner_name,omitempty"`
	Width            *int           `json:"width,omitempty"`
	Height           *int           `json:"height,omitempty"`
	Size             string         `json:"size,omitempty"`
	Style            string         `json:"style,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	Featured         bool           `json:"featured"`
	PromptTemplateID *int64         `json:"prompt_template_id,omitempty"`
	Likes            int64          `json:"likes,omitempty"`
	Views            int64          `json:"views,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type AIAssetPublic struct {
	ID               int64              `json:"id"`
	UserID           int64              `json:"user_id"`
	GenerationJobID  *int64             `json:"generation_job_id,omitempty"`
	SessionID        *int64             `json:"session_id,omitempty"`
	PromptTemplateID *int64             `json:"prompt_template_id,omitempty"`
	AssetType        string             `json:"asset_type"`
	Status           string             `json:"status"`
	Visibility       string             `json:"visibility"`
	ModerationState  string             `json:"moderation_state"`
	SourceURL        string             `json:"source_url,omitempty"`
	MIMEType         string             `json:"mime_type,omitempty"`
	Width            *int               `json:"width,omitempty"`
	Height           *int               `json:"height,omitempty"`
	ByteSize         *int64             `json:"byte_size,omitempty"`
	Checksum         string             `json:"checksum,omitempty"`
	Metadata         map[string]any     `json:"metadata,omitempty"`
	Trace            service.AITraceRef `json:"trace"`
	CreatedAt        time.Time          `json:"created_at"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

type AIRuntime struct {
	SourceDomain string             `json:"source_domain"`
	Lines        []AIRuntimeLine    `json:"lines,omitempty"`
	DefaultLine  *AIRuntimeLine     `json:"default_line,omitempty"`
	LineCount    int                `json:"line_count"`
	KeyCount     int                `json:"key_count"`
	Models       []AIRuntimeModel   `json:"models,omitempty"`
	Media        AIRuntimeMedia     `json:"media"`
	ImageEdit    AIRuntimeImageEdit `json:"image_edit"`
	Chat         AIRuntimeChat      `json:"chat"`
}

type AIRuntimeMedia struct {
	Enabled                   bool     `json:"enabled"`
	PublicBaseURL             string   `json:"public_base_url,omitempty"`
	PresignExpiryMinutes      int      `json:"presign_expiry_minutes"`
	MaxUploadSizeBytes        int64    `json:"max_upload_size_bytes"`
	DefaultVisibility         string   `json:"default_visibility"`
	UploadEndpoint            string   `json:"upload_endpoint"`
	PublicEndpointTemplate    string   `json:"public_endpoint_template"`
	ThumbnailEndpointTemplate string   `json:"thumbnail_endpoint_template"`
	DownloadEndpointTemplate  string   `json:"download_endpoint_template"`
	ThumbnailDownloadTemplate string   `json:"thumbnail_download_template"`
	SupportedBizTypes         []string `json:"supported_biz_types"`
	ThumbnailEnabled          bool     `json:"thumbnail_enabled"`
}

type AIRuntimeImageEdit struct {
	Enabled                bool   `json:"enabled"`
	SourceDomain           string `json:"source_domain"`
	UploadBizType          string `json:"upload_biz_type"`
	ThumbnailRoute         string `json:"thumbnail_route"`
	DownloadRoute          string `json:"download_route"`
	ThumbnailDownloadRoute string `json:"thumbnail_download_route"`
}

type AIRuntimeChat struct {
	SupportedEntries []string `json:"supported_entries,omitempty"`
	ForcedEntry      string   `json:"forced_entry,omitempty"`
}

type AIRuntimeLine struct {
	GroupID            int64          `json:"group_id"`
	Label              string         `json:"label"`
	Platform           string         `json:"platform"`
	Description        string         `json:"description,omitempty"`
	Keys               []AIRuntimeKey `json:"keys,omitempty"`
	KeyIDs             []int64        `json:"key_ids,omitempty"`
	KeyCount           int            `json:"key_count"`
	DefaultKeyID       *int64         `json:"default_key_id,omitempty"`
	DefaultMappedModel string         `json:"default_mapped_model,omitempty"`
}

type AIRuntimeKey struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type AIRuntimeModel struct {
	Name     string `json:"name"`
	Label    string `json:"label,omitempty"`
	GroupID  *int64 `json:"group_id,omitempty"`
	Platform string `json:"platform,omitempty"`
}

func AIChatMessageFromService(msg *service.AISessionMessage, lineName string) *AIChatMessage {
	if msg == nil {
		return nil
	}
	out := &AIChatMessage{
		ID:        msg.ID,
		Role:      msg.Role,
		Content:   msg.Content,
		CreatedAt: msg.CreatedAt,
		Model:     strings.TrimSpace(msg.Model),
		LineName:  strings.TrimSpace(lineName),
	}
	if msg.Trace.GroupID != nil {
		out.LineID = msg.Trace.GroupID
		out.GroupID = msg.Trace.GroupID
	}
	return out
}

func AIPromptTemplateFromService(item *service.AIPromptTemplate, currentUserID int64) *AIPromptTemplate {
	if item == nil {
		return nil
	}
	metadata := cloneAnyMap(item.Metadata)
	status := aiPromptStatusFromMetadata(item.Visibility, item.ModerationState, metadata)
	featured := aiBoolFromMetadata(metadata, "featured")
	usageCount := aiInt64FromMetadata(metadata, "usage_count")
	clonedFromID := aiInt64PtrFromMetadata(metadata, "cloned_from_id")
	lineID := item.Trace.GroupID
	out := &AIPromptTemplate{
		ID:              item.ID,
		Title:           item.Title,
		Content:         item.Content,
		Description:     item.Description,
		Tags:            append([]string(nil), item.Tags...),
		Visibility:      item.Visibility,
		Status:          status,
		ModerationState: item.ModerationState,
		LineID:          lineID,
		GroupID:         lineID,
		LineName:        "",
		OwnerID:         ptrInt64(item.UserID),
		UserID:          ptrInt64(item.UserID),
		IsMine:          currentUserID > 0 && item.UserID == currentUserID,
		ClonedFromID:    clonedFromID,
		UsageCount:      usageCount,
		Featured:        featured,
		Category:        item.Category,
		ModelHint:       item.ModelHint,
		Metadata:        metadata,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
	return out
}

func AIArtworkFromService(item *service.AIAsset, currentUserID int64) *AIArtwork {
	if item == nil {
		return nil
	}
	metadata := sanitizeAIAssetMetadata(item.Metadata)
	title := aiStringFromMetadata(metadata, "title", "Untitled Artwork")
	prompt := aiStringFromMetadata(metadata, "prompt", "")
	negativePrompt := aiStringFromMetadata(metadata, "negative_prompt", "")
	size := aiStringFromMetadata(metadata, "size", "")
	style := aiStringFromMetadata(metadata, "style", "")
	featured := aiBoolFromMetadata(metadata, "featured")
	likes := aiInt64FromMetadata(metadata, "likes")
	views := aiInt64FromMetadata(metadata, "views")
	imageURL := strings.TrimSpace(item.SourceURL)
	if imageURL == "" {
		imageURL = aiStringFromMetadata(metadata, "image_url", "")
	}
	thumbnailURL := aiStringFromMetadata(metadata, "thumbnail_url", "")
	if thumbnailURL == "" {
		thumbnailURL = imageURL
	}
	lineID := item.Trace.GroupID
	out := &AIArtwork{
		ID:               item.ID,
		Title:            title,
		Prompt:           prompt,
		NegativePrompt:   negativePrompt,
		Visibility:       item.Visibility,
		Status:           item.Status,
		ImageURL:         imageURL,
		ThumbnailURL:     thumbnailURL,
		LineID:           lineID,
		GroupID:          lineID,
		LineName:         "",
		OwnerID:          ptrInt64(item.UserID),
		UserID:           ptrInt64(item.UserID),
		OwnerName:        aiStringFromMetadata(metadata, "owner_name", ""),
		Width:            item.Width,
		Height:           item.Height,
		Size:             size,
		Style:            style,
		Tags:             aiStringSliceFromMetadata(metadata, "tags"),
		Featured:         featured,
		PromptTemplateID: item.PromptTemplateID,
		Likes:            likes,
		Views:            views,
		Metadata:         metadata,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
	_ = currentUserID
	return out
}

func AIAssetPublicFromService(item *service.AIAsset) *AIAssetPublic {
	if item == nil {
		return nil
	}
	return &AIAssetPublic{
		ID:               item.ID,
		UserID:           item.UserID,
		GenerationJobID:  item.GenerationJobID,
		SessionID:        item.SessionID,
		PromptTemplateID: item.PromptTemplateID,
		AssetType:        item.AssetType,
		Status:           item.Status,
		Visibility:       item.Visibility,
		ModerationState:  item.ModerationState,
		SourceURL:        item.SourceURL,
		MIMEType:         item.MIMEType,
		Width:            item.Width,
		Height:           item.Height,
		ByteSize:         item.ByteSize,
		Checksum:         item.Checksum,
		Metadata:         sanitizeAIAssetMetadata(item.Metadata),
		Trace:            item.Trace,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}

func AIRuntimeFromMediaRuntime(info service.MediaRuntimeInfo) *AIRuntime {
	sourceDomain := strings.TrimSpace(info.SourceDomain)
	if sourceDomain == "" {
		sourceDomain = strings.TrimSpace(info.PublicBaseURL)
	}
	return &AIRuntime{
		SourceDomain: sourceDomain,
		Media: AIRuntimeMedia{
			Enabled:                   info.Enabled,
			PublicBaseURL:             info.PublicBaseURL,
			PresignExpiryMinutes:      info.PresignExpiryMinutes,
			MaxUploadSizeBytes:        info.MaxUploadSizeBytes,
			DefaultVisibility:         info.DefaultVisibility,
			UploadEndpoint:            info.UploadEndpoint,
			PublicEndpointTemplate:    info.PublicEndpointTemplate,
			ThumbnailEndpointTemplate: info.ThumbnailEndpointTemplate,
			DownloadEndpointTemplate:  info.DownloadEndpointTemplate,
			ThumbnailDownloadTemplate: info.ThumbnailDownloadTemplate,
			SupportedBizTypes:         append([]string(nil), info.SupportedBizTypes...),
			ThumbnailEnabled:          info.ThumbnailEnabled,
		},
		ImageEdit: AIRuntimeImageEdit{
			Enabled:                info.Enabled,
			SourceDomain:           sourceDomain,
			UploadBizType:          "ai_image",
			ThumbnailRoute:         info.ThumbnailEndpointTemplate,
			DownloadRoute:          info.DownloadEndpointTemplate,
			ThumbnailDownloadRoute: info.ThumbnailDownloadTemplate,
		},
		Chat: AIRuntimeChat{
			SupportedEntries: []string{"responses", "chat_completions"},
		},
	}
}

func AIRuntimeFromServiceRuntime(info *service.AIRuntimeInfo, mediaInfo service.MediaRuntimeInfo) *AIRuntime {
	runtime := AIRuntimeFromMediaRuntime(mediaInfo)
	if info == nil {
		return runtime
	}

	runtime.LineCount = info.LineCount
	runtime.KeyCount = info.KeyCount
	runtime.Lines = make([]AIRuntimeLine, 0, len(info.Lines))
	for i := range info.Lines {
		runtime.Lines = append(runtime.Lines, AIRuntimeLine{
			GroupID:            info.Lines[i].GroupID,
			Label:              info.Lines[i].Label,
			Platform:           info.Lines[i].Platform,
			Description:        info.Lines[i].Description,
			Keys:               aiRuntimeKeysFromService(info.Lines[i].Keys),
			KeyIDs:             append([]int64(nil), info.Lines[i].KeyIDs...),
			KeyCount:           info.Lines[i].KeyCount,
			DefaultKeyID:       info.Lines[i].DefaultKeyID,
			DefaultMappedModel: info.Lines[i].DefaultMappedModel,
		})
	}
	if info.DefaultLine != nil {
		runtime.DefaultLine = &AIRuntimeLine{
			GroupID:            info.DefaultLine.GroupID,
			Label:              info.DefaultLine.Label,
			Platform:           info.DefaultLine.Platform,
			Description:        info.DefaultLine.Description,
			Keys:               aiRuntimeKeysFromService(info.DefaultLine.Keys),
			KeyIDs:             append([]int64(nil), info.DefaultLine.KeyIDs...),
			KeyCount:           info.DefaultLine.KeyCount,
			DefaultKeyID:       info.DefaultLine.DefaultKeyID,
			DefaultMappedModel: info.DefaultLine.DefaultMappedModel,
		}
	} else if len(runtime.Lines) > 0 {
		defaultLine := runtime.Lines[0]
		runtime.DefaultLine = &defaultLine
	}
	runtime.Models = make([]AIRuntimeModel, 0, len(info.Models))
	for i := range info.Models {
		runtime.Models = append(runtime.Models, AIRuntimeModel{
			Name:     info.Models[i].Name,
			Label:    info.Models[i].Label,
			GroupID:  info.Models[i].GroupID,
			Platform: info.Models[i].Platform,
		})
	}
	return runtime
}

func cloneAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func CloneAnyMap(src map[string]any) map[string]any {
	return cloneAnyMap(src)
}

func sanitizeAIAssetMetadata(src map[string]any) map[string]any {
	dst := cloneAnyMap(src)
	delete(dst, "bucket")
	delete(dst, "object_key")
	delete(dst, "thumbnail_object_key")
	delete(dst, "storage_kind")
	delete(dst, "storage_path")
	delete(dst, "media_bucket")
	delete(dst, "media_object_key")
	delete(dst, "media_thumbnail_object_key")
	return dst
}

func aiRuntimeKeysFromService(items []service.AIRuntimeKey) []AIRuntimeKey {
	if len(items) == 0 {
		return nil
	}
	out := make([]AIRuntimeKey, 0, len(items))
	for _, item := range items {
		out = append(out, AIRuntimeKey{
			ID:   item.ID,
			Name: item.Name,
		})
	}
	return out
}

func aiPromptStatusFromMetadata(visibility, moderationState string, metadata map[string]any) string {
	if s := strings.TrimSpace(aiStringFromMetadata(metadata, "status", "")); s != "" {
		switch s {
		case "draft", "published", "archived", "hidden":
			return s
		}
	}
	switch domain.NormalizeAIModerationState(moderationState) {
	case domain.AIModerationStateBlocked:
		return "hidden"
	case domain.AIModerationStateForcedPrivate:
		return "archived"
	}
	if domain.NormalizeAIVisibility(visibility) == domain.AIVisibilityPublic {
		return "published"
	}
	return "draft"
}

func aiStringFromMetadata(metadata map[string]any, key, fallback string) string {
	if metadata == nil {
		return fallback
	}
	if v, ok := metadata[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func aiStringSliceFromMetadata(metadata map[string]any, key string) []string {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}

func aiBoolFromMetadata(metadata map[string]any, key string) bool {
	if metadata == nil {
		return false
	}
	if v, ok := metadata[key].(bool); ok {
		return v
	}
	return false
}

func aiInt64FromMetadata(metadata map[string]any, key string) int64 {
	if metadata == nil {
		return 0
	}
	switch v := metadata[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func aiInt64PtrFromMetadata(metadata map[string]any, key string) *int64 {
	if metadata == nil {
		return nil
	}
	switch v := metadata[key].(type) {
	case int64:
		return &v
	case int:
		i := int64(v)
		return &i
	case float64:
		i := int64(v)
		return &i
	default:
		return nil
	}
}

func ptrInt64(v int64) *int64 {
	out := v
	return &out
}

func Int64PtrFromPtrPtr(v **int64) *int64 {
	if v == nil || *v == nil {
		return nil
	}
	out := **v
	return &out
}
