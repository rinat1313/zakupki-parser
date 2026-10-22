package channels

import (
	"errors"
	"sync"
)

type State struct {
	HTTPIngest  bool `json:"http_ingest"`
	KafkaIngest bool `json:"kafka_ingest"`
}

type Store struct {
	mu            sync.RWMutex
	state         State
	hasBrokers    func() bool
}

func NewStore(initial State, hasBrokers func() bool) *Store {
	return &Store{state: initial, hasBrokers: hasBrokers}
}

func (s *Store) Get() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *Store) Set(next State) error {
	if next.KafkaIngest {
		if s.hasBrokers == nil || !s.hasBrokers() {
			return errors.New("kafka ingest requires KAFKA_BROKERS")
		}
	}
	s.mu.Lock()
	s.state = next
	s.mu.Unlock()
	return nil
}
