package gatewayclient

import (
	"encoding/json"
	"strings"

	"openclaw-bridge/shared/protocol"
)

func decodePayload(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{}
	}
	return payload
}

func extractErrorMessage(env envelope) string {
	if env.Error != nil && env.Error.Message != "" {
		return env.Error.Message
	}
	payload := decodePayload(env.Payload)
	if msg := extractErrorMessageFromPayload(payload); msg != "" {
		return msg
	}
	return "unknown error"
}

func extractErrorMessageFromPayload(payload map[string]any) string {
	if errObj, ok := payload["error"].(map[string]any); ok {
		if msg := stringValue(errObj["message"]); msg != "" {
			return msg
		}
	}
	for _, key := range []string{"message", "msg", "reason"} {
		if msg := stringValue(payload[key]); msg != "" {
			return msg
		}
	}
	return "gateway event error"
}

func extractCorrelationID(env envelope, payload map[string]any) string {
	for _, key := range []string{"run_id", "runId", "request_id", "requestId", "req_id", "reqId", "id"} {
		if v := stringValue(payload[key]); v != "" {
			return v
		}
	}
	if env.ID != "" {
		return env.ID
	}
	return ""
}

func extractRunID(payload map[string]any) string {
	for _, key := range []string{"run_id", "runId"} {
		if runID := stringValue(payload[key]); runID != "" {
			return runID
		}
	}
	if runObj, ok := payload["run"].(map[string]any); ok {
		for _, key := range []string{"id", "run_id", "runId"} {
			if runID := stringValue(runObj[key]); runID != "" {
				return runID
			}
		}
	}
	for _, key := range []string{"response", "result", "data", "output"} {
		if nested, ok := payload[key].(map[string]any); ok {
			if runID := extractRunID(nested); runID != "" {
				return runID
			}
		}
	}
	return ""
}

func extractSessionID(payload map[string]any) string {
	for _, key := range []string{"session_id", "sessionId", "sid"} {
		if sid := stringValue(payload[key]); sid != "" {
			return sid
		}
	}
	return ""
}

func extractContent(payload map[string]any) string {
	for _, key := range []string{"content", "text", "token", "chunk", "delta"} {
		if content := stringValue(payload[key]); content != "" {
			return content
		}
	}
	return ""
}

func extractChatText(payload map[string]any) string {
	for _, key := range []string{"response", "result", "data", "output"} {
		if nested, ok := payload[key].(map[string]any); ok {
			if content := extractChatText(nested); content != "" {
				return content
			}
		}
	}

	if msgObj, ok := payload["message"].(map[string]any); ok {
		if content := messageContentText(msgObj["content"]); content != "" {
			return content
		}
	}
	if content := messageContentText(payload["content"]); content != "" {
		return content
	}
	return extractContent(payload)
}

func extractAgentText(payload map[string]any) string {
	for _, key := range []string{"delta", "text", "content", "message"} {
		if msgObj, ok := payload[key].(map[string]any); ok {
			if content := extractChatText(msgObj); content != "" {
				return content
			}
		}
	}
	if content := extractChatText(payload); content != "" {
		return content
	}
	return ""
}

func messageContentText(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		var builder strings.Builder
		for _, item := range t {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if kind := strings.ToLower(strings.TrimSpace(stringValue(part["type"]))); kind != "" && kind != "text" {
				continue
			}
			text := stringValue(part["text"])
			if text == "" {
				text = stringValue(part["value"])
			}
			builder.WriteString(text)
		}
		return builder.String()
	default:
		return ""
	}
}

