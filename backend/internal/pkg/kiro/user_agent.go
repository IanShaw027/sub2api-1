package kiro

import "fmt"

const (
	CodeWhispererSDKVersion = "1.0.27"
	CodeWhispererAPIVersion = "1.0.27"
	SSOOIDCSDKVersion       = "3.738.0"
	SSOOIDCAPIVersion       = "3.738.0"
)

func BuildCodeWhispererStreamingUserAgents(kiroVersion, machineID, systemVersion, nodeVersion string) (string, string) {
	return fmt.Sprintf("aws-sdk-js/%s KiroIDE-%s-%s", CodeWhispererSDKVersion, kiroVersion, machineID),
		fmt.Sprintf("aws-sdk-js/%s ua/2.1 os/%s lang/js md/nodejs#%s api/codewhispererstreaming#%s m/E KiroIDE-%s-%s",
			CodeWhispererSDKVersion, systemVersion, nodeVersion, CodeWhispererAPIVersion, kiroVersion, machineID)
}

func BuildCodeWhispererRuntimeUserAgents(kiroVersion, machineID, systemVersion, nodeVersion string) (string, string) {
	return fmt.Sprintf("aws-sdk-js/%s KiroIDE-%s-%s", CodeWhispererSDKVersion, kiroVersion, machineID),
		fmt.Sprintf("aws-sdk-js/%s ua/2.1 os/%s lang/js md/nodejs#%s api/codewhispererruntime#%s m/N,E KiroIDE-%s-%s",
			CodeWhispererSDKVersion, systemVersion, nodeVersion, CodeWhispererAPIVersion, kiroVersion, machineID)
}

func BuildSSOOIDCUserAgents(kiroVersion, machineID, systemVersion, nodeVersion string) (string, string) {
	return fmt.Sprintf("aws-sdk-js/%s KiroIDE-%s-%s", SSOOIDCSDKVersion, kiroVersion, machineID),
		fmt.Sprintf("aws-sdk-js/%s ua/2.1 os/%s lang/js md/nodejs#%s api/sso-oidc#%s m/E KiroIDE-%s-%s",
			SSOOIDCSDKVersion, systemVersion, nodeVersion, SSOOIDCAPIVersion, kiroVersion, machineID)
}
