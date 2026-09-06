package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"unicode/utf8"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const creationVoiceUploadLimit = 25 << 20

type creationSpeechRequest struct {
	Text     string  `json:"text"`
	VoiceID  string  `json:"voice_id"`
	Language string  `json:"language"`
	Speed    float64 `json:"speed"`
}

func (h *CreationHandler) Speech(c *gin.Context) {
	h.withCreationVoice(c, "tts")
}

func (h *CreationHandler) Transcribe(c *gin.Context) {
	h.withCreationVoice(c, "stt")
}

func (h *CreationHandler) withCreationVoice(c *gin.Context, endpoint string) {
	h.withGatewayContext(c, func(c *gin.Context) {
		key, ok := middleware2.GetAPIKeyFromContext(c)
		// Native Grok voice does not accept Composite API-key contexts.
		if !ok || key == nil || key.Group == nil || key.Group.Platform != service.PlatformGrok {
			imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "Voice requires a Grok group")
			return
		}
		if h.openAI == nil {
			imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "Voice gateway unavailable")
			return
		}
		limit := int64(creationVoiceUploadLimit + (1 << 20))
		if endpoint == "tts" {
			limit = 128 << 10
		}
		body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, limit))
		if err != nil {
			imageTaskJSONError(c, http.StatusRequestEntityTooLarge, "invalid_request_error", "Voice request is too large")
			return
		}
		if endpoint == "tts" {
			body, err = normalizeCreationSpeech(body)
			c.Request.Header.Set("Content-Type", "application/json")
		} else {
			err = validateCreationTranscription(c.GetHeader("Content-Type"), body)
		}
		if err != nil {
			imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return
		}
		restoreCreationRequestBody(c, body)
		h.openAI.GrokVoice(c, endpoint)
	})
}

func normalizeCreationSpeech(body []byte) ([]byte, error) {
	request := creationSpeechRequest{Speed: 1}
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, errors.New("invalid speech request")
	}
	request.Text = strings.TrimSpace(request.Text)
	if count := utf8.RuneCountInString(request.Text); count == 0 || count > 15000 {
		return nil, errors.New("speech text must contain 1 to 15000 characters")
	}
	switch request.VoiceID {
	case "":
		request.VoiceID = "eve"
	case "eve", "ara", "leo", "rex", "sal":
	default:
		return nil, errors.New("unsupported voice")
	}
	switch request.Language {
	case "":
		request.Language = "auto"
	case "auto", "zh", "en":
	default:
		return nil, errors.New("unsupported speech language")
	}
	if math.IsNaN(request.Speed) || math.IsInf(request.Speed, 0) || request.Speed < 0.7 || request.Speed > 1.5 {
		return nil, errors.New("speech speed must be between 0.7 and 1.5")
	}
	// Omit upstream timestamp/format options: this surface returns playable MP3 bytes.
	return json.Marshal(request)
}

func validateCreationTranscription(contentType string, body []byte) error {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		return errors.New("transcription requires an audio file")
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	files := 0
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.New("invalid audio upload")
		}
		if part.FormName() != "file" || part.FileName() == "" {
			_ = part.Close()
			return errors.New("transcription accepts only the audio file field")
		}
		files++
		size, copyErr := io.Copy(io.Discard, part)
		_ = part.Close()
		if copyErr != nil || size == 0 || size > creationVoiceUploadLimit {
			return errors.New("audio file must be non-empty and at most 25 MiB")
		}
	}
	if files != 1 {
		return errors.New("transcription requires exactly one audio file")
	}
	return nil
}
