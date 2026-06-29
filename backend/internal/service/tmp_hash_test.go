//go:build cch_corpus

package service

import (
	"fmt"
	"testing"
)

func TestTmpHash(t *testing.T) {
	first := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"user":"client-user","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	full := []byte(`{"model":"gpt-5.4","instructions":"test","stream":true,"store":false,"user":"client-user","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"again"}]}]}`)
	n := NewFingerprintNormalizer(nil, nil, nil, &CanonicalFingerprintConfig{Enabled: true, AntiBanEnabled: true, EnabledByPlatform: map[string]bool{"openai": true}}, nil)
	acc := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	_, nf, _ := n.ApplyToRequest(nil, full, n.ResolveCanonical(nil, acc, "custom-http-client/1.0"))
	h1, _, _ := openAIWSNonInputFingerprint(first)
	h2, _, _ := openAIWSNonInputFingerprint(full)
	h3, _, _ := openAIWSNonInputFingerprint(nf)
	fmt.Printf("first=%x\nfull=%x\nnorm=%x\n", h1, h2, h3)
}
