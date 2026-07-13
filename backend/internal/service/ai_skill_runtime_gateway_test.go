//go:build unit

package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/integration/skillrunner"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type aiSkillOpenAIChatRuntimeStub struct {
	inputs []AISkillOpenAIChatRuntimeInput
	result *AISkillOpenAIChatRuntimeResult
	err    error
}

func (s *aiSkillOpenAIChatRuntimeStub) ExecuteChatCompat(_ context.Context, input AISkillOpenAIChatRuntimeInput) (*AISkillOpenAIChatRuntimeResult, error) {
	s.inputs = append(s.inputs, input)
	if s.err != nil {
		return nil, s.err
	}
	if s.result == nil {
		return &AISkillOpenAIChatRuntimeResult{}, nil
	}
	copy := *s.result
	copy.ResponseBody = append([]byte(nil), s.result.ResponseBody...)
	return &copy, nil
}

type aiSkillOpenAIImageRuntimeStub struct {
	inputs []AISkillOpenAIImageRuntimeInput
	result *AISkillOpenAIImageRuntimeResult
	err    error
}

func (s *aiSkillOpenAIImageRuntimeStub) ExecuteImages(_ context.Context, input AISkillOpenAIImageRuntimeInput) (*AISkillOpenAIImageRuntimeResult, error) {
	s.inputs = append(s.inputs, input)
	if s.err != nil {
		return nil, s.err
	}
	if s.result == nil {
		return &AISkillOpenAIImageRuntimeResult{}, nil
	}
	copy := *s.result
	copy.ResponseBody = append([]byte(nil), s.result.ResponseBody...)
	return &copy, nil
}

type aiSkillScriptRunnerStub struct {
	inspectArchiveRaw []byte
	inspectResult     *skillrunner.Bundle
	inspectErr        error
	dispatchRequest   *skillrunner.DispatchRequest
	dispatchResult    *skillrunner.DispatchResult
	dispatchErr       error
}

func (s *aiSkillScriptRunnerStub) InspectArchive(_ context.Context, raw []byte) (*skillrunner.Bundle, error) {
	s.inspectArchiveRaw = append([]byte(nil), raw...)
	if s.inspectErr != nil {
		return nil, s.inspectErr
	}
	if s.inspectResult == nil {
		return nil, nil
	}
	copy := *s.inspectResult
	return &copy, nil
}

func (s *aiSkillScriptRunnerStub) Plan(context.Context, skillrunner.PlanRequest) (*skillrunner.SandboxPlan, error) {
	return nil, nil
}

func (s *aiSkillScriptRunnerStub) Dispatch(_ context.Context, req skillrunner.DispatchRequest) (*skillrunner.DispatchResult, error) {
	copyReq := req
	copyReq.Archive = append([]byte(nil), req.Archive...)
	copyReq.Environment = cloneAIMap(req.Environment)
	s.dispatchRequest = &copyReq
	if s.dispatchErr != nil {
		return nil, s.dispatchErr
	}
	if s.dispatchResult == nil {
		return nil, nil
	}
	return &skillrunner.DispatchResult{
		Plan:           s.dispatchResult.Plan,
		HostSkillDir:   s.dispatchResult.HostSkillDir,
		HostScratchDir: s.dispatchResult.HostScratchDir,
		InputPath:      s.dispatchResult.InputPath,
		OutputPath:     s.dispatchResult.OutputPath,
	}, nil
}

