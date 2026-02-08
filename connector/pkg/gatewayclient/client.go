package gatewayclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"openclaw-bridge/connector/pkg/config"
	"openclaw-bridge/shared/protocol"
)

var ErrGatewayAuthFailed = errors.New("gateway auth failed")

type Handlers struct {
	OnEvent        func(sessionID string, event protocol.Event)
	OnDisconnected func(err error)
	OnReady        func()
}

type Client struct {
	cfg      config.GatewayConfig
	logger   *log.Logger
	handlers Handlers

	connMu  sync.RWMutex
	conn    *websocket.Conn
	writeMu sync.Mutex

	stateMu       sync.RWMutex
	ready         bool
	lastSessionID string

	reqMu        sync.RWMutex
	reqToSession map[string]string
}

type envelope struct {
	Type    string          `json:"type"`
	Event   string          `json:"event,omitempty"`
	ID      string          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	OK      *bool           `json:"ok,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Error   *gatewayError   `json:"error,omitempty"`
}

type gatewayError struct {
	Message string `json:"message,omitempty"`
}

func New(cfg config.GatewayConfig, logger *log.Logger, handlers Handlers) *Client {
	return &Client{
		cfg:          cfg,
		logger:       logger,
		handlers:     handlers,
		reqToSession: make(map[string]string),
	}
}

func (c *Client) Run(ctx context.Context) error {
	backoff := time.Duration(c.cfg.ReconnectInitialSeconds) * time.Second
	maxBackoff := time.Duration(c.cfg.ReconnectMaxSeconds) * time.Second

	for {
		select {
		case <-ctx.Done():
			c.closeConn()
			return nil
		default:
		}

		err := c.connectAndServe(ctx)
		if err == nil {
			backoff = time.Duration(c.cfg.ReconnectInitialSeconds) * time.Second
			continue
		}
		if errors.Is(err, ErrGatewayAuthFailed) {
			return fmt.Errorf("%w: %v", ErrGatewayAuthFailed, err)
		}

		if c.handlers.OnDisconnected != nil {
			c.handlers.OnDisconnected(err)
		}
		c.logger.Printf("gateway disconnected err=%v retry_in=%s", err, backoff)

		select {
		case <-ctx.Done():
			c.closeConn()
			return nil
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (c *Client) SendUserMessage(sessionID, content string) error {
	reqID := newID("gw_req_")
	c.trackRequest(reqID, sessionID)

	params := map[string]any{
		"content": content,
	}

	msg := map[string]any{
		"type":   "req",
		"id":     reqID,
		"method": c.cfg.SendMethod,
		"params": params,
	}

	if err := c.writeJSON(msg); err != nil {
		c.untrackRequest(reqID)
		return err
	}
	return nil
}

func (c *Client) SendCancel(sessionID string) error {
	reqID := newID("gw_cancel_")
	c.trackRequest(reqID, sessionID)

	msg := map[string]any{
		"type":   "req",
		"id":     reqID,
		"method": c.cfg.CancelMethod,
		"params": map[string]any{},
	}

	if err := c.writeJSON(msg); err != nil {
		c.untrackRequest(reqID)
		return err
	}
	return nil
}

func (c *Client) IsReady() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.ready
}

func (c *Client) connectAndServe(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.Dial(c.cfg.URL, nil)
	if err != nil {
		return err
	}
	c.setConn(conn)
	c.setReady(false)
	defer func() {
		c.setReady(false)
		c.closeConn()
	}()

	if err := c.waitForChallenge(conn); err != nil {
		return err
	}
	if err := c.performConnect(conn); err != nil {
		return err
	}

	_ = conn.SetReadDeadline(time.Time{})
	c.setReady(true)
	c.logger.Printf("gateway ready url=%s client_id=%s", c.cfg.URL, c.cfg.Client.ID)
	if c.handlers.OnReady != nil {
		c.handlers.OnReady()
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		if msgType != websocket.TextMessage {
			continue
		}

		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}

		switch env.Type {
		case "event":
			c.handleEvent(env)
		case "res":
			if err := c.handleResponse(env); err != nil {
				return err
			}
		}
	}
}

func (c *Client) waitForChallenge(conn *websocket.Conn) error {
	timeout := time.Duration(c.cfg.ChallengeTimeoutSeconds) * time.Second
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("wait challenge: %w", err)
		}
		if msgType != websocket.TextMessage {
			continue
		}

		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}

		if env.Type == "event" && env.Event == "connect.challenge" {
			return nil
		}

		if env.Type == "event" && isErrorEventName(env.Event) {
			return fmt.Errorf("gateway challenge failed: %s", extractErrorMessage(env))
		}

		if env.Type == "res" && env.OK != nil && !*env.OK {
			errMsg := extractErrorMessage(env)
			if isUnauthorized(errMsg) {
				return fmt.Errorf("%w: %s", ErrGatewayAuthFailed, errMsg)
			}
			return fmt.Errorf("gateway pre-connect response failed: %s", errMsg)
		}
	}
}

func (c *Client) performConnect(conn *websocket.Conn) error {
	reqID := newID("gw_connect_")
	connectReq := map[string]any{
		"type":   "req",
		"id":     reqID,
		"method": "connect",
		"params": map[string]any{
			"minProtocol": c.cfg.MinProtocol,
			"maxProtocol": c.cfg.MaxProtocol,
			"auth": map[string]any{
				"token": c.cfg.Auth.Token,
			},
			"client": map[string]any{
				"id":          c.cfg.Client.ID,
				"displayName": c.cfg.Client.DisplayName,
				"version":     c.cfg.Client.Version,
				"platform":    c.cfg.Client.Platform,
				"mode":        c.cfg.Client.Mode,
			},
			"role":      "operator",
			"scopes":    c.cfg.Scopes,
			"caps":      []any{},
			"locale":    c.cfg.Locale,
			"userAgent": c.cfg.UserAgent,
		},
	}

	payload, err := json.Marshal(connectReq)
	if err != nil {
		return err
	}
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return err
	}

	timeout := time.Duration(c.cfg.ChallengeTimeoutSeconds) * time.Second
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("wait connect response: %w", err)
		}
		if msgType != websocket.TextMessage {
			continue
		}

		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}

		if env.Type == "res" && env.ID == reqID {
			if env.OK != nil && *env.OK {
				return nil
			}
			errMsg := extractErrorMessage(env)
			if isUnauthorized(errMsg) {
				return fmt.Errorf("%w: %s", ErrGatewayAuthFailed, errMsg)
			}
			return fmt.Errorf("gateway connect failed: %s", errMsg)
		}

		if env.Type == "event" && isErrorEventName(env.Event) {
			errMsg := extractErrorMessage(env)
			if isUnauthorized(errMsg) {
				return fmt.Errorf("%w: %s", ErrGatewayAuthFailed, errMsg)
			}
			return fmt.Errorf("gateway connect event error: %s", errMsg)
		}
	}
}

func (c *Client) handleResponse(env envelope) error {
	if env.ID == "" {
		return nil
	}

	sessionID, ok := c.requestSession(env.ID)
	if !ok {
		return nil
	}

	if env.OK == nil || *env.OK {
		if strings.HasPrefix(env.ID, "gw_cancel_") {
			c.untrackRequest(env.ID)
		}
		return nil
	}

	errMsg := extractErrorMessage(env)
	if isUnauthorized(errMsg) {
		return fmt.Errorf("%w: %s", ErrGatewayAuthFailed, errMsg)
	}

	c.emitEvent(sessionID, protocol.Event{Type: protocol.EventError, Code: "GATEWAY_REQUEST_FAILED", Message: errMsg})
	c.untrackRequest(env.ID)
	return nil
}

func (c *Client) handleEvent(env envelope) {
	eventName := strings.ToLower(env.Event)
	if eventName == "" {
		return
	}

	payload := decodePayload(env.Payload)
	corrID := extractCorrelationID(env, payload)
	sessionID := c.resolveSessionID(corrID, payload)

	switch {
	case isTokenEventName(eventName):
		content := extractContent(payload)
		if content == "" {
			return
		}
		c.emitEvent(sessionID, protocol.Event{Type: protocol.EventToken, Content: content})
	case isDoneEventName(eventName):
		c.emitEvent(sessionID, protocol.Event{Type: protocol.EventEnd})
		if corrID != "" {
			c.untrackRequest(corrID)
		}
	case isErrorEventName(eventName):
		c.emitEvent(sessionID, protocol.Event{Type: protocol.EventError, Code: "GATEWAY_EVENT_ERROR", Message: extractErrorMessageFromPayload(payload)})
		if corrID != "" {
			c.untrackRequest(corrID)
		}
	case isDisconnectEventName(eventName):
		c.emitEvent(sessionID, protocol.Event{Type: protocol.EventError, Code: "GATEWAY_DISCONNECTED", Message: "gateway disconnected"})
	}
}

func (c *Client) emitEvent(sessionID string, event protocol.Event) {
	if c.handlers.OnEvent == nil {
		return
	}
	c.handlers.OnEvent(sessionID, event)
}

func (c *Client) writeJSON(v any) error {
	if !c.IsReady() {
		return errors.New("gateway not ready")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	conn := c.getConn()
	if conn == nil {
		return errors.New("gateway disconnected")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) trackRequest(reqID, sessionID string) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()
	c.reqToSession[reqID] = sessionID

	c.stateMu.Lock()
	c.lastSessionID = sessionID
	c.stateMu.Unlock()
}

func (c *Client) requestSession(reqID string) (string, bool) {
	c.reqMu.RLock()
	defer c.reqMu.RUnlock()
	sessionID, ok := c.reqToSession[reqID]
	return sessionID, ok
}

func (c *Client) untrackRequest(reqID string) {
	c.reqMu.Lock()
	defer c.reqMu.Unlock()
	delete(c.reqToSession, reqID)
}

func (c *Client) resolveSessionID(corrID string, payload map[string]any) string {
	if sid := extractSessionID(payload); sid != "" {
		return sid
	}
	if corrID != "" {
		if sid, ok := c.requestSession(corrID); ok {
			return sid
		}
	}

	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.lastSessionID
}

func (c *Client) setConn(conn *websocket.Conn) {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	c.conn = conn
}

func (c *Client) getConn() *websocket.Conn {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.conn
}

func (c *Client) closeConn() {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

func (c *Client) setReady(ready bool) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.ready = ready
}

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

func extractCorrelationID(env envelope, payload map[string]any) string {
	for _, key := range []string{"request_id", "requestId", "req_id", "reqId", "id"} {
		if v := stringValue(payload[key]); v != "" {
			return v
		}
	}
	if env.ID != "" {
		return env.ID
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

func isUnauthorized(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden")
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

func stringValue(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		return fmt.Sprintf("%.0f", t)
	default:
		return ""
	}
}

func newID(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
}
