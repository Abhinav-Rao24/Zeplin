package transport

import (
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"
)

// SessionTelemetry tracks metrics for a single conversation session.
// It is thread-safe and uses an online running accumulator to avoid dynamic slice allocations.
type SessionTelemetry struct {
	mu sync.Mutex

	sessionID string

	// Timestamps
	thinkingStart    time.Time
	firstTokenTime   time.Time
	firstAudioTime   time.Time
	lastOutboundPush time.Time

	// Latencies
	ttft time.Duration
	ttfa time.Duration

	// Flags for the current turn
	thinkingStarted    bool
	firstTokenReceived bool
	firstAudioReceived bool

	// Online running accumulator for pacing jitter (against 20.0ms baseline) using Welford's algorithm
	frameCount     int64
	meanInterval   float64 // Running mean interval (ms)
	m2             float64 // Running sum of squared differences from the mean (Welford)
	sumOfIntervals float64 // in milliseconds
	sumOfSquares   float64 // sum((interval_ms - 20.0)^2)
}

// StartThinking is called when the state machine transitions to StateThinking.
func (s *SessionTelemetry) StartThinking() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.thinkingStart = time.Now()
	s.thinkingStarted = true
	s.firstTokenReceived = false
	s.firstAudioReceived = false
	s.firstTokenTime = time.Time{}
	s.firstAudioTime = time.Time{}
	s.lastOutboundPush = time.Time{}

	s.frameCount = 0
	s.meanInterval = 0.0
	s.m2 = 0.0
	s.sumOfIntervals = 0.0
	s.sumOfSquares = 0.0
}

// RecordFirstToken is called when the first text token is read from the Groq stream.
func (s *SessionTelemetry) RecordFirstToken() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.thinkingStarted || s.firstTokenReceived {
		return
	}
	s.firstTokenTime = time.Now()
	s.firstTokenReceived = true
	s.ttft = s.firstTokenTime.Sub(s.thinkingStart)
}

// RecordFirstAudioFrame is called when the Deepgram WebSocket reader receives the first binary frame.
func (s *SessionTelemetry) RecordFirstAudioFrame() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.firstTokenReceived || s.firstAudioReceived {
		return
	}
	s.firstAudioTime = time.Now()
	s.firstAudioReceived = true
	s.ttfa = s.firstAudioTime.Sub(s.firstTokenTime)
}

// RecordOutboundPush records an outbound frame push inside the LiveKit pacing loop.
func (s *SessionTelemetry) RecordOutboundPush() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.lastOutboundPush.IsZero() {
		interval := now.Sub(s.lastOutboundPush)
		intervalMs := float64(interval) / float64(time.Millisecond)

		s.frameCount++
		s.sumOfIntervals += intervalMs
		diff := intervalMs - 20.0
		s.sumOfSquares += diff * diff

		// Welford's algorithm: online mean and squared difference update
		delta := intervalMs - s.meanInterval
		s.meanInterval += delta / float64(s.frameCount)
		delta2 := intervalMs - s.meanInterval
		s.m2 += delta * delta2
	}
	s.lastOutboundPush = now
}

// CompileAndReport compiles statistics and reports them to both slog and a scannable console card.
func (s *SessionTelemetry) CompileAndReport() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var avgInterval float64
	var jitterVariance float64
	var jitterStdDev float64

	n := s.frameCount
	if n > 0 {
		avgInterval = s.sumOfIntervals / float64(n)
		jitterVariance = s.sumOfSquares / float64(n)
		jitterStdDev = math.Sqrt(jitterVariance)
	}

	// Output structured slog logs
	slog.Info("Conversation Turn Performance Report",
		slog.String("session_id", s.sessionID),
		slog.Group("network_transit_durations",
			slog.Duration("ttft_groq_inference", s.ttft),
			slog.Duration("ttfa_deepgram_tts", s.ttfa),
			slog.String("note", "Includes network round-trip and remote model execution"),
		),
		slog.Group("local_processing_and_pacing",
			slog.Int64("frames_pushed", n),
			slog.Float64("pacing_jitter_std_dev_ms", jitterStdDev),
			slog.Float64("pacing_jitter_variance_ms2", jitterVariance),
			slog.Float64("avg_pacing_interval_ms", avgInterval),
			slog.String("note", "Measures client loop pacing stability against 20ms baseline"),
		),
	)

	// Output beautiful scannable console card
	fmt.Printf(`
┌────────────────────────────────────────────────────────┐
│             ZEPLIN PERFORMANCE OBSERVABILITY           │
├────────────────────────────────────────────────────────┤
│ SESSION ID: %-42s │
├────────────────────────────────────────────────────────┤
│ NETWORK TRANSIT & REMOTE LATENCY                       │
│   • Time-to-First-Token (TTFT):   %8.2f ms             │
│   • Time-to-First-Audio (TTFA):   %8.2f ms             │
├────────────────────────────────────────────────────────┤
│ LOCAL PACING STABILITY (20ms Baseline)                 │
│   • Total Frames Pushed:          %8d             │
│   • Avg Pacing Interval:          %8.2f ms             │
│   • Jitter Std Deviation:         %8.2f ms             │
│   • Jitter Variance:              %8.2f ms²            │
└────────────────────────────────────────────────────────┘
`,
		s.sessionID,
		float64(s.ttft)/float64(time.Millisecond),
		float64(s.ttfa)/float64(time.Millisecond),
		n,
		avgInterval,
		jitterStdDev,
		jitterVariance,
	)
}

// TelemetryRegistry is a thread-safe registry of SessionTelemetry trackers.
type TelemetryRegistry struct {
	mu              sync.RWMutex
	sessions        map[string]*SessionTelemetry
	activeSessionID string
}

// NewTelemetryRegistry creates a new TelemetryRegistry.
func NewTelemetryRegistry() *TelemetryRegistry {
	return &TelemetryRegistry{
		sessions: make(map[string]*SessionTelemetry),
	}
}

// Get retrieves or creates the SessionTelemetry tracker for the session ID.
func (r *TelemetryRegistry) Get(sessionID string) *SessionTelemetry {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, exists := r.sessions[sessionID]
	if !exists {
		s = &SessionTelemetry{sessionID: sessionID}
		r.sessions[sessionID] = s
	}
	return s
}

// SetActiveSession registers the currently active session ID.
func (r *TelemetryRegistry) SetActiveSession(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activeSessionID = sessionID
}

// GetActiveSession returns the active session ID.
func (r *TelemetryRegistry) GetActiveSession() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.activeSessionID
}

// GetActiveTelemetry returns the SessionTelemetry for the currently active session.
func (r *TelemetryRegistry) GetActiveTelemetry() *SessionTelemetry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.activeSessionID == "" {
		return nil
	}
	return r.sessions[r.activeSessionID]
}


// RecordFirstAudioFrame records the first audio frame for whichever session is currently generating.
func (r *TelemetryRegistry) RecordFirstAudioFrame() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.sessions {
		s.mu.Lock()
		if s.firstTokenReceived && !s.firstAudioReceived {
			s.firstAudioTime = time.Now()
			s.firstAudioReceived = true
			s.ttfa = s.firstAudioTime.Sub(s.firstTokenTime)
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
	}
}
