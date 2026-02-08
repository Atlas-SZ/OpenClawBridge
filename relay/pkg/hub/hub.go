package hub

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Role string

const (
	RoleConnector Role = "connector"
	RoleClient    Role = "client"
)

type Peer struct {
	ID   string
	Role Role
	Conn *websocket.Conn

	writeMu sync.Mutex
}

func NewPeer(id string, role Role, conn *websocket.Conn) *Peer {
	return &Peer{ID: id, Role: role, Conn: conn}
}

func (p *Peer) SendText(data []byte) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	return p.Conn.WriteMessage(websocket.TextMessage, data)
}

func (p *Peer) SendBinary(data []byte) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	return p.Conn.WriteMessage(websocket.BinaryMessage, data)
}

type Manager struct {
	mu    sync.RWMutex
	peers map[string]*Peer
}

func NewManager() *Manager {
	return &Manager{peers: make(map[string]*Peer)}
}

func (m *Manager) Add(peer *Peer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.peers[peer.ID] = peer
}

func (m *Manager) Remove(peerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.peers, peerID)
}

func (m *Manager) Get(peerID string) (*Peer, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	peer, ok := m.peers[peerID]
	return peer, ok
}
