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
	copyResult := *s.dispatchResult
	return &copyResult, nil
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
		Trace:     AITraceRef{GroupID: &groupID},
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

func TestAISkillScriptRunnerRuntimeDispatchesBundle(t *testing.T) {
	t.Parallel()

	archive := buildAISkillArchiveFromDir(t, filepath.Join("testdata", "skills", "script_python_echo"))
	hostSkillDir := t.TempDir()
	hostScratchDir := t.TempDir()
	inputPath := filepath.Join(hostScratchDir, "input", "request.json")
	outputPath := filepath.Join(hostScratchDir, "output", "response.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(inputPath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(outputPath), 0o755))
	require.NoError(t, os.WriteFile(outputPath, []byte(`{"ok":true,"echo":"sunrise","runtime":"python3.11"}`), 0o600))

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
				ResourceLimits: skillrunner.ResourceLimits{},
			},
			HostSkillDir:   hostSkillDir,
			HostScratchDir: hostScratchDir,
			InputPath:      inputPath,
			OutputPath:     outputPath,
		},
	}
	runtime := NewAISkillScriptRunnerRuntime(runner)

	result, err := runtime.ExecuteScript(context.Background(), AISkillScriptRuntimeInput{
		RunID:          51,
		SkillID:        61,
		VersionID:      71,
		UserID:         81,
		Mode:           AISkillRunModeUse,
		Runtime:        skillrunner.RuntimePython311,
		ScriptName:     "script_python_echo",
		EntryPoint:     "main.py",
		Protocol:       skillrunner.ProtocolJSONFileV1,
		ArchiveBase64:  mustBase64(archive),
		TimeoutSeconds: 45,
		Environment: map[string]string{
			"SUBJECT": "{{subject}}",
		},
		Parameters: map[string]any{
			"subject": "sunrise",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, AISkillRunStatusSucceeded, result.Status)
	require.NotEmpty(t, result.ExternalJobID)
	require.Equal(t, true, result.Output["ok"])
	require.Equal(t, "sunrise", result.Output["echo"])
	require.Equal(t, "python3.11", result.Output["runtime"])

	paths, ok := result.Metadata["paths"].(map[string]any)
	require.True(t, ok)
	hostSkillDirPath, _ := paths["host_skill_dir"].(string)
	hostScratchDirPath, _ := paths["host_scratch_dir"].(string)
	require.NotEmpty(t, hostSkillDirPath)
	require.NotEmpty(t, hostScratchDirPath)

	inputPathValue, _ := paths["input_path"].(string)
	require.NotEmpty(t, inputPathValue)
	rawInput, err := os.ReadFile(inputPathValue)
	require.NoError(t, err)
	require.Equal(t, "sunrise", gjson.GetBytes(rawInput, "parameters.subject").String())
	require.Equal(t, "sunrise", gjson.GetBytes(rawInput, "environment.SUBJECT").String())

	plan, ok := result.Metadata["plan"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "/workspace/skill", plan["workingDir"])
	env, ok := plan["environment"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "sunrise", env["SUBJECT"])
	require.Equal(t, "/sandbox/input/request.json", env["SUB2API_SKILL_INPUT"])
	require.Equal(t, int64(45*time.Second), gjson.GetBytes(mustJSON(t, plan), "resourceLimits.timeout").Int())
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

func mustBase64(raw []byte) string {
	return base64.StdEncoding.EncodeToString(raw)
}
