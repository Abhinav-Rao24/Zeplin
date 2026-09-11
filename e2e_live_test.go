package main

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/Abhinav-Rao24/Zeplin/brain"
	"github.com/Abhinav-Rao24/Zeplin/config"
	"github.com/Abhinav-Rao24/Zeplin/memory"
	"github.com/Abhinav-Rao24/Zeplin/transport"
	"github.com/Abhinav-Rao24/Zeplin/tts"
)

func TestLiveTurnPerformanceE2E(t *testing.T) {
	cfg := config.Load()
	if cfg.GroqAPIKey == "" || cfg.DeepgramAPIKey == "" {
		t.Skip("Skipping live test: missing API keys in .env")
	}

	memStore := memory.NewInMemoryStore()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	brainEngine, err := brain.NewBrain(ctx, cfg.GroqAPIKey, memStore)
	if err != nil {
		t.Fatalf("Failed to initialize Groq brain: %v", err)
	}

	ttsEngine, err := tts.NewDeepgramStreamTTS(cfg.DeepgramAPIKey)
	if err != nil {
		t.Fatalf("Failed to initialize Deepgram TTS: %v", err)
	}
	defer ttsEngine.Close()

	registry := transport.NewTelemetryRegistry()
	sessionID := "live-eval-participant"
	registry.SetActiveSession(sessionID)
	telemetry := registry.Get(sessionID)

	ttsEngine.OnAudioFrame = func() {
		telemetry.RecordFirstAudioFrame()
	}

	brainEngine.OnToken = func(ctx context.Context, sID string, token string) {
		telemetry.RecordFirstToken()
		_ = ttsEngine.Speak(token)
	}
	brainEngine.OnFlush = func(ctx context.Context, sID string) {
		_ = ttsEngine.Flush()
	}

	fmt.Println("\n========================================================")
	fmt.Println("STARTING LIVE END-TO-END TURN TEST (Groq + Deepgram TTS)")
	fmt.Println("========================================================")

	telemetry.StartThinking()
	prompt := "Say 'Hello! Voice agent pipeline test complete.' and keep it under 10 words."

	// Run brain turn in a goroutine
	turnDone := make(chan error, 1)
	go func() {
		turnDone <- brainEngine.ProcessTurn(ctx, sessionID, prompt)
	}()

	// Pacing loop with hybrid spin-yield
	var nextFrameTime time.Time
	framesReceived := 0
	timeout := time.After(15 * time.Second)

pacingLoop:
	for {
		select {
		case <-timeout:
			t.Log("Pacing loop timed out waiting for audio end")
			break pacingLoop
		case chunk, ok := <-ttsEngine.AudioChan:
			if !ok {
				break pacingLoop
			}

			// Deepgram sends variable chunk sizes (often 320 bytes = 40ms).
			// Slice into exact 160-byte (20ms) WebRTC frames at 8kHz mono PCMU.
			const frameSize = 160
			const frameDuration = 20 * time.Millisecond

			for offset := 0; offset < len(chunk); offset += frameSize {
				end := offset + frameSize
				if end > len(chunk) {
					end = len(chunk)
				}
				frame := chunk[offset:end]
				duration := time.Duration(float64(len(frame)) / 8000.0 * 1000.0) * time.Millisecond
				framesReceived++

				telemetry.RecordOutboundPush()

				now := time.Now()
				if nextFrameTime.IsZero() || now.After(nextFrameTime.Add(duration)) {
					nextFrameTime = now.Add(duration)
				} else {
					nextFrameTime = nextFrameTime.Add(duration)
				}

				// Hybrid spin-yield wait
				for {
					remaining := time.Until(nextFrameTime)
					if remaining <= 0 {
						break
					}
					if remaining > 3*time.Millisecond {
						timer := time.NewTimer(remaining - 2*time.Millisecond)
						select {
						case <-ctx.Done():
							timer.Stop()
							return
						case <-timer.C:
						}
					} else {
						runtime.Gosched()
					}
				}
			}

			// If no more frames are queued after a short drain check, we are done
			if len(ttsEngine.AudioChan) == 0 && framesReceived > 10 {
				// Give a brief window to confirm stream completion
				time.Sleep(100 * time.Millisecond)
				if len(ttsEngine.AudioChan) == 0 {
					break pacingLoop
				}
			}
		}
	}

	err = <-turnDone
	if err != nil {
		t.Fatalf("Brain ProcessTurn failed: %v", err)
	}

	fmt.Println("\nREPORTING LIVE OBSERVED TELEMETRY:")
	telemetry.CompileAndReport()
	fmt.Println("========================================================")
}
