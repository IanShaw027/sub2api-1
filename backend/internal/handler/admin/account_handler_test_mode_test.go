package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResolveTestAccountModes(t *testing.T) {
	tests := []struct {
		name          string
		req           TestAccountRequest
		wantMode      string
		wantImageMode string
	}{
		{
			name: "ignores removed web2api image route",
			req: TestAccountRequest{
				Mode:     service.AccountTestModeCompact,
				TestMode: "web2api",
			},
			wantMode:      service.AccountTestModeCompact,
			wantImageMode: "",
		},
		{
			name: "separates default mode from codex image route",
			req: TestAccountRequest{
				Mode:     service.AccountTestModeDefault,
				TestMode: "codex",
			},
			wantMode:      service.AccountTestModeDefault,
			wantImageMode: "codex",
		},
		{
			name: "falls back to legacy compact in test_mode",
			req: TestAccountRequest{
				TestMode: " compact ",
			},
			wantMode:      service.AccountTestModeCompact,
			wantImageMode: "",
		},
		{
			name: "drops removed image route when no explicit mode is sent",
			req: TestAccountRequest{
				TestMode: "web2api",
			},
			wantMode:      "",
			wantImageMode: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMode, gotImageMode := resolveTestAccountModes(tt.req)
			require.Equal(t, tt.wantMode, gotMode)
			require.Equal(t, tt.wantImageMode, gotImageMode)
		})
	}
}
