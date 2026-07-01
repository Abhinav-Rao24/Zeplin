package stt

import (
	"context"
	"fmt"
	"log"
	"os"

	msginterfaces "github.com/deepgram/deepgram-go-sdk/v3/pkg/api/listen/v1/websocket/interfaces"
	"github.com/deepgram/deepgram-go-sdk/v3/pkg/client/interfaces"
	"github.com/deepgram/deepgram-go-sdk/v3/pkg/client/listen"
)

// deepgramCallback implements msginterfaces.LiveMessageCallback
type deepgramCallback struct {
	onTranscript func(transcript string, isFinal bool)
}

func (c *deepgramCallback) Open(or *msginterfaces.OpenResponse) error { return nil }

func (c *deepgramCallback) Message(mr *msginterfaces.MessageResponse) error {
	if len(mr.Channel.Alternatives) > 0 {
		transcript := mr.Channel.Alternatives[0].Transcript
		if transcript != "" {
			if c.onTranscript != nil {
				c.onTranscript(transcript, mr.IsFinal)
			} else {
				fmt.Printf("[Live Transcript] (final: %t) %s\n", mr.IsFinal, transcript)
			}
		}
	}
	return nil
}
func (c *deepgramCallback) Metadata(md *msginterfaces.MetadataResponse) error { return nil }
func (c *deepgramCallback) SpeechStarted(ssr *msginterfaces.SpeechStartedResponse) error { return nil }
func (c *deepgramCallback) UtteranceEnd(ur *msginterfaces.UtteranceEndResponse) error { return nil }
func (c *deepgramCallback) Close(cr *msginterfaces.CloseResponse) error { return nil }
func (c *deepgramCallback) Error(er *msginterfaces.ErrorResponse) error {
	log.Printf("[Deepgram Error] %v", er.ErrMsg)
	return nil
}
func (c *deepgramCallback) UnhandledEvent(data []byte) error { return nil }

// DeepgramStreamSTT represents the STT client
type DeepgramStreamSTT struct {
	ws     *listen.WSCallback
	ctx    context.Context
	cancel context.CancelFunc
}

// NewDeepgramStreamSTT creates and starts a new Deepgram live transcription session.
func NewDeepgramStreamSTT(apiKey string, onTranscript func(transcript string, isFinal bool)) (*DeepgramStreamSTT, error) {
	os.Setenv("DEEPGRAM_API_KEY", apiKey)

	ctx, cancel := context.WithCancel(context.Background())

	tOptions := &interfaces.LiveTranscriptionOptions{
		Model:       "nova-3",
		Language:    "en-US",
		SmartFormat: true,
	}

	cb := &deepgramCallback{
		onTranscript: onTranscript,
	}

	// Initialize the websocket client
	ws, err := listen.NewWebSocketUsingCallbackWithDefaults(ctx, tOptions, cb)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create Deepgram WS: %v", err)
	}

	// Connect
	bConnected := ws.Connect()
	if !bConnected {
		cancel()
		return nil, fmt.Errorf("failed to connect to Deepgram WS")
	}

	log.Println("Deepgram STT connection established.")

	return &DeepgramStreamSTT{
		ws:     ws,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Write implements io.Writer so we can stream data (e.g. from OggWriter) directly to Deepgram
func (d *DeepgramStreamSTT) Write(p []byte) (n int, err error) {
	select {
	case <-d.ctx.Done():
		return 0, fmt.Errorf("deepgram stream context cancelled")
	default:
		_, err := d.ws.Write(p)
		if err != nil {
			log.Printf("Error sending audio to Deepgram: %v", err)
			return 0, err
		}
		return len(p), nil
	}
}

// Close closes the Deepgram connection
func (d *DeepgramStreamSTT) Close() error {
	d.cancel()
	d.ws.Stop()
	return nil
}
