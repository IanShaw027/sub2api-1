package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestShouldUseAIResponsesForChat(t *testing.T) {
	useResponses := true
	require.True(t, shouldUseAIResponsesForChat(&legacyChatRequest{UseResponses: &useResponses}))
	require.True(t, shouldUseAIResponsesForChat(&legacyChatRequest{Prompt: "Please draw a cat"}))
	require.False(t, shouldUseAIResponsesForChat(&legacyChatRequest{Prompt: "Hello"}))
}

func TestBuildAIResponsesChatPayload(t *testing.T) {
	apiKey := &service.APIKey{Group: &service.Group{DefaultMappedModel: "gpt-5.4"}}
	body := buildAIResponsesChatPayload(&legacyChatRequest{
		Prompt: "Hello",
		History: []map[string]any{
			{"role": "system", "content": "Be brief"},
		},
	}, apiKey)

	require.False(t, gjson.GetBytes(body, "stream").Bool())
	require.Equal(t, "gpt-5.4", gjson.GetBytes(body, "model").String())
	require.Equal(t, "message", gjson.GetBytes(body, "input.0.type").String())
	require.Equal(t, "system", gjson.GetBytes(body, "input.0.role").String())
	require.Equal(t, "Be brief", gjson.GetBytes(body, "input.0.content.0.text").String())
	require.Equal(t, "Hello", gjson.GetBytes(body, "input.1.content.0.text").String())
}

func TestBuildAIResponsesImageGenerationPayload(t *testing.T) {
	apiKey := &service.APIKey{Group: &service.Group{DefaultMappedModel: "gpt-5.4"}}
	body := buildAIResponsesImageGenerationPayload(&legacyCreateArtworkRequest{
		Prompt:         "Draw a cat",
		NegativePrompt: ptrString("blurry"),
		Size:           ptrString("1024x1024"),
		Style:          ptrString("vivid"),
	}, apiKey)

	require.False(t, gjson.GetBytes(body, "stream").Bool())
	require.Equal(t, "gpt-image-2", gjson.GetBytes(body, "model").String())
	require.Equal(t, "image_generation", gjson.GetBytes(body, "tool_choice.type").String())
	require.Equal(t, "image_generation", gjson.GetBytes(body, "tools.0.type").String())
	require.Equal(t, "generate", gjson.GetBytes(body, "tools.0.action").String())
	require.Equal(t, "Draw a cat\n\nNegative prompt: blurry", gjson.GetBytes(body, "input.0.content.0.text").String())
	require.Equal(t, "1024x1024", gjson.GetBytes(body, "tools.0.size").String())
	require.Equal(t, "vivid", gjson.GetBytes(body, "tools.0.style").String())
}

func TestBuildAIImageGenerationPayload(t *testing.T) {
	apiKey := &service.APIKey{Group: &service.Group{Platform: service.PlatformOpenAI, DefaultMappedModel: "gpt-image-2"}}
	body := buildAIImageGenerationPayload(&legacyCreateArtworkRequest{
		Prompt:         "Draw a cat",
		NegativePrompt: ptrString("blurry"),
		Size:           ptrString("1024x1024"),
		Style:          ptrString("vivid"),
	}, apiKey)

	require.Equal(t, "gpt-image-2", gjson.GetBytes(body, "model").String())
	require.Equal(t, "Draw a cat\n\nNegative prompt: blurry", gjson.GetBytes(body, "prompt").String())
	require.Equal(t, "1024x1024", gjson.GetBytes(body, "size").String())
	require.Equal(t, "vivid", gjson.GetBytes(body, "style").String())
	require.Equal(t, "b64_json", gjson.GetBytes(body, "response_format").String())
}

func TestBuildAIImageEditPayload(t *testing.T) {
	apiKey := &service.APIKey{Group: &service.Group{Platform: service.PlatformOpenAI, DefaultMappedModel: "gpt-image-2"}}
	body := buildAIImageEditPayload(&legacyCreateArtworkRequest{
		Prompt:         "Replace background",
		NegativePrompt: ptrString("blurry"),
		SourceImage:    ptrString("data:image/png;base64,AAAA"),
		MaskImage:      ptrString("https://example.com/mask.png"),
		Size:           ptrString("1024x1024"),
		Style:          ptrString("editorial"),
	}, apiKey)

	require.Equal(t, "gpt-image-2", gjson.GetBytes(body, "model").String())
	require.Equal(t, "Replace background\n\nNegative prompt: blurry", gjson.GetBytes(body, "prompt").String())
	require.Equal(t, "data:image/png;base64,AAAA", gjson.GetBytes(body, "images.0.image_url").String())
	require.Equal(t, "https://example.com/mask.png", gjson.GetBytes(body, "mask.image_url").String())
	require.Equal(t, "1024x1024", gjson.GetBytes(body, "size").String())
	require.Equal(t, "editorial", gjson.GetBytes(body, "style").String())
}

