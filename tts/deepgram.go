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
	"time"

	"github.com/gorilla/websocket"
)

type ttsCmd struct {
	cmdType string // "Speak", "Flush", "Clear"
	text    string
}

// recentSpeechBuffer maintains a rolling window of recently spoken TTS tokens for echo detection.
// The transport layer compares incoming STT transcripts against this buffer to identify
// acoustic echo that slipped through the browser's AEC hardware filters.
type recentSpeechBuffer struct {
	mu   sync.Mutex
	text string
}

func (r *recentSpeechBuffer) record(token string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.text += token
	// Rolling window: keep the last 600 characters of spoken text.
	if len(r.text) > 600 {
		r.text = r.text[len(r.text)-600:]
	}
}

func (r *recentSpeechBuffer) get() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.text
}

func (r *recentSpeechBuffer) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.text = ""
}

// DeepgramStreamTTS coordinates text streaming to Deepgram's streaming TTS WebSocket.
// It implements automatic reconnection so that a dropped WebSocket does not bring down
// the LiveKit room session or crash the main application loop.
type DeepgramStreamTTS struct {
	conn         *websocket.Conn
	ctx          context.Context
	cancel       context.CancelFunc
	AudioChan    <-chan []byte  // Receive-only view exposed to the transport layer
	audioChanRW  chan []byte    // Bidirectional reference kept internally for draining in Clear()
	mu           sync.Mutex
	clearing     atomic.Bool
	reconnecting atomic.Bool
	cmdChan      chan ttsCmd
	wsURL        string
	apiKey       string
	recentText   recentSpeechBuffer
	OnAudioFrame func()
}

// NewDeepgramStreamTTS instantiates a Gorilla WebSocket client to Deepgram TTS.
func NewDeepgramStreamTTS(apiKey string) (*DeepgramStreamTTS, error) {
	os.Setenv("DEEPGRAM_API_KEY", apiKey)

	ctx, cancel := context.WithCancel(context.Background())

	wsURL := "wss://api.deepgram.com/v1/speak?model=aura-2-thalia-en&encoding=mulaw&sample_rate=8000"

	audioChan := make(chan []byte, 1000)
	cmdChan := make(chan ttsCmd, 1000)

	t := &DeepgramStreamTTS{
		ctx:         ctx,
		cancel:      cancel,
		AudioChan:   audioChan,
		audioChanRW: audioChan,
		cmdChan:     cmdChan,
		wsURL:       wsURL,
		apiKey:      apiKey,
	}

	conn, err := t.dial()
	if err != nil {
		cancel()
		return nil, err
	}
	t.conn = conn

	// Start the reconnect-aware read loop and the command write loop.
	go t.readLoop(audioChan)
	go t.writeLoop()

	return t, nil
}

// dial opens a fresh WebSocket connection to Deepgram TTS.
func (t *DeepgramStreamTTS) dial() (*websocket.Conn, error) {
	headers := http.Header{}
	headers.Add("Authorization", "Token "+t.apiKey)

	log.Println("Dialing Deepgram TTS WebSocket...")
	conn, resp, err := websocket.DefaultDialer.Dial(t.wsURL, headers)
	if err != nil {
		if resp != nil {
			return nil, fmt.Errorf("failed to dial Deepgram TTS WS: %w (status: %s)", err, resp.Status)
		}
		return nil, fmt.Errorf("failed to dial Deepgram TTS WS: %w", err)
	}
	log.Println("Deepgram TTS WebSocket connection established.")
	return conn, nil
}

// reconnect attempts to re-establish the WebSocket connection with exponential backoff.
// Returns true if reconnection succeeded, false if the context was cancelled or retries exhausted.
// The audioChan is never closed during reconnect — the LiveKit pacing loop continues unaffected.
func (t *DeepgramStreamTTS) reconnect() bool {
	t.reconnecting.Store(true)
	defer t.reconnecting.Store(false)

	backoff := 2 * time.Second
	const maxAttempts = 5

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-t.ctx.Done():
			return false
		case <-time.After(backoff):
		}

		log.Printf("[TTS] Reconnect attempt %d/%d...", attempt, maxAttempts)
		conn, err := t.dial()
		if err != nil {
			log.Printf("[TTS] Reconnect attempt %d failed: %v", attempt, err)
			backoff *= 2
			continue
		}

		// Swap the connection under the write mutex so writeLoop picks it up cleanly.
		t.mu.Lock()
		old := t.conn
		t.conn = conn
		t.mu.Unlock()

		if old != nil {
			old.Close()
		}
		return true
	}

	log.Println("[TTS] All reconnect attempts exhausted. TTS unavailable until restart.")
	return false
}

