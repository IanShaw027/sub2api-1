package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	xxhash "github.com/cespare/xxhash/v2"
	"github.com/stretchr/testify/require"
)

func TestComputeClaudeCodeFingerprint_UsesRunePositions(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"你好世界abc"}]}`)
	version := "2.1.92"

	got := computeClaudeCodeFingerprint(body, version)

	// "你好世界abc" at rune index 4 is "a"; 7 and 20 are missing => "0".
	sum := sha256.Sum256([]byte(fingerprintSalt + "a00" + version))
	want := hex.EncodeToString(sum[:])[:3]
	if got != want {
		t.Fatalf("fingerprint mismatch: got=%q want=%q", got, want)
	}
}

func TestComposeClaudeCodeBillingVersion(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"hello world"}]}`)
	version := "2.1.92"

	got := composeClaudeCodeBillingVersion(body, version)

	require.Equal(t, version+"."+computeClaudeCodeFingerprint(body, version), got)
}

func TestSignBillingHeaderCCH_ReplacesPlaceholder(t *testing.T) {
	body := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.161.abc; cc_entrypoint=cli; cch=00000;"}],"messages":[{"role":"user","content":"hello"}]}`)

	signed := signBillingHeaderCCH(body)

	// 占位符应被替换
	require.NotContains(t, string(signed), "cch=00000")
	// 应包含 cch= 后跟 5 位 hex
	require.Contains(t, string(signed), "cch=")
	// 长度不变（等长替换）
	require.Equal(t, len(body), len(signed))

	// 提取 cch 值并验证格式
	idx := strings.Index(string(signed), "cch=")
	cchVal := string(signed)[idx+4 : idx+9]
	for _, c := range cchVal {
		require.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"cch value should be hex, got char: %c", c)
	}
}

func TestSignBillingHeaderCCH_NoPlaceholder(t *testing.T) {
	body := []byte(`{"system":[{"type":"text","text":"no billing header here"}],"messages":[]}`)

	signed := signBillingHeaderCCH(body)

	// 不含占位符的 body 原样返回
	require.Equal(t, body, signed)
}

func TestSignBillingHeaderCCH_Deterministic(t *testing.T) {
	body := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.161.abc; cc_entrypoint=cli; cch=00000;"}],"messages":[{"role":"user","content":"test"}]}`)

	signed1 := signBillingHeaderCCH(body)
	// 用原始 body 再签一次（不能用 signed1，因为占位符已被替换）
	signed2 := signBillingHeaderCCH([]byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.161.abc; cc_entrypoint=cli; cch=00000;"}],"messages":[{"role":"user","content":"test"}]}`))

	require.Equal(t, signed1, signed2, "相同输入应产生相同的 cch")
}

// TestSignBillingHeaderCCH_PrintVector 打印 cch 计算的中间值，
// 供与真实 Claude Code CLI 抓包结果做人工对比。
// 运行方式: go test -tags unit -v -run TestSignBillingHeaderCCH_PrintVector ./internal/service/
func TestSignBillingHeaderCCH_PrintVector(t *testing.T) {
	body := []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.161.abc; cc_entrypoint=cli; cch=00000;"}],"messages":[{"role":"user","content":"hello"}],"model":"claude-sonnet-4-20250514","max_tokens":8096,"stream":true}`)

	h := xxhash.NewWithSeed(cchSeed)
	h.Write(body)
	digest := h.Sum64()
	masked := digest & 0xFFFFF
	cchHex := fmt.Sprintf("%05x", masked)

	t.Logf("=== CCH Verification Vector ===")
	t.Logf("Seed:       0x%016x", cchSeed)
	t.Logf("Body len:   %d", len(body))
	t.Logf("xxHash64:   0x%016x", digest)
	t.Logf("Masked:     0x%05x", masked)
	t.Logf("CCH value:  %s", cchHex)

	signed := signBillingHeaderCCH(body)
	require.Contains(t, string(signed), "cch="+cchHex)
	require.Equal(t, len(body), len(signed))
	t.Logf("Signed OK:  cch=%s", cchHex)
}

// TestSignBillingHeaderCCH_VerifyAgainstRealCLI 用于验证我们的 cch 算法与真实 CLI 输出一致。
//
// 使用方法：
//  1. 用 mitmproxy / Wireshark 抓取真实 Claude Code CLI 发送的请求 body
//  2. 提取 body 中 "cch=XXXXX" 的真实值
//  3. 将 body 中 "cch=XXXXX" 替换回 "cch=00000" 得到 placeholder body
//  4. 添加到 testCases 中
//
// go test -tags unit -v -run TestSignBillingHeaderCCH_VerifyAgainstRealCLI ./internal/service/
func TestSignBillingHeaderCCH_VerifyAgainstRealCLI(t *testing.T) {
	type testCase struct {
		name            string
		placeholderBody []byte // body with cch=00000 (replace real cch back to 00000)
		expectedCCH     string // 5-hex-char cch from real CLI (e.g. "a1b2c")
	}

	testCases := []testCase{
		// TODO: 从真实 CLI 抓包填充。
		// 示例：
		// {
		//     name: "real_cli_2.1.195_sonnet",
		//     placeholderBody: []byte(`{"system":[{"type":"text","text":"x-anthropic-billing-header: cc_version=2.1.195.abc; cc_entrypoint=cli; cch=00000;"}],...}`),
		//     expectedCCH: "a1b2c",
		// },
	}

	if len(testCases) == 0 {
		t.Skip("No real CLI test vectors provided yet — add captured request bodies to testCases")
	}

	passed := 0
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, bytes.Contains(tc.placeholderBody, []byte("cch=00000")),
				"placeholderBody must contain cch=00000")

			signed := signBillingHeaderCCH(tc.placeholderBody)
			expectedTag := "cch=" + tc.expectedCCH
			require.Contains(t, string(signed), expectedTag,
				"cch mismatch: expected %q in signed body", expectedTag)
			require.Equal(t, len(tc.placeholderBody), len(signed), "body length must not change")
			passed++
		})
	}
	t.Logf("Passed %d/%d real CLI verification cases", passed, len(testCases))
}
