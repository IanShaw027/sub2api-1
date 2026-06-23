package service

import (
	"encoding/json"
	"strings"

	"github.com/gorilla/websocket"
)

func writeNativeCaptureWSResponse(conn *websocket.Conn, req TLSFingerprintCaptureNativeSubmitRequest, result *TLSFingerprintCaptureSubmitResult) error {
	if conn == nil {
		return nil
	}
	if err := writeNativeCaptureWSEvent(conn, "response.created", "in_progress", req, nil); err != nil {
		return err
	}
	return writeNativeCaptureWSEvent(conn, "response.completed", "completed", req, result)
}

func writeNativeCaptureWSEvent(conn *websocket.Conn, eventType, status string, req TLSFingerprintCaptureNativeSubmitRequest, result *TLSFingerprintCaptureSubmitResult) error {
	payload, err := buildNativeCaptureWSEventPayload(eventType, status, req, result)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, payload)
}

func buildNativeCaptureWSEventPayload(eventType, status string, req TLSFingerprintCaptureNativeSubmitRequest, result *TLSFingerprintCaptureSubmitResult) ([]byte, error) {
	response := map[string]any{
		"id":     "resp_capture_mock",
		"object": "response",
		"status": status,
	}
	if model := strings.TrimSpace(req.Model); model != "" {
		response["model"] = model
	}
	if status == "completed" {
		response["output"] = []any{}
	}
	event := map[string]any{
		"type":     eventType,
		"response": response,
	}
	if result != nil {
		capture := map[string]any{
			"accepted": result.Accepted,
		}
		if result.IgnoredReason != "" {
			capture["ignored_reason"] = result.IgnoredReason
		}
		if result.FingerprintHash != "" {
			capture["fingerprint_hash"] = result.FingerprintHash
		}
		event["capture"] = capture
	}
	return json.Marshal(event)
}
