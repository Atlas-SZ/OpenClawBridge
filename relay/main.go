package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"

	"openclaw-bridge/relay/pkg/authmap"
	"openclaw-bridge/relay/pkg/hub"
	"openclaw-bridge/relay/pkg/metrics"
	"openclaw-bridge/relay/pkg/ratelimit"
	"openclaw-bridge/relay/pkg/sessions"
	"openclaw-bridge/shared/protocol"
)

type relayServer struct {
	logger *log.Logger

	upgrader websocket.Upgrader

	hub       *hub.Manager
	auth      *authmap.Store
	sessions  *sessions.Store
	ratelimit *ratelimit.Limiter
	metrics   *metrics.Collector
}

func newRelayServer(logger *log.Logger) *relayServer {
	return &relayServer{
		logger: logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(_ *http.Request) bool { return true },
		},
		hub:       hub.NewManager(),
		auth:      authmap.NewStore(),
		sessions:  sessions.NewStore(),
		ratelimit: ratelimit.New(),
		metrics:   metrics.New(),
	}
}

func (s *relayServer) sendControl(peer *hub.Peer, msg protocol.ControlMessage) error {
	data, err := protocol.EncodeControl(msg)
	if err != nil {
		return err
	}
	return peer.SendText(data)
}

func (s *relayServer) sendError(peer *hub.Peer, code, message string) {
	err := s.sendControl(peer, protocol.ControlMessage{
		Type:    protocol.TypeError,
		Code:    code,
		Message: message,
	})
	if err != nil {
		s.logger.Printf("error send error-msg peer=%s err=%v", peer.ID, err)
		s.metrics.IncError()
	}
}

func (s *relayServer) routeBinary(sender *hub.Peer, frame []byte) {
	sessionID, _, _, err := protocol.ParseDataFrame(frame)
	if err != nil {
		s.metrics.IncError()
		s.sendError(sender, "BAD_DATA_FRAME", "invalid data frame")
		return
	}

	session, ok := s.sessions.Get(sessionID)
	if !ok {
		s.metrics.IncError()
		s.sendError(sender, "SESSION_NOT_FOUND", "session not found")
		return
	}

	var target *hub.Peer
	switch sender {
	case session.Client:
		target = session.Connector
	case session.Connector:
		target = session.Client
	default:
		s.metrics.IncError()
		s.sendError(sender, "SESSION_PEER_MISMATCH", "session peer mismatch")
		return
	}

	if err := target.SendBinary(frame); err != nil {
		s.metrics.IncError()
		s.logger.Printf("error forward sid=%s bytes=%d err=%v", sessionID, len(frame), err)
		s.closeSession(sessionID)
		return
	}

	s.metrics.AddForwardedBytes(len(frame))
	s.logger.Printf("forward sid=%s bytes=%d", sessionID, len(frame))
}

func (s *relayServer) closeSession(sessionID string) {
	session, ok := s.sessions.Delete(sessionID)
	if !ok {
		return
	}

	closeMsg := protocol.ControlMessage{
		Type:      protocol.TypeCloseSession,
		SessionID: sessionID,
	}

	_ = s.sendControl(session.Client, closeMsg)
	_ = s.sendControl(session.Connector, closeMsg)
	s.logger.Printf("session closed sid=%s", sessionID)
}

func (s *relayServer) cleanupPeer(peer *hub.Peer) {
	removedHashes := s.auth.DeleteByPeer(peer)
	for _, hash := range removedHashes {
		s.logger.Printf("connector removed hash=%s", hash)
	}

	removedSessions := s.sessions.DeleteByPeer(peer)
	for _, session := range removedSessions {
		other := session.Client
		if other == peer {
			other = session.Connector
		}
		if other != nil {
			_ = s.sendControl(other, protocol.ControlMessage{
				Type:      protocol.TypeCloseSession,
				SessionID: session.ID,
			})
		}
		s.logger.Printf("session removed sid=%s reason=peer_disconnect", session.ID)
	}

	s.hub.Remove(peer.ID)
	_ = peer.Conn.Close()
}

func (s *relayServer) connectorLoop(peer *hub.Peer) {
	defer s.cleanupPeer(peer)

	for {
		msgType, data, err := peer.Conn.ReadMessage()
		if err != nil {
			s.logger.Printf("connector disconnect peer=%s err=%v", peer.ID, err)
			return
		}

		switch msgType {
		case websocket.TextMessage:
			msg, err := protocol.DecodeControl(data)
			if err != nil {
				s.metrics.IncError()
				s.sendError(peer, "BAD_CONTROL", "invalid control message")
				continue
			}
			s.handleControl(peer, msg)
		case websocket.BinaryMessage:
			s.routeBinary(peer, data)
		}
	}
}

