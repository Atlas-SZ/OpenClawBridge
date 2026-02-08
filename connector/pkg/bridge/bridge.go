package bridge

import (
	"log"
	"strings"
	"sync"

	"openclaw-bridge/shared/protocol"
)

type Sender interface {
	SendData(sessionID string, flags byte, payload []byte) error
}

type EchoBridge struct {
	logger *log.Logger
	sender Sender

	mu       sync.RWMutex
	sessions map[string]bool
}

func NewEchoBridge(logger *log.Logger, sender Sender) *EchoBridge {
	return &EchoBridge{
		logger:   logger,
		sender:   sender,
		sessions: make(map[string]bool),
	}
}

func (b *EchoBridge) OpenSession(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sessions[sessionID] = true
}

func (b *EchoBridge) CloseSession(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.sessions, sessionID)
}

func (b *EchoBridge) HandleData(sessionID string, flags byte, payload []byte) {
	b.mu.RLock()
	opened := b.sessions[sessionID]
	b.mu.RUnlock()
	if !opened {
		b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventError, Code: "SESSION_NOT_OPEN", Message: "session not open"})
		return
	}

	event, err := protocol.DecodeEvent(payload)
	if err != nil {
		b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventError, Code: "BAD_EVENT", Message: "invalid event payload"})
		return
	}

	if event.Type != protocol.EventUserMessage {
		b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventError, Code: "UNSUPPORTED_EVENT", Message: "unsupported event type"})
		return
	}

	b.streamEcho(sessionID, flags, event.Content)
}

func (b *EchoBridge) streamEcho(sessionID string, flags byte, content string) {
	text := strings.TrimSpace(content)
	if text == "" {
		b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventEnd})
		return
	}

	parts := strings.Fields(text)
	for _, part := range parts {
		b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventToken, Content: part + " "})
	}
	b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventEnd})
}

func (b *EchoBridge) sendEvent(sessionID string, flags byte, event protocol.Event) {
	payload, err := protocol.EncodeEvent(event)
	if err != nil {
		b.logger.Printf("encode event error sid=%s err=%v", sessionID, err)
		return
	}
	if err := b.sender.SendData(sessionID, flags, payload); err != nil {
		b.logger.Printf("send event error sid=%s err=%v", sessionID, err)
	}
}
