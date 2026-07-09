package kiro

import "testing"

func TestBuildCodeWhispererStreamingUserAgents(t *testing.T) {
	xAmzUA, ua := BuildCodeWhispererStreamingUserAgents("0.10.0", "machine-1", "darwin#24.6.0", "22.21.1")

	wantXAmz := "aws-sdk-js/" + CodeWhispererSDKVersion + " KiroIDE-0.10.0-machine-1"
	if xAmzUA != wantXAmz {
		t.Fatalf("x-amz-user-agent = %q, want %q", xAmzUA, wantXAmz)
	}

	wantUA := "aws-sdk-js/" + CodeWhispererSDKVersion +
		" ua/2.1 os/darwin#24.6.0 lang/js md/nodejs#22.21.1 api/codewhispererstreaming#" +
		CodeWhispererAPIVersion + " m/E KiroIDE-0.10.0-machine-1"
	if ua != wantUA {
		t.Fatalf("User-Agent = %q, want %q", ua, wantUA)
	}
}

func TestBuildCodeWhispererRuntimeUserAgents(t *testing.T) {
	xAmzUA, ua := BuildCodeWhispererRuntimeUserAgents("0.10.0", "machine-1", "darwin#24.6.0", "22.21.1")

	wantXAmz := "aws-sdk-js/" + CodeWhispererSDKVersion + " KiroIDE-0.10.0-machine-1"
	if xAmzUA != wantXAmz {
		t.Fatalf("x-amz-user-agent = %q, want %q", xAmzUA, wantXAmz)
	}

	wantUA := "aws-sdk-js/" + CodeWhispererSDKVersion +
		" ua/2.1 os/darwin#24.6.0 lang/js md/nodejs#22.21.1 api/codewhispererruntime#" +
		CodeWhispererAPIVersion + " m/N,E KiroIDE-0.10.0-machine-1"
	if ua != wantUA {
		t.Fatalf("User-Agent = %q, want %q", ua, wantUA)
	}
}

func TestBuildSSOOIDCUserAgents(t *testing.T) {
	xAmzUA, ua := BuildSSOOIDCUserAgents("0.10.0", "machine-1", "darwin#24.6.0", "22.21.1")

	wantXAmz := "aws-sdk-js/" + SSOOIDCSDKVersion + " KiroIDE-0.10.0-machine-1"
	if xAmzUA != wantXAmz {
		t.Fatalf("x-amz-user-agent = %q, want %q", xAmzUA, wantXAmz)
	}

	wantUA := "aws-sdk-js/" + SSOOIDCSDKVersion +
		" ua/2.1 os/darwin#24.6.0 lang/js md/nodejs#22.21.1 api/sso-oidc#" +
		SSOOIDCAPIVersion + " m/E KiroIDE-0.10.0-machine-1"
	if ua != wantUA {
		t.Fatalf("User-Agent = %q, want %q", ua, wantUA)
	}
}
