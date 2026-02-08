package authmap

import (
	"sync"

	"openclaw-bridge/relay/pkg/hub"
	"openclaw-bridge/shared/protocol"
)

type Entry struct {
	Peer       *hub.Peer
	Generation int
	Caps       protocol.Caps
}

type Store struct {
	mu     sync.RWMutex
	byHash map[string]Entry
}

func NewStore() *Store {
	return &Store{byHash: make(map[string]Entry)}
}

func (s *Store) Set(accessCodeHash string, entry Entry) (previous Entry, replaced bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, replaced = s.byHash[accessCodeHash]
	s.byHash[accessCodeHash] = entry
	return previous, replaced
}

func (s *Store) Get(accessCodeHash string) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.byHash[accessCodeHash]
	return entry, ok
}

func (s *Store) DeleteByHash(accessCodeHash string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byHash, accessCodeHash)
}

func (s *Store) DeleteByPeer(peer *hub.Peer) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := make([]string, 0)
	for hash, entry := range s.byHash {
		if entry.Peer == peer {
			delete(s.byHash, hash)
			deleted = append(deleted, hash)
		}
	}
	return deleted
}
