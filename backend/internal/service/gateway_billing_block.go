package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	xxhash "github.com/cespare/xxhash/v2"
	"github.com/tidwall/gjson"
)

// fingerprintSalt 是计算 cc_version 后缀指纹的盐值。
//
// 来源：与 Parrot src/transform/cc_mimicry.py 的 FINGERPRINT_SALT 完全一致；
// 这是真实 Claude Code CLI 抓包推导出的常量，改动会导致 fp 与 CLI 不一致，
// 进一步触发 Anthropic 的第三方检测。
const fingerprintSalt = "59cf53e54c78"

// computeClaudeCodeFingerprint 复刻真实 Claude Code CLI 的 cc_version 指纹算法：
//
//  1. 取 messages 中第一条 role=user 的纯文本（首块 text）
//  2. 取该文本的第 4、7、20 字符（不足以 '0' 补齐）
//  3. SHA256(SALT + chars + cc_version) 取 hex 前 3 字符
//
// 算法来自 Parrot src/transform/cc_mimicry.py:compute_fingerprint，与官方 CLI 字节对齐。
// 任何偏差都会导致 cc_version=X.Y.Z.{fp} 在上游侧与真实 CLI 不一致。
func computeClaudeCodeFingerprint(body []byte, version string) string {
	firstText := extractFirstUserText(body)
	indices := []int{4, 7, 20}
	runes := []rune(firstText)
	chars := make([]rune, 0, 3)
	for _, i := range indices {
		if i < len(runes) {
			chars = append(chars, runes[i])
		} else {
			chars = append(chars, '0')
		}
	}
	sum := sha256.Sum256([]byte(fingerprintSalt + string(chars) + version))
	return hex.EncodeToString(sum[:])[:3]
}

func composeClaudeCodeBillingVersion(body []byte, version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	return fmt.Sprintf("%s.%s", version, computeClaudeCodeFingerprint(body, version))
}

// extractFirstUserText 提取 messages 中第一条 user 消息的首段 text 内容。
// 兼容 string 和 []block 两种 content 格式。
func extractFirstUserText(body []byte) string {
	messages := gjson.GetBytes(body, "messages")
	if !messages.IsArray() {
		return ""
	}
	first := ""
	messages.ForEach(func(_, msg gjson.Result) bool {
		if msg.Get("role").String() != "user" {
			return true
		}
		content := msg.Get("content")
		if content.Type == gjson.String {
			first = content.String()
			return false
		}
		if content.IsArray() {
			content.ForEach(func(_, block gjson.Result) bool {
				if block.Get("type").String() == "text" {
					first = block.Get("text").String()
					return false
				}
				return true
			})
			return false
		}
		return false
	})
	return first
}

// buildBillingAttributionBlockJSON 构造 system 数组的 billing attribution block。
//
// 形态严格对齐真实 Claude Code CLI：
//
//	{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.161.{fp}; cc_entrypoint=cli; cch=00000;"}
//
// cch=00000 是签名占位符，由 signBillingHeaderCCH 在 buildUpstreamRequest 阶段
// 替换为基于完整 body 的 xxhash64 5 位十六进制摘要。
//
// 此 block 不带 cache_control（与真实 CLI 一致；cache breakpoint 由后续的
// Claude Code prompt block 承担）。
func buildBillingAttributionText(body []byte, cliVersion string) (string, error) {
	if cliVersion == "" {
		return "", fmt.Errorf("cliVersion required")
	}
	fp := computeClaudeCodeFingerprint(body, cliVersion)
	return fmt.Sprintf(
		"x-anthropic-billing-header: cc_version=%s.%s; cc_entrypoint=cli; cch=00000;",
		cliVersion, fp,
	), nil
}

// cchSeed 是 Claude Code Zig 层 xxHash64 cch 计算的种子常量。
// 来自 bun-anthropic/src/http/Attestation.zig 中的 xxHash64 seed。
const cchSeed uint64 = 0x4d659218e32a3268

// signBillingHeaderCCH 对最终 body 执行 cch 签名：
//  1. 在 body 中查找占位符 "cch=00000"
//  2. 对含占位符的 body 做 xxHash64(body, cchSeed)
//  3. 取低 20 位 (& 0xFFFFF)，格式化为 5 位 hex
//  4. 替换占位符为 "cch={hash}"
//
// 与真实 CLI Zig 层行为对齐：hash 输入是含占位符的 body，替换后 body 长度不变。
// 不含占位符的 body 直接返回（非 Claude Code 伪装路径）。
func signBillingHeaderCCH(body []byte) []byte {
	placeholder := []byte("cch=00000")
	start := bytes.Index(body, []byte("x-anthropic-billing-header:"))
	if start < 0 {
		return body
	}
	offset := bytes.Index(body[start:], placeholder)
	if offset < 0 {
		return body
	}
	offset += start
	h := xxhash.NewWithSeed(cchSeed)
	_, _ = h.Write(body)
	digest := h.Sum64()
	signed := []byte(fmt.Sprintf("cch=%05x", digest&0xFFFFF))
	out := append([]byte(nil), body[:offset]...)
	out = append(out, signed...)
	out = append(out, body[offset+len(placeholder):]...)
	return out
}

// cchRealValueRe 匹配 body 中 cch=XXXXX（5 位 hex，非占位符 00000）。
var cchRealValueRe = regexp.MustCompile(`cch=([0-9a-f]{5})`)

// verifyCCHFromRealCLI 从真实 Claude Code CLI 请求的 body 中提取实际 cch 值，
// 将其替换回占位符 cch=00000 后用我们的算法重算，返回 (realCCH, ourCCH, match)。
//
// 如果 body 中不包含合法的 cch=XXXXX（非 00000），返回 ("", "", false)。
// 用于线上验证我们的 xxHash64 实现与真实 CLI Zig 层是否对齐。
func verifyCCHFromRealCLI(body []byte) (realCCH, ourCCH string, match bool) {
	loc := cchRealValueRe.FindSubmatchIndex(body)
	if loc == nil {
		return "", "", false
	}
	realCCH = string(body[loc[2]:loc[3]])
	if realCCH == "00000" {
		// 占位符，不是真实值
		return "", "", false
	}

	// 将真实 cch 替换回占位符
	placeholderBody := make([]byte, len(body))
	copy(placeholderBody, body)
	copy(placeholderBody[loc[2]:loc[3]], []byte("00000"))

	// 用我们的算法重算
	h := xxhash.NewWithSeed(cchSeed)
	_, _ = h.Write(placeholderBody)
	digest := h.Sum64()
	ourCCH = fmt.Sprintf("%05x", digest&0xFFFFF)

	return realCCH, ourCCH, realCCH == ourCCH
}
