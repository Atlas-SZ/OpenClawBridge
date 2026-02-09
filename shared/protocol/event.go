package protocol

import (
	"encoding/json"
	"fmt"
)

const (
	EventUserMessage = "user_message"
	EventToken       = "token"
	EventEnd         = "end"
	EventError       = "error"
)

type ImageItem struct {
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

type Event struct {
	Type    string      `json:"type"`
	Content string      `json:"content,omitempty"`
	Images  []ImageItem `json:"images,omitempty"`
	Action  string      `json:"action,omitempty"`
	Code    string      `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
}

func EncodeEvent(event Event) ([]byte, error) {
	if event.Type == "" {
		return nil, fmt.Errorf("missing event type")
	}
	return json.Marshal(event)
}

func DecodeEvent(data []byte) (Event, error) {
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return Event{}, err
	}
	if event.Type == "" {
		return Event{}, fmt.Errorf("missing event type")
	}
	return event, nil
}
