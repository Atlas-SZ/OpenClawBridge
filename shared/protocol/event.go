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
	EventMedia       = "media"
)

type MediaItem struct {
	Type     string `json:"type,omitempty"`
	URL      string `json:"url,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	FileName string `json:"fileName,omitempty"`
	Content  string `json:"content,omitempty"`
}

type Event struct {
	Type        string      `json:"type"`
	Content     string      `json:"content,omitempty"`
	Action      string      `json:"action,omitempty"`
	Code        string      `json:"code,omitempty"`
	Message     string      `json:"message,omitempty"`
	Attachments []MediaItem `json:"attachments,omitempty"`
	Media       []MediaItem `json:"media,omitempty"`
	To          string      `json:"to,omitempty"`
	Channel     string      `json:"channel,omitempty"`
	AccountID   string      `json:"accountId,omitempty"`
	SessionKey  string      `json:"sessionKey,omitempty"`
	MediaURL    string      `json:"mediaUrl,omitempty"`
	MediaURLs   []string    `json:"mediaUrls,omitempty"`
	GifPlayback bool        `json:"gifPlayback,omitempty"`
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
