package memory

import (
	"context"
	"sync"

	"github.com/cloudwego/eino/schema"
)

// MemoryStore defines the pluggable interface for managing session conversational histories.
type MemoryStore interface {
	// Read retrieves the conversation history for a given session ID.
	Read(ctx context.Context, sessionID string) ([]*schema.Message, error)
	// Write overwrites the conversation history for a given session ID.
	Write(ctx context.Context, sessionID string, history []*schema.Message) error
	// Append adds one or more messages to the conversation history for a given session ID.
	Append(ctx context.Context, sessionID string, messages ...*schema.Message) error
}

// InMemoryStore is a thread-safe in-memory implementation of the MemoryStore interface.
type InMemoryStore struct {
	mu      sync.RWMutex
	history map[string][]*schema.Message
}

// NewInMemoryStore creates a new instance of InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		history: make(map[string][]*schema.Message),
	}
}

// Read retrieves a copy of the conversation history for the session.
func (s *InMemoryStore) Read(ctx context.Context, sessionID string) ([]*schema.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages, ok := s.history[sessionID]
	if !ok {
		return nil, nil
	}

	// Return a copy to ensure thread safety
	copied := make([]*schema.Message, len(messages))
	copy(copied, messages)
	return copied, nil
}

// Write stores a copy of the conversation history for the session.
func (s *InMemoryStore) Write(ctx context.Context, sessionID string, history []*schema.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy to prevent external mutation
	copied := make([]*schema.Message, len(history))
	copy(copied, history)
	s.history[sessionID] = copied
	return nil
}

// Append adds the provided messages to the session's conversation history.
func (s *InMemoryStore) Append(ctx context.Context, sessionID string, messages ...*schema.Message) error {
	if len(messages) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current := s.history[sessionID]
	for _, msg := range messages {
		current = append(current, msg)
	}
	s.history[sessionID] = current
	return nil
}
