package channels

import (
	"testing"
)

func TestSetKafkaWithoutBrokers(t *testing.T) {
	s := NewStore(State{HTTPIngest: true}, func() bool { return false })
	err := s.Set(State{KafkaIngest: true})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSetKafkaWithBrokers(t *testing.T) {
	s := NewStore(State{HTTPIngest: true}, func() bool { return true })
	if err := s.Set(State{KafkaIngest: true}); err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	if !got.KafkaIngest {
		t.Fatal("kafka ingest not set")
	}
}

func TestGetCopy(t *testing.T) {
	s := NewStore(State{HTTPIngest: true, KafkaIngest: false}, func() bool { return true })
	a := s.Get()
	a.HTTPIngest = false
	if !s.Get().HTTPIngest {
		t.Fatal("mutating copy should not affect store")
	}
}