func TestAIArtworkGenerationEndpoint(t *testing.T) {
	require.Equal(t, "/openai/v1/images/generations", aiArtworkGenerationEndpoint(&service.APIKey{
		Group: &service.Group{Platform: service.PlatformOpenAI},
	}))
	webBridgePrice := 0.1
	require.Equal(t, "/openai/v1/images2api/generations", aiArtworkGenerationEndpoint(&service.APIKey{
		Group: &service.Group{Platform: service.PlatformOpenAI, Images2APIPrice1K: &webBridgePrice},
	}))
	require.Equal(t, "/openai/v1/images2api/generations", aiArtworkGenerationEndpoint(&service.APIKey{
		Group: &service.Group{Platform: service.PlatformAnthropic},
	}))
}

func TestAIArtworkEditEndpoint(t *testing.T) {
	require.Equal(t, "/openai/v1/images/edits", aiArtworkEditEndpoint(&service.APIKey{
		Group: &service.Group{Platform: service.PlatformOpenAI},
	}))
	webBridgePrice := 0.1
	require.Equal(t, "/openai/v1/images2api/edits", aiArtworkEditEndpoint(&service.APIKey{
		Group: &service.Group{Platform: service.PlatformOpenAI, Images2APIPrice1K: &webBridgePrice},
	}))
	require.Equal(t, "/openai/v1/images2api/edits", aiArtworkEditEndpoint(&service.APIKey{
		Group: &service.Group{Platform: service.PlatformAnthropic},
	}))
}

func TestDecodeAIImageResultFromResponses(t *testing.T) {
	body := []byte(`{"id":"resp_1","model":"gpt-5.4","output":[{"type":"image_generation_call","result":"aGVsbG8=","revised_prompt":"draw a cat","output_format":"webp"}]}`)
	imageBytes, mimeType, revisedPrompt, err := decodeAIImageResult(context.Background(), body)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), imageBytes)
	require.Equal(t, "image/webp", mimeType)
	require.Equal(t, "draw a cat", revisedPrompt)
}

func TestExtractAIResponseText(t *testing.T) {
	body := []byte(`{"id":"resp_1","model":"gpt-5.4","output":[{"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`)
	require.Equal(t, "ok", extractAIResponseText(body))
}

