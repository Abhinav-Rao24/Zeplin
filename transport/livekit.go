package transport

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Abhinav-Rao24/Zeplin/brain"
	"github.com/Abhinav-Rao24/Zeplin/stt"
	"github.com/Abhinav-Rao24/Zeplin/tts"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/oggwriter"
)

// ConnectLiveKit establishes a connection to a LiveKit room, hooks up the STT writer per participant, and publishes the agent's outbound voice track with Deepgram TTS streaming.
func ConnectLiveKit(url, apiKey, apiSecret, deepgramAPIKey string, brainInstance *brain.Brain) (*lksdk.Room, error) {
	ctx, cancel := context.WithCancel(context.Background())

	roomCB := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: func(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				if track.Kind() == webrtc.RTPCodecTypeAudio {
					sessionID := rp.Identity()
					log.Printf("Audio track subscribed from %s", sessionID)

					// Instantiate STT engine per participant
					sttEngine, err := stt.NewDeepgramStreamSTT(deepgramAPIKey, func(transcript string, isFinal bool) {
						if isFinal {
							go func() {
								if err := brainInstance.ProcessTurn(context.Background(), sessionID, transcript); err != nil {
									log.Printf("Error processing turn for session %s: %v", sessionID, err)
								}
							}()
						}
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

					// Read RTP packets and write them to Deepgram as OGG
					go func() {
						defer ogg.Close()
						defer sttEngine.Close()
						for {
							rtpPacket, _, err := track.ReadRTP()
							if err != nil {
								log.Printf("Track read error or closed: %v", err)
								return
							}
							
							if err := ogg.WriteRTP(rtpPacket); err != nil {
								log.Printf("Error writing RTP to OGG container: %v", err)
								return
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
		MimeType:     webrtc.MimeTypeOpus,
		ClockRate:    48000,
		Channels:     1,
		SDPFmtpLine:  "minptime=10;useinbandfec=1",
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

	// Instantiate Deepgram streaming TTS WS client
	audioChan := make(chan []byte, 1000)
	ttsEngine, err := tts.NewDeepgramStreamTTS(deepgramAPIKey, func(audio []byte) {
		select {
		case audioChan <- audio:
		default:
			log.Println("Warning: Outbound audio buffer full, dropping Opus frame.")
		}
	})
	if err != nil {
		room.Disconnect()
		cancel()
		return nil, fmt.Errorf("failed to initialize TTS engine: %w", err)
	}

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
		defer ttsEngine.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case frame, ok := <-audioChan:
				if !ok {
					return
				}
				err := outboundTrack.WriteSample(pionmedia.Sample{
					Data:     frame,
					Duration: 20 * time.Millisecond,
				}, nil)
				if err != nil {
					log.Printf("Error writing audio sample to outbound track: %v", err)
				}
				// Pace playing output stream to prevent choppy playback.
				time.Sleep(20 * time.Millisecond)
			}
		}
	}()

	return room, nil
}


