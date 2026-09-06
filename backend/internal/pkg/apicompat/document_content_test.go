package apicompat

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

const inlinePDFTestData = "JVBERi0xLjQ="
const inlinePDFTestURI = "data:application/pdf;base64," + inlinePDFTestData

func documentTestAnthropicRequest(t *testing.T, block AnthropicContentBlock) *AnthropicRequest {
	t.Helper()
	content, err := json.Marshal([]AnthropicContentBlock{
		{Type: "text", Text: "Read the attachment"},
		{Type: "image", Source: &AnthropicImageSource{Type: "base64", MediaType: "image/png", Data: "iVBORw0KGgo="}},
		block,
		{Type: "text", Text: "Keep the summary short"},
	})
	require.NoError(t, err)
	return &AnthropicRequest{Model: "claude-sonnet-4", MaxTokens: 2048, Messages: []AnthropicMessage{{Role: "user", Content: content}}}
}

func TestInlineDocumentPDFPreservedAcrossAnthropicAndOpenAIBridges(t *testing.T) {
	document := AnthropicContentBlock{Type: "document", Title: "report.pdf", Source: &AnthropicImageSource{Type: "base64", MediaType: "application/pdf", Data: inlinePDFTestData}}
	req := documentTestAnthropicRequest(t, document)
	responses, err := AnthropicToResponses(req)
	require.NoError(t, err)
	var input []ResponsesInputItem
	require.NoError(t, json.Unmarshal(responses.Input, &input))
	require.Len(t, input, 1)
	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(input[0].Content, &parts))
	require.Len(t, parts, 4)
	require.Equal(t, "input_text", parts[0].Type)
	require.Equal(t, "input_image", parts[1].Type)
	require.Equal(t, ResponsesContentPart{Type: "input_file", Filename: "report.pdf", FileData: inlinePDFTestURI}, parts[2])
	require.Equal(t, "Keep the summary short", parts[3].Text)

	for name, convert := range map[string]func(*AnthropicRequest) (*ChatCompletionsRequest, error){
		"direct optimized":     AnthropicToChatCompletionsRequest,
		"direct compatibility": AnthropicRequestToChatCompletions,
		"via responses": func(*AnthropicRequest) (*ChatCompletionsRequest, error) {
			return ResponsesToChatCompletionsRequest(responses)
		},
	} {
		t.Run(name, func(t *testing.T) {
			chat, err := convert(req)
			require.NoError(t, err)
			require.Len(t, chat.Messages, 1)
			var content []ChatContentPart
			require.NoError(t, json.Unmarshal(chat.Messages[0].Content, &content))
			require.Len(t, content, 4)
			require.Equal(t, "text", content[0].Type)
			require.Equal(t, "image_url", content[1].Type)
			require.Equal(t, ChatContentPart{Type: "file", File: &ChatFile{Filename: "report.pdf", FileData: inlinePDFTestURI}}, content[2])
			require.Equal(t, "Keep the summary short", content[3].Text)

			roundTripResponses, err := ChatCompletionsToResponses(chat)
			require.NoError(t, err)
			roundTrip, err := ResponsesToAnthropicRequest(roundTripResponses)
			require.NoError(t, err)
			require.Len(t, roundTrip.Messages, 1)
			var blocks []AnthropicContentBlock
			require.NoError(t, json.Unmarshal(roundTrip.Messages[0].Content, &blocks))
			require.Len(t, blocks, 4)
			require.Equal(t, document, blocks[2])
		})
	}
}

func TestInlineDocumentChatFileBecomesAnthropicDocument(t *testing.T) {
	content, err := json.Marshal([]ChatContentPart{{Type: "file", File: &ChatFile{Filename: "uploaded.pdf", FileData: inlinePDFTestURI}}})
	require.NoError(t, err)
	responses, err := ChatCompletionsToResponses(&ChatCompletionsRequest{Model: "claude-sonnet-4", Messages: []ChatMessage{{Role: "user", Content: content}}})
	require.NoError(t, err)
	anthropic, err := ResponsesToAnthropicRequest(responses)
	require.NoError(t, err)
	require.Len(t, anthropic.Messages, 1)
	var blocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(anthropic.Messages[0].Content, &blocks))
	require.Len(t, blocks, 1)
	require.Equal(t, "document", blocks[0].Type)
	require.Equal(t, "uploaded.pdf", blocks[0].Title)
	require.Equal(t, &AnthropicImageSource{Type: "base64", MediaType: "application/pdf", Data: inlinePDFTestData}, blocks[0].Source)
}

