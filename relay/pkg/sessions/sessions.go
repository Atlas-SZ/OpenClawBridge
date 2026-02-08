package sessions

import (
	"sync"
	"time"

	"openclaw-bridge/relay/pkg/hub"
)

type Session struct {
	ID        string
	Client    *hub.Peer
	Connector *hub.Peer
	E2EE      bool
	CreatedAt time.Time
}

type Store struct {
	mu   sync.RWMutex
	data map[string]*Session
}

func NewStore() *Store {
	return &Store{data: make(map[string]*Session)}
}

func (s *Store) Set(session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[session.ID] = session
}

func (s *Store) Get(sessionID string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.data[sessionID]
	return session, ok
}

func (s *Store) Delete(sessionID string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.data[sessionID]
	if !ok {
		return nil, false
	}
	delete(s.data, sessionID)
	return session, true
}

func (s *Store) DeleteByPeer(peer *hub.Peer) []*Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := make([]*Session, 0)
	for id, session := range s.data {
		if session.Client == peer || session.Connector == peer {
			removed = append(removed, session)
			delete(s.data, id)
		}
	}
	return removed
}
