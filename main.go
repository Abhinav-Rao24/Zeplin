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

//go:embed all:client
var clientFS embed.FS

func main() {
	log.Println("Starting Zeplin Guitar Co-Pilot...")

	// ── Configuration ─────────────────────────────────────────────────────
	cfg := config.Load()
	log.Println("Configuration loaded.")

	// ── SQLite Persistent Session Store (pure Go, zero CGO) ───────────────
	dbPath := "zeplin_guitar.db"
	sqlStore, err := memory.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize SQLite store: %v", err)
	}
	defer sqlStore.Close()
	log.Printf("SQLite session store initialized at %s", dbPath)

	// ── Groq Brain (guitar coach persona) ─────────────────────────────────
	ctx := context.Background()
	brainEngine, err := brain.NewBrain(ctx, cfg.GroqAPIKey, sqlStore)
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
	orchestrator := lessons.NewOrchestrator(brainEngine, ttsEngine, nil)
	orchestrator.SetStore(sqlStore)
	log.Println("Lesson orchestrator initialized with SQLite persistence.")

	// ── HTTP Client Server (embed.FS + /api/token → localhost:8080) ────────
	subFS, err := fs.Sub(clientFS, "client")
	if err != nil {
		log.Fatalf("Failed to sub into client FS: %v", err)
	}
	transport.ServeClient(transport.HTTPServerConfig{
		Port:             cfg.HTTPPort,
		LivekitURL:       cfg.LivekitURL,
		LivekitAPIKey:    cfg.LivekitAPIKey,
		LivekitAPISecret: cfg.LivekitAPISecret,
		DefaultRoom:      "voice-agent-room",
		Store:            sqlStore,
	}, subFS)
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
