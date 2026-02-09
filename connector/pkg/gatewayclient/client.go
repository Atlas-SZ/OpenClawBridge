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

	mapMu        sync.RWMutex
	reqToSession map[string]string
	runToSession map[string]string
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
		reqToSession: map[string]string{},
		runToSession: map[string]string{},
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

func (c *Client) connectAndServe(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.Dial(c.cfg.URL, nil)
	if err != nil {
		return err
	}
	c.setConn(conn)
	c.setReady(false)
	connDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-connDone:
		}
	}()
	defer func() {
		close(connDone)
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
	c.logger.Printf("gateway ready url=%s client_id=%s", c.cfg.URL, gatewayClientID)
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

		if env.Type == "event" && isErrorEventName(strings.ToLower(strings.TrimSpace(env.Event))) {
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
				"id":          gatewayClientID,
				"displayName": gatewayClientDisplayName,
				"version":     gatewayClientVersion,
				"platform":    gatewayClientPlatform,
				"mode":        gatewayClientMode,
			},
			"role":      "operator",
			"scopes":    normalizeScopes(c.cfg.Scopes),
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

		if env.Type == "event" && isErrorEventName(strings.ToLower(strings.TrimSpace(env.Event))) {
			errMsg := extractErrorMessage(env)
			if isUnauthorized(errMsg) {
				return fmt.Errorf("%w: %s", ErrGatewayAuthFailed, errMsg)
			}
			return fmt.Errorf("gateway connect event error: %s", errMsg)
		}
	}
}

func (c *Client) SendUserMessage(sessionID string, event protocol.Event) error {
	content := strings.TrimSpace(event.Content)
	if content == "" {
		return errors.New("content is required")
	}

	reqID := newID("gw_req_")
	params := map[string]any{
		"sessionKey":     gatewaySessionKey(sessionID),
		"message":        content,
		"idempotencyKey": reqID,
	}
	if images := normalizeImages(event.Images); len(images) > 0 {
		params["images"] = images
	}

	c.trackRequest(reqID, sessionID)
	msg := map[string]any{
		"type":   "req",
		"id":     reqID,
		"method": "agent",
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
		"method": "chat.abort",
		"params": map[string]any{
			"sessionKey": gatewaySessionKey(sessionID),
		},
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

func (c *Client) handleResponse(env envelope) error {
	if env.ID == "" {
		return nil
	}

	sessionID, ok := c.requestSession(env.ID)
	if !ok {
		return nil
	}

	payload := decodePayload(env.Payload)
	if runID := extractRunID(payload); runID != "" {
		c.trackRun(runID, sessionID)
	}

	if env.OK != nil && *env.OK {
		status := strings.ToLower(strings.TrimSpace(stringValue(payload["status"])))
		switch {
		case isPendingStatus(status):
			if content := extractContent(payload); content != "" {
				c.emitEvent(sessionID, protocol.Event{Type: protocol.EventToken, Content: content})
			}
		case isFinalStatus(status):
			if content := extractContent(payload); content != "" {
				c.emitEvent(sessionID, protocol.Event{Type: protocol.EventToken, Content: content})
			}
			c.emitEvent(sessionID, protocol.Event{Type: protocol.EventEnd})
			c.clearSessionTracks(sessionID)
		case isErrorStatus(status):
			c.emitEvent(sessionID, protocol.Event{
				Type:    protocol.EventError,
				Code:    "GATEWAY_REQUEST_FAILED",
				Message: extractErrorMessageFromPayload(payload),
			})
			c.clearSessionTracks(sessionID)
		default:
			if content := extractContent(payload); content != "" {
				c.emitEvent(sessionID, protocol.Event{Type: protocol.EventToken, Content: content})
				c.emitEvent(sessionID, protocol.Event{Type: protocol.EventEnd})
				c.clearSessionTracks(sessionID)
			}
		}
		return nil
	}

	errMsg := extractErrorMessage(env)
	c.emitEvent(sessionID, protocol.Event{Type: protocol.EventError, Code: "GATEWAY_REQUEST_FAILED", Message: errMsg})
	c.clearSessionTracks(sessionID)
	return nil
}

func (c *Client) handleEvent(env envelope) {
	payload := decodePayload(env.Payload)
	corrID := extractCorrelationID(env, payload)
	runID := extractRunID(payload)
	sessionID := c.resolveSessionID(corrID, runID, payload)
	if sessionID == "" {
		return
	}

	events := mapGatewayEvent(sessionID, env)
	for _, event := range events {
		c.emitEvent(sessionID, event)
		if event.Type == protocol.EventEnd || event.Type == protocol.EventError {
			c.clearSessionTracks(sessionID)
		}
	}
}

func (c *Client) resolveSessionID(corrID, runID string, payload map[string]any) string {
	if sid := extractSessionID(payload); sid != "" {
		return sid
	}
	if corrID != "" {
		if sid, ok := c.requestSession(corrID); ok {
			return sid
		}
	}
	if runID != "" {
		if sid, ok := c.runSession(runID); ok {
			return sid
		}
	}
	if corrID != "" {
		if sid, ok := c.runSession(corrID); ok {
			return sid
		}
	}
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.lastSessionID
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
	c.mapMu.Lock()
	defer c.mapMu.Unlock()
	c.reqToSession[reqID] = sessionID

	c.stateMu.Lock()
	c.lastSessionID = sessionID
	c.stateMu.Unlock()
}

func (c *Client) untrackRequest(reqID string) {
	c.mapMu.Lock()
	defer c.mapMu.Unlock()
	delete(c.reqToSession, reqID)
}

func (c *Client) requestSession(reqID string) (string, bool) {
	c.mapMu.RLock()
	defer c.mapMu.RUnlock()
	sid, ok := c.reqToSession[reqID]
	return sid, ok
}

func (c *Client) trackRun(runID, sessionID string) {
	c.mapMu.Lock()
	defer c.mapMu.Unlock()
	c.runToSession[runID] = sessionID
}

func (c *Client) runSession(runID string) (string, bool) {
	c.mapMu.RLock()
	defer c.mapMu.RUnlock()
	sid, ok := c.runToSession[runID]
	return sid, ok
}

func (c *Client) clearSessionTracks(sessionID string) {
	c.mapMu.Lock()
	defer c.mapMu.Unlock()
	for reqID, sid := range c.reqToSession {
		if sid == sessionID {
			delete(c.reqToSession, reqID)
		}
	}
	for runID, sid := range c.runToSession {
		if sid == sessionID {
			delete(c.runToSession, runID)
		}
	}
}

func (c *Client) emitEvent(sessionID string, event protocol.Event) {
	if c.handlers.OnEvent == nil {
		return
	}
	c.handlers.OnEvent(sessionID, event)
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

func normalizeImages(images []protocol.ImageItem) []map[string]any {
	out := make([]map[string]any, 0, len(images))
	for _, item := range images {
		data := strings.TrimSpace(item.Data)
		if data == "" {
			continue
		}
		m := map[string]any{"data": data}
		if mimeType := strings.TrimSpace(item.MimeType); mimeType != "" {
			m["mimeType"] = mimeType
		}
		out = append(out, m)
	}
	return out
}

func isUnauthorized(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "unauthorized") || strings.Contains(lower, "forbidden")
}

func gatewaySessionKey(sessionID string) string {
	return "bridge_" + sessionID
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
