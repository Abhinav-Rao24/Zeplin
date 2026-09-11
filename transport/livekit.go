package transport

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/Abhinav-Rao24/Zeplin/brain"
	"github.com/Abhinav-Rao24/Zeplin/stt"
	"github.com/Abhinav-Rao24/Zeplin/tts"
	"github.com/aflyingHusky/go-webrtcvad"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/opus"
	"github.com/pion/webrtc/v4"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/oggwriter"
)

// echoOverlapRatio computes the fraction of words in candidate that appear in reference.
// A value > 0.40 indicates the STT transcript is likely an acoustic echo of the bot's own
// TTS output that slipped through browser-level AEC, not genuine human speech.
func echoOverlapRatio(candidate, reference string) float64 {
	candWords := strings.Fields(strings.ToLower(candidate))
	refWords := strings.Fields(strings.ToLower(reference))
	if len(candWords) == 0 || len(refWords) == 0 {
		return 0
	}
	refSet := make(map[string]bool, len(refWords))
	for _, w := range refWords {
		refSet[w] = true
	}
	matches := 0
	for _, w := range candWords {
		if refSet[w] {
			matches++
		}
	}
	return float64(matches) / float64(len(candWords))
}

// ConnectLiveKit establishes a connection to a LiveKit room, hooks up the STT writer per
// participant, and publishes the agent's outbound voice track with Deepgram TTS streaming.
func ConnectLiveKit(url, apiKey, apiSecret, deepgramAPIKey string, brainInstance *brain.Brain, ttsEngine *tts.DeepgramStreamTTS) (*lksdk.Room, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// interruptChan signals the pacing loop to drain the TTS audio queue during a barge-in.
	interruptChan := make(chan struct{}, 1)

	// Typed agent state machine. Replaces the old pair of isBrainActive / isPacingActive
	// atomic.Bool flags with a single, explicitly-typed state for clearer reasoning.
	//   StateListening → StateThinking → StateSpeaking → StateListening
	sm := &AgentStateMachine{}

	telemetryRegistry := NewTelemetryRegistry()
	ttsEngine.OnAudioFrame = func() {
		telemetryRegistry.RecordFirstAudioFrame()
	}

	roomCB := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: func(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				if track.Kind() == webrtc.RTPCodecTypeAudio {
					sessionID := rp.Identity()
					log.Printf("Audio track subscribed from %s", sessionID)

					triggerInterrupt := func() {
						// Only interrupt if the agent is actively thinking or speaking.
						// During StateListening there is no active generation to cancel.
						if sm.Is(StateListening) {
							return
						}

						log.Printf("[Barge-In] Interruption triggered! Agent state: %s", sm.Get())

						// 1. Send Clear to Deepgram TTS to stop current audio generation.
						ttsEngine.Clear()

						// 2. Cancel the active Groq streaming turn.
						brainInstance.Interrupt(sessionID)

						// 3. Signal the pacing loop to drain any unplayed audio frames.
						select {
						case interruptChan <- struct{}{}:
						default:
						}
					}

					// Instantiate STT engine per participant.
					sttEngine, err := stt.NewDeepgramStreamSTT(deepgramAPIKey, func(transcript string, isFinal bool) {
						if transcript == "" {
							return
						}

						// ── Software Echo Guard ────────────────────────────────────────────────
						// When the agent is actively playing audio (StateSpeaking), an incoming
						// STT transcript might be the bot's own voice looping back through the
						// mic after bypassing the browser's AEC hardware filter.
						//
						// We compare the transcript's word set against the rolling buffer of
						// recently spoken TTS tokens. If >40 % of the transcript's words appear
						// in the buffer, it is almost certainly acoustic self-echo — drop it.
						// If the overlap is low, a real human is speaking and we allow barge-in.
						// ──────────────────────────────────────────────────────────────────────
						if sm.Is(StateSpeaking) {
							recentSpoken := ttsEngine.RecentSpokenText()
							overlap := echoOverlapRatio(transcript, recentSpoken)
							if overlap > 0.40 {
								log.Printf("[Echo Guard] Dropped STT (%.0f%% overlap with recent TTS): %q", overlap*100, transcript)
								return
							}
						}

						if !isFinal {
							// Interim transcripts are unreliable noise sources — do NOT trigger
							// a hard barge-in here. The VAD layer handles real-time interruption
							// with a sustained-voice threshold to prevent false positives.
							return
						}

						// Final transcript: trigger interrupt (catches cases where no interim
						// arrived) then hand off to the brain for a new response turn.
						triggerInterrupt()
						go func() {
							telemetryRegistry.SetActiveSession(sessionID)
							telemetryRegistry.Get(sessionID).StartThinking()
							sm.Set(StateThinking)
							log.Printf("[Brain] Turn started for session %s: %q", sessionID, transcript)

							// Allow 60 ms for stray buffered Groq HTTP tokens to hit the
							// clearing gate in Speak() and be silently dropped before we
							// call StartNewTurn() to re-open the gate for the new response.
							time.Sleep(60 * time.Millisecond)
							ttsEngine.StartNewTurn()

							if err := brainInstance.ProcessTurn(context.Background(), sessionID, transcript); err != nil {
								log.Printf("Error processing turn for session %s: %v", sessionID, err)
							}

							// If ProcessTurn finished without producing any audio (e.g., error
							// path), the pacing loop never transitioned us to StateSpeaking, so
							// reset manually to avoid getting stuck in StateThinking.
							if sm.Is(StateThinking) {
								sm.Set(StateListening)
							}
							log.Printf("[Brain] Turn ended. Agent state: %s", sm.Get())
						}()
					})
					if err != nil {
						log.Printf("Error creating STT engine for %s: %v", sessionID, err)
						return
					}

					// Wrap the STT writer (Deepgram) with an OGG writer and stream Opus RTP packets.
					ogg, err := oggwriter.NewWith(sttEngine, 48000, 2)
					if err != nil {
						log.Printf("Error creating OGG writer: %v", err)
						sttEngine.Close()
						return
					}

					log.Println("Successfully attached OGG writer to incoming track")

					// RTP read loop: forwards audio to Deepgram STT and runs local WebRTC VAD.
					go func() {
						defer ogg.Close()
						defer sttEngine.Close()

						// Initialize local WebRTC VAD in aggressive mode (3) to
						// filter background noise and room tones.
						vad, err := webrtcvad.New()
						if err != nil {
							log.Printf("Error creating local VAD: %v", err)
							return
						}
						if err := vad.SetMode(3); err != nil {
							log.Printf("Error setting VAD mode: %v", err)
							return
						}

						// Initialize Opus decoder (48 kHz, stereo) for PCM conversion.
						dec, err := opus.NewDecoderWithOutput(48000, 2)
						if err != nil {
							log.Printf("Error creating Opus decoder: %v", err)
							return
						}

						pcmBuf := make([]int16, 11520) // max Opus frame = 120 ms at 48 kHz stereo
						var vadBuffer []byte
						consecutivePositive := 0

						for {
							rtpPacket, _, err := track.ReadRTP()
							if err != nil {
								log.Printf("Track read error or closed: %v", err)
								return
							}

							// 1. Forward to Deepgram STT via OGG container.
							if err := ogg.WriteRTP(rtpPacket); err != nil {
								log.Printf("Error writing RTP to OGG container: %v", err)
								return
							}

							// 2. Decode Opus payload to raw signed-16 PCM.
							if len(rtpPacket.Payload) == 0 {
								continue
							}
							sampleCount, err := dec.DecodeToInt16(rtpPacket.Payload, pcmBuf)
							if err != nil {
								continue
							}

							// 3. Downmix stereo → mono and decimate 6:1 (48 kHz → 8 kHz).
							for i := 0; i < sampleCount; i += 6 {
								left := pcmBuf[2*i]
								right := pcmBuf[2*i+1]
								mono := int16((int32(left) + int32(right)) / 2)
								vadBuffer = append(vadBuffer, byte(mono&0xff), byte(mono>>8))
							}

							// 4. Feed exact 320-byte (20 ms) chunks to WebRTC VAD.
							for len(vadBuffer) >= 320 {
								chunk := vadBuffer[:320]

								// ── Dual-Layer Echo Mitigation ────────────────────────────────
								// Listening: 3 consecutive positive frames (60 ms) → barge-in.
								// Speaking:  10 consecutive positive frames (200 ms) → barge-in.
								// The higher speaking threshold requires the human to sustain
								// clear speech for 200 ms before breaking through — laptop speaker
								// echo and short noise bursts rarely sustain that long at the same
								// energy level as real speech in aggressive VAD mode 3.
								vadThreshold := 3
								if sm.Is(StateSpeaking) {
									vadThreshold = 10
								}

								activeVoice, err := vad.Process(8000, chunk)
								if err != nil {
									log.Printf("VAD process error: %v", err)
								} else if activeVoice {
									consecutivePositive++
									if consecutivePositive >= vadThreshold {
										triggerInterrupt()
									}
								} else {
									consecutivePositive = 0
								}
								vadBuffer = vadBuffer[320:]
							}
						}
					}()
				}
			},
			OnTrackUnsubscribed: func(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				log.Printf("Track unsubscribed from %s", rp.Identity())
			},
		},
		OnDisconnected: func() {
			log.Println("Disconnected from LiveKit room. Stopping outbound TTS stream.")
			cancel()
		},
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

	// Initialize the outbound audio track (PCMU / μ-law at 8 kHz mono).
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

					// μ-law: 1 byte per sample at 8000 Hz.
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

					// If this was the last frame in the queue, transition back to listening
					// immediately to reduce latency for the user's response.
					isLastFrame := (offset+frameSize >= len(chunk)) && len(ttsEngine.AudioChan) == 0
					if isLastFrame && sm.Is(StateSpeaking) {
						transitionToListening(sm, telemetryRegistry)
						nextFrameTime = time.Time{}
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
			registry.Get(activeSession).CompileAndReport()
		}
	} else {
		sm.Set(StateListening)
	}
}
