package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	archiveImportMaxSize          = 64 << 20 // 64 MiB
	archiveImportMaxFiles         = 5000
	archiveImportMaxJSONSize      = 4 << 20 // 4 MiB per JSON entry
	archiveImportMaxTotalJSONSize = archiveImportMaxSize
)

// ArchiveImportResult 是 zip 导入的聚合结果，分别返回 sub2api 备份和 codex session 两条管线的统计。
type ArchiveImportResult struct {
	Format         string                    `json:"format"`
	TotalEntries   int                       `json:"total_entries"`
	SubAPIEntries  int                       `json:"sub2api_entries"`
	CodexEntries   int                       `json:"codex_entries"`
	UnknownEntries int                       `json:"unknown_entries"`
	UnknownNames   []string                  `json:"unknown_names,omitempty"`
	SubAPIResult   *DataImportResult         `json:"sub2api_result,omitempty"`
	CodexResult    *CodexSessionImportResult `json:"codex_result,omitempty"`
	ParseErrors    []ArchiveImportParseError `json:"parse_errors,omitempty"`
}

type ArchiveImportParseError struct {
	Entry   string `json:"entry"`
	Message string `json:"message"`
}

// ImportArchive 接受一个 multipart 上传的 zip，自动识别 sub2api 备份格式与 codex OAuth 单文件格式
// 并复用各自的导入管线。
// POST /api/v1/admin/accounts/import/archive (multipart/form-data, file=<zip>)
func (h *AccountHandler) ImportArchive(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}
	if fileHeader.Size <= 0 {
		response.BadRequest(c, "uploaded file is empty")
		return
	}
	if fileHeader.Size > archiveImportMaxSize {
		response.BadRequest(c, fmt.Sprintf("archive too large (max %d bytes)", archiveImportMaxSize))
		return
	}

	dedupMode := strings.TrimSpace(c.PostForm("dedup_mode"))
	skipDefaultGroupBind := parseFormBool(c.PostForm("skip_default_group_bind"), true)
	updateExistingCodex := parseFormBool(c.PostForm("update_existing"), true)
	confirmMixedRisk := parseFormBool(c.PostForm("confirm_mixed_channel_risk"), false)

	buf, err := readUploadedArchive(fileHeader)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	parsed, parseErrs, err := parseImportArchive(buf)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(parsed.subAPIAccounts) == 0 && len(parsed.subAPIProxies) == 0 && len(parsed.codexContents) == 0 {
		response.BadRequest(c, "未能在压缩包中识别到任何 sub2api 备份或 codex session JSON")
		return
	}

	// 幂等键基于文件内容，避免重复点击导致重复入库。
	idempotencyPayload := map[string]any{
		"sha256":             parsed.archiveDigest,
		"size":               fileHeader.Size,
		"dedup_mode":         dedupMode,
		"skip_default_group": skipDefaultGroupBind,
		"update_existing":    updateExistingCodex,
		"confirm_mixed":      confirmMixedRisk,
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_archive", idempotencyPayload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		result := ArchiveImportResult{
			TotalEntries:   parsed.totalJSONEntries,
			SubAPIEntries:  len(parsed.subAPIAccounts),
			CodexEntries:   len(parsed.codexContents),
			UnknownEntries: parsed.unknownEntries,
			UnknownNames:   parsed.unknownNames,
			ParseErrors:    parseErrs,
		}

		switch {
		case len(parsed.subAPIAccounts) > 0 && len(parsed.codexContents) > 0:
			result.Format = "mixed"
		case len(parsed.subAPIAccounts) > 0:
			result.Format = "sub2api"
		case len(parsed.codexContents) > 0:
			result.Format = "codex"
		}

		if len(parsed.subAPIAccounts) > 0 || len(parsed.subAPIProxies) > 0 {
			payload := DataPayload{
				Type:       dataType,
				Version:    dataVersion,
				ExportedAt: time.Now().UTC().Format(time.RFC3339),
				Proxies:    parsed.subAPIProxies,
				Accounts:   parsed.subAPIAccounts,
			}
			if payload.Proxies == nil {
				payload.Proxies = []DataProxy{}
			}
			if payload.Accounts == nil {
				payload.Accounts = []DataAccount{}
			}
			if err := validateDataHeader(payload); err != nil {
				return result, err
			}
			subResult, err := h.importData(ctx, payload, &skipDefaultGroupBind, dedupMode)
			if err != nil {
				return result, err
			}
			result.SubAPIResult = &subResult
		}

		if len(parsed.codexContents) > 0 {
			req := CodexSessionImportRequest{
				Contents:                parsed.codexContents,
				UpdateExisting:          &updateExistingCodex,
				SkipDefaultGroupBind:    &skipDefaultGroupBind,
				ConfirmMixedChannelRisk: &confirmMixedRisk,
			}
			entries, err := parseCodexSessionImportEntries(req)
			if err != nil {
				return result, err
			}
			codexResult, err := h.importCodexSessions(ctx, req, entries)
			if err != nil {
				return result, err
			}
			result.CodexResult = &codexResult
		}

		return result, nil
	})
}

type parsedArchive struct {
	subAPIProxies    []DataProxy
	subAPIAccounts   []DataAccount
	codexContents    []string
	unknownEntries   int
	unknownNames     []string
	totalJSONEntries int
	archiveDigest    string
}

