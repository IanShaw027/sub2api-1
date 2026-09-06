package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const localImageResultMaxBytes = 32 << 20

func materializeLocalImageResult(ctx context.Context, result json.RawMessage, client *http.Client) (json.RawMessage, error) {
	if len(result) > localImageResultMaxBytes {
		return nil, fmt.Errorf("local image result exceeds size limit")
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(result, &payload); err != nil {
		return nil, err
	}
	var images []map[string]json.RawMessage
	if err := json.Unmarshal(payload["data"], &images); err != nil || len(images) == 0 {
		return nil, fmt.Errorf("image result contains no images")
	}
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		transport.DialContext = safeDialContext
		client = &http.Client{Transport: transport, Timeout: time.Minute}
		defer transport.CloseIdleConnections()
	}
	fetcher := NewImageResultUploader(nil, "", localImageResultMaxBytes, client)
	encodedBytes := 0
	for _, item := range images {
		data, mime, err := fetcher.fetchImageBytes(ctx, item)
		if err != nil {
			return nil, err
		}
		encodedBytes += base64.StdEncoding.EncodedLen(len(data))
		if encodedBytes > localImageResultMaxBytes {
			return nil, fmt.Errorf("local image result exceeds size limit")
		}
		item["b64_json"], _ = json.Marshal(base64.StdEncoding.EncodeToString(data))
		item["mime_type"], _ = json.Marshal(mime)
		delete(item, "url")
	}
	payload["data"], _ = json.Marshal(images)
	out, err := json.Marshal(payload)
	if len(out) > localImageResultMaxBytes {
		return nil, fmt.Errorf("local image result exceeds size limit")
	}
	return out, err
}
