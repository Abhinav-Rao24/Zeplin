package tts

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type ttsCmd struct {
	cmdType string // "Speak", "Flush", "Clear"
	text    string
}

// DeepgramStreamTTS coordinates text streaming to Deepgram's streaming TTS WebSocket
type DeepgramStreamTTS struct {
	conn      *websocket.Conn
	ctx       context.Context
	cancel    context.CancelFunc
	AudioChan <-chan []byte
	mu        sync.Mutex
	clearing  atomic.Bool
	cmdChan   chan ttsCmd
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
	cmdChan := make(chan ttsCmd, 1000)

	tts := &DeepgramStreamTTS{
		conn:      conn,
		ctx:       ctx,
		cancel:    cancel,
		AudioChan: audioChan,
		cmdChan:   cmdChan,
	}

	// Start the read loop
	go tts.readLoop(audioChan)
	// Start the write loop
	go tts.writeLoop()

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
			if t.clearing.Load() {
				continue // Drop straggling audio frames from the previous interrupted generation
			}
			// Stream raw PCMU frames into the audio channel
			select {
			case audioChan <- message:
			default:
				log.Println("Warning: Outbound audio buffer full, dropping PCMU frame.")
			}
		case websocket.TextMessage:
			// Log metadata or warnings
			log.Printf("[Deepgram TTS] %s", string(message))
		}
	}
}

func (t *DeepgramStreamTTS) writeLoop() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case cmd := <-t.cmdChan:
			var payload map[string]string
			if cmd.cmdType == "Speak" {
				payload = map[string]string{
					"type": "Speak",
					"text": cmd.text,
				}
			} else if cmd.cmdType == "Flush" {
				payload = map[string]string{
					"type": "Flush",
				}
			} else if cmd.cmdType == "Clear" {
				payload = map[string]string{
					"type": "Clear",
				}
			}

			msg, err := json.Marshal(payload)
			if err != nil {
				log.Printf("Error marshaling TTS payload: %v", err)
				continue
			}

			t.mu.Lock()
			err = t.conn.WriteMessage(websocket.TextMessage, msg)
			t.mu.Unlock()
			if err != nil {
				log.Printf("Error writing TTS command to WS: %v", err)
			}
		}
	}
}

// Speak sends a text token chunk to the TTS stream
func (t *DeepgramStreamTTS) Speak(text string) error {
	// If we're in a post-interruption clearing state, drop this token.
	// This prevents stray buffered tokens from the cancelled Groq stream from
	// going to Deepgram after the Clear command was already sent.
	// clearing is reset by StartNewTurn() when the new brain turn is ready.
	if t.clearing.Load() {
		return nil
	}

	select {
	case <-t.ctx.Done():
		return fmt.Errorf("deepgram tts stream context cancelled")
	case t.cmdChan <- ttsCmd{cmdType: "Speak", text: text}:
	default:
		log.Println("Warning: TTS command queue full, dropping token.")
	}
	return nil
}

// StartNewTurn re-enables Speak() after a barge-in clear.
// Must be called immediately before a new brain turn starts generating tokens.
func (t *DeepgramStreamTTS) StartNewTurn() {
	t.clearing.Store(false)
}

// Flush signals that text stream chunk is finalized
func (t *DeepgramStreamTTS) Flush() error {
	select {
	case <-t.ctx.Done():
		return fmt.Errorf("deepgram tts stream context cancelled")
	case t.cmdChan <- ttsCmd{cmdType: "Flush"}:
	default:
		log.Println("Warning: TTS command queue full, dropping Flush.")
	}
	return nil
}

// Clear sends a message to Deepgram to instantly wipe its TTS buffer
func (t *DeepgramStreamTTS) Clear() error {
	t.clearing.Store(true)

	// Immediately drain the command queue of any pending Speaks or Flushes
	drainingCmds := true
	for drainingCmds {
		select {
		case <-t.cmdChan:
		default:
			drainingCmds = false
		}
	}

	// Immediately drain any remaining unplayed frames from the audio channel
	drainingAudio := true
	for drainingAudio {
		select {
		case <-t.AudioChan:
		default:
			drainingAudio = false
		}
	}

	select {
	case <-t.ctx.Done():
		return fmt.Errorf("deepgram tts stream context cancelled")
	case t.cmdChan <- ttsCmd{cmdType: "Clear"}:
	default:
		log.Println("Warning: TTS command queue full, dropping Clear.")
	}
	return nil
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
