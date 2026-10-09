package transport

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Abhinav-Rao24/Zeplin/brain"
	"github.com/Abhinav-Rao24/Zeplin/dsp"
	"github.com/Abhinav-Rao24/Zeplin/lessons"
	"github.com/Abhinav-Rao24/Zeplin/stt"
	"github.com/Abhinav-Rao24/Zeplin/tts"
	"github.com/aflyingHusky/go-webrtcvad"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/opus"
	"github.com/pion/webrtc/v4"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"
)

var stopWords = map[string]bool{
	"i": true, "me": true, "my": true, "we": true, "our": true, "you": true, "your": true,
	"he": true, "she": true, "it": true, "they": true, "what": true, "who": true, "which": true,
	"is": true, "am": true, "are": true, "was": true, "were": true, "be": true, "been": true,
	"have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
	"a": true, "an": true, "the": true, "and": true, "but": true, "if": true, "or": true,
	"as": true, "until": true, "while": true, "of": true, "at": true, "by": true, "for": true,
	"with": true, "about": true, "against": true, "between": true, "into": true, "through": true,
	"during": true, "before": true, "after": true, "above": true, "below": true, "to": true,
	"from": true, "up": true, "down": true, "in": true, "out": true, "on": true, "off": true,
	"over": true, "under": true, "again": true, "further": true, "then": true, "once": true,
	"here": true, "there": true, "when": true, "where": true, "why": true, "how": true,
	"all": true, "any": true, "both": true, "each": true, "few": true, "more": true,
	"most": true, "other": true, "some": true, "such": true, "no": true, "nor": true,
	"not": true, "only": true, "own": true, "same": true, "so": true, "than": true,
	"too": true, "very": true, "can": true, "will": true, "just": true, "should": true,
	"now": true, "that": true, "this": true,
}

// isAcousticEcho evaluates whether candidate is an acoustic bleed/echo of recent TTS speech.
// It ignores universal stop words ("can", "you", "hear", "me") to avoid dropping legitimate
// user questions while accurately catching multi-word phrases or content bleed from laptop speakers.
func isAcousticEcho(candidate, reference string) bool {
	candNorm := normalizeUtterance(candidate)
	refNorm := normalizeUtterance(reference)
	if candNorm == "" || refNorm == "" {
		return false
	}

	candWords := strings.Fields(candNorm)
	if len(candWords) == 0 {
		return false
	}

	// 1. Verbatim multi-word phrase matching (>= 4 words)
	if len(candWords) >= 4 && strings.Contains(refNorm, candNorm) {
		return true
	}

	// 2. Content word overlap (ignoring stop words)
	var candContent []string
	for _, w := range candWords {
		clean := strings.Trim(w, "!?,.:;\"'")
		if len(clean) >= 2 && !stopWords[clean] {
			candContent = append(candContent, clean)
		}
	}

	// Conversational questions made of stop words ("can you hear me", "who are you")
	// are human speech and must NEVER be flagged as echo.
	if len(candContent) == 0 {
		return false
	}

	refWords := strings.Fields(refNorm)
	refContentSet := make(map[string]bool)
	for _, w := range refWords {
		clean := strings.Trim(w, "!?,.:;\"'")
		if len(clean) >= 2 && !stopWords[clean] {
			refContentSet[clean] = true
		}
	}

	matches := 0
	for _, w := range candContent {
		if refContentSet[w] {
			matches++
		}
	}

	// Require at least 2 distinct content words to match AND >= 60% overlap
	if matches < 2 {
		return false
	}
	return float64(matches)/float64(len(candContent)) >= 0.60
}

// normalizeUtterance cleans punctuation, symbols, and whitespace for transcript comparison.
func normalizeUtterance(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.TrimRight(s, "?!.,:; ")
}

// isStaleOrPrefix determines whether an incoming STT transcript is identical to or an
// incomplete prefix/substring of an already finalized turn utterance.
func isStaleOrPrefix(candidate, finalized string) bool {
	cand := normalizeUtterance(candidate)
	fin := normalizeUtterance(finalized)
	if cand == "" || fin == "" {
		return false
	}
	return cand == fin || strings.HasPrefix(fin, cand)
}

