package tts

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

// DeepgramStreamTTS coordinates text streaming to Deepgram's streaming TTS WebSocket
type DeepgramStreamTTS struct {
	conn      *websocket.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	AudioChan <-chan []byte
	mu        sync.Mutex
}

// NewDeepgramStreamTTS instantiates a custom Gorilla WebSocket client to Deepgram TTS
func NewDeepgramStreamTTS(apiKey string) (*DeepgramStreamTTS, error) {
	os.Setenv("DEEPGRAM_API_KEY", apiKey)

	ctx, cancel := context.WithCancel(context.Background())

	url := "wss://api.deepgram.com/v1/speak?model=aura-2-thalia-en&encoding=mulaw&sample_rate=8000"

	headers := http.Header{}
	headers.Add("Authorization", "Token "+apiKey)

	log.Println("Dialing Deepgram TTS WebSocket...")
	conn, resp, err := websocket.DefaultDialer.Dial(url, headers)
	if err != nil {
		cancel()
		if resp != nil {
			return nil, fmt.Errorf("failed to dial Deepgram TTS WS: %w (status: %s)", err, resp.Status)
		}
		return nil, fmt.Errorf("failed to dial Deepgram TTS WS: %w", err)
	}
	
	log.Println("Deepgram TTS WebSocket connection established.")

	audioChan := make(chan []byte, 1000)

	tts := &DeepgramStreamTTS{
		conn:      conn,
		ctx:       ctx,
		cancel:    cancel,
		AudioChan: audioChan,
	}

	// Start the read loop
	go tts.readLoop(audioChan)

	return tts, nil
}

func (t *DeepgramStreamTTS) readLoop(audioChan chan<- []byte) {
	defer func() {
		t.cancel()
		t.conn.Close()
		close(audioChan)
		log.Println("Deepgram TTS read loop terminated.")
	}()

	for {
		messageType, message, err := t.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("Deepgram TTS read error: %v", err)
			}
			return
		}

		select {
		case <-t.ctx.Done():
			return
		default:
		}

		switch messageType {
		case websocket.BinaryMessage:
			// Stream raw Opus frames into the audio channel
			select {
			case audioChan <- message:
			default:
				log.Println("Warning: Outbound audio buffer full, dropping Opus frame.")
			}
		case websocket.TextMessage:
			// Log metadata or warnings
			log.Printf("[Deepgram TTS] %s", string(message))
		}
	}
}

// Speak sends a text token chunk to the TTS stream
func (t *DeepgramStreamTTS) Speak(text string) error {
	select {
	case <-t.ctx.Done():
		return fmt.Errorf("deepgram tts stream context cancelled")
	default:
	}

	payload := map[string]string{
		"type": "Speak",
		"text": text,
	}
	
	msg, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn.WriteMessage(websocket.TextMessage, msg)
}

// Flush signals that text stream chunk is finalized
func (t *DeepgramStreamTTS) Flush() error {
	select {
	case <-t.ctx.Done():
		return fmt.Errorf("deepgram tts stream context cancelled")
	default:
	}

	payload := map[string]string{
		"type": "Flush",
	}

	msg, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn.WriteMessage(websocket.TextMessage, msg)
}

// Clear sends a message to Deepgram to instantly wipe its TTS buffer
func (t *DeepgramStreamTTS) Clear() error {
	select {
	case <-t.ctx.Done():
		return fmt.Errorf("deepgram tts stream context cancelled")
	default:
	}

	payload := map[string]string{
		"type": "Clear",
	}

	msg, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn.WriteMessage(websocket.TextMessage, msg)
}

// Close terminates the WS connection
func (t *DeepgramStreamTTS) Close() error {
	t.cancel()
	t.mu.Lock()
	defer t.mu.Unlock()
	// Attempt a clean closure
	t.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	return t.conn.Close()
}
