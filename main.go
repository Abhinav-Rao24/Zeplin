package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Abhinav-Rao24/Zeplin/brain"
	"github.com/Abhinav-Rao24/Zeplin/config"
	"github.com/Abhinav-Rao24/Zeplin/lessons"
	"github.com/Abhinav-Rao24/Zeplin/memory"
	"github.com/Abhinav-Rao24/Zeplin/transport"
	"github.com/Abhinav-Rao24/Zeplin/tts"
)

//go:embed client
var clientFS embed.FS

func main() {
	log.Println("Starting Zeplin Guitar Co-Pilot...")

	// ── Configuration ─────────────────────────────────────────────────────
	cfg := config.Load()
	log.Println("Configuration loaded.")

	// ── In-Memory Session Store ────────────────────────────────────────────
	memStore := memory.NewInMemoryStore()
	log.Println("Session memory store initialized.")

	// ── Groq Brain (guitar coach persona) ─────────────────────────────────
	ctx := context.Background()
	brainEngine, err := brain.NewBrain(ctx, cfg.GroqAPIKey, memStore)
	if err != nil {
		log.Fatalf("Failed to initialize Groq brain: %v", err)
	}
	log.Println("Guitar coach brain (Groq/Llama) initialized.")

	// ── Deepgram TTS Streaming Engine ─────────────────────────────────────
	ttsEngine, err := tts.NewDeepgramStreamTTS(cfg.DeepgramAPIKey)
	if err != nil {
		log.Fatalf("Failed to initialize Deepgram TTS: %v", err)
	}
	defer ttsEngine.Close()
	log.Println("Deepgram TTS engine initialized.")

	// ── Lesson Orchestrator ────────────────────────────────────────────────
	// The orchestrator is constructed without a publisher first; the publisher
	// is injected after the LiveKit room is connected (so it can hold a room ref).
	orchestrator := lessons.NewOrchestrator(brainEngine, ttsEngine, nil)
	log.Println("Lesson orchestrator initialized.")

	// ── HTTP Client Server (embed.FS → localhost:8080) ─────────────────────
	// Sub into the client/ directory so the server root maps to client/index.html
	subFS, err := fs.Sub(clientFS, "client")
	if err != nil {
		log.Fatalf("Failed to sub into client FS: %v", err)
	}
	transport.ServeClient(cfg.HTTPPort, subFS)
	log.Printf("Client available at http://localhost:%s", cfg.HTTPPort)

	// ── LiveKit Room Connection ────────────────────────────────────────────
	room, err := transport.ConnectLiveKit(
		cfg.LivekitURL,
		cfg.LivekitAPIKey,
		cfg.LivekitAPISecret,
		cfg.DeepgramAPIKey,
		brainEngine,
		ttsEngine,
		orchestrator,
	)
	if err != nil {
		log.Fatalf("Failed to connect to LiveKit: %v", err)
	}
	defer room.Disconnect()

	// ── Signal Handling ────────────────────────────────────────────────────
	log.Println("Zeplin Guitar Co-Pilot is active. Open http://localhost:" + cfg.HTTPPort + " in your browser.")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down Zeplin Guitar Co-Pilot...")
}
