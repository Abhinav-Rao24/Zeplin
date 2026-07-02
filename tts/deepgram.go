package tts

import (
	"context"
	"fmt"
	"log"
	"os"

	interfaces "github.com/deepgram/deepgram-go-sdk/v3/pkg/client/interfaces/v1"
	speak "github.com/deepgram/deepgram-go-sdk/v3/pkg/client/speak"
	speakInterfaces "github.com/deepgram/deepgram-go-sdk/v3/pkg/api/speak/v1/websocket/interfaces"
)

type ttsCallback struct {
	onAudio func([]byte)
}

func (c *ttsCallback) Open(or *speakInterfaces.OpenResponse) error {
	log.Println("Deepgram TTS connection opened.")
	return nil
}

func (c *ttsCallback) Binary(ar []byte) error {
	if c.onAudio != nil {
		c.onAudio(ar)
	}
	return nil
}

func (c *ttsCallback) Metadata(md *speakInterfaces.MetadataResponse) error {
	return nil
}

func (c *ttsCallback) Flush(fr *speakInterfaces.FlushedResponse) error {
	return nil
}

func (c *ttsCallback) Close(cr *speakInterfaces.CloseResponse) error {
	log.Println("Deepgram TTS connection closed.")
	return nil
}

func (c *ttsCallback) Warning(dw *interfaces.DeepgramWarning) error {
	log.Printf("[Deepgram TTS Warning] %s", dw.WarnMsg)
	return nil
}

func (c *ttsCallback) Error(de *interfaces.DeepgramError) error {
	log.Printf("[Deepgram TTS Error] %s", de.ErrMsg)
	return nil
}

func (c *ttsCallback) Clear(cr *speakInterfaces.ClearedResponse) error {
	return nil
}

func (c *ttsCallback) UnhandledEvent(data []byte) error {
	return nil
}

// DeepgramStreamTTS coordinates text streaming to Deepgram's streaming TTS WebSocket
type DeepgramStreamTTS struct {
	client *speak.WSCallback
	ctx    context.Context
	cancel context.CancelFunc
}

// NewDeepgramStreamTTS instantiates a new streaming TTS websocket client with Deepgram
func NewDeepgramStreamTTS(apiKey string, onAudio func([]byte)) (*DeepgramStreamTTS, error) {
	os.Setenv("DEEPGRAM_API_KEY", apiKey)

	ctx, cancel := context.WithCancel(context.Background())

	cOptions := &interfaces.ClientOptions{}
	sOptions := &interfaces.WSSpeakOptions{
		Model:      "aura-2-thalia-en",
		Encoding:   "opus",
		SampleRate: 48000,
	}

	cb := &ttsCallback{
		onAudio: onAudio,
	}

	client, err := speak.NewWSUsingCallback(ctx, apiKey, cOptions, sOptions, cb)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create Deepgram TTS WS: %w", err)
	}

	// Connect
	bConnected := client.Connect()
	if !bConnected {
		cancel()
		return nil, fmt.Errorf("failed to connect to Deepgram TTS WS")
	}

	log.Println("Deepgram TTS connection established.")

	return &DeepgramStreamTTS{
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Speak sends a text token chunk to the TTS stream
func (t *DeepgramStreamTTS) Speak(text string) error {
	select {
	case <-t.ctx.Done():
		return fmt.Errorf("deepgram tts stream context cancelled")
	default:
		return t.client.Speak(text)
	}
}

// Flush signals that text stream chunk is finalized
func (t *DeepgramStreamTTS) Flush() error {
	select {
	case <-t.ctx.Done():
		return fmt.Errorf("deepgram tts stream context cancelled")
	default:
		return t.client.Flush()
	}
}

// Close terminates the WS connection
func (t *DeepgramStreamTTS) Close() error {
	t.cancel()
	t.client.Stop()
	return nil
}
