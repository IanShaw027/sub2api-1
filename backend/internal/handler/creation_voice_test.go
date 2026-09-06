package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCreationSpeechNormalizesNativeGrokPayload(t *testing.T) {
	body, err := normalizeCreationSpeech([]byte(`{"text":" hello ","with_timestamps":true,"output_format":{"codec":"pcm"}}`))
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Equal(t, map[string]any{"text": "hello", "voice_id": "eve", "language": "auto", "speed": float64(1)}, payload)
}

func TestCreationSpeechRejectsInvalidInputs(t *testing.T) {
	for _, body := range []string{
		`{}`, `{"text":"  "}`, `{"text":4}`, `{"text":"hi","voice_id":"custom-other-account"}`,
		`{"text":"hi","language":"invalid"}`, `{"text":"hi","speed":0}`, `{"text":"hi","speed":1.6}`,
		`{"text":"` + strings.Repeat("x", 15001) + `"}`,
	} {
		t.Run(body[:min(len(body), 50)], func(t *testing.T) {
			_, err := normalizeCreationSpeech([]byte(body))
			require.Error(t, err)
		})
	}
	_, err := normalizeCreationSpeech([]byte(`{"text":"` + strings.Repeat("中", 15000) + `","speed":0.7,"language":"zh","voice_id":"ara"}`))
	require.NoError(t, err)
}

func TestCreationTranscriptionValidatesSingleBoundedUpload(t *testing.T) {
	for _, tc := range []struct {
		name  string
		count int
		size  int
		valid bool
	}{
		{"one audio", 1, 16, true}, {"missing file", 0, 0, false},
		{"empty file", 1, 0, false}, {"multiple files", 2, 16, false},
		{"oversized file", 1, creationVoiceUploadLimit + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			for i := 0; i < tc.count; i++ {
				part, err := writer.CreateFormFile("file", "recording.mp3")
				require.NoError(t, err)
				_, err = part.Write(bytes.Repeat([]byte("a"), tc.size))
				require.NoError(t, err)
			}
			require.NoError(t, writer.Close())
			err := validateCreationTranscription(writer.FormDataContentType(), body.Bytes())
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
	require.Error(t, validateCreationTranscription("application/json", []byte(`{"audio_url":"https://example.com"}`)))
}

func TestCreationVoiceRejectsUnsupportedGroupBeforeGatewayDispatch(t *testing.T) {
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformComposite, service.PlatformAnthropic} {
		t.Run(platform, func(t *testing.T) {
			for _, endpoint := range []string{"tts", "stt"} {
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/creation/audio/speech", strings.NewReader(`{"text":"hello"}`))
				middleware2.InjectGatewayContextFromAPIKey(c, &service.APIKey{Group: &service.Group{Platform: platform}, User: &service.User{ID: 1}}, nil)
				c.Set(creationGatewayPreparedKey, true)
				(&CreationHandler{}).withCreationVoice(c, endpoint)
				require.Equal(t, http.StatusNotFound, recorder.Code)
			}
		})
	}
}
