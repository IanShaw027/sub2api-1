package service

import "testing"

func TestIsOpenAIUnsupportedPreviousResponseIDError(t *testing.T) {
	cases := []struct {
		name string
		code string
		msg  string
		want bool
	}{
		{
			name: "unsupported parameter previous_response_id",
			msg:  "Unsupported parameter: previous_response_id",
			want: true,
		},
		{
			name: "unknown parameter previous_response_id",
			msg:  "Unknown parameter: previous_response_id.",
			want: true,
		},
		{
			name: "previous_response_id not supported",
			msg:  "previous_response_id is not supported for this endpoint",
			want: true,
		},
		{
			name: "code unsupported_parameter with previous_response_id mention",
			code: "unsupported_parameter",
			msg:  "previous_response_id cannot be used here",
			want: true,
		},
		{
			name: "encrypted content unrelated",
			msg:  "The encrypted content could not be verified",
			want: false,
		},
		{
			name: "previous_response_id absent",
			msg:  "Unsupported parameter: store",
			want: false,
		},
		{
			name: "empty",
			msg:  "",
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOpenAIUnsupportedPreviousResponseIDError(tc.code, tc.msg); got != tc.want {
				t.Fatalf("isOpenAIUnsupportedPreviousResponseIDError(%q,%q) = %v, want %v", tc.code, tc.msg, got, tc.want)
			}
		})
	}
}
