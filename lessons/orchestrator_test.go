package lessons

import (
	"math"
	"testing"

	"github.com/Abhinav-Rao24/Zeplin/dsp"
	"github.com/Abhinav-Rao24/Zeplin/memory"
)

func TestChordLibrary(t *testing.T) {
	expectedChords := []string{
		"C_Major", "G_Major", "D_Major", "E_Minor",
		"A_Minor", "E_Major", "A_Major", "D_Minor",
	}

	for _, name := range expectedChords {
		def := ChordByName(name)
		if def == nil {
			t.Fatalf("Expected chord %q not found in library", name)
		}
		if len(def.PitchClasses) != 3 {
			t.Errorf("Expected 3 pitch classes for triad %q, got %d", name, len(def.PitchClasses))
		}
		if len(def.OpenStringFingering) != 6 {
			t.Errorf("Expected 6 strings in fingering for %q, got %d", name, len(def.OpenStringFingering))
		}
	}
}

func TestLessonSessionProgression(t *testing.T) {
	session := NewLessonSession("test-user", "Student")

	if session.CurrentStep.TargetChord != "E_Minor" {
		t.Fatalf("Expected initial target E_Minor, got %s", session.CurrentStep.TargetChord)
	}

	// Strum 3 clean times to advance
	streak := session.IncrStreak()
	if streak != 1 {
		t.Errorf("Expected streak 1, got %d", streak)
	}
	session.IncrStreak()
	streak = session.IncrStreak()
	if streak != 3 {
		t.Errorf("Expected streak 3, got %d", streak)
	}

	// Advance step
	advanced := session.AdvanceStep()
	if !advanced {
		t.Fatalf("Expected AdvanceStep to succeed")
	}

	if session.CurrentStep.TargetChord != "A_Minor" {
		t.Errorf("Expected step 2 to be A_Minor, got %s", session.CurrentStep.TargetChord)
	}
	if session.SuccessStreak != 0 {
		t.Errorf("Expected streak to reset on advance, got %d", session.SuccessStreak)
	}

	// Mistake resets streak
	session.IncrStreak()
	session.RecordMistake(MistakeWrongChord)
	session.ResetStreak()
	if session.SuccessStreak != 0 {
		t.Errorf("Expected streak 0 after reset, got %d", session.SuccessStreak)
	}
	if session.MistakeLog[MistakeWrongChord] != 1 {
		t.Errorf("Expected 1 mistake recorded, got %d", session.MistakeLog[MistakeWrongChord])
	}
}

func TestTransitionExercise(t *testing.T) {
	session := NewLessonSession("test-user", "Student")
	// Step 3 is Em -> Am transition
	session.setStep(2)

	if session.CurrentStep.Exercise != ExerciseTransition {
		t.Fatalf("Expected ExerciseTransition for step 3, got %s", session.CurrentStep.Exercise)
	}

	// Initially targeting first chord (Em)
	target1 := session.ActiveTarget()
	if target1 != "E_Minor" {
		t.Errorf("Expected first target E_Minor, got %s", target1)
	}

	// Flip transition
	session.FlipTransition()
	target2 := session.ActiveTarget()
	if target2 != "A_Minor" {
		t.Errorf("Expected flipped target A_Minor, got %s", target2)
	}
}

func TestTuningOffsetCalculation(t *testing.T) {
	const refE2 = 82.41

	// In tune
	offsetInTune := 1200 * math.Log2(refE2/refE2)
	if math.Abs(offsetInTune) > 0.001 {
		t.Errorf("Expected 0 offset for 82.41 Hz, got %f", offsetInTune)
	}

	// 10 cents flat: 82.41 * 2^(-10/1200) ≈ 81.93 Hz
	hzFlat := refE2 * math.Pow(2, -10.0/1200.0)
	offsetFlat := 1200 * math.Log2(hzFlat/refE2)
	if math.Abs(offsetFlat-(-10.0)) > 0.01 {
		t.Errorf("Expected -10 cents offset, got %f", offsetFlat)
	}
}

func TestSpeechGatingSuppressesChord(t *testing.T) {
	var published []dsp.UIStateUpdate
	orch := NewOrchestrator(nil, nil, func(u dsp.UIStateUpdate) {
		published = append(published, u)
	})

	sessionID := "student-1"
	sess := orch.GetOrCreateSession(sessionID)

	// Enable speech active
	orch.SetSpeechActive(sessionID, true)

	// Trigger chord event while speaking
	orch.HandleChordEvent(sessionID, dsp.ChordEvent{
		DetectedChord: "E_Minor",
		Confidence:    0.95,
	})

	if sess.SuccessStreak != 0 {
		t.Errorf("Expected chord event to be suppressed while speech is active, but streak became %d", sess.SuccessStreak)
	}
	if len(published) != 0 {
		t.Errorf("Expected no published state update while speech active, got %d", len(published))
	}

	// Disable speech active
	orch.SetSpeechActive(sessionID, false)

	// Trigger chord event when listening
	orch.HandleChordEvent(sessionID, dsp.ChordEvent{
		DetectedChord: "E_Minor",
		Confidence:    0.95,
	})

	if sess.SuccessStreak != 1 {
		t.Errorf("Expected streak to increment to 1 after speech inactive, got %d", sess.SuccessStreak)
	}
	if len(published) != 1 {
		t.Errorf("Expected 1 published state update, got %d", len(published))
	}
}

