package brain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
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
	store      memory.MemoryStore
	OnToken    func(ctx context.Context, sessionID string, token string)
	OnFlush    func(ctx context.Context, sessionID string)
	OnComplete func(ctx context.Context, sessionID string, fullText string)

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
		Role: openai.ChatMessageRoleSystem,
		Content: `You are Zeplin, an ambient, intelligent AI music tutor and guitar co-pilot.
Your responses are spoken aloud to the student in real-time, so follow these guidelines:

1. COMMUNICATIVE & CONVERSATIONAL: You are a true conversational companion and expert music instructor. If the student greets you, asks questions about music, artists, gear, or their day, talk with them warmly, naturally, and intelligently.
2. PHYSICAL HAND MECHANICS & DSP REASONING:
   When telemetry indicates a muted string or inaccurate chord, explain the underlying physical biomechanics:
   - String 2 (B string) or String 1 (High E) muted on C Major or G Major: Often caused by the adjacent fretting finger (ring or index) leaning flat instead of arching on its fingertip. Advise arching the knuckles and playing directly on fingertips.
   - String 5 (A string) or 4 (D string) buzzing on D Major or C Major: Often insufficient pressure right behind the fret wire, or thumb slipping too low behind the neck.
   - Wrong bass note / inversion (e.g., E bass on C Major): Remind the student not to strum the low 6th string, or use the thumb to lightly mute the 6th string.
3. SPOKEN VOICE DELIVERY: Keep your answers natural, encouraging, and concise for real-time speech (1 to 3 sentences, around 15–35 words) so instruction never overpowers the student's tempo. Never use markdown formatting, bullet points, asterisks, or code.
4. TONE: Warm, witty, encouraging, and relaxed, like a great friend who is a world-class guitarist.`,
	}
	
	userMsg := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: text,
	}

	messages := []openai.ChatCompletionMessage{sysMsg}
	messages = append(messages, history...)
	messages = append(messages, userMsg)

	modelName := os.Getenv("GROQ_MODEL")
	if modelName == "" {
		modelName = "qwen/qwen3.8-27b"
	}

	req := openai.ChatCompletionRequest{
		Model:    modelName,
		Messages: messages,
		Stream:   true,
	}

	// 3. Invoke the stream
	stream, err := b.client.CreateChatCompletionStream(turnCtx, req)
	if err != nil {
		log.Printf("[Brain] Groq stream initiation error: %v", err)
		fallback := "Sorry, I had a quick hiccup—can you repeat that?"
		if b.OnToken != nil {
			b.OnToken(ctx, sessionID, fallback)
		}
		if b.OnFlush != nil {
			b.OnFlush(ctx, sessionID)
		}
		if b.OnComplete != nil {
			b.OnComplete(ctx, sessionID, fallback)
		}
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
			if fullResponse == "" {
				fallback := "Sorry, I had a brief connection blip. What were you saying?"
				if b.OnToken != nil {
					b.OnToken(ctx, sessionID, fallback)
				}
				if b.OnFlush != nil {
					b.OnFlush(ctx, sessionID)
				}
			}
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
		if b.OnComplete != nil {
			b.OnComplete(ctx, sessionID, fullResponse)
		}
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
