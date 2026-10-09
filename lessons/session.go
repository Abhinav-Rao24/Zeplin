package lessons

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Exercise defines the type of practice currently assigned.
type Exercise string

const (
	// ExerciseHoldChord asks the student to hold a single chord cleanly for N strums.
	ExerciseHoldChord Exercise = "hold_chord"
	// ExerciseTransition asks the student to alternate between two chords.
	ExerciseTransition Exercise = "chord_transition"
)

// LessonStep is one entry in the curriculum sequence.
type LessonStep struct {
	ID             int
	Name           string
	Exercise       Exercise
	TargetChord    string   // primary chord (or first chord in a transition)
	SecondChord    string   // second chord for ExerciseTransition; empty otherwise
	RepsToAdvance  int      // consecutive clean strums needed to advance
}

// V1Curriculum defines the beginner lesson progression.
var V1Curriculum = []LessonStep{
	{ID: 1, Name: "E Minor — Introduction",    Exercise: ExerciseHoldChord,    TargetChord: "E_Minor", RepsToAdvance: 3},
	{ID: 2, Name: "A Minor — Introduction",    Exercise: ExerciseHoldChord,    TargetChord: "A_Minor", RepsToAdvance: 3},
	{ID: 3, Name: "Em → Am Transition",         Exercise: ExerciseTransition,   TargetChord: "E_Minor", SecondChord: "A_Minor", RepsToAdvance: 4},
	{ID: 4, Name: "G Major — Introduction",     Exercise: ExerciseHoldChord,    TargetChord: "G_Major", RepsToAdvance: 3},
	{ID: 5, Name: "C Major — Introduction",     Exercise: ExerciseHoldChord,    TargetChord: "C_Major", RepsToAdvance: 5},
	{ID: 6, Name: "D Major — Introduction",     Exercise: ExerciseHoldChord,    TargetChord: "D_Major", RepsToAdvance: 3},
	{ID: 7, Name: "E Major — Introduction",     Exercise: ExerciseHoldChord,    TargetChord: "E_Major", RepsToAdvance: 3},
	{ID: 8, Name: "A Major — Introduction",     Exercise: ExerciseHoldChord,    TargetChord: "A_Major", RepsToAdvance: 3},
	{ID: 9, Name: "D Minor — Introduction",     Exercise: ExerciseHoldChord,    TargetChord: "D_Minor", RepsToAdvance: 3},
	{ID: 10, Name: "G → C → D Progression",    Exercise: ExerciseTransition,   TargetChord: "G_Major", SecondChord: "C_Major", RepsToAdvance: 5},
}

// MistakeType enumerates the categories of chord errors.
type MistakeType string

const (
	MistakeWrongChord  MistakeType = "wrong_chord"
	MistakeMutedString MistakeType = "muted_string"
	MistakeInversion   MistakeType = "wrong_bass"
	MistakeLowEnergy   MistakeType = "low_energy"
)

// StrumRecord represents a single recorded strum's telemetry.
type StrumRecord struct {
	Timestamp     time.Time
	DetectedChord string
	TargetChord   string
	Confidence    float64
	Inversion     bool
	BassNote      string
	MutedStrings  []string
	IsCorrect     bool
}

// LessonSession holds all runtime state for a single learning session.
// It is NOT concurrency-safe on its own; the Orchestrator holds the mutex.
type LessonSession struct {
	SessionID    string
	StudentName  string
	StartedAt    time.Time
	lastGreetingAt time.Time

	// Curriculum progress
	CurriculumIdx int        // index into V1Curriculum
	CurrentStep   LessonStep
	SuccessStreak int        // consecutive clean strums on current chord
	TotalStrums   int
	TotalCorrect  int

	// Transition state: which chord the student should be on next
	TransitionTurn int // 0 = TargetChord, 1 = SecondChord

	// Mistake frequency map for personalized coaching
	MistakeLog map[MistakeType]int

	// Suppression: set true when student is speaking (VAD active)
	SpeechActive bool

	// RecentStrums holds the rolling window of the last 5 strums for DSP-grounded tutoring
	RecentStrums []StrumRecord

	// TuningOffsetCents is the measured offset at session start (±cents from A=440)
	TuningOffsetCents float64
	TuningCalibrated  bool

	mu sync.Mutex
}