func mapGatewayEvent(sessionID string, env envelope) []protocol.Event {
	payload := decodePayload(env.Payload)
	eventName := strings.ToLower(strings.TrimSpace(env.Event))
	status := strings.ToLower(strings.TrimSpace(stringValue(payload["status"])))

	if status != "" {
		switch {
		case isPendingStatus(status):
			if content := extractContent(payload); content != "" {
				return []protocol.Event{{Type: protocol.EventToken, Content: content}}
			}
			return nil
		case isFinalStatus(status):
			events := []protocol.Event{}
			if content := extractContent(payload); content != "" {
				events = append(events, protocol.Event{Type: protocol.EventToken, Content: content})
			}
			events = append(events, protocol.Event{Type: protocol.EventEnd})
			return events
		case isErrorStatus(status):
			return []protocol.Event{{
				Type:    protocol.EventError,
				Code:    "GATEWAY_EVENT_ERROR",
				Message: extractErrorMessageFromPayload(payload),
			}}
		}
	}

	if isChatEventName(eventName) {
		state := strings.ToLower(strings.TrimSpace(stringValue(payload["state"])))
		text := extractChatText(payload)
		switch state {
		case "delta":
			if text == "" {
				return nil
			}
			return []protocol.Event{{Type: protocol.EventToken, Content: text}}
		case "final", "done", "completed":
			events := []protocol.Event{}
			if text != "" {
				events = append(events, protocol.Event{Type: protocol.EventToken, Content: text})
			}
			events = append(events, protocol.Event{Type: protocol.EventEnd})
			return events
		case "error":
			return []protocol.Event{{Type: protocol.EventError, Code: "GATEWAY_EVENT_ERROR", Message: extractErrorMessageFromPayload(payload)}}
		case "aborted", "cancelled", "canceled":
			return []protocol.Event{{Type: protocol.EventEnd}}
		default:
			if text == "" {
				return nil
			}
			return []protocol.Event{{Type: protocol.EventToken, Content: text}}
		}
	}

	if isAgentEventName(eventName) {
		events := []protocol.Event{}
		if text := extractAgentText(payload); text != "" {
			events = append(events, protocol.Event{Type: protocol.EventToken, Content: text})
		}

		terminal := strings.ToLower(strings.TrimSpace(stringValue(payload["state"])))
		if terminal == "" {
			terminal = strings.ToLower(strings.TrimSpace(stringValue(payload["type"])))
		}
		switch terminal {
		case "final", "done", "completed", "end", "ended", "finish", "finished":
			events = append(events, protocol.Event{Type: protocol.EventEnd})
		case "error", "failed":
			events = append(events, protocol.Event{Type: protocol.EventError, Code: "GATEWAY_EVENT_ERROR", Message: extractErrorMessageFromPayload(payload)})
		}
		if len(events) == 0 {
			return nil
		}
		return events
	}

	switch {
	case isTokenEventName(eventName):
		if content := extractContent(payload); content != "" {
			return []protocol.Event{{Type: protocol.EventToken, Content: content}}
		}
	case isDoneEventName(eventName):
		return []protocol.Event{{Type: protocol.EventEnd}}
	case isErrorEventName(eventName):
		return []protocol.Event{{Type: protocol.EventError, Code: "GATEWAY_EVENT_ERROR", Message: extractErrorMessageFromPayload(payload)}}
	case isDisconnectEventName(eventName):
		return []protocol.Event{{Type: protocol.EventError, Code: "GATEWAY_DISCONNECTED", Message: "gateway disconnected"}}
	}

	_ = sessionID
	return nil
}

func isPendingStatus(status string) bool {
	switch status {
	case "accepted", "queued", "started", "running", "in_flight", "inflight", "pending":
		return true
	default:
		return false
	}
}

func isFinalStatus(status string) bool {
	switch status {
	case "ok", "done", "completed", "final", "ended", "end", "aborted", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func isErrorStatus(status string) bool {
	switch status {
	case "error", "failed":
		return true
	default:
		return false
	}
}

func isTokenEventName(name string) bool {
	return strings.Contains(name, "token") || strings.Contains(name, "chunk")
}

func isDoneEventName(name string) bool {
	return strings.Contains(name, "completed") || strings.Contains(name, "done") || strings.HasSuffix(name, ".end")
}

func isErrorEventName(name string) bool {
	return strings.Contains(name, "error")
}

func isDisconnectEventName(name string) bool {
	return strings.Contains(name, "disconnect")
}

func isChatEventName(name string) bool {
	return strings.Contains(name, "chat")
}

func isAgentEventName(name string) bool {
	return strings.Contains(name, "agent")
}
