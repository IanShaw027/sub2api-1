package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	kiropkg "github.com/Wei-Shaw/sub2api/internal/pkg/kiro"
	"github.com/stretchr/testify/require"
)

func TestExecuteCodeInExplicitSandbox_DefaultDisabled(t *testing.T) {
	stdout, stderr, hadErr := executeCodeInExplicitSandbox(context.Background(), "print('host should not run')", "python", nil)

	require.True(t, hadErr)
	require.Empty(t, stdout)
	require.Contains(t, stderr, "code execution sandbox is not configured")
}

func TestExecuteCodeInExplicitSandbox_UsesConfiguredCommandWithCodeOnStdin(t *testing.T) {
	settings := DefaultKiroRuntimeSettings()
	settings.CodeExecutionSandboxCommand = `printf 'sandbox:%s' "$(cat)"`

	stdout, stderr, hadErr := executeCodeInExplicitSandbox(context.Background(), "print(42)", "python", settings)

	require.False(t, hadErr)
	require.Empty(t, stderr)
	require.Equal(t, "sandbox:print(42)", stdout)
}

func TestExecuteCodeInExplicitSandbox_DoesNotInheritHostEnvironment(t *testing.T) {
	t.Setenv("KIRO_SANDBOX_SHOULD_NOT_LEAK", "host-secret")
	settings := DefaultKiroRuntimeSettings()
	settings.CodeExecutionSandboxCommand = `printf 'leak=%s lang=%s code=%s' "${KIRO_SANDBOX_SHOULD_NOT_LEAK:-}" "$KIRO_CODE_EXECUTION_LANGUAGE" "$(cat)"`

	stdout, stderr, hadErr := executeCodeInExplicitSandbox(context.Background(), "print(1)", "python", settings)

	require.False(t, hadErr)
	require.Empty(t, stderr)
	require.Equal(t, "leak= lang=python code=print(1)", stdout)
}

func TestExecuteKiroShadowTool_CodeExecutionDefaultReturnsTypedSandboxError(t *testing.T) {
	state := &kiroToolState{ToolUseID: "code-1", Name: "code_execution"}
	state.InputBuilder.WriteString(`{"language":"python","code":"print('host must not run')"}`)
	svc := &KiroGatewayService{}

	blocks, summary, err := svc.executeKiroShadowTool(context.Background(), nil, state, kiropkg.ShadowToolBridge{
		AnthropicType: "code_execution",
		AnthropicName: "code_execution",
	})

	require.NoError(t, err)
	require.Len(t, blocks, 2)
	require.Equal(t, "server_tool_use", blocks[0]["type"])
	result := blocks[1]
	require.Equal(t, "code_execution_tool_result", result["type"])
	require.Equal(t, true, result["is_error"])
	require.Contains(t, summary, "code execution sandbox is not configured")
	content, _ := result["content"].([]any)
	require.Len(t, content, 1)
	codeResult, _ := content[0].(map[string]any)
	require.Equal(t, float64(1), float64(codeResult["return_code"].(int)))
	require.Contains(t, codeResult["stderr"], "sandbox is not configured")
}

func TestExecuteKiroShadowTool_CodeExecutionUsesConfiguredSandboxCommand(t *testing.T) {
	kiroRuntimeSettingsCache.Store((*cachedKiroRuntimeSettings)(nil))
	kiroRuntimeSettingsSF.Forget("kiro_runtime")
	t.Cleanup(func() {
		kiroRuntimeSettingsCache.Store((*cachedKiroRuntimeSettings)(nil))
		kiroRuntimeSettingsSF.Forget("kiro_runtime")
	})
	repo := &kiroRuntimeSettingRepoStub{
		values: map[string]string{
			SettingKeyKiroCodeExecutionSandboxCommand: `printf 'lang=%s code=%s' "$KIRO_CODE_EXECUTION_LANGUAGE" "$(cat)"`,
		},
	}
	svc := &KiroGatewayService{settingService: NewSettingService(repo, &config.Config{})}
	state := &kiroToolState{ToolUseID: "code-2", Name: "code_execution"}
	state.InputBuilder.WriteString(`{"language":"python","code":"print(7)"}`)

	blocks, summary, err := svc.executeKiroShadowTool(context.Background(), nil, state, kiropkg.ShadowToolBridge{
		AnthropicType: "code_execution",
		AnthropicName: "code_execution",
	})

	require.NoError(t, err)
	require.Len(t, blocks, 2)
	result := blocks[1]
	require.NotContains(t, result, "is_error")
	require.Contains(t, summary, "lang=python code=print(7)")
	content, _ := result["content"].([]any)
	codeResult, _ := content[0].(map[string]any)
	require.Equal(t, 0, codeResult["return_code"])
	require.Equal(t, "lang=python code=print(7)", strings.TrimSpace(codeResult["stdout"].(string)))
}
