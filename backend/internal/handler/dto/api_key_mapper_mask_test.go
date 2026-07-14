package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "standard", in: "sk-1234567890abcdef", want: "sk-1***********cdef"},
		{name: "short", in: "12345678", want: "********"},
		{name: "empty", in: "   ", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskAPIKey(tt.in); got != tt.want {
				t.Fatalf("MaskAPIKey(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestUsageLogMappersMaskNestedAPIKey(t *testing.T) {
	log := &service.UsageLog{APIKey: &service.APIKey{Key: "sk-1234567890abcdef"}}
	if got := UsageLogFromService(log).APIKey.Key; got != "sk-1***********cdef" {
		t.Fatalf("user usage API key = %q", got)
	}
	if got := UsageLogFromServiceAdmin(log).APIKey.Key; got != "sk-1***********cdef" {
		t.Fatalf("admin usage API key = %q", got)
	}
}
