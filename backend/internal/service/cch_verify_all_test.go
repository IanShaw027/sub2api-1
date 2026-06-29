//go:build cch_corpus

package service

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	xxhash "github.com/cespare/xxhash/v2"
)

// TestCCH_VerifyAllDebugBodies 用所有保存的真实 CLI body 验证 cch seed 是否正确。
// 只测试干净的文件（body 中只有 1 次 cch= 出现）。
func TestCCH_VerifyAllDebugBodies(t *testing.T) {
	debugDir := "/opt/sub2api/data/logs/gateway-debug"
	matches, err := filepath.Glob(filepath.Join(debugDir, "cch_debug_*.json"))
	if err != nil || len(matches) == 0 {
		t.Skipf("no debug files found in %s", debugDir)
	}

	cchRe := regexp.MustCompile(`cch=([0-9a-f]{5})`)
	seed := cchSeed // 来自 gateway_billing_block.go

	var total, clean, matched, mismatched int

	for _, path := range matches {
		total++
		body, err := os.ReadFile(path)
		if err != nil {
			t.Logf("SKIP %s: read error: %v", filepath.Base(path), err)
			continue
		}

		// 统计 cch= 出现次数，跳过被污染的文件
		allMatches := cchRe.FindAllSubmatch(body, -1)
		if len(allMatches) != 1 {
			t.Logf("SKIP %s: %d cch= occurrences (contaminated)", filepath.Base(path), len(allMatches))
			continue
		}
		clean++

		realCCH := string(allMatches[0][1])
		// 从文件名中提取 expected cch 做交叉验证
		base := filepath.Base(path)
		filenameCCH := strings.TrimSuffix(strings.TrimPrefix(base, "cch_debug_"), ".json")
		if realCCH != filenameCCH {
			t.Logf("WARN %s: filename cch=%s but body cch=%s", base, filenameCCH, realCCH)
		}

		// 找到 cch= 的位置并替换回占位符
		cchTag := []byte("cch=" + realCCH)
		idx := bytes.Index(body, cchTag)
		if idx < 0 {
			t.Logf("SKIP %s: cch=%s not found", base, realCCH)
			continue
		}

		placeholderBody := make([]byte, len(body))
		copy(placeholderBody, body)
		copy(placeholderBody[idx+4:idx+9], []byte("00000"))

		// 计算
		h := xxhash.NewWithSeed(seed)
		h.Write(placeholderBody)
		digest := h.Sum64()
		ourCCH := fmt.Sprintf("%05x", digest&0xFFFFF)

		if ourCCH == realCCH {
			matched++
			t.Logf("MATCH  %s: cch=%s (size=%d)", base, realCCH, len(body))
		} else {
			mismatched++
			t.Errorf("MISMATCH %s: real=%s ours=%s (size=%d, xxh64=0x%016x)", base, realCCH, ourCCH, len(body), digest)
		}
	}

	t.Logf("")
	t.Logf("=== SUMMARY ===")
	t.Logf("Total files:  %d", total)
	t.Logf("Clean files:  %d", clean)
	t.Logf("Matched:      %d", matched)
	t.Logf("Mismatched:   %d", mismatched)

	if mismatched > 0 {
		t.Errorf("seed 0x%016x has %d mismatches out of %d clean files", seed, mismatched, clean)
	} else if matched > 0 {
		t.Logf("seed 0x%016x verified against %d clean bodies", seed, matched)
	}
}
