package memory

import (
	"context"
	"sync"

	"github.com/sashabaranov/go-openai"
)

// MemoryStore defines the pluggable interface for managing session conversational histories.
type MemoryStore interface {
	// Read retrieves the conversation history for a given session ID.
	Read(ctx context.Context, sessionID string) ([]openai.ChatCompletionMessage, error)
	// Write overwrites the conversation history for a given session ID.
	Write(ctx context.Context, sessionID string, history []openai.ChatCompletionMessage) error
	// Append adds one or more messages to the conversation history for a given session ID.
	Append(ctx context.Context, sessionID string, messages ...openai.ChatCompletionMessage) error
}

// InMemoryStore is a thread-safe in-memory implementation of the MemoryStore interface.
type InMemoryStore struct {
	mu      sync.RWMutex
	history map[string][]openai.ChatCompletionMessage
}

// NewInMemoryStore creates a new instance of InMemoryStore.
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		history: make(map[string][]openai.ChatCompletionMessage),
	}
}

// Read retrieves a copy of the conversation history for the session.
func (s *InMemoryStore) Read(ctx context.Context, sessionID string) ([]openai.ChatCompletionMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	messages, ok := s.history[sessionID]
	if !ok {
		return nil, nil
	}

	// Return a copy to ensure thread safety
	copied := make([]openai.ChatCompletionMessage, len(messages))
	copy(copied, messages)
	return copied, nil
}

// Write stores a copy of the conversation history for the session.
func (s *InMemoryStore) Write(ctx context.Context, sessionID string, history []openai.ChatCompletionMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy to prevent external mutation
	copied := make([]openai.ChatCompletionMessage, len(history))
	copy(copied, history)
	s.history[sessionID] = copied
	return nil
}

// Append adds the provided messages to the session's conversation history.
func (s *InMemoryStore) Append(ctx context.Context, sessionID string, messages ...openai.ChatCompletionMessage) error {
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
