package transport

import (
	"context"
	"encoding/json"
	"log"

	lksdk "github.com/livekit/server-sdk-go/v2"

	"github.com/Abhinav-Rao24/Zeplin/dsp"
	"github.com/Abhinav-Rao24/Zeplin/lessons"
)

// DataChannelHandler wires the LiveKit room's OnDataReceived callback to the
// lesson orchestrator. It decodes JSON payloads from the browser AudioWorklet
// and dispatches them by event type.
//
// Expected event types from the browser:
//   - "chord_detected"   → dsp.ChordEvent  → orchestrator.HandleChordEvent
//   - "calibration_f0"   → dsp.TuningCalibrationEvent → orchestrator.HandleTuningCalibration
//
// The function returns the configured RoomCallback that must be used when
// calling lksdk.ConnectToRoom.
func AttachDataChannelHandler(roomCB *lksdk.RoomCallback, orch *lessons.Orchestrator) {
	roomCB.OnDataReceived = func(data []byte, params lksdk.DataReceiveParams) {
		senderID := "unknown"
		if params.SenderIdentity != "" {
			senderID = params.SenderIdentity
		}

		// Peek at the "event" discriminator field before full decode.
		var envelope struct {
			Event string `json:"event"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			log.Printf("[DataChannel] Malformed JSON from %s: %v", senderID, err)
			return
		}

		switch envelope.Event {
		case "chord_detected":
			var evt dsp.ChordEvent
			if err := json.Unmarshal(data, &evt); err != nil {
				log.Printf("[DataChannel] Failed to decode ChordEvent from %s: %v", senderID, err)
				return
			}
			log.Printf("[DataChannel] ChordEvent from %s: %s (conf=%.2f)", senderID, evt.DetectedChord, evt.Confidence)
			orch.HandleChordEvent(senderID, evt)

		case "calibration_f0":
			var evt dsp.TuningCalibrationEvent
			if err := json.Unmarshal(data, &evt); err != nil {
				log.Printf("[DataChannel] Failed to decode TuningCalibrationEvent from %s: %v", senderID, err)
				return
			}
			log.Printf("[DataChannel] Tuning calibration from %s: %s @ %.1f Hz", senderID, evt.Note, evt.Hz)
			orch.HandleTuningCalibration(senderID, evt)

		default:
			log.Printf("[DataChannel] Unknown event type %q from %s — ignoring", envelope.Event, senderID)
		}
	}
}

// PublishDataChannelUpdate sends a UIStateUpdate to all participants in the
// room as a reliable DataChannel message (JSON encoded).
func PublishDataChannelUpdate(room *lksdk.Room, update dsp.UIStateUpdate) {
	data, err := json.Marshal(update)
	if err != nil {
		log.Printf("[DataChannel] Failed to marshal UIStateUpdate: %v", err)
		return
	}
	if err := room.LocalParticipant.PublishData(data, lksdk.WithDataPublishReliable(true)); err != nil {
		log.Printf("[DataChannel] Failed to publish UIStateUpdate: %v", err)
	}
}

// SpeechActiveNotifier returns a function that the VAD/STT layer can call
// to set the speech-active gate on the orchestrator, suppressing chord
// analysis during vocal activity.
func SpeechActiveNotifier(orch *lessons.Orchestrator, sessionID string) func(active bool) {
	return func(active bool) {
		orch.SetSpeechActive(sessionID, active)
	}
}

// SpeechTurnHandler returns a function to be called by the STT layer on a
// finalized transcript. It routes the speech to the orchestrator's brain path.
func SpeechTurnHandler(orch *lessons.Orchestrator, ctx context.Context) func(sessionID, transcript string) {
	return func(sessionID, transcript string) {
		go orch.HandleSpeech(ctx, sessionID, transcript)
	}
}