func TestInlineDocumentTextIsDecodedInsteadOfBase64Prompt(t *testing.T) {
	for _, source := range []*AnthropicImageSource{
		{Type: "text", MediaType: "text/plain", Data: "Original document text"},
		{Type: "base64", MediaType: "text/plain", Data: base64.StdEncoding.EncodeToString([]byte("Original document text"))},
	} {
		block := AnthropicContentBlock{Type: "document", Title: "notes.txt", Source: source}
		part, err := anthropicDocumentToResponses(block)
		require.NoError(t, err)
		require.Equal(t, ResponsesContentPart{Type: "input_text", Text: "notes.txt\n\nOriginal document text"}, part)
		chat, err := anthropicDocumentToChat(block)
		require.NoError(t, err)
		require.Equal(t, ChatContentPart{Type: "text", Text: part.Text}, chat)
	}
	file := ResponsesContentPart{Type: "input_file", Filename: "notes.txt", FileData: "data:text/plain;charset=utf-8;base64," + base64.StdEncoding.EncodeToString([]byte("Original document text"))}
	block, err := responsesFileToAnthropic(file)
	require.NoError(t, err)
	require.Equal(t, "document", block.Type)
	require.Equal(t, "text", block.Source.Type)
	require.Equal(t, "Original document text", block.Source.Data)
	chat, err := responsesFileToChat(file)
	require.NoError(t, err)
	require.Equal(t, "text", chat.Type)
	require.Equal(t, "notes.txt\n\nOriginal document text", chat.Text)
}

func TestInlineDocumentUnsupportedAnthropicSourcesFailExplicitly(t *testing.T) {
	for name, source := range map[string]*AnthropicImageSource{
		"missing":           nil,
		"file ID":           {Type: "file"},
		"remote URL":        {Type: "url"},
		"unsupported media": {Type: "base64", MediaType: "application/msword", Data: inlinePDFTestData},
		"invalid base64":    {Type: "base64", MediaType: "application/pdf", Data: "not-base64"},
		"empty PDF":         {Type: "base64", MediaType: "application/pdf"},
		"not PDF":           {Type: "base64", MediaType: "application/pdf", Data: "aGVsbG8="},
		"binary text":       {Type: "base64", MediaType: "text/plain", Data: "/w=="},
	} {
		t.Run(name, func(t *testing.T) {
			req := documentTestAnthropicRequest(t, AnthropicContentBlock{Type: "document", Source: source})
			_, err := AnthropicToResponses(req)
			require.Error(t, err)
			_, err = AnthropicToChatCompletionsRequest(req)
			require.Error(t, err)
			_, err = AnthropicRequestToChatCompletions(req)
			require.Error(t, err)
		})
	}
}

func TestInlineDocumentUnsupportedResponsesFilesFailAnthropicBridge(t *testing.T) {
	for name, part := range map[string]ResponsesContentPart{
		"file ID":          {Type: "input_file", FileID: "file-private-123"},
		"remote URL":       {Type: "input_file", FileData: "https://storage.test/report.pdf"},
		"empty file":       {Type: "input_file", Filename: "empty.pdf"},
		"unsupported MIME": {Type: "input_file", FileData: "data:image/png;base64,iVBORw0KGgo="},
		"malformed base64": {Type: "input_file", FileData: "data:application/pdf;base64,invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			content, err := json.Marshal([]ResponsesContentPart{{Type: "input_text", Text: "read this"}, part})
			require.NoError(t, err)
			input, err := json.Marshal([]ResponsesInputItem{{Type: "message", Role: "user", Content: content}})
			require.NoError(t, err)
			_, err = ResponsesToAnthropicRequest(&ResponsesRequest{Model: "claude-sonnet-4", Input: input})
			require.Error(t, err, "the file must not disappear while the text prompt is forwarded")
		})
	}
}

func TestInlineDocumentChatFileIDIsRejectedOnlyAtCrossProviderBoundary(t *testing.T) {
	chat := &ChatCompletionsRequest{Model: "claude-sonnet-4", Messages: []ChatMessage{{Role: "user", Content: json.RawMessage(`[{"type":"file","file":{"file_id":"file-123"}}]`)}}}
	responses, err := ChatCompletionsToResponses(chat)
	require.NoError(t, err, "retain existing OpenAI-to-OpenAI file ID passthrough")
	_, err = ResponsesToAnthropicRequest(responses)
	require.ErrorContains(t, err, "file_id")
	back, err := ResponsesToChatCompletionsRequest(responses)
	require.NoError(t, err)
	require.Len(t, back.Messages, 1)
	var parts []ChatContentPart
	require.NoError(t, json.Unmarshal(back.Messages[0].Content, &parts))
	require.Len(t, parts, 1)
	require.Equal(t, "file-123", parts[0].File.FileID)
}
