package transport

import "sync/atomic"

// AgentState represents the current conversational phase of the Zeplin agent.
type AgentState int32

const (
	// StateListening: the user is (potentially) speaking; local VAD is evaluating incoming audio.
	StateListening AgentState = iota
	// StateThinking: the Groq LLM stream is active and generating tokens.
	StateThinking
	// StateSpeaking: Deepgram TTS is actively sending audio frames downstream to LiveKit.
	StateSpeaking
)

// String returns a human-readable label for the state.
func (s AgentState) String() string {
	switch s {
	case StateListening:
		return "Listening"
	case StateThinking:
		return "Thinking"
	case StateSpeaking:
		return "Speaking"
	default:
		return "Unknown"
	}
}

// AgentStateMachine is a thread-safe, lock-free agent state machine backed by atomic.Int32.
// It replaces the pair of isBrainActive / isPacingActive atomic.Bool flags with a single
// explicitly-typed state so the entire system can reason about the agent's phase uniformly.
type AgentStateMachine struct {
	state atomic.Int32
}

// Set atomically transitions to the given state.
func (m *AgentStateMachine) Set(s AgentState) {
	m.state.Store(int32(s))
}

// Get returns the current agent state.
func (m *AgentStateMachine) Get() AgentState {
	return AgentState(m.state.Load())
}

// Is returns true if the current state matches s.
func (m *AgentStateMachine) Is(s AgentState) bool {
	return m.Get() == s
}