func TestCompleteAIGenerationJob_ReturnsUpdateErrorAndMarksFailed(t *testing.T) {
	repo := &aiGatewayBridgeRepoStub{
		job: &service.AIGenerationJob{
			ID:     42,
			UserID: 7,
			Status: service.AIGenerationJobStatusRunning,
			Model:  "gpt-image-2",
			Prompt: "draw a cat",
		},
		updateErrs: []error{errors.New("persist completion failed"), nil},
	}
	h := &AIHandler{aiService: service.NewAICenterService(repo, nil)}

	status := service.AIGenerationJobStatusSucceeded
	model := "gpt-image-2"
	params := map[string]any{"gateway_status": 200}
	err := h.completeAIGenerationJob(context.Background(), 7, 42, nil, nil, &service.AIUpdateGenerationJobInput{
		Model:      &model,
		Status:     &status,
		Parameters: &params,
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "persist completion failed")
	require.Len(t, repo.updateStatuses, 2)
	require.Equal(t, service.AIGenerationJobStatusSucceeded, repo.updateStatuses[0])
	require.Equal(t, service.AIGenerationJobStatusFailed, repo.updateStatuses[1])
	require.Equal(t, service.AIGenerationJobStatusFailed, repo.job.Status)
}

func TestPropagateAIGenerationJobFailure_ReturnsJoinedErrorWhenFailUpdateFails(t *testing.T) {
	repo := &aiGatewayBridgeRepoStub{
		job: &service.AIGenerationJob{
			ID:     99,
			UserID: 8,
			Status: service.AIGenerationJobStatusRunning,
			Model:  "gpt-image-2",
			Prompt: "draw a fox",
		},
		updateErrs: []error{errors.New("persist failure status failed")},
	}
	h := &AIHandler{aiService: service.NewAICenterService(repo, nil)}

	cause := errors.New("gateway timeout")
	err := h.propagateAIGenerationJobFailure(context.Background(), 8, 99, nil, nil, cause)

	require.Error(t, err)
	require.ErrorContains(t, err, "gateway timeout")
	require.ErrorContains(t, err, "persist failure status failed")
	require.Len(t, repo.updateStatuses, 1)
	require.Equal(t, service.AIGenerationJobStatusFailed, repo.updateStatuses[0])
	require.Equal(t, service.AIGenerationJobStatusRunning, repo.job.Status)
}

func ptrString(v string) *string {
	return &v
}

type aiGatewayBridgeRepoStub struct {
	job            *service.AIGenerationJob
	updateErrs     []error
	updateStatuses []string
}

func (s *aiGatewayBridgeRepoStub) CreateSession(context.Context, *service.AISession) error {
	return nil
}

func (s *aiGatewayBridgeRepoStub) GetSessionByID(context.Context, int64) (*service.AISession, error) {
	return nil, errors.New("not implemented")
}

func (s *aiGatewayBridgeRepoStub) GetSessionByUserAndID(context.Context, int64, int64) (*service.AISession, error) {
	return nil, errors.New("not implemented")
}

func (s *aiGatewayBridgeRepoStub) ListSessions(context.Context, int64, pagination.PaginationParams, string) ([]service.AISession, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}

func (s *aiGatewayBridgeRepoStub) UpdateSession(context.Context, *service.AISession) error {
	return nil
}

func (s *aiGatewayBridgeRepoStub) DeleteSession(context.Context, int64) error {
	return nil
}

func (s *aiGatewayBridgeRepoStub) CreateSessionMessages(context.Context, []*service.AISessionMessage) error {
	return nil
}

func (s *aiGatewayBridgeRepoStub) ListSessionMessages(context.Context, int64, pagination.PaginationParams) ([]service.AISessionMessage, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}

func (s *aiGatewayBridgeRepoStub) CreatePromptTemplate(context.Context, *service.AIPromptTemplate, *service.AIPromptTemplateVersion) error {
	return nil
}

func (s *aiGatewayBridgeRepoStub) UpdatePromptTemplate(context.Context, *service.AIPromptTemplate, *service.AIPromptTemplateVersion) error {
	return nil
}

func (s *aiGatewayBridgeRepoStub) GetPromptTemplateByID(context.Context, int64) (*service.AIPromptTemplate, error) {
	return nil, errors.New("not implemented")
}

func (s *aiGatewayBridgeRepoStub) GetPromptTemplateByUserAndID(context.Context, int64, int64) (*service.AIPromptTemplate, error) {
	return nil, errors.New("not implemented")
}

func (s *aiGatewayBridgeRepoStub) ListPromptTemplates(context.Context, int64, bool, pagination.PaginationParams, service.AIListPromptTemplatesFilter) ([]service.AIPromptTemplate, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}

func (s *aiGatewayBridgeRepoStub) DeletePromptTemplate(context.Context, int64) error {
	return nil
}

func (s *aiGatewayBridgeRepoStub) CreateGenerationJob(context.Context, *service.AIGenerationJob) error {
	return nil
}

func (s *aiGatewayBridgeRepoStub) UpdateGenerationJob(_ context.Context, job *service.AIGenerationJob) error {
	s.updateStatuses = append(s.updateStatuses, job.Status)
	if len(s.updateErrs) > 0 {
		err := s.updateErrs[0]
		s.updateErrs = s.updateErrs[1:]
		if err != nil {
			return err
		}
	}
	cloned := *job
	s.job = &cloned
	return nil
}

func (s *aiGatewayBridgeRepoStub) GetGenerationJobByID(context.Context, int64) (*service.AIGenerationJob, error) {
	return nil, errors.New("not implemented")
}

func (s *aiGatewayBridgeRepoStub) GetGenerationJobByUserAndID(_ context.Context, userID, id int64) (*service.AIGenerationJob, error) {
	if s.job == nil || s.job.ID != id || s.job.UserID != userID {
		return nil, errors.New("job not found")
	}
	cloned := *s.job
	return &cloned, nil
}

func (s *aiGatewayBridgeRepoStub) ListGenerationJobs(context.Context, int64, bool, pagination.PaginationParams, service.AIListGenerationJobsFilter) ([]service.AIGenerationJob, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}

func (s *aiGatewayBridgeRepoStub) CreateAssets(context.Context, []*service.AIAsset) error {
	return nil
}

func (s *aiGatewayBridgeRepoStub) UpdateAsset(context.Context, *service.AIAsset) error {
	return nil
}

func (s *aiGatewayBridgeRepoStub) GetAssetByID(context.Context, int64) (*service.AIAsset, error) {
	return nil, errors.New("not implemented")
}

func (s *aiGatewayBridgeRepoStub) GetAssetByUserAndID(context.Context, int64, int64) (*service.AIAsset, error) {
	return nil, errors.New("not implemented")
}

func (s *aiGatewayBridgeRepoStub) ListAssets(context.Context, int64, bool, pagination.PaginationParams, service.AIListAssetsFilter) ([]service.AIAsset, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}

func (s *aiGatewayBridgeRepoStub) CreateAuditLog(context.Context, *service.AIAuditLog) error {
	return nil
}

func (s *aiGatewayBridgeRepoStub) ListAuditLogs(context.Context, pagination.PaginationParams, service.AIListAuditLogsFilter) ([]service.AIAuditLog, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("not implemented")
}