// ConnectLiveKit establishes a connection to a LiveKit room, hooks up the STT writer per
// participant, and publishes the agent's outbound voice track with Deepgram TTS streaming.
func ConnectLiveKit(url, apiKey, apiSecret, deepgramAPIKey string, brainInstance *brain.Brain, ttsEngine *tts.DeepgramStreamTTS, orchestrators ...*lessons.Orchestrator) (*lksdk.Room, error) {
	ctx, cancel := context.WithCancel(context.Background())

	var orch *lessons.Orchestrator
	if len(orchestrators) > 0 {
		orch = orchestrators[0]
	}

	// interruptChan signals the pacing loop to drain the TTS audio queue during a barge-in.
	interruptChan := make(chan struct{}, 1)

	// Typed agent state machine. Replaces the old pair of isBrainActive / isPacingActive
	// atomic.Bool flags with a single, explicitly-typed state for clearer reasoning.
	//   StateListening â†’ StateThinking â†’ StateSpeaking â†’ StateListening
	sm := &AgentStateMachine{}

	telemetryRegistry := NewTelemetryRegistry()
	ttsEngine.OnAudioFrame = func() {
		telemetryRegistry.RecordFirstAudioFrame()
	}
	ttsEngine.OnFlushed = func() {
		if ttsEngine.QueueLen() == 0 && sm.Is(StateSpeaking) {
			transitionToListening(sm, telemetryRegistry)
		}
	}

	// Interrupt function to immediately halt active TTS playback and Groq inference
	triggerInterrupt := func(targetSession string) {
		if sm.Is(StateListening) {
			return
		}

		log.Printf("[Barge-In] Interruption triggered for %s! Agent state: %s", targetSession, sm.Get())

		// 1. Send Clear to Deepgram TTS to stop current audio generation.
		ttsEngine.Clear()

		// 2. Cancel the active Groq streaming turn.
		brainInstance.Interrupt(targetSession)

		// 3. Signal the pacing loop to drain any unplayed audio frames.
		select {
		case interruptChan <- struct{}{}:
		default:
		}
	}

	if orch != nil {
		orch.SetInterruptHandler(triggerInterrupt)
		orch.SetStateChecker(sm)
	}

	roomCB := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: func(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				if track.Kind() == webrtc.RTPCodecTypeAudio {
					sessionID := rp.Identity()
					log.Printf("Audio track subscribed from %s", sessionID)
					if orch != nil {
						orch.StartSession(sessionID)
					}

					localInterrupt := func() {
						triggerInterrupt(sessionID)
					}

					var (
						transcriptMu      sync.Mutex
						interimTimer      *time.Timer
						hardLimitTimer    *time.Timer
						pendingText       string
						activeTurnID      uint64
						lastFinalizedText string
						turnStartedAt     time.Time
					)

					finalizeAndDispatchTurn := func(text string) {
						text = strings.TrimSpace(text)
						if text == "" {
							return
						}

						// 1. Explicitly stop and reset all timers on finalization
						if interimTimer != nil {
							interimTimer.Stop()
							interimTimer = nil
						}
						if hardLimitTimer != nil {
							hardLimitTimer.Stop()
							hardLimitTimer = nil
						}
						pendingText = ""

						// 2. Utterance Deduplication:
						// If text matches or prefixes what was already finalized, discard immediately.
						if isStaleOrPrefix(text, lastFinalizedText) {
							log.Printf("[STT Guard] Discarding duplicate/trailing STT packet for already finalized turn: %q (already finalized: %q)", text, lastFinalizedText)
							return
						}

						// 3. Acoustic Echo Guard: If the text is acoustic echo of what was recently spoken or being spoken, drop it!
						recentSpoken := ttsEngine.RecentSpokenText()
						if isAcousticEcho(text, recentSpoken) {
							log.Printf("[Echo Guard] Discarding turn dispatch (acoustic echo of recent speech): %q", text)
							return
						}

						activeTurnID++
						thisTurnID := activeTurnID
						lastFinalizedText = text
						turnStartedAt = time.Now()

						if sm.Is(StateSpeaking) || sm.Is(StateThinking) {
							localInterrupt()
						}
						go func(t string, tID uint64) {
							telemetryRegistry.SetActiveSession(sessionID)
							telemetryRegistry.Get(sessionID).StartThinking()
							sm.Set(StateThinking)
							log.Printf("[Brain] Turn started (TurnID=%d) for session %s: %q", tID, sessionID, t)

							// Allow 60 ms for stray buffered Groq HTTP tokens to hit the
							// clearing gate in Speak() and be silently dropped before we
							// call StartNewTurn() to re-open the gate for the new response.
							time.Sleep(60 * time.Millisecond)
							ttsEngine.StartNewTurn()

							if orch != nil {
								orch.HandleSpeech(context.Background(), sessionID, t)
							} else {
								if err := brainInstance.ProcessTurn(context.Background(), sessionID, t); err != nil {
									log.Printf("Error processing turn for session %s: %v", sessionID, err)
								}
							}

							if sm.Is(StateThinking) {
								sm.Set(StateListening)
							}
							log.Printf("[Brain] Turn ended (TurnID=%d). Agent state: %s", tID, sm.Get())
						}(text, thisTurnID)
					}

					// Instantiate STT engine per participant.
					sttEngine, err := stt.NewDeepgramStreamSTT(deepgramAPIKey, func(transcript string, isFinal bool) {
						transcript = strings.TrimSpace(transcript)
						if transcript == "" {
							return
						}

						transcriptMu.Lock()
						defer transcriptMu.Unlock()

						// 1. Utterance Deduplication:
						// If this transcript corresponds to the already finalized utterance,
						// DISCARD IT IMMEDIATELY. This completely stops Deepgram's delayed
						// isFinal:true packets from triggering duplicate responses!
						if isStaleOrPrefix(transcript, lastFinalizedText) {
							return
						}

						// If previous turn's audio finished or was flushed, cleanly revert to Listening
						if sm.Is(StateSpeaking) && ttsEngine.QueueLen() == 0 && ttsEngine.IsFlushed() {
							transitionToListening(sm, telemetryRegistry)
						}

						// Check acoustic echo against recent TTS
						recentSpoken := ttsEngine.RecentSpokenText()
						isEcho := isAcousticEcho(transcript, recentSpoken)

						// 2. Final Utterance Handling (MUST NOT DROP unless acoustic echo):
						// When isFinal: true arrives, the user completed their sentence.
						if isFinal {
							if isEcho {
								log.Printf("[Echo Guard] Dropped final STT (acoustic echo): %q", transcript)
								return
							}
							if sm.Is(StateSpeaking) {
								log.Printf("[Barge-In] Turn completion arrived while speaking; interrupting agent: %q", transcript)
								localInterrupt()
								sm.Set(StateListening)
							}
							finalizeAndDispatchTurn(transcript)
							return
						}

						// 3. Barge-In Text Guard for Interim Transcripts:
						// Only trigger barge-in during StateSpeaking/StateThinking if:
						// - At least 600ms have elapsed since turn start (grace period)
						// - Transcript is NOT acoustic echo
						if sm.Is(StateSpeaking) || sm.Is(StateThinking) {
							if isEcho {
								log.Printf("[Echo Guard] Dropped interim STT (acoustic echo of recent TTS): %q", transcript)
								return
							}
							if time.Since(turnStartedAt) < 600*time.Millisecond {
								// In grace period right after agent started response, ignore stray interim triggers
								return
							}
							log.Printf("[Barge-In] Confirmed new speech word from STT: %q", transcript)
							localInterrupt()
						}

						// 4. Decoupled 750ms Text Stability Timer:
						// Tracks transcript text ONLY. If words have not changed from pendingText, let the timer keep ticking down.
						if transcript == pendingText && interimTimer != nil {
							return
						}

						pendingText = transcript
						if interimTimer != nil {
							interimTimer.Stop()
						}
						// 750ms stability timer: if words do not change for 750ms, trigger response immediately!
						interimTimer = time.AfterFunc(750*time.Millisecond, func() {
							transcriptMu.Lock()
							textToFire := pendingText
							pendingText = ""
							interimTimer = nil
							if hardLimitTimer != nil {
								hardLimitTimer.Stop()
								hardLimitTimer = nil
							}
							transcriptMu.Unlock()

							if textToFire != "" && !isStaleOrPrefix(textToFire, lastFinalizedText) {
								log.Printf("[STT Stability Timer] Finalizing text after 750ms stability: %q", textToFire)
								finalizeAndDispatchTurn(textToFire)
							}
						})

						// 5. Hard Fallback Ceiling (1.5-second absolute ceiling):
						// If any transcribed user text has existed for 1.5s, force the turn to finish immediately.
						if hardLimitTimer == nil {
							hardLimitTimer = time.AfterFunc(1500*time.Millisecond, func() {
								transcriptMu.Lock()
								textToFire := pendingText
								pendingText = ""
								if interimTimer != nil {
									interimTimer.Stop()
									interimTimer = nil
								}
								hardLimitTimer = nil
								transcriptMu.Unlock()

								if textToFire != "" && !isStaleOrPrefix(textToFire, lastFinalizedText) {
									log.Printf("[STT Hard Ceiling] Reached 1.5s absolute ceiling, forcing turn: %q", textToFire)
									finalizeAndDispatchTurn(textToFire)
								}
							})
						}
					})
					if err != nil {
						log.Printf("Error creating STT engine for %s: %v", sessionID, err)
						return
					}

					// Initialize Opus decoder configured to output 16 kHz mono directly.
					dec, err := opus.NewDecoderWithOutput(16000, 1)
					if err != nil {
						log.Printf("Error creating Opus decoder: %v", err)
						sttEngine.Close()
						return
					}

					// Initialize local WebRTC VAD in Mode 3 (very aggressive) to reject stationary
					// ambient noise (fans, coolers, air conditioning, room reverberation).
					vad, err := webrtcvad.New()
					if err != nil {
						log.Printf("Error creating local VAD: %v", err)
						sttEngine.Close()
						return
					}
					if err := vad.SetMode(3); err != nil {
						log.Printf("Error setting VAD mode: %v", err)
					}

					pcm16Buf := make([]int16, 1920) // up to 120ms at 16kHz mono
					var vadBuffer []byte
					consecutivePositive := 0
					packetCount := 0

					// Dynamic rolling noise floor baseline (RMS_floor)
					// Initializes around typical quiet ambient room level (150).
					// Tracks stationary background noise (coolers, fans, AC) during non-speech intervals using EMA.
					rmsFloor := 150.0
					const emaAlpha = 0.05 // Exponential Moving Average smoothing factor (~1s time constant)

					// RTP read loop: decodes Opus directly to 16kHz PCM → sends to Deepgram STT & WebRTC VAD.
					go func() {
						defer sttEngine.Close()

						for {
							rtpPacket, _, err := track.ReadRTP()
							if err != nil {
								log.Printf("Track read error or closed: %v", err)
								return
							}

							if len(rtpPacket.Payload) == 0 {
								continue
							}

							packetCount++
							sampleCount, err := dec.DecodeToInt16(rtpPacket.Payload, pcm16Buf)
							if err != nil {
								log.Printf("[Audio] Opus decode error (packet %d): %v", packetCount, err)
								continue
							}
							if sampleCount == 0 {
								continue
							}

							// Compute RMS energy of decoded frame
							var sumSq int64
							for i := 0; i < sampleCount; i++ {
								s := int64(pcm16Buf[i])
								sumSq += s * s
							}
							rms := int(math.Sqrt(float64(sumSq) / float64(sampleCount)))

							if packetCount == 1 || packetCount%100 == 0 || (rms > 250 && packetCount%20 == 0) {
								log.Printf("[Audio] Packet %d from %s: payload=%dB, samples=%d, RMS=%d (Floor=%.1f)",
									packetCount, sessionID, len(rtpPacket.Payload), sampleCount, rms, rmsFloor)
							}

							// Convert int16 samples to raw 16kHz little-endian bytes
							pcm16Bytes := make([]byte, sampleCount*2)
							for i := 0; i < sampleCount; i++ {
								s := pcm16Buf[i]
								pcm16Bytes[2*i] = byte(s & 0xff)
								pcm16Bytes[2*i+1] = byte(s >> 8)
							}

							// 1. Forward raw 16kHz signed-16 PCM directly to Deepgram STT
							if _, err := sttEngine.Write(pcm16Bytes); err != nil {
								log.Printf("Error writing PCM to Deepgram: %v", err)
							}

							// 2. Feed exact 640-byte (20 ms at 16kHz) chunks to WebRTC VAD
							vadBuffer = append(vadBuffer, pcm16Bytes...)
							for len(vadBuffer) >= 640 {
								chunk := vadBuffer[:640]
								vadBuffer = vadBuffer[640:]

								activeVoice, err := vad.Process(16000, chunk)
								if err != nil {
									log.Printf("VAD process error: %v", err)
									continue
								}

								isSpeaking := sm.Is(StateSpeaking)

								// ── Update Rolling Noise Floor Baseline (RMS_floor) ──────────────
								// Update baseline only when WebRTC VAD indicates silence/non-speech.
								// In quiet/fan-only intervals, rmsFloor adapts dynamically to the ambient environment.
								if !activeVoice {
									rmsFloor = (1.0-emaAlpha)*rmsFloor + emaAlpha*float64(rms)
									if rmsFloor < 50.0 {
										rmsFloor = 50.0
									}
								}

								// ── Layer 1 Noise Gate: Dynamic SNR Thresholding ─────────────────
								// Speech must clear the rolling ambient noise floor by a healthy SNR margin:
								// When listening: 2.5x ambient noise floor (minimum 250 RMS)
								// When speaking:  3.5x ambient noise floor (minimum 450 RMS) to prevent self-interruption
								minRms := int(rmsFloor * 2.5)
								if minRms < 250 {
									minRms = 250
								}
								if isSpeaking {
									minRms = int(rmsFloor * 3.5)
									if minRms < 450 {
										minRms = 450
									}
								}

								isRealVoice := activeVoice && rms >= minRms

								if isRealVoice {
									consecutivePositive++
									if consecutivePositive == 1 {
										log.Printf("[VAD] Voice detected from %s (RMS: %d, speaking: %t)", sessionID, rms, isSpeaking)
									}
									// Require sustained vocal energy (>= 3 frames / 60ms) before flagging speech active,
									// preventing brief guitar pick transients from prematurely suppressing chord detection.
									if orch != nil && consecutivePositive >= 3 {
										orch.SetSpeechActive(sessionID, true)
									}

									// Layer 3 Barge-In Threshold:
									// When Speaking or Thinking: NEVER allow raw WebRTC VAD energy to barge in,
									// because speaker acoustic bleed directly trips raw VAD without echo cancellation.
									// Barge-in during speech is handled exclusively by confirmed STT words (barge-in guard above).
									// When Listening: 3 frames (60 ms) flags speech active.
									if !isSpeaking && !sm.Is(StateThinking) && consecutivePositive >= 3 {
										// Local VAD confirmed speech starting during listening state.
									}
								} else {
									if orch != nil && consecutivePositive > 0 {
										orch.SetSpeechActive(sessionID, false)
									}
									consecutivePositive = 0
								}
							}
						}
					}()
				}
			},
			OnTrackUnsubscribed: func(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				log.Printf("Track unsubscribed from %s", rp.Identity())
				if orch != nil {
					orch.CloseSession(rp.Identity())
				}
			},
		},
		OnDisconnected: func() {
			log.Println("Disconnected from LiveKit room. Stopping outbound TTS stream.")
			cancel()
		},
	}

	if orch != nil {
		AttachDataChannelHandler(roomCB, orch)
	}

	roomName := strings.TrimSpace(os.Getenv("LIVEKIT_ROOM_NAME"))
	if roomName == "" {
		roomName = "voice-agent-room"
	}

	room, err := lksdk.ConnectToRoom(url, lksdk.ConnectInfo{
		APIKey:              apiKey,
		APISecret:           apiSecret,
		RoomName:            roomName,
		ParticipantIdentity: "zeplin-agent",
	}, roomCB)
	if err != nil {
		cancel()
		return nil, err
	}

	log.Printf("Connected to LiveKit room %s", room.Name())

	if orch != nil {
		orch.SetPublisher(func(update dsp.UIStateUpdate) {
			PublishDataChannelUpdate(room, update)
		})
	}

	// Initialize the outbound audio track (PCMU / Î¼-law at 8 kHz mono).
	capability := webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypePCMU,
		ClockRate: 8000,
		Channels:  1,
	}
	outboundTrack, err := lksdk.NewLocalSampleTrack(capability)
	if err != nil {
		room.Disconnect()
		cancel()
		return nil, fmt.Errorf("failed to create outbound track: %w", err)
	}

	_, err = room.LocalParticipant.PublishTrack(outboundTrack, &lksdk.TrackPublicationOptions{
		Name: "agent-voice",
	})
	if err != nil {
		room.Disconnect()
		cancel()
		return nil, fmt.Errorf("failed to publish outbound track: %w", err)
	}
	log.Println("Agent outbound voice track published successfully.")

	// Wire brain callbacks to stream Groq tokens directly to the TTS pipeline.
	brainInstance.OnToken = func(ctx context.Context, sessionID string, token string) {
		telemetryRegistry.Get(sessionID).RecordFirstToken()
		if err := ttsEngine.Speak(token); err != nil {
			log.Printf("Error sending text token to TTS: %v", err)
		}
	}
	brainInstance.OnFlush = func(ctx context.Context, sessionID string) {
		if err := ttsEngine.Flush(); err != nil {
			log.Printf("Error flushing TTS: %v", err)
		}
	}
	brainInstance.OnComplete = func(ctx context.Context, sessionID string, fullText string) {
		if orch != nil {
			orch.PublishFeedback(sessionID, fullText)
		}
	}

	// Outbound audio pacer loop: pulls PCMU frames from the TTS engine and writes
	// them to the LiveKit track at exactly the right 20ms cadence.
	//
	// Design: We block on reading a frame from ttsEngine.AudioChan FIRST. This
	// guarantees that no frames are ever skipped or dropped due to microsecond
	// jitter in the network/generator. Once a frame is received, we write it
	// to the WebRTC track, record telemetry, and then pace outbound delivery using
	// a high-precision hybrid spin-yield loop locked to exact 20.00ms intervals.
	go func() {
		var nextFrameTime time.Time

		drainQueue := func() {
			nextFrameTime = time.Time{}
			for {
				select {
				case <-ttsEngine.AudioChan:
					// Drop unplayed frame.
				default:
					transitionToListening(sm, telemetryRegistry)
					log.Println("Pacing queue drained. Agent state: Listening")
					return
				}
			}
		}

		for {
			// Prioritize any pending barge-in before blocking on the next frame.
			select {
			case <-interruptChan:
				drainQueue()
				continue
			default:
			}

			select {
			case <-ctx.Done():
				return
			case <-interruptChan:
				drainQueue()
				continue
			case chunk, ok := <-ttsEngine.AudioChan:
				if !ok {
					if sm.Is(StateSpeaking) {
						transitionToListening(sm, telemetryRegistry)
					}
					return
				}

				// Deepgram delivers variable-sized audio blocks (often 320 bytes = 40ms).
				// Standard WebRTC frame size for 8kHz mono PCMU is exactly 160 bytes (20ms).
				// We slice chunks into 160-byte frames so every outbound packet is exact 20.00ms.
				const frameSize = 160

				for offset := 0; offset < len(chunk); offset += frameSize {
					end := offset + frameSize
					if end > len(chunk) {
						end = len(chunk)
					}
					frame := chunk[offset:end]

					// First frame of a turn transitions the agent to StateSpeaking.
					if !sm.Is(StateSpeaking) {
						sm.Set(StateSpeaking)
					}

					// Î¼-law: 1 byte per sample at 8000 Hz.
					durationMs := float64(len(frame)) / 8000.0 * 1000.0
					duration := time.Duration(durationMs) * time.Millisecond

					if err := outboundTrack.WriteSample(pionmedia.Sample{
						Data:     frame,
						Duration: duration,
					}, nil); err != nil {
						log.Printf("Error writing audio sample to outbound track: %v", err)
					}

					activeTelemetry := telemetryRegistry.GetActiveTelemetry()
					if activeTelemetry != nil {
						activeTelemetry.RecordOutboundPush()
					}

					now := time.Now()
					if nextFrameTime.IsZero() || now.After(nextFrameTime.Add(duration)) {
						nextFrameTime = now.Add(duration)
					} else {
						nextFrameTime = nextFrameTime.Add(duration)
					}

					// High-precision hybrid spin-yield pacing:
					// 1. Coarse sleep for (remaining - 2ms) using a timer when remaining > 3ms to release CPU.
					// 2. Fine spin-yield with runtime.Gosched() for the final <=3ms to eliminate OS scheduler jitter (<2ms).
					for {
						remaining := time.Until(nextFrameTime)
						if remaining <= 0 {
							break
						}

						if remaining > 3*time.Millisecond {
							coarseTimer := time.NewTimer(remaining - 2*time.Millisecond)
							select {
							case <-ctx.Done():
								coarseTimer.Stop()
								return
							case <-interruptChan:
								coarseTimer.Stop()
								drainQueue()
								goto nextPacingCycle
							case <-coarseTimer.C:
							}
						} else {
							select {
							case <-ctx.Done():
								return
							case <-interruptChan:
								drainQueue()
								goto nextPacingCycle
							default:
								runtime.Gosched()
							}
						}
					}
				}

				// If all frames of this chunk were pushed and no more audio is buffered,
				// check if turn audio has completed/flushed and revert cleanly to Listening.
				if ttsEngine.QueueLen() == 0 && ttsEngine.IsFlushed() && sm.Is(StateSpeaking) {
					transitionToListening(sm, telemetryRegistry)
					nextFrameTime = time.Time{}
				}
			nextPacingCycle:
			}
		}
	}()

	return room, nil
}

func transitionToListening(sm *AgentStateMachine, registry *TelemetryRegistry) {
	if sm.Is(StateSpeaking) {
		sm.Set(StateListening)
		activeSession := registry.GetActiveSession()
		if activeSession != "" {
			go registry.Get(activeSession).CompileAndReport()
		}
	} else {
		sm.Set(StateListening)
	}
}

