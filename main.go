package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Abhinav-Rao24/Zeplin/config"
	"github.com/Abhinav-Rao24/Zeplin/stt"
	"github.com/Abhinav-Rao24/Zeplin/transport"
)

func main() {
	log.Println("Starting Zeplin Voice AI Framework...")

	// Initialize configuration
	cfg := config.Load()
	log.Println("Configuration loaded successfully.")

	// 1. Initialize Deepgram STT
	sttEngine, err := stt.NewDeepgramStreamSTT(cfg.DeepgramAPIKey)
	if err != nil {
		log.Fatalf("Failed to initialize STT engine: %v", err)
	}
	defer sttEngine.Close()

	// 2. Connect to LiveKit Room and hook up the STT Engine
	room, err := transport.ConnectLiveKit(cfg.LivekitURL, cfg.LivekitAPIKey, cfg.LivekitAPISecret, sttEngine)
	if err != nil {
		log.Fatalf("Failed to connect to LiveKit: %v", err)
	}
	defer room.Disconnect()

	// 3. Keep the application running
	log.Println("Voice Ingestion Layer is now active. Waiting for participants...")
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down Zeplin Voice AI Framework...")
}
