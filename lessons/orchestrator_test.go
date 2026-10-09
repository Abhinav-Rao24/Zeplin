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

func TestStrumTelemetryWindowAndMutedDetection(t *testing.T) {
	orch := NewOrchestrator(nil, nil, func(u dsp.UIStateUpdate) {})
	sessionID := "student-telemetry"
	sess := orch.GetOrCreateSession(sessionID)

	// Step 1: Default target is E_Minor
	// Strum 1: Clean E_Minor
	orch.HandleChordEvent(sessionID, dsp.ChordEvent{
		DetectedChord: "E_Minor",
		Confidence:    0.95,
		BassNote:      "E2",
	})

	// Strum 2: Low confidence E_Minor (triggers muted B string index 4 detection)
	orch.HandleChordEvent(sessionID, dsp.ChordEvent{
		DetectedChord: "E_Minor",
		Confidence:    0.72,
		BassNote:      "E2",
	})

	// Strum 3: Wrong chord (sounded like G_Major)
	orch.HandleChordEvent(sessionID, dsp.ChordEvent{
		DetectedChord: "G_Major",
		Confidence:    0.88,
		BassNote:      "G2",
	})

	// Strum 4: Clean E_Minor
	orch.HandleChordEvent(sessionID, dsp.ChordEvent{
		DetectedChord: "E_Minor",
		Confidence:    0.91,
		BassNote:      "E2",
	})

	// Strum 5: Inversion (wrong bass A2)
	orch.HandleChordEvent(sessionID, dsp.ChordEvent{
		DetectedChord: "E_Minor",
		Confidence:    0.85,
		Inversion:     true,
		BassNote:      "A2",
	})

	if len(sess.RecentStrums) != 5 {
		t.Fatalf("Expected 5 strums in rolling window, got %d", len(sess.RecentStrums))
	}

	// Strum 6: Another clean E_Minor -> rolling window should remain capped at 5
	orch.HandleChordEvent(sessionID, dsp.ChordEvent{
		DetectedChord: "E_Minor",
		Confidence:    0.96,
		BassNote:      "E2",
	})

	if len(sess.RecentStrums) != 5 {
		t.Fatalf("Expected rolling window to cap at 5, got %d", len(sess.RecentStrums))
	}

	summary := sess.FormatRecentTelemetry()
	if summary == "" {
		t.Fatalf("Expected non-empty telemetry summary")
	}

	// Verify the summary contains the telemetry details
	if !containsStr(summary, "Muted string(s): B") && !containsStr(summary, "Wrong chord") {
		t.Errorf("Telemetry summary missing expected mistake context:\n%s", summary)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})())
}

// TestMutedStringPhysicalDiagnosis verifies Validation Protocol V4:
// When a string is muted or buzzing during an otherwise correct shape,
// the DSP registers it, updates the fretboard with a muted indicator on that string,
// and gives an explicit physical instruction.
func TestMutedStringPhysicalDiagnosis(t *testing.T) {
	var lastUpdate dsp.UIStateUpdate
	orch := NewOrchestrator(nil, nil, func(u dsp.UIStateUpdate) {
		lastUpdate = u
	})

	sessionID := "student-muted-test"
	sess := orch.GetOrCreateSession(sessionID)
	orch.SetTargetChordQuiet(sessionID, "G_Major")

	// Strum G Major with muted B string (confidence 0.74 < 0.78 threshold)
	orch.HandleChordEvent(sessionID, dsp.ChordEvent{
		DetectedChord: "G_Major",
		Confidence:    0.74,
		BassNote:      "G2",
	})

	if sess.MistakeLog[MistakeMutedString] != 1 {
		t.Fatalf("Expected 1 muted string mistake recorded, got %d", sess.MistakeLog[MistakeMutedString])
	}

	// Verify UI state has 'muted' on index 4 (B string)
	if len(lastUpdate.StringStatus) != 6 {
		t.Fatalf("Expected 6 strings in StringStatus, got %d", len(lastUpdate.StringStatus))
	}
	if lastUpdate.StringStatus[4] != "muted" {
		t.Errorf("Expected StringStatus[4] (B string) to be 'muted', got %q", lastUpdate.StringStatus[4])
	}

	// Verify explicit physical feedback
	expectedMsg := "Your B string is muted. Arch your fretting finger."
	if lastUpdate.FeedbackText != expectedMsg {
		t.Errorf("Expected feedback %q, got %q", expectedMsg, lastUpdate.FeedbackText)
	}
}