// readLoop is the outer reconnect-aware read driver.
// audioChan is only closed when the context is explicitly cancelled via Close().
func (t *DeepgramStreamTTS) readLoop(audioChan chan<- []byte) {
	defer func() {
		close(audioChan)
		log.Println("Deepgram TTS read loop terminated.")
	}()

	for {
		if t.ctx.Err() != nil {
			return
		}
		if err := t.runOneShotRead(audioChan); err != nil {
			if t.ctx.Err() != nil {
				return
			}
			log.Printf("[TTS] WebSocket read error: %v. Initiating reconnect...", err)
			if !t.reconnect() {
				return
			}
			log.Println("[TTS] Reconnected to Deepgram TTS WebSocket.")
		}
	}
}

// runOneShotRead reads messages from the current conn until the connection errors.
// It returns the error so the outer readLoop can decide whether to reconnect.
func (t *DeepgramStreamTTS) runOneShotRead(audioChan chan<- []byte) error {
	for {
		t.mu.Lock()
		conn := t.conn
		t.mu.Unlock()

		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		select {
		case <-t.ctx.Done():
			return nil
		default:
		}

		switch messageType {
		case websocket.BinaryMessage:
			if t.clearing.Load() {
				continue // Drop straggling audio frames from the previous interrupted generation.
			}
			if t.OnAudioFrame != nil {
				t.OnAudioFrame()
			}
			select {
			case audioChan <- message:
			default:
				log.Println("Warning: Outbound audio buffer full, dropping PCMU frame.")
			}
		case websocket.TextMessage:
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
			// Skip writes during the reconnect window to prevent writing to a dead connection.
			if t.reconnecting.Load() {
				continue
			}

			var payload map[string]string
			switch cmd.cmdType {
			case "Speak":
				payload = map[string]string{"type": "Speak", "text": cmd.text}
			case "Flush":
				payload = map[string]string{"type": "Flush"}
			case "Clear":
				payload = map[string]string{"type": "Clear"}
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
				// Log and continue — readLoop will detect the broken connection and reconnect.
				log.Printf("Error writing TTS command to WS: %v", err)
			}
		}
	}
}

// Speak sends a text token chunk to the TTS stream.
// It silently drops the token if a barge-in clear is in progress, and records
// the spoken text in the rolling echo-detection buffer.
func (t *DeepgramStreamTTS) Speak(text string) error {
	if t.clearing.Load() {
		return nil
	}

	// Record in the echo buffer so the STT callback can detect acoustic feedback.
	t.recentText.record(text)

	select {
	case <-t.ctx.Done():
		return fmt.Errorf("deepgram tts stream context cancelled")
	case t.cmdChan <- ttsCmd{cmdType: "Speak", text: text}:
	default:
		log.Println("Warning: TTS command queue full, dropping token.")
	}
	return nil
}

// RecentSpokenText returns the last ~600 characters of text sent to Deepgram TTS.
// The STT echo guard in transport/livekit.go uses this to detect and discard acoustic
// echo transcripts that the browser's AEC hardware filter did not fully suppress.
func (t *DeepgramStreamTTS) RecentSpokenText() string {
	return t.recentText.get()
}

// StartNewTurn re-enables Speak() after a barge-in clear and resets the echo buffer.
// Must be called immediately before a new brain turn begins generating tokens.
func (t *DeepgramStreamTTS) StartNewTurn() {
	t.clearing.Store(false)
	t.recentText.reset()
}

// Flush signals to Deepgram that the current text chunk is complete.
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

// Clear instantly wipes the TTS pipeline: drains pending commands and audio frames,
// then sends the Clear signal to Deepgram.
func (t *DeepgramStreamTTS) Clear() error {
	t.clearing.Store(true)

	// Drain pending Speak/Flush commands.
	for {
		select {
		case <-t.cmdChan:
		default:
			goto drainedCmds
		}
	}
drainedCmds:

	// Drain buffered audio frames that have not yet been played.
	for {
		select {
		case <-t.audioChanRW:
		default:
			goto drainedAudio
		}
	}
drainedAudio:

	select {
	case <-t.ctx.Done():
		return fmt.Errorf("deepgram tts stream context cancelled")
	case t.cmdChan <- ttsCmd{cmdType: "Clear"}:
	default:
		log.Println("Warning: TTS command queue full, dropping Clear.")
	}
	return nil
}

// Close terminates the WebSocket connection cleanly.
func (t *DeepgramStreamTTS) Close() error {
	t.cancel()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	return t.conn.Close()
}