func TestAISkillRuntimeGatewayExecutesPromptChatThroughCompatRuntime(t *testing.T) {
	t.Parallel()

	chatRuntime := &aiSkillOpenAIChatRuntimeStub{
		result: &AISkillOpenAIChatRuntimeResult{
			Forward: &OpenAIForwardResult{
				RequestID:     "up_req_chat_1",
				Model:         "gpt-5.4-mini",
				UpstreamModel: "gpt-5.4-mini",
				Usage: OpenAIUsage{
					InputTokens:  12,
					OutputTokens: 6,
				},
			},
			ResponseBody: []byte(`{"id":"chatcmpl_skill_1","choices":[{"message":{"role":"assistant","content":"billing summary"}}]}`),
		},
	}
	gateway := NewAISkillRuntimeGateway(chatRuntime, nil, nil)

	groupID := int64(88)
	dispatch, err := gateway.Execute(context.Background(), AISkillExecutionRequest{
		RunID:     11,
		SkillID:   21,
		VersionID: 31,
		UserID:    41,
		Mode:      AISkillRunModeUse,
		Type:      AISkillTypePromptChat,
		// Group scheduling must come from trusted skill metadata, not client Trace.
		Skill: &AISkill{Trace: AITraceRef{GroupID: &groupID}},
		PromptChat: &AISkillPromptChatExecution{
			SystemPrompt:       "You are helping with {{topic}}",
			UserPromptTemplate: "Summarize {{topic}}",
			Parameters: map[string]any{
				"topic":       "billing",
				"temperature": 0.2,
			},
			Attachments: []AISkillRunAttachment{
				{URL: "https://example.invalid/ref.png", Purpose: "reference"},
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, dispatch)
	require.Equal(t, AISkillRunStatusSucceeded, dispatch.Status)
	require.Equal(t, PlatformOpenAI, dispatch.Provider)
	require.Equal(t, "chatcmpl_skill_1", dispatch.ExternalJobID)
	require.Equal(t, "billing summary", dispatch.Output["text"])

	require.Len(t, chatRuntime.inputs, 1)
	require.Equal(t, defaultAISkillChatModel, chatRuntime.inputs[0].Model)
	require.Equal(t, groupID, *chatRuntime.inputs[0].GroupID)
	require.NotEmpty(t, chatRuntime.inputs[0].SessionHash)
	require.NotEmpty(t, chatRuntime.inputs[0].PromptCacheKey)
	require.Equal(t, "You are helping with billing", gjson.GetBytes(chatRuntime.inputs[0].Body, "messages.0.content").String())
	require.Equal(t, "Summarize billing", gjson.GetBytes(chatRuntime.inputs[0].Body, "messages.1.content.0.text").String())
	require.Equal(t, "https://example.invalid/ref.png", gjson.GetBytes(chatRuntime.inputs[0].Body, "messages.1.content.1.image_url.url").String())
	require.Equal(t, 0.2, gjson.GetBytes(chatRuntime.inputs[0].Body, "temperature").Float())
}

func TestAISkillRuntimeGatewayExecutesPromptImageThroughImagesRuntime(t *testing.T) {
	t.Parallel()

	imageRuntime := &aiSkillOpenAIImageRuntimeStub{
		result: &AISkillOpenAIImageRuntimeResult{
			Forward: &OpenAIForwardResult{
				RequestID:     "up_req_img_1",
				Model:         "gpt-image-1",
				UpstreamModel: "gpt-image-1",
				ImageCount:    1,
				ImageSize:     "1K",
			},
			ResponseBody: []byte(`{"created":123,"data":[{"b64_json":"YWJj","revised_prompt":"draw a fox"}]}`),
		},
	}
	gateway := NewAISkillRuntimeGateway(nil, imageRuntime, nil)

	dispatch, err := gateway.Execute(context.Background(), AISkillExecutionRequest{
		RunID:     12,
		SkillID:   22,
		VersionID: 32,
		UserID:    42,
		Mode:      AISkillRunModeUse,
		Type:      AISkillTypePromptImage,
		PromptImage: &AISkillPromptImageExecution{
			Model:                  "gpt-image-1",
			PromptTemplate:         "draw a {{subject}}",
			NegativePromptTemplate: "no {{bad}}",
			Size:                   "1024x1024",
			ImageCount:             1,
			Parameters: map[string]any{
				"subject": "fox",
				"bad":     "blur",
				"quality": "high",
			},
			Attachments: []AISkillRunAttachment{
				{URL: "https://example.invalid/ref.png", Purpose: "reference"},
				{URL: "https://example.invalid/mask.png", Purpose: "mask"},
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, dispatch)
	require.Equal(t, AISkillRunStatusSucceeded, dispatch.Status)
	require.Equal(t, PlatformOpenAI, dispatch.Provider)
	require.Equal(t, "up_req_img_1", dispatch.ExternalJobID)

	require.Len(t, imageRuntime.inputs, 1)
	require.Equal(t, openAIImagesEditsEndpoint, imageRuntime.inputs[0].Path)
	require.Equal(t, "application/json", imageRuntime.inputs[0].ContentType)
	require.Equal(t, "draw a fox\nNegative prompt: no blur", gjson.GetBytes(imageRuntime.inputs[0].Body, "prompt").String())
	require.Equal(t, "https://example.invalid/ref.png", gjson.GetBytes(imageRuntime.inputs[0].Body, "images.0.image_url").String())
	require.Equal(t, "https://example.invalid/mask.png", gjson.GetBytes(imageRuntime.inputs[0].Body, "mask.image_url").String())
	require.Equal(t, "high", gjson.GetBytes(imageRuntime.inputs[0].Body, "quality").String())

	images, ok := dispatch.Output["images"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, images, 1)
	require.Equal(t, "draw a fox", dispatch.Output["revised_prompt"])
}

func TestAISkillRuntimeGatewayExecutesScriptThroughScriptRuntime(t *testing.T) {
	t.Parallel()

	runner := &aiSkillScriptRunnerStub{
		inspectResult: &skillrunner.Bundle{
			Digest: "sha256:script-test",
			Manifest: skillrunner.Manifest{
				Metadata: skillrunner.ManifestMeta{
					Name:    "script_python_echo",
					Version: "v1",
				},
				Spec: skillrunner.ScriptSpec{
					Type:       skillrunner.SkillTypeScript,
					Runtime:    skillrunner.RuntimePython311,
					Entrypoint: "main.py",
					Protocol:   skillrunner.ProtocolJSONFileV1,
				},
			},
		},
		dispatchResult: &skillrunner.DispatchResult{
			Plan: &skillrunner.SandboxPlan{
				Environment: map[string]string{
					"SUB2API_SKILL_INPUT": "/sandbox/input/request.json",
				},
			},
		},
	}
	gateway := NewAISkillRuntimeGateway(nil, nil, NewAISkillScriptRunnerRuntime(runner))

	dispatch, err := gateway.Execute(context.Background(), AISkillExecutionRequest{
		RunID:     13,
		SkillID:   23,
		VersionID: 33,
		UserID:    43,
		Mode:      AISkillRunModeUse,
		Type:      AISkillTypeScript,
		Script: &AISkillScriptExecution{
			Runtime:                skillrunner.RuntimePython311,
			ScriptName:             "script_python_echo",
			EntryPoint:             "main.py",
			Protocol:               skillrunner.ProtocolJSONFileV1,
			ApprovedArtifactDigest: "sha256:script-test",
			VersionStatus:          AISkillVersionStatusApproved,
			ArchiveBase64:          base64.StdEncoding.EncodeToString(buildAISkillArchiveFromDir(t, filepath.Join("testdata", "skills", "script_python_echo"))),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, dispatch)
	require.Equal(t, AISkillRunStatusDispatched, dispatch.Status)
	require.Equal(t, "skillrunner", dispatch.Provider)
	require.Equal(t, "sha256:script-test", dispatch.ExternalJobID)
	require.NotNil(t, runner.dispatchRequest, "script runtime should receive the dispatch request")
}

func TestAISkillScriptRunnerRuntimeDispatchesBundle(t *testing.T) {
	t.Parallel()

	archive := buildAISkillArchiveFromDir(t, filepath.Join("testdata", "skills", "script_python_echo"))
	hostSkillDir := t.TempDir()
	hostScratchDir := t.TempDir()
	archiveRoot := t.TempDir()
	archivePath := filepath.Join(archiveRoot, "script_python_echo.zip")
	inputPath := filepath.Join(hostScratchDir, "input", "request.json")
	outputPath := filepath.Join(hostScratchDir, "output", "response.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(inputPath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(outputPath), 0o755))
	require.NoError(t, os.WriteFile(archivePath, archive, 0o600))
	require.NoError(t, os.WriteFile(outputPath, []byte(`{"ok":true,"echo":"sunrise","runtime":"python3.11"}`), 0o600))

	// The resolver canonicalizes the archive path via EvalSymlinks (e.g. on
	// macOS /var -> /private/var), so archive_source reflects the resolved path.
	resolvedArchivePath, err := filepath.EvalSymlinks(archivePath)
	require.NoError(t, err)

	runner := &aiSkillScriptRunnerStub{
		inspectResult: &skillrunner.Bundle{
			Digest: "sha256:script-test",
			Manifest: skillrunner.Manifest{
				Metadata: skillrunner.ManifestMeta{
					Name:    "script_python_echo",
					Version: "v1",
				},
				Spec: skillrunner.ScriptSpec{
					Type:       skillrunner.SkillTypeScript,
					Runtime:    skillrunner.RuntimePython311,
					Entrypoint: "main.py",
					Protocol:   skillrunner.ProtocolJSONFileV1,
				},
			},
		},
		dispatchResult: &skillrunner.DispatchResult{
			Plan: &skillrunner.SandboxPlan{
				WorkingDir: "/workspace/skill",
				Environment: map[string]string{
					"SUB2API_SKILL_INPUT":  "/sandbox/input/request.json",
					"SUB2API_SKILL_OUTPUT": "/sandbox/output/response.json",
				},
				Mounts: []skillrunner.Mount{
					{Source: hostSkillDir, Target: "/workspace/skill", ReadOnly: true},
					{Source: hostScratchDir, Target: "/sandbox", ReadOnly: false},
				},
				ResourceLimits: skillrunner.ResourceLimits{},
			},
			HostSkillDir:   hostSkillDir,
			HostScratchDir: hostScratchDir,
			InputPath:      inputPath,
			OutputPath:     outputPath,
		},
	}
	runtime := NewAISkillScriptRunnerRuntime(runner)
	// Constrain on-disk archive reads to the test archive root (resolved the
	// same way production resolves SUB2API_AI_SKILL_ARCHIVE_ROOT).
	runtime.archiveRoot = resolveAISkillArchiveRoot(archiveRoot)

	result, err := runtime.ExecuteScript(context.Background(), AISkillScriptRuntimeInput{
		RunID:                  51,
		SkillID:                61,
		VersionID:              71,
		UserID:                 81,
		Mode:                   AISkillRunModeUse,
		Runtime:                skillrunner.RuntimePython311,
		ScriptName:             "script_python_echo",
		EntryPoint:             "main.py",
		Protocol:               skillrunner.ProtocolJSONFileV1,
		ArchivePath:            archivePath,
		ApprovedArtifactDigest: "sha256:script-test",
		VersionStatus:          AISkillVersionStatusApproved,
		ReviewerUserID:         int64Ptr(909),
		TimeoutSeconds:         45,
		Environment: map[string]string{
			"SUBJECT": "{{subject}}",
		},
		Parameters: map[string]any{
			"subject": "sunrise",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, AISkillRunStatusDispatched, result.Status)
	require.Equal(t, "sha256:script-test", result.ExternalJobID)

	outputBundle, ok := result.Output["bundle"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "sha256:script-test", outputBundle["digest"])

	outputPlan, ok := result.Output["plan"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "/workspace/skill", outputPlan["workingDir"])
	outputPlanEnv, ok := outputPlan["environment"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "sunrise", outputPlanEnv["SUBJECT"])
	require.Equal(t, "/sandbox/input/request.json", outputPlanEnv["SUB2API_SKILL_INPUT"])
	require.Equal(t, "/sandbox/output/response.json", outputPlanEnv["SUB2API_SKILL_OUTPUT"])
	require.Equal(t, int64(45*time.Second), gjson.GetBytes(mustJSON(t, outputPlan), "resourceLimits.timeout").Int())

	// Host infrastructure paths must never be returned to clients.
	_, hasPaths := result.Output["paths"]
	require.False(t, hasPaths, "output must not expose host paths")
	planMounts, ok := outputPlan["mounts"].([]any)
	require.True(t, ok)
	require.Len(t, planMounts, 2)
	for _, raw := range planMounts {
		mount, ok := raw.(map[string]any)
		require.True(t, ok)
		require.Empty(t, mount["source"], "plan mount source must be scrubbed")
	}
	planJSON := mustJSON(t, result.Output["plan"])
	require.NotContains(t, string(planJSON), hostSkillDir)
	require.NotContains(t, string(planJSON), hostScratchDir)
	outputJSON := mustJSON(t, result.Output)
	require.NotContains(t, string(outputJSON), hostScratchDir)
	require.NotContains(t, string(outputJSON), inputPath)
	require.NotContains(t, string(outputJSON), outputPath)

	outputEnvironment, ok := result.Output["environment"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "sunrise", outputEnvironment["SUBJECT"])

	dispatchInput, ok := result.Output["dispatch_input"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, int64(51), dispatchInput["run_id"])
	require.Equal(t, int64(61), dispatchInput["skill_id"])
	require.Equal(t, int64(71), dispatchInput["version_id"])
	require.Equal(t, int64(81), dispatchInput["user_id"])
	require.Equal(t, "use", dispatchInput["mode"])
	require.Equal(t, "script_python_echo", dispatchInput["script_name"])
	require.Equal(t, "python3.11", dispatchInput["runtime"])
	require.Equal(t, "main.py", dispatchInput["entry_point"])
	require.Equal(t, "json-file-v1", dispatchInput["protocol"])
	require.Equal(t, resolvedArchivePath, dispatchInput["archive_source"])
	dispatchParameters, ok := dispatchInput["parameters"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "sunrise", dispatchParameters["subject"])
	dispatchEnvironment, ok := dispatchInput["environment"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "sunrise", dispatchEnvironment["SUBJECT"])

	require.NotNil(t, runner.dispatchRequest)
	require.Equal(t, archive, runner.dispatchRequest.Archive)
	require.Equal(t, "script_python_echo", runner.dispatchRequest.Bundle.Manifest.Metadata.Name)
	require.Equal(t, skillrunner.ReviewStatusApproved, runner.dispatchRequest.Review.Status)
	// The review gate must carry the normalized approved digest and the real
	// reviewer identity, not a hardcoded service account.
	require.Equal(t, "script-test", runner.dispatchRequest.Review.ArtifactDigest)
	require.Equal(t, "user:909", runner.dispatchRequest.Review.ApprovedBy)
	require.Equal(t, map[string]any{"SUBJECT": "sunrise"}, runner.dispatchRequest.Environment)
	require.Equal(t, "skillrunner", result.Metadata["provider"])
	require.Equal(t, "sha256:script-test", result.Metadata["bundle_digest"])
	require.Equal(t, resolvedArchivePath, result.Metadata["archive_source"])
}

func TestAISkillScriptRunnerRuntimeRejectsMissingApprovedDigest(t *testing.T) {
	t.Parallel()

	archive := buildAISkillArchiveFromDir(t, filepath.Join("testdata", "skills", "script_python_echo"))
	archiveBase64 := base64.StdEncoding.EncodeToString(archive)

	runner := &aiSkillScriptRunnerStub{
		inspectResult: &skillrunner.Bundle{
			Digest: "deadbeef",
			Manifest: skillrunner.Manifest{
				Metadata: skillrunner.ManifestMeta{Name: "script_python_echo", Version: "v1"},
				Spec: skillrunner.ScriptSpec{
					Type:       skillrunner.SkillTypeScript,
					Runtime:    skillrunner.RuntimePython311,
					Entrypoint: "main.py",
					Protocol:   skillrunner.ProtocolJSONFileV1,
				},
			},
		},
	}
	runtime := NewAISkillScriptRunnerRuntime(runner)

	_, err := runtime.ExecuteScript(context.Background(), AISkillScriptRuntimeInput{
		RunID:         1,
		Runtime:       skillrunner.RuntimePython311,
		ScriptName:    "script_python_echo",
		EntryPoint:    "main.py",
		Protocol:      skillrunner.ProtocolJSONFileV1,
		ArchiveBase64: archiveBase64,
		// No ApprovedArtifactDigest: must fail closed.
		VersionStatus: AISkillVersionStatusApproved,
	})
	require.ErrorIs(t, err, ErrAISkillScriptNotApproved)
	require.Nil(t, runner.dispatchRequest, "dispatch must not be reached without an approved digest")
}

func TestAISkillScriptRunnerRuntimeRejectsDigestMismatch(t *testing.T) {
	t.Parallel()

	archive := buildAISkillArchiveFromDir(t, filepath.Join("testdata", "skills", "script_python_echo"))
	archiveBase64 := base64.StdEncoding.EncodeToString(archive)

	runner := &aiSkillScriptRunnerStub{
		inspectResult: &skillrunner.Bundle{
			Digest: "abc123", // what the runner actually sees
			Manifest: skillrunner.Manifest{
				Metadata: skillrunner.ManifestMeta{Name: "script_python_echo", Version: "v1"},
				Spec: skillrunner.ScriptSpec{
					Type:       skillrunner.SkillTypeScript,
					Runtime:    skillrunner.RuntimePython311,
					Entrypoint: "main.py",
					Protocol:   skillrunner.ProtocolJSONFileV1,
				},
			},
		},
	}
	runtime := NewAISkillScriptRunnerRuntime(runner)

	_, err := runtime.ExecuteScript(context.Background(), AISkillScriptRuntimeInput{
		RunID:                  1,
		Runtime:                skillrunner.RuntimePython311,
		ScriptName:             "script_python_echo",
		EntryPoint:             "main.py",
		Protocol:               skillrunner.ProtocolJSONFileV1,
		ArchiveBase64:          archiveBase64,
		ApprovedArtifactDigest: "different-digest", // does not match bundle
		VersionStatus:          AISkillVersionStatusApproved,
	})
	require.ErrorIs(t, err, ErrAISkillScriptArtifactMismatch)
	require.Nil(t, runner.dispatchRequest, "dispatch must not be reached on digest mismatch")
}

func TestAISkillScriptRunnerRuntimeAcceptsMatchingDigestWithPrefixVariance(t *testing.T) {
	t.Parallel()

	archive := buildAISkillArchiveFromDir(t, filepath.Join("testdata", "skills", "script_python_echo"))
	archiveBase64 := base64.StdEncoding.EncodeToString(archive)

	runner := &aiSkillScriptRunnerStub{
		inspectResult: &skillrunner.Bundle{
			Digest: "ABC123", // uppercase, no prefix
			Manifest: skillrunner.Manifest{
				Metadata: skillrunner.ManifestMeta{Name: "script_python_echo", Version: "v1"},
				Spec: skillrunner.ScriptSpec{
					Type:       skillrunner.SkillTypeScript,
					Runtime:    skillrunner.RuntimePython311,
					Entrypoint: "main.py",
					Protocol:   skillrunner.ProtocolJSONFileV1,
				},
			},
		},
		dispatchResult: &skillrunner.DispatchResult{Plan: &skillrunner.SandboxPlan{}},
	}
	runtime := NewAISkillScriptRunnerRuntime(runner)

	// Stored digest uses sha256: prefix and lowercase; bundle digest is uppercase
	// without prefix. Normalization must treat them as equal.
	result, err := runtime.ExecuteScript(context.Background(), AISkillScriptRuntimeInput{
		RunID:                  1,
		Runtime:                skillrunner.RuntimePython311,
		ScriptName:             "script_python_echo",
		EntryPoint:             "main.py",
		Protocol:               skillrunner.ProtocolJSONFileV1,
		ArchiveBase64:          archiveBase64,
		ApprovedArtifactDigest: "sha256:abc123",
		VersionStatus:          AISkillVersionStatusApproved,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, runner.dispatchRequest)
	require.Equal(t, skillrunner.ReviewStatusApproved, runner.dispatchRequest.Review.Status)
}

func TestAISkillScriptRunnerRuntimeRejectsMissingRuntimeBundleFields(t *testing.T) {
	t.Parallel()

	archive := buildAISkillArchiveFromDir(t, filepath.Join("testdata", "skills", "script_python_echo"))
	archiveBase64 := base64.StdEncoding.EncodeToString(archive)

	baseInput := AISkillScriptRuntimeInput{
		RunID:                  1,
		ArchiveBase64:          archiveBase64,
		ApprovedArtifactDigest: "sha256:abc123",
		VersionStatus:          AISkillVersionStatusApproved,
		ScriptName:             "script_python_echo",
		Runtime:                skillrunner.RuntimePython311,
		EntryPoint:             "main.py",
		Protocol:               skillrunner.ProtocolJSONFileV1,
	}

	for _, tc := range []struct {
		name  string
		input AISkillScriptRuntimeInput
	}{
		{
			name:  "runtime",
			input: func() AISkillScriptRuntimeInput { in := baseInput; in.Runtime = ""; return in }(),
		},
		{
			name:  "entrypoint",
			input: func() AISkillScriptRuntimeInput { in := baseInput; in.EntryPoint = ""; return in }(),
		},
		{
			name:  "protocol",
			input: func() AISkillScriptRuntimeInput { in := baseInput; in.Protocol = ""; return in }(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			runner := &aiSkillScriptRunnerStub{
				inspectResult: &skillrunner.Bundle{
					Digest: "ABC123",
					Manifest: skillrunner.Manifest{
						Metadata: skillrunner.ManifestMeta{Name: "script_python_echo", Version: "v1"},
						Spec: skillrunner.ScriptSpec{
							Type:       skillrunner.SkillTypeScript,
							Runtime:    skillrunner.RuntimePython311,
							Entrypoint: "main.py",
							Protocol:   skillrunner.ProtocolJSONFileV1,
						},
					},
				},
				dispatchResult: &skillrunner.DispatchResult{Plan: &skillrunner.SandboxPlan{}},
			}
			runtime := NewAISkillScriptRunnerRuntime(runner)

			result, err := runtime.ExecuteScript(context.Background(), tc.input)
			require.ErrorIs(t, err, ErrAISkillExecutionSpecInvalid)
			require.Nil(t, result)
			require.Nil(t, runner.dispatchRequest, "dispatch must not run when %s is missing", tc.name)
		})
	}
}

func TestAISkillScriptRunnerRuntimeRejectsArchivePathWithoutRoot(t *testing.T) {
	t.Parallel()

	archive := buildAISkillArchiveFromDir(t, filepath.Join("testdata", "skills", "script_python_echo"))
	archivePath := filepath.Join(t.TempDir(), "skill.zip")
	require.NoError(t, os.WriteFile(archivePath, archive, 0o600))

	runner := &aiSkillScriptRunnerStub{}
	runtime := NewAISkillScriptRunnerRuntime(runner)
	runtime.archiveRoot = "" // no archive root configured

	_, err := runtime.ExecuteScript(context.Background(), AISkillScriptRuntimeInput{
		RunID:       1,
		Runtime:     skillrunner.RuntimePython311,
		ScriptName:  "script_python_echo",
		EntryPoint:  "main.py",
		Protocol:    skillrunner.ProtocolJSONFileV1,
		ArchivePath: archivePath,
	})
	require.ErrorIs(t, err, ErrAISkillScriptArchivePathInvalid)
	require.Nil(t, runner.dispatchRequest, "dispatch must not be reached")
}

func TestAISkillScriptRunnerRuntimeRejectsArchivePathOutsideRoot(t *testing.T) {
	t.Parallel()

	archive := buildAISkillArchiveFromDir(t, filepath.Join("testdata", "skills", "script_python_echo"))
	archiveRoot := t.TempDir()
	// Place the archive outside the configured root (a sibling directory).
	outsideDir := t.TempDir()
	archivePath := filepath.Join(outsideDir, "skill.zip")
	require.NoError(t, os.WriteFile(archivePath, archive, 0o600))

	runner := &aiSkillScriptRunnerStub{}
	runtime := NewAISkillScriptRunnerRuntime(runner)
	runtime.archiveRoot = resolveAISkillArchiveRoot(archiveRoot)

	_, err := runtime.ExecuteScript(context.Background(), AISkillScriptRuntimeInput{
		RunID:       1,
		Runtime:     skillrunner.RuntimePython311,
		ScriptName:  "script_python_echo",
		EntryPoint:  "main.py",
		Protocol:    skillrunner.ProtocolJSONFileV1,
		ArchivePath: archivePath,
	})
	require.ErrorIs(t, err, ErrAISkillScriptArchivePathInvalid)
	require.Nil(t, runner.dispatchRequest)
}

func TestAISkillScriptRunnerRuntimeRejectsArchivePathTraversal(t *testing.T) {
	t.Parallel()

	archive := buildAISkillArchiveFromDir(t, filepath.Join("testdata", "skills", "script_python_echo"))
	archiveRoot := t.TempDir()
	secretDir := t.TempDir()
	secretPath := filepath.Join(secretDir, "secret.zip")
	require.NoError(t, os.WriteFile(secretPath, archive, 0o600))

	// Traversal attempt: a path nominally under the root that escapes via "..".
	traversal := filepath.Join(archiveRoot, "..", filepath.Base(secretDir), "secret.zip")

	runner := &aiSkillScriptRunnerStub{}
	runtime := NewAISkillScriptRunnerRuntime(runner)
	runtime.archiveRoot = resolveAISkillArchiveRoot(archiveRoot)

	_, err := runtime.ExecuteScript(context.Background(), AISkillScriptRuntimeInput{
		RunID:       1,
		Runtime:     skillrunner.RuntimePython311,
		ScriptName:  "script_python_echo",
		EntryPoint:  "main.py",
		Protocol:    skillrunner.ProtocolJSONFileV1,
		ArchivePath: traversal,
	})
	require.ErrorIs(t, err, ErrAISkillScriptArchivePathInvalid)
	require.Nil(t, runner.dispatchRequest)
}

func TestAISkillScriptRunnerRuntimeRejectsArchiveSymlink(t *testing.T) {
	t.Parallel()

	archive := buildAISkillArchiveFromDir(t, filepath.Join("testdata", "skills", "script_python_echo"))
	archiveRoot := t.TempDir()
	// The real file lives outside the root; a symlink inside the root points to it.
	outsideDir := t.TempDir()
	realPath := filepath.Join(outsideDir, "secret.zip")
	require.NoError(t, os.WriteFile(realPath, archive, 0o600))
	symlinkPath := filepath.Join(archiveRoot, "link.zip")
	if err := os.Symlink(realPath, symlinkPath); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	runner := &aiSkillScriptRunnerStub{}
	runtime := NewAISkillScriptRunnerRuntime(runner)
	runtime.archiveRoot = resolveAISkillArchiveRoot(archiveRoot)

	_, err := runtime.ExecuteScript(context.Background(), AISkillScriptRuntimeInput{
		RunID:       1,
		Runtime:     skillrunner.RuntimePython311,
		ScriptName:  "script_python_echo",
		EntryPoint:  "main.py",
		Protocol:    skillrunner.ProtocolJSONFileV1,
		ArchivePath: symlinkPath,
	})
	require.ErrorIs(t, err, ErrAISkillScriptArchivePathInvalid)
	require.Nil(t, runner.dispatchRequest)
}

func buildAISkillArchiveFromDir(t *testing.T, dir string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entry, err := writer.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		_, err = entry.Write(raw)
		return err
	})
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return buffer.Bytes()
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}

// TestAISkillScriptApprovalDigestBindsRunExecution exercises the full digest
// binding: approval captures the artifact digest, run-time recomputes it from
// the same stored source and the real bundle inspector, and they match. It then
// proves that mutating the approved source afterwards causes run-time rejection.
func TestAISkillScriptApprovalDigestBindsRunExecution(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newAISkillStoreStub()

	skill := &AISkill{
		CreatorUserID: 700,
		Name:          "Echo Script",
		Description:   "echoes input",
		Type:          AISkillTypeScript,
	}
	require.NoError(t, store.CreateSkill(ctx, skill))

	version := &AISkillVersion{
		SkillID:       skill.ID,
		CreatorUserID: skill.CreatorUserID,
		Version:       1,
		Type:          AISkillTypeScript,
		Status:        AISkillVersionStatusSubmitted,
		ExecutionSpec: AISkillExecutionSpec{
			Type: AISkillTypeScript,
			Script: &AISkillScriptSpec{
				Runtime:    skillrunner.RuntimeNode20,
				ScriptName: "echo_script",
				EntryPoint: "main.mjs",
				Protocol:   skillrunner.ProtocolJSONFileV1,
			},
		},
		BillingPolicy: AISkillBillingPolicy{Mode: AISkillBillingModeFree},
		Metadata: map[string]any{
			"source_code": "console.log('hello world')",
		},
	}
	require.NoError(t, store.CreateVersion(ctx, version))

	// Approve: this must persist a non-empty artifact digest.
	reviewSvc := NewAISkillReviewService(store, store, store)
	approved, err := reviewSvc.ApproveVersion(ctx, 999, version.ID, &AIReviewSkillVersionInput{})
	require.NoError(t, err)
	require.NotEmpty(t, approved.ApprovedArtifactDigest, "approval must capture a digest")

	stored, err := store.GetVersionByID(ctx, version.ID)
	require.NoError(t, err)
	require.Equal(t, approved.ApprovedArtifactDigest, stored.ApprovedArtifactDigest)

	// Build the run-time script execution from the approved version using the
	// production code path, then run it through the REAL script runner.
	runSkill, err := store.GetSkillByID(ctx, skill.ID)
	require.NoError(t, err)
	spec, err := normalizeAISkillExecutionSpec(stored.Type, stored.ExecutionSpec)
	require.NoError(t, err)
	resolved, err := resolveAISkillScriptArtifact(runSkill, stored, spec.Script)
	require.NoError(t, err)

	runtime := NewAISkillScriptRunnerRuntime(skillrunner.NewScriptRunner())
	result, err := runtime.ExecuteScript(ctx, AISkillScriptRuntimeInput{
		RunID:                  1,
		SkillID:                skill.ID,
		VersionID:              stored.ID,
		UserID:                 700,
		Mode:                   AISkillRunModeUse,
		Runtime:                resolved.Runtime,
		ScriptName:             resolved.ScriptName,
		EntryPoint:             resolved.EntryPoint,
		Protocol:               resolved.Protocol,
		ArchiveBase64:          resolved.ArchiveBase64,
		ApprovedArtifactDigest: stored.ApprovedArtifactDigest,
		VersionStatus:          stored.Status,
		ReviewerUserID:         int64Ptr(999),
	})
	require.NoError(t, err, "run with the approved digest must succeed")
	require.NotNil(t, result)
	// The runner-computed bundle digest must equal the approved digest.
	require.Equal(t,
		normalizeAISkillArtifactDigest(stored.ApprovedArtifactDigest),
		normalizeAISkillArtifactDigest(result.Metadata["bundle_digest"].(string)),
	)

	// Now tamper: change the source after approval. The approved digest no longer
	// matches the regenerated artifact, so run-time must reject execution.
	tampered := *stored
	tampered.Metadata = map[string]any{"source_code": "console.log('evil')"}
	tamperedSpec, err := normalizeAISkillExecutionSpec(tampered.Type, tampered.ExecutionSpec)
	require.NoError(t, err)
	tamperedArtifact, err := resolveAISkillScriptArtifact(runSkill, &tampered, tamperedSpec.Script)
	require.NoError(t, err)

	_, err = runtime.ExecuteScript(ctx, AISkillScriptRuntimeInput{
		RunID:                  2,
		Runtime:                tamperedArtifact.Runtime,
		ScriptName:             tamperedArtifact.ScriptName,
		EntryPoint:             tamperedArtifact.EntryPoint,
		Protocol:               tamperedArtifact.Protocol,
		ArchiveBase64:          tamperedArtifact.ArchiveBase64,
		ApprovedArtifactDigest: stored.ApprovedArtifactDigest, // stale approval
		VersionStatus:          AISkillVersionStatusApproved,
	})
	require.ErrorIs(t, err, ErrAISkillScriptArtifactMismatch,
		"tampered source must not match the approved digest")
}

func TestResolveAISkillGroupIDIgnoresClientTraceGroupID(t *testing.T) {
	t.Parallel()
	clientGroup := int64(999)
	skillGroup := int64(42)
	got := resolveAISkillGroupID(AISkillExecutionRequest{
		Trace: AITraceRef{GroupID: &clientGroup},
		Skill: &AISkill{Trace: AITraceRef{GroupID: &skillGroup}},
	})
	require.NotNil(t, got)
	require.Equal(t, skillGroup, *got, "client-provided trace.group_id must not drive account scheduling")
}

func TestResolveAISkillGroupIDFallsBackWhenNoTrustedGroup(t *testing.T) {
	t.Parallel()
	clientGroup := int64(999)
	got := resolveAISkillGroupID(AISkillExecutionRequest{
		Trace: AITraceRef{GroupID: &clientGroup},
	})
	require.Nil(t, got)
}
