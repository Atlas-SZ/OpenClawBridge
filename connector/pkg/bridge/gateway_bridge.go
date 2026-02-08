package bridge

import (
	"fmt"
	"log"
	"sync"

	"openclaw-bridge/shared/protocol"
)

type RelaySender interface {
	SendData(sessionID string, flags byte, payload []byte) error
}

type GatewaySender interface {
	SendUserMessage(sessionID, content string) error
	SendCancel(sessionID string) error
	IsReady() bool
}

type sessionState struct {
	flags byte
}

type GatewayBridge struct {
	logger  *log.Logger
	relay   RelaySender
	gateway GatewaySender

	mu       sync.RWMutex
	sessions map[string]sessionState
}

func NewGatewayBridge(logger *log.Logger, relay RelaySender) *GatewayBridge {
	return &GatewayBridge{
		logger:   logger,
		relay:    relay,
		sessions: make(map[string]sessionState),
	}
}

func (b *GatewayBridge) BindGateway(gateway GatewaySender) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.gateway = gateway
}

func (b *GatewayBridge) OpenSession(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.sessions[sessionID]; !ok {
		b.sessions[sessionID] = sessionState{}
	}
}

func (b *GatewayBridge) CloseSession(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.sessions, sessionID)
}

func (b *GatewayBridge) HandleData(sessionID string, flags byte, payload []byte) {
	event, err := protocol.DecodeEvent(payload)
	if err != nil {
		b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventError, Code: "BAD_EVENT", Message: "invalid event payload"})
		return
	}

	b.mu.Lock()
	if _, ok := b.sessions[sessionID]; !ok {
		b.mu.Unlock()
		b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventError, Code: "SESSION_NOT_OPEN", Message: "session not open"})
		return
	}
	state := b.sessions[sessionID]
	state.flags = flags
	b.sessions[sessionID] = state
	gateway := b.gateway
	b.mu.Unlock()

	if gateway == nil {
		b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventError, Code: "GATEWAY_NOT_CONFIGURED", Message: "gateway client not configured"})
		return
	}
	if !gateway.IsReady() {
		b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventError, Code: "GATEWAY_NOT_READY", Message: "gateway not ready"})
		return
	}

	switch event.Type {
	case protocol.EventUserMessage:
		if err := gateway.SendUserMessage(sessionID, event.Content); err != nil {
			b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventError, Code: "GATEWAY_SEND_FAILED", Message: err.Error()})
		}
	case "control":
		if event.Action != "stop" {
			b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventError, Code: "UNSUPPORTED_CONTROL", Message: "unsupported control action"})
			return
		}
		if err := gateway.SendCancel(sessionID); err != nil {
			b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventError, Code: "GATEWAY_CANCEL_FAILED", Message: err.Error()})
		}
	default:
		b.sendEvent(sessionID, flags, protocol.Event{Type: protocol.EventError, Code: "UNSUPPORTED_EVENT", Message: "unsupported event type"})
	}
}

func (b *GatewayBridge) HandleGatewayEvent(sessionID string, event protocol.Event) {
	sid, flags, ok := b.resolveSession(sessionID)
	if !ok {
		b.logger.Printf("drop gateway event without active session type=%s", event.Type)
		return
	}
	b.sendEvent(sid, flags, event)
}

func (b *GatewayBridge) HandleGatewayDisconnected(err error) {
	b.mu.RLock()
	active := make([]struct {
		sessionID string
		flags     byte
	}, 0, len(b.sessions))
	for sid, state := range b.sessions {
		active = append(active, struct {
			sessionID string
			flags     byte
		}{sessionID: sid, flags: state.flags})
	}
	b.mu.RUnlock()

	for _, s := range active {
		b.sendEvent(s.sessionID, s.flags, protocol.Event{Type: protocol.EventError, Code: "GATEWAY_DISCONNECTED", Message: fmt.Sprintf("gateway disconnected: %v", err)})
	}
}

func (b *GatewayBridge) resolveSession(sessionID string) (resolvedSessionID string, flags byte, ok bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if sessionID != "" {
		state, exists := b.sessions[sessionID]
		if exists {
			return sessionID, state.flags, true
		}
		return "", 0, false
	}

	if len(b.sessions) == 1 {
		for sid, state := range b.sessions {
			return sid, state.flags, true
		}
	}
	return "", 0, false
}

func (b *GatewayBridge) sendEvent(sessionID string, flags byte, event protocol.Event) {
	payload, err := protocol.EncodeEvent(event)
	if err != nil {
		b.logger.Printf("encode event error sid=%s err=%v", sessionID, err)
		return
	}
	if err := b.relay.SendData(sessionID, flags, payload); err != nil {
		b.logger.Printf("relay send error sid=%s err=%v", sessionID, err)
	}
}