func parseImportArchive(buf []byte) (*parsedArchive, []ArchiveImportParseError, error) {
	reader, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return nil, nil, fmt.Errorf("无效的 zip 文件: %w", err)
	}
	if len(reader.File) > archiveImportMaxFiles {
		return nil, nil, fmt.Errorf("压缩包内文件过多 (最多 %d)", archiveImportMaxFiles)
	}

	parsed := &parsedArchive{}
	parseErrs := make([]ArchiveImportParseError, 0)
	parsed.archiveDigest = sha256Hex(buf)

	proxyKeySeen := map[string]struct{}{}
	var totalJSONBytes int64

	for _, f := range reader.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// 仅处理 JSON 文件，忽略其他附件（README、签名等）
		if !strings.EqualFold(filepath.Ext(f.Name), ".json") {
			continue
		}
		if int64(f.UncompressedSize64) > archiveImportMaxJSONSize {
			parseErrs = append(parseErrs, ArchiveImportParseError{
				Entry:   f.Name,
				Message: fmt.Sprintf("JSON 文件过大 (>%d bytes)", archiveImportMaxJSONSize),
			})
			continue
		}
		if totalJSONBytes+int64(f.UncompressedSize64) > archiveImportMaxTotalJSONSize {
			return nil, parseErrs, fmt.Errorf("压缩包内 JSON 总大小超过 %d 字节限制", archiveImportMaxTotalJSONSize)
		}
		parsed.totalJSONEntries++

		raw, err := readZipEntry(f, archiveImportMaxJSONSize)
		if err != nil {
			parseErrs = append(parseErrs, ArchiveImportParseError{Entry: f.Name, Message: err.Error()})
			continue
		}
		if int64(len(raw)) > archiveImportMaxJSONSize {
			parseErrs = append(parseErrs, ArchiveImportParseError{
				Entry:   f.Name,
				Message: fmt.Sprintf("JSON 文件过大 (>%d bytes)", archiveImportMaxJSONSize),
			})
			continue
		}
		totalJSONBytes += int64(len(raw))
		if totalJSONBytes > archiveImportMaxTotalJSONSize {
			return nil, parseErrs, fmt.Errorf("压缩包内 JSON 总大小超过 %d 字节限制", archiveImportMaxTotalJSONSize)
		}

		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 {
			continue
		}

		// 优先尝试 sub2api 备份格式 (顶层包含 accounts 数组)
		if isSubAPIBackupJSON(trimmed) {
			var payload DataPayload
			if err := json.Unmarshal(trimmed, &payload); err != nil {
				parseErrs = append(parseErrs, ArchiveImportParseError{
					Entry:   f.Name,
					Message: "sub2api 备份解析失败: " + err.Error(),
				})
				continue
			}
			parsed.subAPIAccounts = append(parsed.subAPIAccounts, payload.Accounts...)
			for _, p := range payload.Proxies {
				key := p.ProxyKey
				if key == "" {
					key = buildProxyKey(p.Protocol, p.Host, p.Port, p.Username, p.Password)
				}
				if _, ok := proxyKeySeen[key]; ok {
					continue
				}
				proxyKeySeen[key] = struct{}{}
				parsed.subAPIProxies = append(parsed.subAPIProxies, p)
			}
			continue
		}

		// 其次尝试 codex OAuth 单条 JSON
		if isCodexSessionJSON(trimmed) {
			parsed.codexContents = append(parsed.codexContents, string(trimmed))
			continue
		}

		parsed.unknownEntries++
		if len(parsed.unknownNames) < 10 {
			parsed.unknownNames = append(parsed.unknownNames, f.Name)
		}
	}

	return parsed, parseErrs, nil
}

func readUploadedArchive(fileHeader *multipart.FileHeader) ([]byte, error) {
	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("无法读取上传文件: %w", err)
	}
	defer func() { _ = src.Close() }()

	limited := io.LimitReader(src, archiveImportMaxSize+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("读取上传文件失败: %w", err)
	}
	if int64(len(buf)) > archiveImportMaxSize {
		return nil, fmt.Errorf("上传文件超过 %d 字节限制", archiveImportMaxSize)
	}
	return buf, nil
}

func readZipEntry(f *zip.File, limit int64) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	raw, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("JSON 文件过大 (>%d bytes)", limit)
	}
	return raw, nil
}

// isSubAPIBackupJSON 判断 JSON 是否为 sub2api 导出的备份结构。
// 仅当顶层是对象、并且至少包含 accounts/proxies 任一数组字段时返回 true。
func isSubAPIBackupJSON(raw []byte) bool {
	var probe struct {
		Type     string          `json:"type"`
		Accounts json.RawMessage `json:"accounts"`
		Proxies  json.RawMessage `json:"proxies"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	if strings.TrimSpace(probe.Type) == dataType || strings.TrimSpace(probe.Type) == legacyDataType {
		return true
	}
	hasAccountsArray := len(probe.Accounts) > 0 && bytes.HasPrefix(bytes.TrimSpace(probe.Accounts), []byte("["))
	hasProxiesArray := len(probe.Proxies) > 0 && bytes.HasPrefix(bytes.TrimSpace(probe.Proxies), []byte("["))
	return hasAccountsArray || hasProxiesArray
}

// isCodexSessionJSON 判断 JSON 是否符合 codex OAuth 单文件结构 (含 access_token / accessToken)。
func isCodexSessionJSON(raw []byte) bool {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	for _, key := range []string{"access_token", "accessToken"} {
		if v, ok := probe[key]; ok && len(bytes.TrimSpace(v)) > 0 {
			return true
		}
	}
	if v, ok := probe["tokens"]; ok && len(bytes.TrimSpace(v)) > 0 {
		return true
	}
	return false
}

func parseFormBool(value string, defaultVal bool) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultVal
	}
	if v, err := strconv.ParseBool(value); err == nil {
		return v
	}
	return defaultVal
}

func sha256Hex(buf []byte) string {
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])
}