// NewLessonSession creates a session starting at the first curriculum step.
func NewLessonSession(sessionID, studentName string) *LessonSession {
	s := &LessonSession{
		SessionID:    sessionID,
		StudentName:  studentName,
		StartedAt:    time.Now(),
		MistakeLog:   make(map[MistakeType]int),
		RecentStrums: make([]StrumRecord, 0, 5),
	}
	s.setStep(0)
	return s
}

// setStep sets the current lesson step by curriculum index (caller must hold lock).
func (s *LessonSession) setStep(idx int) {
	if idx >= len(V1Curriculum) {
		idx = len(V1Curriculum) - 1 // clamp at final step
	}
	s.CurriculumIdx = idx
	s.CurrentStep = V1Curriculum[idx]
	s.SuccessStreak = 0
	s.TransitionTurn = 0
}

// AdvanceStep moves to the next curriculum step. Returns true if advanced,
// false if already at the final step.
func (s *LessonSession) AdvanceStep() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.CurriculumIdx >= len(V1Curriculum)-1 {
		return false
	}
	s.setStep(s.CurriculumIdx + 1)
	return true
}

// RecordMistake increments the frequency counter for a mistake type.
func (s *LessonSession) RecordMistake(m MistakeType) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MistakeLog[m]++
}

// ActiveTarget returns the chord the student should be playing right now,
// taking into account which side of a chord transition they're on.
func (s *LessonSession) ActiveTarget() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	step := s.CurrentStep
	if step.Exercise == ExerciseTransition && s.TransitionTurn == 1 {
		return step.SecondChord
	}
	return step.TargetChord
}

// FlipTransition switches the expected chord in a transition exercise.
func (s *LessonSession) FlipTransition() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.CurrentStep.Exercise == ExerciseTransition {
		s.TransitionTurn = 1 - s.TransitionTurn
	}
}

// IncrStreak increments and returns the current success streak.
func (s *LessonSession) IncrStreak() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SuccessStreak++
	s.TotalStrums++
	s.TotalCorrect++
	return s.SuccessStreak
}

// ResetStreak resets the success streak (on mistake) and increments TotalStrums.
func (s *LessonSession) ResetStreak() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SuccessStreak = 0
	s.TotalStrums++
}

// RecordStrum appends a strum to the rolling 5-strum telemetry window.
func (s *LessonSession) RecordStrum(rec StrumRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RecentStrums = append(s.RecentStrums, rec)
	if len(s.RecentStrums) > 5 {
		s.RecentStrums = s.RecentStrums[len(s.RecentStrums)-5:]
	}
}

// FormatRecentTelemetry formats the last 5 strums into a concise summary for LLM context.
func (s *LessonSession) FormatRecentTelemetry() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.RecentStrums) == 0 {
		return "No recent strums recorded."
	}
	var lines []string
	for i, r := range s.RecentStrums {
		status := "Clean"
		if !r.IsCorrect {
			if r.DetectedChord != r.TargetChord {
				status = "Wrong chord (" + r.DetectedChord + ")"
			} else if r.Inversion {
				status = "Wrong bass (" + r.BassNote + " ringing)"
			} else if len(r.MutedStrings) > 0 {
				status = "Muted string(s): " + strings.Join(r.MutedStrings, ", ")
			}
		}
		lines = append(lines, fmt.Sprintf("- Strum %d: Target %s -> Detected %s (Confidence: %.0f%%, Status: %s)",
			i+1, r.TargetChord, r.DetectedChord, r.Confidence*100, status))
	}
	return strings.Join(lines, "\n")
}

// FormatPracticeSummary formats overall session statistics for retrospective queries.
func (s *LessonSession) FormatPracticeSummary() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	dur := time.Since(s.StartedAt).Round(time.Second)
	accuracy := 0.0
	if s.TotalStrums > 0 {
		accuracy = float64(s.TotalCorrect) / float64(s.TotalStrums) * 100.0
	}
	return fmt.Sprintf("Duration: %s | Total Strums: %d | Clean Strums: %d (%.0f%% accuracy)",
		dur, s.TotalStrums, s.TotalCorrect, accuracy)
}


