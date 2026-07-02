package transport

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
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

// ConnectLiveKit establishes a connection to a LiveKit room, hooks up the STT writer per participant, and publishes the agent's outbound voice track with Deepgram TTS streaming.
func ConnectLiveKit(url, apiKey, apiSecret, deepgramAPIKey string, brainInstance *brain.Brain, ttsEngine *tts.DeepgramStreamTTS) (*lksdk.Room, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// interruptChan signals the pacing loop to drain the TTS audio queue during a barge-in
	interruptChan := make(chan struct{}, 1)

	// State machine variables to track agent speech resiliently
	var isBrainActive atomic.Bool
	var isPacingActive atomic.Bool

	roomCB := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: func(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				if track.Kind() == webrtc.RTPCodecTypeAudio {
					sessionID := rp.Identity()
					log.Printf("Audio track subscribed from %s", sessionID)

					triggerInterrupt := func() {
						// Only interrupt if the agent is actively thinking or speaking
						isSpeaking := isBrainActive.Load() || isPacingActive.Load()
						if !isSpeaking {
							return
						}

						log.Printf("[Barge-In] Interruption triggered! BrainActive: %t, PacingActive: %t", isBrainActive.Load(), isPacingActive.Load())
						
						// 1. Send Clear message to Deepgram TTS to stop generation
						ttsEngine.Clear()
						
						// 2. Interrupt any active Brain generation
						brainInstance.Interrupt(sessionID)
						
						// 3. Signal pacing loop to drain unplayed frames
						select {
						case interruptChan <- struct{}{}:
						default:
						}
					}

					// Instantiate STT engine per participant
					sttEngine, err := stt.NewDeepgramStreamSTT(deepgramAPIKey, func(transcript string, isFinal bool) {
						if transcript == "" {
							return
						}

						if !isFinal {
							// Interim transcript = confirmed user words arriving while agent speaks.
							// This is the only reliable barge-in signal that proves a real human is
							// speaking (not self-echo or ambient noise).
							triggerInterrupt()
							return
						}

						// Final transcript: trigger interrupt (in case no interim arrived) then run brain.
						triggerInterrupt()
						go func() {
							isBrainActive.Store(true)
							log.Printf("[Brain] Turn started for session %s: %q", sessionID, transcript)
							// Wait briefly for stray buffered tokens from the cancelled Groq HTTP stream
							// to hit Speak() and be dropped (they're blocked by clearing=true).
							// After this window, StartNewTurn() opens the gate for the new response.
							time.Sleep(60 * time.Millisecond)
							ttsEngine.StartNewTurn()
							if err := brainInstance.ProcessTurn(context.Background(), sessionID, transcript); err != nil {
								log.Printf("Error processing turn for session %s: %v", sessionID, err)
							}
							isBrainActive.Store(false)
							log.Printf("[Brain] Turn ended. BrainActive=%v, PacingActive=%v", isBrainActive.Load(), isPacingActive.Load())
						}()
					})
					if err != nil {
						log.Printf("Error creating STT engine for %s: %v", sessionID, err)
						return
					}

					// Bypass local PCM decoding to avoid CGO requirements on Windows.
					// Wrap the STT writer (Deepgram) with an OGG writer, and stream the Opus RTP packets.
					ogg, err := oggwriter.NewWith(sttEngine, 48000, 2)
					if err != nil {
						log.Printf("Error creating OGG writer: %v", err)
						sttEngine.Close()
						return
					}
					
					log.Println("Successfully attached OGG writer to incoming track")

					// Read RTP packets, run local WebRTC VAD, and write them to Deepgram as OGG
					go func() {
						defer ogg.Close()
						defer sttEngine.Close()

						// Initialize local WebRTC VAD
						vad, err := webrtcvad.New()
						if err != nil {
							log.Printf("Error creating local VAD: %v", err)
							return
						}
						// Configure to aggressive mode (3) to strictly filter background noise
						if err := vad.SetMode(3); err != nil {
							log.Printf("Error setting VAD mode: %v", err)
							return
						}

						// Initialize Opus Decoder (48kHz, 2 channels stereo)
						dec, err := opus.NewDecoderWithOutput(48000, 2)
						if err != nil {
							log.Printf("Error creating Opus decoder: %v", err)
							return
						}

						pcmBuf := make([]int16, 11520) // max possible Opus frame size is 120ms
						var vadBuffer []byte
						consecutivePositive := 0

						for {
							rtpPacket, _, err := track.ReadRTP()
							if err != nil {
								log.Printf("Track read error or closed: %v", err)
								return
							}
							
							// 1. Write to OGG container for Deepgram STT
							if err := ogg.WriteRTP(rtpPacket); err != nil {
								log.Printf("Error writing RTP to OGG container: %v", err)
								return
							}

							// 2. Decode Opus payload to raw PCM
							if len(rtpPacket.Payload) == 0 {
								continue
							}

							sampleCount, err := dec.DecodeToInt16(rtpPacket.Payload, pcmBuf)
							if err != nil {
								continue
							}

							// 3. Downmix stereo -> mono, decimate 6:1 (48kHz stereo -> 8kHz mono)
							for i := 0; i < sampleCount; i += 6 {
								left := pcmBuf[2*i]
								right := pcmBuf[2*i+1]
								mono := int16((int32(left) + int32(right)) / 2)

								// Convert to little-endian bytes and append to VAD buffer
								vadBuffer = append(vadBuffer, byte(mono&0xff), byte(mono>>8))
							}

							// 4. Feed 320-byte (20ms) chunks to VAD Process(8000, frame)
							for len(vadBuffer) >= 320 {
								chunk := vadBuffer[:320]
								activeVoice, err := vad.Process(8000, chunk)
								if err != nil {
									log.Printf("VAD process error: %v", err)
								} else if activeVoice {
									consecutivePositive++
									if consecutivePositive >= 3 {
										// 3 consecutive positive VAD frames (60ms) -> trigger interrupt!
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

	room, err := lksdk.ConnectToRoom(url, lksdk.ConnectInfo{
		APIKey:              apiKey,
		APISecret:           apiSecret,
		RoomName:            "voice-agent-room", // Designated virtual room
		ParticipantIdentity: "zeplin-agent",
	}, roomCB)

	if err != nil {
		cancel()
		return nil, err
	}

	log.Printf("Connected to LiveKit room %s", room.Name())

	// Initialize outbound audio track using Pion's pure-Go structures to bypass Windows CGO dependency.
	capability := webrtc.RTPCodecCapability{
		MimeType:     webrtc.MimeTypePCMU,
		ClockRate:    8000,
		Channels:     1,
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

	// Register brain callback functions to stream Eino tokens chunk-by-chunk to the active TTS stream
	brainInstance.OnToken = func(ctx context.Context, sessionID string, token string) {
		if err := ttsEngine.Speak(token); err != nil {
			log.Printf("Error sending text token to TTS: %v", err)
		}
	}
	brainInstance.OnFlush = func(ctx context.Context, sessionID string) {
		if err := ttsEngine.Flush(); err != nil {
			log.Printf("Error flushing TTS: %v", err)
		}
	}

	// Start outbound audio pacer loop to write ready-to-use Opus frames cleanly to the track.
	go func() {
		drainQueue := func() {
			draining := true
			for draining {
				select {
				case <-ttsEngine.AudioChan:
					// Drop unplayed frame
				default:
					draining = false
				}
			}
			isPacingActive.Store(false)
			log.Println("Pacing queue drained successfully due to barge-in.")
		}

		for {
			// Prioritize barge-in / cancellation check
			select {
			case <-interruptChan:
				drainQueue()
			default:
			}

			select {
			case <-ctx.Done():
				return
			case <-interruptChan:
				drainQueue()
			case frame, ok := <-ttsEngine.AudioChan:
				if !ok {
					return
				}
				
				isPacingActive.Store(true)

				// mulaw (PCMU) = 1 byte per sample. 8000 Hz = 8000 bytes/sec.
				durationMs := float64(len(frame)) / 8000.0 * 1000.0
				duration := time.Duration(durationMs) * time.Millisecond
				
				err := outboundTrack.WriteSample(pionmedia.Sample{
					Data:     frame,
					Duration: duration,
				}, nil)
				if err != nil {
					log.Printf("Error writing audio sample to outbound track: %v", err)
				}
				// Pace playing output stream to prevent choppy playback.
				// Wait for duration, OR break instantly if interrupted
				select {
				case <-time.After(duration):
					// Sleep finished
				case <-interruptChan:
					// Interrupted during pacing! Drain queue and drop.
					drainQueue()
					log.Println("Pacing queue drained successfully due to barge-in during sleep.")
				}
				
				// Reset pacing state if queue is currently empty
				if len(ttsEngine.AudioChan) == 0 {
					isPacingActive.Store(false)
				}
			}
		}
	}()

	return room, nil
}


