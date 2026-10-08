package lessons

import (
	"math"
	"testing"

	"github.com/Abhinav-Rao24/Zeplin/dsp"
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
