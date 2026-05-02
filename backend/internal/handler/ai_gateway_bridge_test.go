package handler

import (
	"context"
	"testing"

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

func ptrString(v string) *string {
	return &v
}
