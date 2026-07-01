package brain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/Abhinav-Rao24/Zeplin/memory"
	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"google.golang.org/genai"
)

// Brain coordinates the conversational LLM brain via an Eino graph and MemoryStore.
type Brain struct {
	graph compose.Runnable[map[string]any, *schema.Message]
	store memory.MemoryStore
}

// NewBrain initializes a new Brain instance, building and compiling the type-safe Eino graph.
func NewBrain(ctx context.Context, apiKey string, store memory.MemoryStore) (*Brain, error) {
	if apiKey == "" {
		return nil, errors.New("gemini API key is required")
	}
	if store == nil {
		return nil, errors.New("memory store is required")
	}

	// 1. Initialize the official Google GenAI client
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Google GenAI client: %w", err)
	}

	// 2. Initialize Eino's Gemini ChatModel wrapper
	chatModel, err := gemini.NewChatModel(ctx, &gemini.Config{
		Client: client,
		Model:  "gemini-2.5-flash",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Eino Gemini ChatModel: %w", err)
	}

	// 3. Create the prompt template with system instructions, history placeholder, and current input
	chatTemplate := prompt.FromMessages(
		schema.FString,
		schema.SystemMessage("You are a helpful voice assistant named Zeplin. Keep your responses short and conversational, as they will be read aloud."),
		schema.MessagesPlaceholder("history", true),
		schema.UserMessage("{input}"),
	)

	// 4. Scaffold the type-safe directed Eino graph
	g := compose.NewGraph[map[string]any, *schema.Message]()

	// 5. Add ChatTemplate and ChatModel nodes to the graph
	err = g.AddChatTemplateNode("template_node", chatTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to add template node to graph: %w", err)
	}

	err = g.AddChatModelNode("model_node", chatModel)
	if err != nil {
		return nil, fmt.Errorf("failed to add model node to graph: %w", err)
	}

	// 6. Connect the graph components
	err = g.AddEdge(compose.START, "template_node")
	if err != nil {
		return nil, fmt.Errorf("failed to connect START to template_node: %w", err)
	}

	err = g.AddEdge("template_node", "model_node")
	if err != nil {
		return nil, fmt.Errorf("failed to connect template_node to model_node: %w", err)
	}

	err = g.AddEdge("model_node", compose.END)
	if err != nil {
		return nil, fmt.Errorf("failed to connect model_node to END: %w", err)
	}

	// 7. Compile the graph into a Runnable
	runnable, err := g.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to compile Eino graph: %w", err)
	}

	return &Brain{
		graph: runnable,
		store: store,
	}, nil
}

// ProcessTurn processes a finalized transcript turn: reads memory, runs Eino graph, streams response, and persists conversation.
func (b *Brain) ProcessTurn(ctx context.Context, sessionID string, text string) error {
	log.Printf("[Brain] Processing turn for session %s: %q", sessionID, text)

	// 1. Fetch historical transcripts from MemoryStore
	history, err := b.store.Read(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to read memory for session %s: %w", sessionID, err)
	}

	// 2. Invoke the Eino graph using Stream method to capture real-time tokens
	sr, err := b.graph.Stream(ctx, map[string]any{
		"history": history,
		"input":   text,
	})
	if err != nil {
		return fmt.Errorf("failed to start streaming response: %w", err)
	}
	defer sr.Close()

	fmt.Printf("[AI Response for %s]: ", sessionID)

	var chunks []*schema.Message
	for {
		chunk, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fmt.Println() // Ensure clean newline on error
			return fmt.Errorf("error receiving stream chunk: %w", err)
		}

		// Print chunk content to stdout in real-time
		fmt.Print(chunk.Content)
		chunks = append(chunks, chunk)
	}
	fmt.Println() // Print newline when stream finishes

	// 3. Concatenate output chunks into a finalized message
	assistantMsg, err := schema.ConcatMessages(chunks)
	if err != nil {
		return fmt.Errorf("failed to concatenate AI response chunks: %w", err)
	}

	// 4. Create user message from transcript
	userMsg := schema.UserMessage(text)

	// 5. Append both user message and assistant message back to the session history
	err = b.store.Append(ctx, sessionID, userMsg, assistantMsg)
	if err != nil {
		return fmt.Errorf("failed to save conversation turn to memory: %w", err)
	}

	log.Printf("[Brain] Completed turn for session %s. Saved %d messages.", sessionID, 2)
	return nil
}
