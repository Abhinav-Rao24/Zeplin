package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Abhinav-Rao24/Zeplin/brain"
	"github.com/Abhinav-Rao24/Zeplin/config"
	"github.com/Abhinav-Rao24/Zeplin/memory"
	"github.com/Abhinav-Rao24/Zeplin/transport"
	"github.com/Abhinav-Rao24/Zeplin/tts"
)

func main() {
	log.Println("Starting Zeplin Voice AI Framework...")

	// Initialize configuration
	cfg := config.Load()
	log.Println("Configuration loaded successfully.")

	// Initialize thread-safe memory store
	memStore := memory.NewInMemoryStore()
	log.Println("Session Memory Store initialized.")

	// Initialize Groq brain layer
	ctx := context.Background()
	brainEngine, err := brain.NewBrain(ctx, cfg.GroqAPIKey, memStore)
	if err != nil {
		log.Fatalf("Failed to initialize Groq brain: %v", err)
	}
	log.Println("Groq Brain and Llama 3.1 node initialized successfully.")

	// Initialize Long-lived Deepgram TTS streaming engine
	ttsEngine, err := tts.NewDeepgramStreamTTS(cfg.DeepgramAPIKey)
	if err != nil {
		log.Fatalf("Failed to initialize Deepgram TTS: %v", err)
	}
	defer ttsEngine.Close()

	// Connect to LiveKit Room and hook up the STT Engine and Eino Brain
	room, err := transport.ConnectLiveKit(cfg.LivekitURL, cfg.LivekitAPIKey, cfg.LivekitAPISecret, cfg.DeepgramAPIKey, brainEngine, ttsEngine)
	if err != nil {
		log.Fatalf("Failed to connect to LiveKit: %v", err)
	}
	defer room.Disconnect()

	// Keep the application running
	log.Println("Voice Ingestion & AI Brain Layer is now active. Waiting for participants...")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down Zeplin Voice AI Framework...")
}
