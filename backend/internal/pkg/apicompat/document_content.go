package apicompat

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"strings"
	"unicode/utf8"
)

func anthropicDocumentToResponses(block AnthropicContentBlock) (ResponsesContentPart, error) {
	source := block.Source
	if source == nil {
		return ResponsesContentPart{}, fmt.Errorf("document requires an inline source")
	}
	var decoded []byte
	switch source.Type {
	case "text":
		if source.MediaType != "" && source.MediaType != "text/plain" {
			return ResponsesContentPart{}, fmt.Errorf("text document requires text/plain media_type")
		}
		decoded = []byte(source.Data)
	case "base64":
		var err error
		decoded, err = decodeInlineDocument(source.MediaType, source.Data)
		if err != nil {
			return ResponsesContentPart{}, err
		}
		if source.MediaType == "application/pdf" {
			filename := block.Title
			if strings.TrimSpace(filename) == "" {
				filename = "document.pdf"
			}
			return ResponsesContentPart{Type: "input_file", Filename: filename, FileData: "data:application/pdf;base64," + source.Data}, nil
		}
	default:
		return ResponsesContentPart{}, fmt.Errorf("unsupported document source %q: only inline PDF or text documents can be bridged", source.Type)
	}
	if len(decoded) == 0 || !utf8.Valid(decoded) {
		return ResponsesContentPart{}, fmt.Errorf("text document requires nonempty UTF-8 text")
	}
	text := string(decoded)
	if block.Title != "" {
		text = block.Title + "\n\n" + text
	}
	return ResponsesContentPart{Type: "input_text", Text: text}, nil
}

func decodeInlineDocument(mediaType, data string) ([]byte, error) {
	if mediaType != "application/pdf" && mediaType != "text/plain" {
		return nil, fmt.Errorf("unsupported document media_type %q: expected application/pdf or text/plain", mediaType)
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(decoded) == 0 {
		return nil, fmt.Errorf("document requires nonempty valid base64 data")
	}
	if mediaType == "application/pdf" && !bytes.HasPrefix(decoded, []byte("%PDF-")) {
		return nil, fmt.Errorf("document data does not contain a PDF header")
	}
	if mediaType == "text/plain" && !utf8.Valid(decoded) {
		return nil, fmt.Errorf("text document requires UTF-8 data")
	}
	return decoded, nil
}

func responsesFileToAnthropic(part ResponsesContentPart) (AnthropicContentBlock, error) {
	if part.FileID != "" {
		return AnthropicContentBlock{}, fmt.Errorf("input_file file_id cannot be bridged to Anthropic; provide inline file_data")
	}
	header, data, found := strings.Cut(part.FileData, ",")
	if !found || !strings.HasPrefix(header, "data:") || !strings.HasSuffix(header, ";base64") {
		return AnthropicContentBlock{}, fmt.Errorf("input_file requires an inline base64 PDF or text data URI")
	}
	mediaType, params, err := mime.ParseMediaType(strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64"))
	if err != nil {
		return AnthropicContentBlock{}, fmt.Errorf("input_file contains an invalid media type")
	}
	if charset := strings.ToLower(params["charset"]); charset != "" && charset != "utf-8" {
		return AnthropicContentBlock{}, fmt.Errorf("input_file supports only UTF-8 document encoding")
	}
	decoded, err := decodeInlineDocument(mediaType, data)
	if err != nil {
		return AnthropicContentBlock{}, err
	}
	source := &AnthropicImageSource{Type: "base64", MediaType: mediaType, Data: data}
	if mediaType == "text/plain" {
		source.Type, source.Data = "text", string(decoded)
	}
	return AnthropicContentBlock{Type: "document", Title: part.Filename, Source: source}, nil
}

func anthropicDocumentToChat(block AnthropicContentBlock) (ChatContentPart, error) {
	part, err := anthropicDocumentToResponses(block)
	if err != nil {
		return ChatContentPart{}, err
	}
	if part.Type == "input_text" {
		return ChatContentPart{Type: "text", Text: part.Text}, nil
	}
	return ChatContentPart{Type: "file", File: &ChatFile{Filename: part.Filename, FileData: part.FileData}}, nil
}

func responsesFileToChat(part ResponsesContentPart) (ChatContentPart, error) {
	// OpenAI file IDs can remain references inside an OpenAI protocol bridge.
	if part.FileID != "" && part.FileData == "" {
		return ChatContentPart{Type: "file", File: &ChatFile{Filename: part.Filename, FileID: part.FileID}}, nil
	}
	block, err := responsesFileToAnthropic(part)
	if err != nil {
		return ChatContentPart{}, err
	}
	return anthropicDocumentToChat(block)
}
