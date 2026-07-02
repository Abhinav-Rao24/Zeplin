package brain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/Abhinav-Rao24/Zeplin/memory"
	"github.com/sashabaranov/go-openai"
)

type turnState struct {
	cancel context.CancelFunc
}

// Brain coordinates the conversational LLM brain.
type Brain struct {
	client  *openai.Client
	store   memory.MemoryStore
	OnToken func(ctx context.Context, sessionID string, token string)
	OnFlush func(ctx context.Context, sessionID string)

	mu          sync.Mutex
	activeTurns map[string]*turnState
}

// NewBrain initializes a new Brain instance using Groq API via go-openai.
func NewBrain(ctx context.Context, apiKey string, store memory.MemoryStore) (*Brain, error) {
	if apiKey == "" {
		return nil, errors.New("groq API key is required")
	}
	if store == nil {
		return nil, errors.New("memory store is required")
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.groq.com/openai/v1"
	client := openai.NewClientWithConfig(config)

	return &Brain{
		client:      client,
		store:       store,
		activeTurns: make(map[string]*turnState),
	}, nil
}

// ProcessTurn processes a finalized transcript turn: reads memory, runs Groq stream, and persists conversation.
func (b *Brain) ProcessTurn(ctx context.Context, sessionID string, text string) error {
	log.Printf("[Brain] Processing turn for session %s: %q", sessionID, text)

	// Intercept any active generation
	b.Interrupt(sessionID)

	turnCtx, cancel := context.WithCancel(ctx)
	tState := &turnState{cancel: cancel}
	b.mu.Lock()
	b.activeTurns[sessionID] = tState
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		if b.activeTurns[sessionID] == tState {
			delete(b.activeTurns, sessionID)
		}
		b.mu.Unlock()
		cancel()
	}()

	// 1. Fetch historical transcripts from MemoryStore
	history, err := b.store.Read(turnCtx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to read memory for session %s: %w", sessionID, err)
	}

	// 2. Prepare messages for Groq
	sysMsg := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: "You are a helpful voice assistant named Zeplin. Keep your responses short and conversational, as they will be read aloud.",
	}
	
	userMsg := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: text,
	}

	messages := []openai.ChatCompletionMessage{sysMsg}
	messages = append(messages, history...)
	messages = append(messages, userMsg)

	req := openai.ChatCompletionRequest{
		Model:    "llama-3.1-8b-instant",
		Messages: messages,
		Stream:   true,
	}

	// 3. Invoke the stream
	stream, err := b.client.CreateChatCompletionStream(turnCtx, req)
	if err != nil {
		return fmt.Errorf("failed to start streaming response: %w", err)
	}
	defer stream.Close()

	fmt.Printf("[AI Response for %s]: ", sessionID)

	var fullResponse string
	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			// Could be context canceled from interruption
			if errors.Is(turnCtx.Err(), context.Canceled) {
				fmt.Println(" [Interrupted]")
				break
			}
			fmt.Println()
			return fmt.Errorf("error receiving stream chunk: %w", err)
		}

		if len(response.Choices) > 0 {
			content := response.Choices[0].Delta.Content
			if content != "" {
				fmt.Print(content)
				fullResponse += content
				if b.OnToken != nil {
					b.OnToken(ctx, sessionID, content)
				}
			}
		}
	}
	fmt.Println()
	if b.OnFlush != nil {
		b.OnFlush(ctx, sessionID)
	}

	// 4. Save to memory if response is not empty
	if fullResponse != "" {
		assistantMsg := openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleAssistant,
			Content: fullResponse,
		}
		err = b.store.Append(ctx, sessionID, userMsg, assistantMsg)
		if err != nil {
			return fmt.Errorf("failed to save conversation turn to memory: %w", err)
		}
		log.Printf("[Brain] Completed turn for session %s. Saved 2 messages.", sessionID)
	} else {
		// Just save user message if interrupted before any text generated
		err = b.store.Append(ctx, sessionID, userMsg)
		if err != nil {
			return fmt.Errorf("failed to save conversation turn to memory: %w", err)
		}
		log.Printf("[Brain] Turn interrupted before response for session %s. Saved 1 message.", sessionID)
	}

	return nil
}

// Interrupt immediately cancels the active generation turn for a session.
func (b *Brain) Interrupt(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if tState, ok := b.activeTurns[sessionID]; ok {
		log.Printf("[Brain] Interrupted active generation for session %s", sessionID)
		tState.cancel()
		delete(b.activeTurns, sessionID)
	}
}
