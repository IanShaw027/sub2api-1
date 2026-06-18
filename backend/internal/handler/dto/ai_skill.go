package dto

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler/skillkit"
)

type SkillAuthor struct {
	ID        *int64  `json:"id,omitempty"`
	Name      string  `json:"name,omitempty"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type SkillPricing struct {
	Mode            string   `json:"mode"`
	Amount          float64  `json:"amount"`
	Currency        string   `json:"currency"`
	SettlementRatio *float64 `json:"settlement_ratio,omitempty"`
}

type SkillStats struct {
	Installs int      `json:"installs"`
	Runs     int      `json:"runs"`
	Revenue  float64  `json:"revenue"`
	Rating   *float64 `json:"rating,omitempty"`
	Versions int      `json:"versions"`
}

type SkillVersionSummary struct {
	ID          int64      `json:"id"`
	SkillID     *int64     `json:"skill_id,omitempty"`
	Version     string     `json:"version"`
	Status      string     `json:"status"`
	Changelog   string     `json:"changelog,omitempty"`
	SourceLock  bool       `json:"source_locked"`
	IsCurrent   bool       `json:"is_current"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

type SkillSummary struct {
	ID             int64                `json:"id"`
	Slug           string               `json:"slug"`
	Name           string               `json:"name"`
	Tagline        string               `json:"tagline,omitempty"`
	Description    string               `json:"description,omitempty"`
	Type           string               `json:"type"`
	Visibility     string               `json:"visibility"`
	Status         string               `json:"status"`
	Category       string               `json:"category,omitempty"`
	Tags           []string             `json:"tags,omitempty"`
	CoverImageURL  string               `json:"cover_image_url,omitempty"`
	Pricing        SkillPricing         `json:"pricing"`
	SourceLocked   bool                 `json:"source_locked"`
	CanViewSource  bool                 `json:"can_view_source"`
	Installed      bool                 `json:"installed"`
	Owned          bool                 `json:"owned"`
	Editable       bool                 `json:"editable"`
	Author         SkillAuthor          `json:"author"`
	Stats          SkillStats           `json:"stats"`
	LatestVersion  *SkillVersionSummary `json:"latest_version,omitempty"`
	CurrentVersion *SkillVersionSummary `json:"current_version,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

type SkillDetail struct {
	SkillSummary
	VariableSchema []map[string]any `json:"variable_schema,omitempty"`
	Content        map[string]any   `json:"content,omitempty"`
	Metadata       map[string]any   `json:"metadata,omitempty"`
	Examples       []string         `json:"examples,omitempty"`
	Readme         string           `json:"readme,omitempty"`
	InstallNote    string           `json:"install_note,omitempty"`
	CanInstall     bool             `json:"can_install"`
	CanRun         bool             `json:"can_run"`
}

type SkillVersionRecord struct {
	SkillVersionSummary
	VariableSchema []map[string]any `json:"variable_schema,omitempty"`
	Content        map[string]any   `json:"content,omitempty"`
	Metadata       map[string]any   `json:"metadata,omitempty"`
}

type SkillRunRecord struct {
	ID            int64      `json:"id"`
	SkillID       int64      `json:"skill_id"`
	SkillName     string     `json:"skill_name"`
	VersionID     *int64     `json:"version_id,omitempty"`
	Version       string     `json:"version,omitempty"`
	Status        string     `json:"status"`
	Trigger       string     `json:"trigger,omitempty"`
	InputPreview  string     `json:"input_preview,omitempty"`
	OutputPreview string     `json:"output_preview,omitempty"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	DurationMS    *float64   `json:"duration_ms,omitempty"`
	Cost          float64    `json:"cost"`
	Currency      string     `json:"currency"`
	CreatedAt     time.Time  `json:"created_at"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

type SkillRevenueSummary struct {
	TotalRevenue   float64 `json:"total_revenue"`
	TotalSales     int64   `json:"total_sales"`
	TotalRuns      int64   `json:"total_runs"`
	PendingAmount  float64 `json:"pending_amount"`
	SettledAmount  float64 `json:"settled_amount"`
	RefundedAmount float64 `json:"refunded_amount"`
	Currency       string  `json:"currency"`
}

type SkillRevenuePoint struct {
	Date    string  `json:"date"`
	Revenue float64 `json:"revenue"`
	Sales   int64   `json:"sales"`
	Runs    int64   `json:"runs"`
}

type SkillRevenueOrder struct {
	ID        int64     `json:"id"`
	BuyerName string    `json:"buyer_name,omitempty"`
	Version   string    `json:"version,omitempty"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type SkillRevenueDetail struct {
	Summary SkillRevenueSummary `json:"summary"`
	Trend   []SkillRevenuePoint `json:"trend,omitempty"`
	Orders  []SkillRevenueOrder `json:"orders,omitempty"`
}

type SkillReviewItem struct {
	ID                     int64      `json:"id"`
	SkillID                int64      `json:"skill_id"`
	SkillName              string     `json:"skill_name"`
	SkillSlug              string     `json:"skill_slug"`
	VersionID              int64      `json:"version_id"`
	VersionName            string     `json:"version_name"`
	LatestPublishedVersion string     `json:"latest_published_version,omitempty"`
	ReviewStatus           string     `json:"review_status"`
	Visibility             string     `json:"visibility"`
	RiskLevel              string     `json:"risk_level"`
	Category               string     `json:"category,omitempty"`
	AuthorName             string     `json:"author_name,omitempty"`
	Summary                string     `json:"summary,omitempty"`
	Changelog              string     `json:"changelog,omitempty"`
	ReviewNote             string     `json:"review_note,omitempty"`
	RejectionReason        string     `json:"rejection_reason,omitempty"`
	ReviewerName           string     `json:"reviewer_name,omitempty"`
	Tags                   []string   `json:"tags,omitempty"`
	Requests24H            int64      `json:"requests_24h"`
	Revenue30D             float64    `json:"revenue_30d"`
	SubmittedAt            time.Time  `json:"submitted_at"`
	ReviewedAt             *time.Time `json:"reviewed_at,omitempty"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type SkillGovernanceItem struct {
	ID                     int64     `json:"id"`
	SkillID                int64     `json:"skill_id"`
	SkillName              string    `json:"skill_name"`
	SkillSlug              string    `json:"skill_slug"`
	CurrentVersion         string    `json:"current_version,omitempty"`
	LatestPublishedVersion string    `json:"latest_published_version,omitempty"`
	GovernanceStatus       string    `json:"governance_status"`
	LatestReviewStatus     string    `json:"latest_review_status"`
	Visibility             string    `json:"visibility"`
	Category               string    `json:"category,omitempty"`
	AuthorName             string    `json:"author_name,omitempty"`
	ReviewNote             string    `json:"review_note,omitempty"`
	Tags                   []string  `json:"tags,omitempty"`
	Requests24H            int64     `json:"requests_24h"`
	SuccessRate            float64   `json:"success_rate"`
	Revenue30D             float64   `json:"revenue_30d"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type SkillRuntimeItem struct {
	ID             int64      `json:"id"`
	SkillID        int64      `json:"skill_id"`
	SkillName      string     `json:"skill_name"`
	SkillSlug      string     `json:"skill_slug"`
	CurrentVersion string     `json:"current_version,omitempty"`
	HealthStatus   string     `json:"health_status"`
	Requests24H    int64      `json:"requests_24h"`
	SuccessRate    float64    `json:"success_rate"`
	AvgLatencyMS   float64    `json:"avg_latency_ms"`
	P95LatencyMS   float64    `json:"p95_latency_ms"`
	ErrorRate      float64    `json:"error_rate"`
	QueueDepth     int64      `json:"queue_depth"`
	LastError      string     `json:"last_error,omitempty"`
	LastRunAt      *time.Time `json:"last_run_at,omitempty"`
	LastAlertAt    *time.Time `json:"last_alert_at,omitempty"`
}

type SkillRuntimeEvent struct {
	ID          int64     `json:"id"`
	SkillID     *int64    `json:"skill_id,omitempty"`
	SkillName   string    `json:"skill_name,omitempty"`
	Level       string    `json:"level"`
	Message     string    `json:"message"`
	MetricName  string    `json:"metric_name,omitempty"`
	MetricValue *float64  `json:"metric_value,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type SkillSettlementItem struct {
	ID                int64     `json:"id"`
	SkillID           int64     `json:"skill_id"`
	SkillName         string    `json:"skill_name"`
	SkillSlug         string    `json:"skill_slug"`
	AuthorName        string    `json:"author_name,omitempty"`
	PeriodLabel       string    `json:"period_label"`
	SettlementStatus  string    `json:"settlement_status"`
	GrossAmount       float64   `json:"gross_amount"`
	PlatformFeeAmount float64   `json:"platform_fee_amount"`
	PayoutAmount      float64   `json:"payout_amount"`
	FrozenAmount      float64   `json:"frozen_amount"`
	Currency          string    `json:"currency"`
	Note              string    `json:"note,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func SkillSummaryFromDomain(skill *domain.AISkill, viewerUserID int64, canViewSource bool, latestVersion, currentVersion *domain.AISkillVersion) *SkillSummary {
	if skill == nil {
		return nil
	}
	meta := cloneAnyMap(skill.Metadata)
	owned := viewerUserID > 0 && skill.UserID == viewerUserID
	pricing := pricingFromMetadata(skill.Price, meta)
	sourceLocked := skill.SourceVisibility == domain.AISkillSourceVisibilityHidden || boolFromMetadata(meta, "source_locked")
	installed := boolFromMetadata(meta, "installed")
	installCount := intFromMetadata(meta, "install_count", skill.LikeCount)
	return &SkillSummary{
		ID:            skill.ID,
		Slug:          stringOrFallback(meta["slug"], fmt.Sprintf("skill-%d", skill.ID)),
		Name:          skill.Title,
		Tagline:       firstNonEmpty(skill.Summary, stringOrFallback(meta["tagline"], "")),
		Description:   skill.Description,
		Type:          skill.Type,
		Visibility:    visibilityForResponse(skill.Visibility),
		Status:        skillStatusFromMetadata(meta, skill.Visibility),
		Category:      skill.Category,
		Tags:          append([]string(nil), skill.Tags...),
		CoverImageURL: stringOrFallback(meta["cover_image_url"], ""),
		Pricing:       pricing,
		SourceLocked:  sourceLocked,
		CanViewSource: canViewSource,
		Installed:     installed,
		Owned:         owned,
		Editable:      owned,
		Author:        SkillAuthor{},
		Stats: SkillStats{
			Installs: installCount,
			Runs:     skill.RunCount,
			Revenue:  skill.TotalIncome,
			Versions: skill.LatestVersion,
		},
		LatestVersion:  SkillVersionSummaryFromDomain(latestVersion, skill.CurrentVersionID),
		CurrentVersion: SkillVersionSummaryFromDomain(currentVersion, skill.CurrentVersionID),
		CreatedAt:      skill.CreatedAt,
		UpdatedAt:      skill.UpdatedAt,
	}
}

func SkillDetailFromDomain(skill *domain.AISkill, viewerUserID int64, canViewSource bool, latestVersion, currentVersion *domain.AISkillVersion) *SkillDetail {
	summary := SkillSummaryFromDomain(skill, viewerUserID, canViewSource, latestVersion, currentVersion)
	if summary == nil {
		return nil
	}
	meta := cloneAnyMap(skill.Metadata)
	variableSchema := variableSchemaFromMetadata(meta)
	versionMeta := map[string]any{}
	if currentVersion != nil {
		versionMeta = cloneAnyMap(currentVersion.Metadata)
	}
	if len(variableSchema) == 0 {
		variableSchema = variableSchemaFromMetadata(versionMeta)
	}
	readme := firstNonEmpty(stringOrFallback(meta["readme"], ""), stringOrFallback(versionMeta["readme"], ""))
	installNote := firstNonEmpty(stringOrFallback(meta["install_note"], ""), stringOrFallback(versionMeta["install_note"], ""))

	var content map[string]any
	if canViewSource {
		content = contentFromMetadata(meta)
		if len(content) == 0 {
			content = contentFromMetadata(versionMeta)
		}
		if len(content) == 0 {
			content = nil
		}
	}

	responseMeta := map[string]any{}
	examples := []string{}
	if canViewSource {
		responseMeta = meta
		examples = stringListFromAny(meta["examples"])
		if len(examples) == 0 {
			examples = stringListFromAny(versionMeta["examples"])
		}
	}
	return &SkillDetail{
		SkillSummary:   *summary,
		VariableSchema: variableSchema,
		Content:        content,
		Metadata:       responseMeta,
		Examples:       examples,
		Readme:         readme,
		InstallNote:    installNote,
		CanInstall:     !summary.Owned,
		CanRun:         latestVersion != nil || currentVersion != nil || summary.Owned,
	}
}

func SkillVersionSummaryFromDomain(version *domain.AISkillVersion, currentVersionID *int64) *SkillVersionSummary {
	if version == nil {
		return nil
	}
	meta := cloneAnyMap(version.Metadata)
	return &SkillVersionSummary{
		ID:          version.ID,
		SkillID:     ptrInt64(version.SkillID),
		Version:     versionNameFromDomain(version),
		Status:      versionStatusFromDomain(version),
		Changelog:   version.ChangeNote,
		SourceLock:  boolFromMetadata(meta, "source_locked"),
		IsCurrent:   currentVersionID != nil && *currentVersionID == version.ID,
		CreatedAt:   version.CreatedAt,
		PublishedAt: version.ReviewedAt,
	}
}

func SkillVersionRecordFromDomain(version *domain.AISkillVersion, currentVersionID *int64, canViewSource bool) *SkillVersionRecord {
	summary := SkillVersionSummaryFromDomain(version, currentVersionID)
	if summary == nil {
		return nil
	}
	if !canViewSource {
		return &SkillVersionRecord{
			SkillVersionSummary: *summary,
		}
	}
	meta := cloneAnyMap(version.Metadata)
	content := contentFromMetadata(meta)
	if len(content) == 0 {
		content = nil
	}
	return &SkillVersionRecord{
		SkillVersionSummary: *summary,
		VariableSchema:      variableSchemaFromMetadata(meta),
		Content:             content,
		Metadata:            meta,
	}
}

func SkillRunRecordFromQuery(record skillkit.RunRecord) SkillRunRecord {
	return SkillRunRecord{
		ID:            record.ID,
		SkillID:       record.SkillID,
		SkillName:     record.SkillName,
		VersionID:     record.VersionID,
		Version:       record.VersionName,
		Status:        normalizeRunStatus(record.Status),
		Trigger:       record.Trigger,
		InputPreview:  truncatePreview(record.InputPreview),
		OutputPreview: truncatePreview(record.OutputPreview),
		ErrorMessage:  record.ErrorMessage,
		DurationMS:    record.DurationMS,
		Cost:          record.Cost,
		Currency:      fallbackString(record.Currency, "CNY"),
		CreatedAt:     record.CreatedAt,
		StartedAt:     record.StartedAt,
		FinishedAt:    record.FinishedAt,
	}
}

func SkillRevenueDetailFromQuery(detail *skillkit.RevenueDetail) *SkillRevenueDetail {
	if detail == nil {
		return nil
	}
	out := &SkillRevenueDetail{
		Summary: SkillRevenueSummary{
			TotalRevenue:   detail.Summary.TotalRevenue,
			TotalSales:     detail.Summary.TotalSales,
			TotalRuns:      detail.Summary.TotalRuns,
			PendingAmount:  detail.Summary.PendingAmount,
			SettledAmount:  detail.Summary.SettledAmount,
			RefundedAmount: detail.Summary.RefundedAmount,
			Currency:       fallbackString(detail.Summary.Currency, "CNY"),
		},
		Trend:  make([]SkillRevenuePoint, 0, len(detail.Trend)),
		Orders: make([]SkillRevenueOrder, 0, len(detail.Orders)),
	}
	for _, item := range detail.Trend {
		out.Trend = append(out.Trend, SkillRevenuePoint{
			Date:    item.Date,
			Revenue: item.Revenue,
			Sales:   item.Sales,
			Runs:    item.Runs,
		})
	}
	for _, item := range detail.Orders {
		out.Orders = append(out.Orders, SkillRevenueOrder{
			ID:        item.ID,
			BuyerName: item.BuyerName,
			Version:   item.Version,
			Amount:    item.Amount,
			Currency:  item.Currency,
			Status:    item.Status,
			CreatedAt: item.CreatedAt,
		})
	}
	return out
}

func SkillReviewItemFromQuery(record skillkit.ReviewRecord) SkillReviewItem {
	return SkillReviewItem{
		ID:                     record.ID,
		SkillID:                record.SkillID,
		SkillName:              record.SkillName,
		SkillSlug:              record.SkillSlug,
		VersionID:              record.VersionID,
		VersionName:            record.VersionName,
		LatestPublishedVersion: record.LatestPublishedVersion,
		ReviewStatus:           record.ReviewStatus,
		Visibility:             record.Visibility,
		RiskLevel:              record.RiskLevel,
		Category:               record.Category,
		AuthorName:             record.AuthorName,
		Summary:                record.Summary,
		Changelog:              record.Changelog,
		ReviewNote:             record.ReviewNote,
		RejectionReason:        record.RejectionReason,
		ReviewerName:           record.ReviewerName,
		Tags:                   append([]string(nil), record.Tags...),
		Requests24H:            record.Requests24H,
		Revenue30D:             record.Revenue30D,
		SubmittedAt:            record.SubmittedAt,
		ReviewedAt:             record.ReviewedAt,
		UpdatedAt:              record.UpdatedAt,
	}
}

func SkillGovernanceItemFromQuery(record skillkit.GovernanceRecord) SkillGovernanceItem {
	return SkillGovernanceItem{
		ID:                     record.ID,
		SkillID:                record.SkillID,
		SkillName:              record.SkillName,
		SkillSlug:              record.SkillSlug,
		CurrentVersion:         record.CurrentVersion,
		LatestPublishedVersion: record.LatestPublishedVersion,
		GovernanceStatus:       record.GovernanceStatus,
		LatestReviewStatus:     record.LatestReviewStatus,
		Visibility:             record.Visibility,
		Category:               record.Category,
		AuthorName:             record.AuthorName,
		ReviewNote:             record.ReviewNote,
		Tags:                   append([]string(nil), record.Tags...),
		Requests24H:            record.Requests24H,
		SuccessRate:            record.SuccessRate,
		Revenue30D:             record.Revenue30D,
		CreatedAt:              record.CreatedAt,
		UpdatedAt:              record.UpdatedAt,
	}
}

func SkillRuntimeItemFromQuery(record skillkit.RuntimeRecord) SkillRuntimeItem {
	return SkillRuntimeItem{
		ID:             record.ID,
		SkillID:        record.SkillID,
		SkillName:      record.SkillName,
		SkillSlug:      record.SkillSlug,
		CurrentVersion: record.CurrentVersion,
		HealthStatus:   record.HealthStatus,
		Requests24H:    record.Requests24H,
		SuccessRate:    record.SuccessRate,
		AvgLatencyMS:   record.AvgLatencyMS,
		P95LatencyMS:   record.P95LatencyMS,
		ErrorRate:      record.ErrorRate,
		QueueDepth:     record.QueueDepth,
		LastError:      record.LastError,
		LastRunAt:      record.LastRunAt,
		LastAlertAt:    record.LastAlertAt,
	}
}

func SkillRuntimeEventFromQuery(record skillkit.RuntimeEvent) SkillRuntimeEvent {
	return SkillRuntimeEvent{
		ID:          record.ID,
		SkillID:     record.SkillID,
		SkillName:   record.SkillName,
		Level:       record.Level,
		Message:     record.Message,
		MetricName:  record.MetricName,
		MetricValue: record.MetricValue,
		CreatedAt:   record.CreatedAt,
	}
}

func SkillSettlementItemFromQuery(record skillkit.SettlementRecord) SkillSettlementItem {
	return SkillSettlementItem{
		ID:                record.ID,
		SkillID:           record.SkillID,
		SkillName:         record.SkillName,
		SkillSlug:         record.SkillSlug,
		AuthorName:        record.AuthorName,
		PeriodLabel:       record.PeriodLabel,
		SettlementStatus:  record.SettlementStatus,
		GrossAmount:       record.GrossAmount,
		PlatformFeeAmount: record.PlatformFeeAmount,
		PayoutAmount:      record.PayoutAmount,
		FrozenAmount:      record.FrozenAmount,
		Currency:          record.Currency,
		Note:              record.Note,
		CreatedAt:         record.CreatedAt,
		UpdatedAt:         record.UpdatedAt,
	}
}

func SkillReviewSummaryFromItems(items []SkillReviewItem) map[string]any {
	summary := map[string]any{
		"pending_count":   0,
		"approved_count":  0,
		"rejected_count":  0,
		"high_risk_count": 0,
	}
	for _, item := range items {
		switch item.ReviewStatus {
		case "approved":
			count, _ := summary["approved_count"].(int)
			summary["approved_count"] = count + 1
		case "rejected":
			count, _ := summary["rejected_count"].(int)
			summary["rejected_count"] = count + 1
		default:
			count, _ := summary["pending_count"].(int)
			summary["pending_count"] = count + 1
		}
		if item.RiskLevel == "high" {
			count, _ := summary["high_risk_count"].(int)
			summary["high_risk_count"] = count + 1
		}
	}
	return summary
}

func SkillGovernanceSummaryFromItems(items []SkillGovernanceItem) map[string]any {
	summary := map[string]any{
		"total_count":            len(items),
		"online_count":           0,
		"force_private_count":    0,
		"disabled_count":         0,
		"pending_versions_count": 0,
	}
	for _, item := range items {
		switch item.GovernanceStatus {
		case "online":
			count, _ := summary["online_count"].(int)
			summary["online_count"] = count + 1
		case "force_private":
			count, _ := summary["force_private_count"].(int)
			summary["force_private_count"] = count + 1
		case "disabled":
			count, _ := summary["disabled_count"].(int)
			summary["disabled_count"] = count + 1
		}
		if item.LatestReviewStatus == "pending" {
			count, _ := summary["pending_versions_count"].(int)
			summary["pending_versions_count"] = count + 1
		}
	}
	return summary
}

func SkillRuntimeSummaryFromItems(items []SkillRuntimeItem) map[string]any {
	summary := map[string]any{
		"total_skills":   len(items),
		"active_skills":  0,
		"requests_24h":   int64(0),
		"success_rate":   0.0,
		"p95_latency_ms": 0.0,
		"warning_count":  0,
		"critical_count": 0,
	}
	if len(items) == 0 {
		return summary
	}
	var successSum float64
	var p95Max float64
	for _, item := range items {
		if item.Requests24H > 0 {
			count, _ := summary["active_skills"].(int)
			summary["active_skills"] = count + 1
		}
		requests, _ := summary["requests_24h"].(int64)
		summary["requests_24h"] = requests + item.Requests24H
		successSum += item.SuccessRate
		if item.P95LatencyMS > p95Max {
			p95Max = item.P95LatencyMS
		}
		switch item.HealthStatus {
		case "warning":
			count, _ := summary["warning_count"].(int)
			summary["warning_count"] = count + 1
		case "critical":
			count, _ := summary["critical_count"].(int)
			summary["critical_count"] = count + 1
		}
	}
	summary["success_rate"] = successSum / float64(len(items))
	summary["p95_latency_ms"] = p95Max
	return summary
}

func SkillSettlementSummaryFromItems(items []SkillSettlementItem) map[string]any {
	summary := map[string]any{
		"pending_amount":      0.0,
		"settled_amount":      0.0,
		"frozen_amount":       0.0,
		"pending_skill_count": 0,
		"currency":            "CNY",
	}
	for _, item := range items {
		summary["currency"] = fallbackString(item.Currency, "CNY")
		switch item.SettlementStatus {
		case "settled":
			amount, _ := summary["settled_amount"].(float64)
			summary["settled_amount"] = amount + item.PayoutAmount
		default:
			amount, _ := summary["pending_amount"].(float64)
			summary["pending_amount"] = amount + item.PayoutAmount
			count, _ := summary["pending_skill_count"].(int)
			summary["pending_skill_count"] = count + 1
		}
		amount, _ := summary["frozen_amount"].(float64)
		summary["frozen_amount"] = amount + item.FrozenAmount
	}
	return summary
}

func versionNameFromDomain(version *domain.AISkillVersion) string {
	meta := cloneAnyMap(version.Metadata)
	if value := stringOrFallback(meta["version_name"], ""); value != "" {
		return value
	}
	return fmt.Sprintf("v%d", version.Version)
}

func versionStatusFromDomain(version *domain.AISkillVersion) string {
	meta := cloneAnyMap(version.Metadata)
	switch stringOrFallback(meta["service_status_override"], "") {
	case "disabled":
		return "archived"
	case "draft":
		return "draft"
	}
	switch strings.TrimSpace(version.ReviewStatus) {
	case domain.AISkillVersionReviewStatusDraft, domain.AISkillVersionReviewStatusRejected:
		return "draft"
	default:
		if value := stringOrFallback(meta["presentation_status"], ""); value != "" {
			return value
		}
		return "published"
	}
}

func skillStatusFromMetadata(meta map[string]any, visibility string) string {
	if value := stringOrFallback(meta["status"], ""); value != "" {
		return value
	}
	if visibility == domain.AIVisibilityPublic {
		return "published"
	}
	return "draft"
}

func visibilityForResponse(raw string) string {
	if strings.TrimSpace(raw) == domain.AIVisibilityPublic {
		return domain.AIVisibilityPublic
	}
	return domain.AIVisibilityPrivate
}

func pricingFromMetadata(price float64, meta map[string]any) SkillPricing {
	pricing := SkillPricing{
		Mode:     "free",
		Amount:   0,
		Currency: "CNY",
	}
	if raw, ok := meta["pricing"].(map[string]any); ok {
		pricing.Mode = firstNonEmpty(stringOrFallback(raw["mode"], ""), pricing.Mode)
		pricing.Amount = floatOrZero(raw["amount"])
		pricing.Currency = fallbackString(stringOrFallback(raw["currency"], ""), pricing.Currency)
		if value, ok := optionalFloat(raw["settlement_ratio"]); ok {
			pricing.SettlementRatio = &value
		}
	}
	if price > 0 && pricing.Amount == 0 {
		pricing.Mode = "paid"
		pricing.Amount = price
	}
	return pricing
}

func variableSchemaFromMetadata(meta map[string]any) []map[string]any {
	raw, ok := meta["variable_schema"]
	if !ok {
		return []map[string]any{}
	}
	items, ok := raw.([]any)
	if !ok {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		record, ok := item.(map[string]any)
		if ok {
			out = append(out, cloneAnyMap(record))
		}
	}
	return out
}

func contentFromMetadata(meta map[string]any) map[string]any {
	raw, ok := meta["content"]
	if !ok {
		return map[string]any{}
	}
	record, ok := raw.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return cloneAnyMap(record)
}

func truncatePreview(raw string) string {
	text := strings.TrimSpace(raw)
	if len(text) <= 240 {
		return text
	}
	return text[:240]
}

func normalizeRunStatus(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "running", "processing", "dispatched":
		return "running"
	case "failed", "error":
		return "failed"
	case "cancelled", "canceled":
		return "cancelled"
	case "queued", "prepared":
		return "queued"
	default:
		return "succeeded"
	}
}

func stringListFromAny(raw any) []string {
	items, ok := raw.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func boolFromMetadata(meta map[string]any, key string) bool {
	raw, ok := meta[key]
	if !ok {
		return false
	}
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "1", "yes":
			return true
		}
	}
	return false
}

func intFromMetadata(meta map[string]any, key string, fallback int) int {
	raw, ok := meta[key]
	if !ok || raw == nil {
		return fallback
	}
	switch value := raw.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return fallback
		}
		parsed, err := strconv.Atoi(text)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func stringOrFallback(raw any, fallback string) string {
	if raw == nil {
		return fallback
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	if text == "" || text == "<nil>" {
		return fallback
	}
	return text
}

func fallbackString(raw string, fallback string) string {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	return raw
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func floatOrZero(raw any) float64 {
	switch value := raw.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func optionalFloat(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	default:
		return 0, false
	}
}