func TestStartSession(t *testing.T) {
	var published []dsp.UIStateUpdate
	orch := NewOrchestrator(nil, nil, func(u dsp.UIStateUpdate) {
		published = append(published, u)
	})

	orch.StartSession("student-2")

	if len(published) != 1 {
		t.Fatalf("Expected 1 UI state update from StartSession, got %d", len(published))
	}

	u := published[0]
	if u.TargetChord != "E_Minor" {
		t.Errorf("Expected initial target E_Minor, got %s", u.TargetChord)
	}
	// E minor has all 6 strings played (no muted strings)
	for i, s := range u.StringStatus {
		if s != "ok" {
			t.Errorf("Expected string %d to be ok for E_Minor, got %s", i, s)
		}
	}
}

type mockStore struct {
	savedStep   int
	savedStreak int
	mistakes    []string
	savedRecord bool
}

func (m *mockStore) SaveCurriculumProgress(studentID string, stepID, streak int, completedSteps []int) error {
	m.savedStep = stepID
	m.savedStreak = streak
	return nil
}

func (m *mockStore) GetCurriculumProgress(studentID string) (int, int, []int, error) {
	return 2, 1, []int{1}, nil // step 2 = A_Minor
}

func (m *mockStore) RecordMistake(studentID, chordName, mistakeType, details string) error {
	m.mistakes = append(m.mistakes, chordName+":"+mistakeType)
	return nil
}

func (m *mockStore) SaveSession(rec memory.SessionRecord) error {
	m.savedRecord = true
	return nil
}

func TestOrchestratorWithSQLiteStore(t *testing.T) {
	store := &mockStore{}
	orch := NewOrchestrator(nil, nil, func(u dsp.UIStateUpdate) {})
	orch.SetStore(store)

	// Resuming session should restore step 2 (A_Minor)
	sess := orch.GetOrCreateSession("student-3")
	if sess.CurrentStep.TargetChord != "A_Minor" {
		t.Errorf("Expected restored target A_Minor, got %s", sess.CurrentStep.TargetChord)
	}
	if sess.SuccessStreak != 1 {
		t.Errorf("Expected restored streak 1, got %d", sess.SuccessStreak)
	}

	// Mistake records into store
	orch.HandleChordEvent("student-3", dsp.ChordEvent{
		DetectedChord: "G_Major", // Wrong chord
		Confidence:    0.9,
	})
	if len(store.mistakes) != 1 {
		t.Errorf("Expected 1 mistake recorded in store, got %d", len(store.mistakes))
	}

	// Close session saves record
	orch.CloseSession("student-3")
	if !store.savedRecord {
		t.Errorf("Expected session record to be saved on CloseSession")
	}
}

func TestStrumBargeInAndAcousticGuard(t *testing.T) {
	orch := NewOrchestrator(nil, nil, func(u dsp.UIStateUpdate) {})
	var interruptedSession string
	orch.SetInterruptHandler(func(sessionID string) {
		interruptedSession = sessionID
	})

	// Case 1: Low-energy speaker bleed transient (< -28 dBFS) -> must NOT trigger barge-in
	interruptedSession = ""
	orch.HandleChordEvent("student-barge-in", dsp.ChordEvent{
		DetectedChord: "G_Major",
		Confidence:    0.85,
		RmsDBFS:       -34.0, // Speaker bleed level
	})
	if interruptedSession != "" {
		t.Errorf("Expected speaker bleed transient (-34 dBFS) to be suppressed from barge-in, but got %q", interruptedSession)
	}

	// Case 2: Genuine physical strum (>= -28 dBFS, confidence >= 0.70) -> MUST trigger barge-in
	interruptedSession = ""
	orch.HandleChordEvent("student-barge-in", dsp.ChordEvent{
		DetectedChord: "G_Major",
		Confidence:    0.92,
		RmsDBFS:       -18.5, // Physical guitar pick attack level
	})
	if interruptedSession != "student-barge-in" {
		t.Errorf("Expected genuine strum to trigger barge-in for student-barge-in, got %q", interruptedSession)
	}
}
