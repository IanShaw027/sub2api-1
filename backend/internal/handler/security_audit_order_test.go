package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type promptAuditOrderCase struct {
	file       string
	function   string
	auditToken string
}

func TestPromptAuditGatePrecedesAccountBillingAndUpstreamSideEffects(t *testing.T) {
	tests := []promptAuditOrderCase{
		{file: "gateway_handler.go", function: "Messages", auditToken: "checkSecurityAudit"},
		{file: "gateway_handler_chat_completions.go", function: "ChatCompletions", auditToken: "checkSecurityAudit"},
		{file: "gateway_handler_responses.go", function: "Responses", auditToken: "checkSecurityAudit"},
		{file: "gemini_v1beta_handler.go", function: "GeminiV1BetaModels", auditToken: "checkSecurityAudit"},
		{file: "openai_gateway_handler.go", function: "Responses", auditToken: "checkSecurityAudit"},
		{file: "openai_gateway_handler.go", function: "Messages", auditToken: "checkSecurityAudit"},
		{file: "openai_chat_completions.go", function: "ChatCompletions", auditToken: "checkSecurityAudit"},
		{file: "openai_images.go", function: "Images", auditToken: "checkSecurityAudit"},
		{file: "grok_media.go", function: "handleGrokMedia", auditToken: "checkSecurityAudit"},
		{file: "openai_embeddings.go", function: "Embeddings", auditToken: "checkSecurityAudit"},
		{file: "openai_alpha_search.go", function: "AlphaSearch", auditToken: "checkSecurityAudit"},
		{file: "image_task_handler.go", function: "SubmitWithLifecycle", auditToken: "checkSecurityAuditBeforeSubmit"},
		{file: "batch_image_handler.go", function: "Submit", auditToken: "checkSecurityAuditBeforeSubmit"},
	}
	sideEffectTokens := []string{
		"CheckBillingEligibility(", "SelectAccount", ".Forward", "acquireResponsesUserSlot(",
		"AcquireUserSlot", "TryAcquireUserSlot", "acquireImageGenerationSlot(",
		"h.tasks.Create(", "h.service.Submit(", "beforeStart(", "h.run(",
	}
	for _, tt := range tests {
		t.Run(tt.file+"/"+tt.function, func(t *testing.T) {
			functionSource := stripGoComments(goFunctionSource(t, tt.file, tt.function))
			auditIndex := strings.Index(functionSource, tt.auditToken)
			require.NotEqual(t, -1, auditIndex, "missing Prompt Audit gate")
			foundSideEffect := false
			for _, sideEffect := range sideEffectTokens {
				index := strings.Index(functionSource, sideEffect)
				if index < 0 {
					continue
				}
				foundSideEffect = true
				require.Lessf(t, auditIndex, index, "%s must run before %s", tt.auditToken, sideEffect)
			}
			require.True(t, foundSideEffect, "coverage case must contain a downstream side effect")
		})
	}
}

func TestAsyncImageSubmissionEntrypointsSharePromptAuditGate(t *testing.T) {
	parseBody := func(filename, name string) *ast.BlockStmt {
		source := "package handler\n" + goFunctionSource(t, filename, name)
		parsed, err := parser.ParseFile(token.NewFileSet(), filename, source, 0)
		require.NoError(t, err)
		return parsed.Decls[0].(*ast.FuncDecl).Body
	}
	submit := parseBody("image_task_handler.go", "Submit")
	require.Len(t, submit.List, 1, "public Submit must only delegate to the audited implementation")
	statement, ok := submit.List[0].(*ast.ExprStmt)
	require.True(t, ok)
	call, ok := statement.X.(*ast.CallExpr)
	require.True(t, ok)
	require.Equal(t, "h.SubmitWithLifecycle", promptAuditCallName(call.Fun))
	require.Len(t, call.Args, 3)
	require.Equal(t, "c", promptAuditCallName(call.Args[0]))
	require.Equal(t, "nil", promptAuditCallName(call.Args[1]))
	require.Equal(t, "nil", promptAuditCallName(call.Args[2]))

	lifecycle := parseBody("image_task_handler.go", "SubmitWithLifecycle")
	guardFound := false
	for _, statement := range lifecycle.List {
		guard, ok := statement.(*ast.IfStmt)
		if !ok {
			continue
		}
		condition, ok := guard.Cond.(*ast.UnaryExpr)
		if !ok || condition.Op != token.NOT {
			continue
		}
		call, ok := condition.X.(*ast.CallExpr)
		if !ok || promptAuditCallName(call.Fun) != "h.checkSecurityAuditBeforeSubmit" {
			continue
		}
		require.Len(t, guard.Body.List, 1, "denied audit must return before task creation or callbacks")
		_, returns := guard.Body.List[0].(*ast.ReturnStmt)
		require.True(t, returns)
		guardFound = true
	}
	require.True(t, guardFound, "the shared implementation must reject a denied audit")

	creation := parseBody("creation_handler.go", "ImagesAsync")
	delegations, jobCreates, callbackJobCreates := 0, 0, 0
	ast.Inspect(creation, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := promptAuditCallName(call.Fun)
		require.NotContains(t, []string{"h.asyncImage.execute", "h.asyncImage.run", "h.asyncImage.tasks.Create", "h.openAI.Images", "h.openAI.GrokImages"}, name,
			"creation must not bypass the shared audited submission path")
		if name == "h.creationService.CreateImageJob" {
			jobCreates++
		}
		if name != "h.asyncImage.SubmitWithLifecycle" {
			return true
		}
		delegations++
		require.Len(t, call.Args, 3)
		require.Equal(t, "c", promptAuditCallName(call.Args[0]))
		callback, ok := call.Args[1].(*ast.FuncLit)
		require.True(t, ok, "creation persistence must be deferred until after the shared audit")
		ast.Inspect(callback.Body, func(node ast.Node) bool {
			if nested, ok := node.(*ast.CallExpr); ok && promptAuditCallName(nested.Fun) == "h.creationService.CreateImageJob" {
				callbackJobCreates++
			}
			return true
		})
		return true
	})
	require.Equal(t, 1, delegations)
	require.Equal(t, 1, jobCreates)
	require.Equal(t, jobCreates, callbackJobCreates, "all creation persistence must occur inside the audited beforeStart callback")
}

func promptAuditCallName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return promptAuditCallName(value.X) + "." + value.Sel.Name
	default:
		return ""
	}
}

func stripGoComments(source string) string {
	source = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(source, "")
	return regexp.MustCompile(`(?m)//.*$`).ReplaceAllString(source, "")
}

func goFunctionSource(t *testing.T, filename, functionName string) string {
	t.Helper()
	raw, err := os.ReadFile(filename)
	require.NoError(t, err)
	files := token.NewFileSet()
	parsed, err := parser.ParseFile(files, filename, raw, 0)
	require.NoError(t, err)
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != functionName || function.Body == nil {
			continue
		}
		start := files.Position(function.Pos()).Offset
		end := files.Position(function.End()).Offset
		require.Greater(t, end, start)
		return string(raw[start:end])
	}
	t.Fatalf("function %s not found in %s", functionName, filename)
	return ""
}
