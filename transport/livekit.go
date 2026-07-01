package transport

import (
	"io"
	"log"

	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/oggwriter"
)

// ConnectLiveKit establishes a connection to a LiveKit room and hooks up the STT writer
func ConnectLiveKit(url, apiKey, apiSecret string, sttWriter io.Writer) (*lksdk.Room, error) {
	roomCB := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: func(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				if track.Kind() == webrtc.RTPCodecTypeAudio {
					log.Printf("Audio track subscribed from %s", rp.Identity())

					// Bypass local PCM decoding to avoid CGO requirements on Windows.
					// Wrap the STT writer (Deepgram) with an OGG writer, and stream the Opus RTP packets.
					ogg, err := oggwriter.NewWith(sttWriter, 48000, 2)
					if err != nil {
						log.Printf("Error creating OGG writer: %v", err)
						return
					}
					
					log.Println("Successfully attached OGG writer to incoming track")

					// Read RTP packets and write them to Deepgram as OGG
					go func() {
						defer ogg.Close()
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
