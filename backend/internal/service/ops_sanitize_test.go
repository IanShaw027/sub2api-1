package service

import (
	"testing"
)

func TestSanitizeAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "长 API Key",
			input:    "sk-1234567890abcdefghij",
			expected: "sk-12345***",
		},
		{
			name:     "短 API Key",
			input:    "sk-123",
			expected: "***",
		},
		{
			name:     "空字符串",
			input:    "",
			expected: "***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeAPIKey(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeAPIKey(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHashSensitiveData(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "Email",
			input: "user@example.com",
		},
		{
			name:  "User ID",
			input: "12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hashSensitiveData(tt.input)
			if len(result) != 16 {
				t.Errorf("hashSensitiveData(%q) length = %d, want 16", tt.input, len(result))
			}
			// 验证相同输入产生相同哈希
			result2 := hashSensitiveData(tt.input)
			if result != result2 {
				t.Errorf("hashSensitiveData not deterministic")
			}
		})
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		notContains []string
	}{
		{
			name:  "脱敏 API Key",
			input: "Error: API key sk-1234567890abcdefghij is invalid",
			contains: []string{"sk-12345***"},
			notContains: []string{"sk-1234567890abcdefghij"},
		},
		{
			name:  "脱敏 Bearer Token",
			input: "Authorization failed: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			contains: []string{"***"},
			notContains: []string{"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
		},
		{
			name:  "脱敏 Email",
			input: "User user@example.com not found",
			contains: []string{"email_"},
			notContains: []string{"user@example.com"},
		},
		{
			name:  "脱敏 URL 查询参数",
			input: "Request failed: https://api.example.com/v1/chat?api_key=secret123&model=gpt-4",
			contains: []string{"api_key=***"},
			notContains: []string{"secret123"},
		},
		{
			name:  "多种敏感信息混合",
			input: "Error with key=sk-abc123def456 for user@test.com using token=bearer_xyz789",
			contains: []string{"***", "email_"},
			notContains: []string{"sk-abc123def456", "user@test.com", "bearer_xyz789"},
		},
		{
			name:  "无敏感信息",
			input: "Normal error message without sensitive data",
			contains: []string{"Normal error message"},
			notContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeErrorMessage(tt.input)

			for _, s := range tt.contains {
				if !contains(result, s) {
					t.Errorf("sanitizeErrorMessage(%q) should contain %q, got %q", tt.input, s, result)
				}
			}

			for _, s := range tt.notContains {
				if contains(result, s) {
					t.Errorf("sanitizeErrorMessage(%q) should not contain %q, got %q", tt.input, s, result)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
