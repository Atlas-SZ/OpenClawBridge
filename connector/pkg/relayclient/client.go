package relayclient

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"openclaw-bridge/connector/pkg/config"
	"openclaw-bridge/shared/protocol"
)

type OnControlFunc func(protocol.ControlMessage)
type OnDataFunc func(sessionID string, flags byte, payload []byte)

type Client struct {
	cfg    config.Config
	logger *log.Logger

	onControl OnControlFunc
	onData    OnDataFunc

	connMu  sync.RWMutex
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func New(cfg config.Config, logger *log.Logger, onControl OnControlFunc, onData OnDataFunc) *Client {
	return &Client{
		cfg:       cfg,
		logger:    logger,
		onControl: onControl,
		onData:    onData,
	}
}

func (c *Client) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			c.closeConn()
			return nil
		default:
		}

		if err := c.connectAndServe(ctx); err != nil {
			c.logger.Printf("relay disconnected err=%v", err)
		}

		select {
		case <-ctx.Done():
			c.closeConn()
			return nil
		case <-time.After(time.Duration(c.cfg.ReconnectSeconds) * time.Second):
		}
	}
}

func (c *Client) connectAndServe(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.Dial(c.cfg.RelayURL, nil)
	if err != nil {
		return err
	}
	c.setConn(conn)

	if err := c.SendControl(protocol.ControlMessage{
		Type:           protocol.TypeRegister,
		AccessCodeHash: c.cfg.AccessCodeHash,
		Generation:     c.cfg.Generation,
		Caps:           &c.cfg.Caps,
	}); err != nil {
		c.closeConn()
		return err
	}

	heartbeatStop := make(chan struct{})
	go c.heartbeatLoop(ctx, heartbeatStop)
	defer close(heartbeatStop)

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			c.closeConn()
			return err
		}

		switch msgType {
		case websocket.TextMessage:
			msg, err := protocol.DecodeControl(data)
			if err != nil {
				continue
			}
			if c.onControl != nil {
				c.onControl(msg)
			}
		case websocket.BinaryMessage:
			sessionID, flags, payload, err := protocol.ParseDataFrame(data)
			if err != nil {
				continue
			}
			if c.onData != nil {
				c.onData(sessionID, flags, payload)
			}
		}
	}
}

func (c *Client) heartbeatLoop(ctx context.Context, stop <-chan struct{}) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			_ = c.SendControl(protocol.ControlMessage{Type: protocol.TypeHeartbeat})
		}
	}
}

func (c *Client) SendControl(msg protocol.ControlMessage) error {
	data, err := protocol.EncodeControl(msg)
	if err != nil {
		return err
	}
	return c.write(websocket.TextMessage, data)
}

func (c *Client) SendData(sessionID string, flags byte, payload []byte) error {
	frame, err := protocol.BuildDataFrame(sessionID, flags, payload)
	if err != nil {
		return err
	}
	return c.write(websocket.BinaryMessage, frame)
}

func (c *Client) write(msgType int, data []byte) error {
	conn := c.getConn()
	if conn == nil {
		return errors.New("relay not connected")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteMessage(msgType, data)
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
