package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCreationVoiceSettlesObservedAudioBeforeClientControlledErrorExit(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "grok_audio.go", nil, 0)
	require.NoError(t, err)
	settleAt, errorExitAt := -1, -1
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "GrokRealtime" {
			continue
		}
		for index, statement := range function.Body.List {
			branch, ok := statement.(*ast.IfStmt)
			if !ok {
				continue
			}
			ast.Inspect(branch, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if method, ok := call.Fun.(*ast.SelectorExpr); ok && method.Sel.Name == "recordGrokVoiceUsage" {
					settleAt = index
				}
				return true
			})
			if condition, ok := branch.Cond.(*ast.BinaryExpr); ok {
				if name, ok := condition.X.(*ast.Ident); ok && name.Name == "proxyErr" {
					errorExitAt = index
				}
			}
		}
	}
	// GrokRealtime must charge observed audio before malformed frames or abnormal
	// close codes can return from its proxy-error branch.
	require.GreaterOrEqual(t, settleAt, 0)
	require.Greater(t, errorExitAt, settleAt)
	require.NotNil(t, grokRealtimeBillingResult("grok-voice-latest", time.Minute, true))
	require.Nil(t, grokRealtimeBillingResult("grok-voice-latest", time.Minute, false))
}
