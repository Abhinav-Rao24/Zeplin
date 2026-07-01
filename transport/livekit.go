package transport

import (
	"context"
	"log"

	"github.com/Abhinav-Rao24/Zeplin/brain"
	"github.com/Abhinav-Rao24/Zeplin/stt"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/oggwriter"
)

// ConnectLiveKit establishes a connection to a LiveKit room and hooks up the STT writer per participant
func ConnectLiveKit(url, apiKey, apiSecret, deepgramAPIKey string, brainInstance *brain.Brain) (*lksdk.Room, error) {
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
	}

	room, err := lksdk.ConnectToRoom(url, lksdk.ConnectInfo{
		APIKey:              apiKey,
		APISecret:           apiSecret,
		RoomName:            "voice-agent-room", // Designated virtual room
		ParticipantIdentity: "zeplin-agent",
	}, roomCB)

	if err != nil {
		return nil, err
	}

	log.Printf("Connected to LiveKit room %s", room.Name())
	return room, nil
}