func (s *relayServer) clientLoop(peer *hub.Peer) {
	defer s.cleanupPeer(peer)

	for {
		msgType, data, err := peer.Conn.ReadMessage()
		if err != nil {
			s.logger.Printf("client disconnect peer=%s err=%v", peer.ID, err)
			return
		}

		switch msgType {
		case websocket.TextMessage:
			msg, err := protocol.DecodeControl(data)
			if err != nil {
				s.metrics.IncError()
				s.sendError(peer, "BAD_CONTROL", "invalid control message")
				continue
			}
			s.handleControl(peer, msg)
		case websocket.BinaryMessage:
			s.routeBinary(peer, data)
		}
	}
}

func (s *relayServer) handleControl(peer *hub.Peer, msg protocol.ControlMessage) {
	if msg.Type == protocol.TypeHeartbeat {
		return
	}
	if msg.Type == protocol.TypeCloseSession && msg.SessionID != "" {
		s.closeSession(msg.SessionID)
		return
	}
	s.sendError(peer, "UNSUPPORTED_CONTROL", "unsupported control message in this state")
}

func (s *relayServer) handleTunnel(w http.ResponseWriter, r *http.Request) {
	if !s.ratelimit.Allow(r.RemoteAddr) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.metrics.IncError()
		s.logger.Printf("upgrade tunnel error=%v", err)
		return
	}

	msgType, data, err := conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		return
	}
	if msgType != websocket.TextMessage {
		_ = conn.Close()
		return
	}

	registerMsg, err := protocol.DecodeControl(data)
	if err != nil || registerMsg.Type != protocol.TypeRegister || registerMsg.AccessCodeHash == "" {
		_ = conn.Close()
		return
	}

	peer := hub.NewPeer(newID("c_"), hub.RoleConnector, conn)
	s.hub.Add(peer)

	caps := protocol.Caps{}
	if registerMsg.Caps != nil {
		caps = *registerMsg.Caps
	}

	prev, replaced := s.auth.Set(registerMsg.AccessCodeHash, authmap.Entry{
		Peer:       peer,
		Generation: registerMsg.Generation,
		Caps:       caps,
	})
	if replaced && prev.Peer != nil && prev.Peer != peer {
		s.logger.Printf("connector replaced hash=%s old=%s new=%s", registerMsg.AccessCodeHash, prev.Peer.ID, peer.ID)
		_ = prev.Peer.Conn.Close()
	}

	s.logger.Printf("connector registered peer=%s hash=%s", peer.ID, registerMsg.AccessCodeHash)
	s.connectorLoop(peer)
}

func (s *relayServer) handleClient(w http.ResponseWriter, r *http.Request) {
	if !s.ratelimit.Allow(r.RemoteAddr) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.metrics.IncError()
		s.logger.Printf("upgrade client error=%v", err)
		return
	}

	msgType, data, err := conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		return
	}
	if msgType != websocket.TextMessage {
		_ = conn.Close()
		return
	}

	connectMsg, err := protocol.DecodeControl(data)
	if err != nil || connectMsg.Type != protocol.TypeConnect || connectMsg.AccessCode == "" {
		_ = conn.Close()
		return
	}

	clientPeer := hub.NewPeer(newID("u_"), hub.RoleClient, conn)
	s.hub.Add(clientPeer)

	hash := protocol.HashAccessCode(connectMsg.AccessCode)
	connectorEntry, ok := s.auth.Get(hash)
	if !ok || connectorEntry.Peer == nil {
		s.sendError(clientPeer, "CONNECTOR_NOT_FOUND", "connector not online")
		s.cleanupPeer(clientPeer)
		return
	}

	sessionID := newID("s_")
	session := &sessions.Session{
		ID:        sessionID,
		Client:    clientPeer,
		Connector: connectorEntry.Peer,
		E2EE:      connectMsg.E2EE,
		CreatedAt: time.Now().UTC(),
	}
	s.sessions.Set(session)

	if err := s.sendControl(clientPeer, protocol.ControlMessage{
		Type:      protocol.TypeConnectOK,
		SessionID: sessionID,
		Caps:      &connectorEntry.Caps,
	}); err != nil {
		s.metrics.IncError()
		s.closeSession(sessionID)
		s.cleanupPeer(clientPeer)
		return
	}

	if err := s.sendControl(connectorEntry.Peer, protocol.ControlMessage{
		Type:      protocol.TypeSessionOpen,
		SessionID: sessionID,
		E2EE:      connectMsg.E2EE,
	}); err != nil {
		s.metrics.IncError()
		s.closeSession(sessionID)
		s.cleanupPeer(clientPeer)
		return
	}

	s.logger.Printf("session open sid=%s client=%s connector=%s", sessionID, clientPeer.ID, connectorEntry.Peer.ID)
	s.clientLoop(clientPeer)
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "fallback"
	}
	return prefix + hex.EncodeToString(buf)
}

func main() {
	addr := flag.String("addr", ":8080", "relay listen address")
	flag.Parse()

	logger := log.New(os.Stdout, "[relay] ", log.LstdFlags|log.Lmicroseconds)
	server := newRelayServer(logger)

	mux := http.NewServeMux()
	mux.HandleFunc("/tunnel", server.handleTunnel)
	mux.HandleFunc("/client", server.handleClient)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	logger.Printf("listening addr=%s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		logger.Fatalf("server exited error=%v", err)
	}
}