// TestChordProgressionAndMultimodalSync verifies Validation Protocol V5:
// Completing the target repetitions smoothly advances to the next chord,
// updating UI state simultaneously without requiring a page reload.
func TestChordProgressionAndMultimodalSync(t *testing.T) {
	var updates []dsp.UIStateUpdate
	orch := NewOrchestrator(nil, nil, func(u dsp.UIStateUpdate) {
		updates = append(updates, u)
	})

	sessionID := "student-progression"
	sess := orch.GetOrCreateSession(sessionID)

	// Step 1: E_Minor requires 3 reps
	if sess.CurrentStep.TargetChord != "E_Minor" {
		t.Fatalf("Initial target chord should be E_Minor, got %s", sess.CurrentStep.TargetChord)
	}

	for i := 0; i < 3; i++ {
		orch.HandleChordEvent(sessionID, dsp.ChordEvent{
			DetectedChord: "E_Minor",
			Confidence:    0.95,
			BassNote:      "E2",
		})
	}

	// Should advance to step 2: A_Minor
	if sess.CurrentStep.TargetChord != "A_Minor" {
		t.Errorf("Expected target chord to advance to A_Minor, got %s", sess.CurrentStep.TargetChord)
	}
	if sess.SuccessStreak != 0 {
		t.Errorf("Expected streak to reset to 0 upon step advance, got %d", sess.SuccessStreak)
	}

	// Verify that the final update emitted announces the next chord
	if len(updates) == 0 {
		t.Fatalf("Expected UI state updates during chord progression")
	}
	last := updates[len(updates)-1]
	if last.TargetChord != "A_Minor" {
		t.Errorf("Expected last UI update TargetChord to be A_Minor, got %s", last.TargetChord)
	}
}

// TestCumulativePracticeSummaryAndRetrospective verifies practice analytics
// tracking accuracy and formatting detailed statistics for student queries.
func TestCumulativePracticeSummaryAndRetrospective(t *testing.T) {
	orch := NewOrchestrator(nil, nil, func(u dsp.UIStateUpdate) {})
	sessionID := "student-summary-test"
	sess := orch.GetOrCreateSession(sessionID)

	// 1. Strum 3 clean times on E_Minor (advances to A_Minor)
	for i := 0; i < 3; i++ {
		orch.HandleChordEvent(sessionID, dsp.ChordEvent{
			DetectedChord: "E_Minor",
			Confidence:    0.94,
			BassNote:      "E2",
		})
	}
	// 2. Strum 3 clean times on A_Minor (advances to Step 3: Em -> Am transition)
	for i := 0; i < 3; i++ {
		orch.HandleChordEvent(sessionID, dsp.ChordEvent{
			DetectedChord: "A_Minor",
			Confidence:    0.94,
			BassNote:      "A2",
		})
	}
	// 3. Strum 1 clean time on first half of transition (E_Minor)
	orch.HandleChordEvent(sessionID, dsp.ChordEvent{
		DetectedChord: "E_Minor",
		Confidence:    0.94,
		BassNote:      "E2",
	})
	// Total clean so far: 7. Now simulate 2 muted strums and 1 wrong chord
	for i := 0; i < 2; i++ {
		orch.HandleChordEvent(sessionID, dsp.ChordEvent{
			DetectedChord: "A_Minor",
			Confidence:    0.73, // Muted string issue
			BassNote:      "A2",
		})
	}
	orch.HandleChordEvent(sessionID, dsp.ChordEvent{
		DetectedChord: "G_Major", // Wrong chord
		Confidence:    0.89,
		BassNote:      "G2",
	})

	if sess.TotalStrums != 10 {
		t.Fatalf("Expected 10 total strums, got %d", sess.TotalStrums)
	}
	if sess.TotalCorrect != 7 {
		t.Fatalf("Expected 7 correct strums, got %d", sess.TotalCorrect)
	}

	summary := sess.FormatPracticeSummary()
	if !containsStr(summary, "Total Strums: 10") {
		t.Errorf("Summary missing total strums: %s", summary)
	}
	if !containsStr(summary, "Clean Strums: 7") {
		t.Errorf("Summary missing clean strums: %s", summary)
	}
	if !containsStr(summary, "70% accuracy") {
		t.Errorf("Summary missing accuracy calculation: %s", summary)
	}
}


