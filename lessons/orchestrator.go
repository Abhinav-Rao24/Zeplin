package lessons

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/Abhinav-Rao24/Zeplin/brain"
	"github.com/Abhinav-Rao24/Zeplin/dsp"
	"github.com/Abhinav-Rao24/Zeplin/memory"
	"github.com/Abhinav-Rao24/Zeplin/tts"
)

// Store defines persistence operations needed by the Orchestrator.
type Store interface {
	SaveCurriculumProgress(studentID string, stepID, streak int, completedSteps []int) error
	GetCurriculumProgress(studentID string) (stepID, streak int, completedSteps []int, err error)
	RecordMistake(studentID, chordName, mistakeType, details string) error
	SaveSession(rec memory.SessionRecord) error
}

// FeedbackPublisher is a function that sends a UI state update to the
// browser client over the LiveKit DataChannel. Injected at construction.
type FeedbackPublisher func(update dsp.UIStateUpdate)

// Orchestrator is the central pedagogical coordinator. It receives two
// types of input:
//   - HandleChordEvent: structured DSP telemetry from the browser AudioWorklet
//   - HandleSpeech: finalized STT transcripts from the student
//
// It emits spoken feedback via the TTS engine and visual state via the
// DataChannel publisher. The architecture is deliberately rule-based for
// chord evaluation (sub-100ms response, no LLM call per strum) and
// LLM-backed only for conversational student questions.
type Orchestrator struct {
	brain     *brain.Brain
	tts       *tts.DeepgramStreamTTS
	publisher FeedbackPublisher
	store     Store

	mu       sync.Mutex
	sessions map[string]*LessonSession // keyed by participant identity

	// Feedback debounce: avoids repeating the same line on every strum
	lastFeedbackAt   time.Time
	lastFeedbackText string
	feedbackCooldown time.Duration

	// OnInterrupt is invoked when a genuine physical strum arrives while the agent is speaking
	OnInterrupt func(sessionID string)
}

// SetInterruptHandler wires the barge-in interruption handler.
func (o *Orchestrator) SetInterruptHandler(fn func(sessionID string)) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.OnInterrupt = fn
}

// TriggerInterrupt invokes the registered interrupt handler.
func (o *Orchestrator) TriggerInterrupt(sessionID string) {
	o.mu.Lock()
	fn := o.OnInterrupt
	o.mu.Unlock()
	if fn != nil {
		fn(sessionID)
	}
}

// NewOrchestrator creates a new Orchestrator.
func NewOrchestrator(b *brain.Brain, t *tts.DeepgramStreamTTS, pub FeedbackPublisher) *Orchestrator {
	return &Orchestrator{
		brain:            b,
		tts:              t,
		publisher:        pub,
		sessions:         make(map[string]*LessonSession),
		feedbackCooldown: 1500 * time.Millisecond,
	}
}

// SetPublisher injects or updates the FeedbackPublisher after room initialization.
func (o *Orchestrator) SetPublisher(pub FeedbackPublisher) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.publisher = pub
}

// SetStore injects a persistent SQLite store into the Orchestrator.
func (o *Orchestrator) SetStore(s Store) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.store = s
}

// GetOrCreateSession returns the LessonSession for a participant, creating
// one on first contact. If a store is available, it restores previous curriculum progress.
func (o *Orchestrator) GetOrCreateSession(sessionID string) *LessonSession {
	o.mu.Lock()
	defer o.mu.Unlock()
	if sess, ok := o.sessions[sessionID]; ok {
		return sess
	}
	sess := NewLessonSession(sessionID, sessionID)
	if o.store != nil {
		stepID, streak, _, err := o.store.GetCurriculumProgress(sessionID)
		if err == nil && stepID > 0 {
			sess.setStep(stepID - 1)
			sess.SuccessStreak = streak
			log.Printf("[Orchestrator] Resumed curriculum at step %d (streak %d) for %s", stepID, streak, sessionID)
		}
	}
	o.sessions[sessionID] = sess
	log.Printf("[Orchestrator] New lesson session started for participant: %s", sessionID)
	return sess
}

// StartSession initializes the lesson for a participant, publishes initial state,
// and gives the introductory verbal prompt.
func (o *Orchestrator) StartSession(sessionID string) {
	sess := o.GetOrCreateSession(sessionID)

	o.mu.Lock()
	if !sess.lastGreetingAt.IsZero() && time.Since(sess.lastGreetingAt) < 3*time.Second {
		o.mu.Unlock()
		return
	}
	sess.lastGreetingAt = time.Now()
	o.mu.Unlock()

	target := sess.ActiveTarget()
	targetDef := ChordByName(target)
	display := target
	if targetDef != nil {
		display = targetDef.DisplayName
	}

	var greeting string
	if sess.CurriculumIdx > 0 {
		greeting = fmt.Sprintf("Welcome back. We are practicing %s. Give me a strum when you're ready.", display)
	} else {
		greeting = fmt.Sprintf("Hey there! Welcome to Zeplin. I'm your guitar co-pilot. Let's start with %s. Strum when you're ready.", display)
	}

	o.speak(greeting)
	o.publishState(sess, dsp.ChordEvent{}, greeting)
}

// SetTargetChordQuiet updates target chord without triggering speech.
func (o *Orchestrator) SetTargetChordQuiet(sessionID, chordName string) {
	sess := o.GetOrCreateSession(sessionID)
	def := ChordByName(chordName)
	if def == nil {
		return
	}
	sess.mu.Lock()
	sess.CurrentStep.TargetChord = chordName
	sess.SuccessStreak = 0
	sess.mu.Unlock()
}

// SetTargetChord switches the active target chord and provides spoken instruction.
func (o *Orchestrator) SetTargetChord(sessionID, chordName string) {
	sess := o.GetOrCreateSession(sessionID)
	def := ChordByName(chordName)
	if def == nil {
		return
	}
	sess.mu.Lock()
	if sess.CurrentStep.TargetChord == chordName {
		sess.mu.Unlock()
		return
	}
	sess.CurrentStep.TargetChord = chordName
	sess.SuccessStreak = 0
	sess.mu.Unlock()

	msg := fmt.Sprintf("Switching to %s. Strum all strings cleanly.", def.DisplayName)
	o.speak(msg)
	o.publishState(sess, dsp.ChordEvent{}, msg)
}

// SetSpeechActive sets the VAD gate for chord suppression.
// While speech is active, chord events are silently dropped to prevent
// vocal formants from polluting the chromagram analysis.
func (o *Orchestrator) SetSpeechActive(sessionID string, active bool) {
	sess := o.GetOrCreateSession(sessionID)
	sess.mu.Lock()
	sess.SpeechActive = active
	sess.mu.Unlock()
}

// ── Chord Event Handler ────────────────────────────────────────────────────

// HandleChordEvent processes a confirmed chord detection from the browser DSP engine.
// This runs on the DataChannel receive goroutine and must return promptly.
func (o *Orchestrator) HandleChordEvent(sessionID string, evt dsp.ChordEvent) {
	sess := o.GetOrCreateSession(sessionID)

	// ── VAD Gate ──────────────────────────────────────────────────────────
	sess.mu.Lock()
	speechActive := sess.SpeechActive
	sess.mu.Unlock()
	if speechActive {
		log.Printf("[Orchestrator] Chord event suppressed (speech active): %s", evt.DetectedChord)
		return
	}

	// ── Milestone 2: Instrument Strum Barge-In with Acoustic Echo Guard ────
	// If Zeplin is speaking and a genuine physical strum arrives (energy > -28 dBFS and confidence >= 0.70):
	// Instantly trigger barge-in to stop TTS speech and yield the floor to the instrument.
	// Low-level speaker bleed transients (< -28 dBFS) are guarded and dropped.
	if evt.RmsDBFS >= -28.0 && evt.Confidence >= 0.70 {
		o.TriggerInterrupt(sessionID)
	}

	target := sess.ActiveTarget()
	targetDef := ChordByName(target)

	log.Printf("[Orchestrator] Chord: detected=%q target=%q bass=%s conf=%.2f inv=%v",
		evt.DetectedChord, target, evt.BassNote, evt.Confidence, evt.Inversion)

	mutedIssues := findMutedIssues(evt, targetDef)

	// ── Telemetry Recording (Milestone 3) ─────────────────────────────────
	var mutedNames []string
	for _, idx := range mutedIssues {
		mutedNames = append(mutedNames, stringIndexToName(idx))
	}
	isClean := (evt.DetectedChord == target && !evt.Inversion && len(mutedIssues) == 0)
	sess.RecordStrum(StrumRecord{
		Timestamp:     time.Now(),
		DetectedChord: evt.DetectedChord,
		TargetChord:   target,
		Confidence:    evt.Confidence,
		Inversion:     evt.Inversion,
		BassNote:      evt.BassNote,
		MutedStrings:  mutedNames,
		IsCorrect:     isClean,
	})

	// ── Case 1: Correct chord, clean voicing ─────────────────────────────
	if evt.DetectedChord == target && !evt.Inversion && len(mutedIssues) == 0 {
		streak := sess.IncrStreak()
		step := sess.CurrentStep

		if streak >= step.RepsToAdvance {
			o.onExerciseComplete(sess, evt)
		} else {
			// Sparse positive reinforcement — not every strum, only key moments.
			if streak == 1 {
				o.speakCooled("Good. Hold it steady.")
			} else if streak == step.RepsToAdvance-1 {
				o.speakCooled("One more.")
			}
			o.publishState(sess, evt, "")
		}
		return
	}

	// ── Case 2: Correct chord, wrong bass (inversion / stray open string) ─
	if evt.DetectedChord == target && evt.Inversion {
		sess.RecordMistake(MistakeInversion)
		if o.store != nil {
			_ = o.store.RecordMistake(sess.SessionID, target, string(MistakeInversion), evt.BassNote)
		}
		sess.ResetStreak()
		msg := fmt.Sprintf("Watch your %s string — it's ringing out. Mute it.", evt.BassNote)
		o.speak(msg)
		o.publishState(sess, evt, msg)
		return
	}

	// ── Case 3: Correct chord name, but one string muted/buzzing ─────────
	if evt.DetectedChord == target && len(mutedIssues) > 0 {
		sess.RecordMistake(MistakeMutedString)
		if o.store != nil {
			_ = o.store.RecordMistake(sess.SessionID, target, string(MistakeMutedString), stringIndexToName(mutedIssues[0]))
		}
		sess.ResetStreak()
		stringName := stringIndexToName(mutedIssues[0])
		msg := fmt.Sprintf("Your %s string is muted. Arch your fretting finger.", stringName)
		o.speak(msg)
		o.publishState(sess, evt, msg)
		return
	}

	// ── Case 4: Wrong chord entirely ──────────────────────────────────────
	sess.RecordMistake(MistakeWrongChord)
	if o.store != nil {
		_ = o.store.RecordMistake(sess.SessionID, target, string(MistakeWrongChord), evt.DetectedChord)
	}
	sess.ResetStreak()

	targetDisplay := displayName(target)
	detectedDisplay := displayName(evt.DetectedChord)
	mistakeCount := sess.MistakeLog[MistakeWrongChord]

	var msg string
	switch {
	case mistakeCount > 5:
		msg = fmt.Sprintf("Still not %s. Try placing your fingers one at a time, then strum.", targetDisplay)
	case mistakeCount > 2:
		msg = fmt.Sprintf("Check your shape — that was %s, not %s.", detectedDisplay, targetDisplay)
	default:
		msg = fmt.Sprintf("That sounded like %s. Aim for %s.", detectedDisplay, targetDisplay)
	}
	o.speak(msg)
	o.publishState(sess, evt, msg)
}

// onExerciseComplete handles curriculum advancement when the streak target is reached.
func (o *Orchestrator) onExerciseComplete(sess *LessonSession, evt dsp.ChordEvent) {
	step := sess.CurrentStep

	if step.Exercise == ExerciseTransition {
		sess.FlipTransition()
		newTarget := sess.ActiveTarget()
		if def := ChordByName(newTarget); def != nil {
			msg := fmt.Sprintf("Good. Now switch to %s.", def.DisplayName)
			o.speak(msg)
			o.publishState(sess, evt, msg)
		}
		return
	}

	// Advance to next curriculum step
	advanced := sess.AdvanceStep()
	if advanced {
		if o.store != nil {
			_ = o.store.SaveCurriculumProgress(sess.SessionID, sess.CurriculumIdx+1, sess.SuccessStreak, nil)
		}
		nextDef := ChordByName(sess.CurrentStep.TargetChord)
		if nextDef != nil {
			msg := fmt.Sprintf("Excellent! Now let's learn %s.", nextDef.DisplayName)
			o.speak(msg)
			o.publishState(sess, evt, fmt.Sprintf("Next: %s", nextDef.DisplayName))
		}
	} else {
		o.speak("You've completed all the beginner chords. Excellent work!")
		o.publishState(sess, evt, "All done!")
	}
}

// CloseSession finalizes and logs a completed practice session to SQLite.
func (o *Orchestrator) CloseSession(sessionID string) {
	o.mu.Lock()
	sess, ok := o.sessions[sessionID]
	if ok {
		delete(o.sessions, sessionID)
	}
	o.mu.Unlock()

	if !ok || o.store == nil {
		return
	}

	ended := time.Now()
	dur := int(ended.Sub(sess.StartedAt).Seconds())
	rec := memory.SessionRecord{
		ID:                fmt.Sprintf("sess_%s_%d", sessionID, sess.StartedAt.Unix()),
		StudentID:         sessionID,
		StartedAt:         sess.StartedAt,
		EndedAt:           ended,
		DurationSec:       dur,
		TotalStrums:       sess.TotalStrums,
		CleanStrums:       sess.TotalCorrect,
		TuningOffsetCents: sess.TuningOffsetCents,
	}
	if err := o.store.SaveSession(rec); err != nil {
		log.Printf("[Orchestrator] Error saving session record: %v", err)
	} else {
		log.Printf("[Orchestrator] Practice session saved to SQLite for %s (%d strums, %ds)", sessionID, sess.TotalStrums, dur)
	}
}

// ── Speech Handler ─────────────────────────────────────────────────────────

// HandleSpeech processes a finalized STT transcript. It enriches the
// student's question with lesson context before routing to the LLM brain,
// giving the brain factual grounding for its answer.
func (o *Orchestrator) HandleSpeech(ctx context.Context, sessionID string, transcript string) {
	sess := o.GetOrCreateSession(sessionID)

	step := sess.CurrentStep
	streak := sess.SuccessStreak
	mistakes := sess.MistakeLog

	// Build a structured context preamble for the brain
	var lines []string
	lines = append(lines, fmt.Sprintf("[Lesson] Exercise: %s", step.Name))
	lines = append(lines, fmt.Sprintf("[Lesson] Target chord: %s", displayName(step.TargetChord)))
	lines = append(lines, fmt.Sprintf("[Lesson] Success streak: %d of %d", streak, step.RepsToAdvance))
	if len(mistakes) > 0 {
		lines = append(lines, fmt.Sprintf("[Lesson] Recurring issues: %s", formatMistakes(mistakes)))
	}
	lines = append(lines, fmt.Sprintf("[Lesson Stats]: %s", sess.FormatPracticeSummary()))
	if len(sess.RecentStrums) == 0 {
		lines = append(lines, "[Recent Strum Telemetry]: No guitar strum detected yet.")
		lines = append(lines, "[Tutor Instruction]: If the student asks what chord they just played or what chord that was, tell them directly that you didn't hear a strum, and invite them to strum clearly close to the mic. Do NOT guess or claim they played the lesson's target chord.")
	} else {
		lines = append(lines, fmt.Sprintf("[Recent Strum Telemetry (last 5)]:\n%s", sess.FormatRecentTelemetry()))
		lastStrum := sess.RecentStrums[len(sess.RecentStrums)-1]
		lines = append(lines, fmt.Sprintf("[Most Recent Strum Fact]: Detected as %s (confidence %.0f%%, bass %s)",
			displayName(lastStrum.DetectedChord), lastStrum.Confidence*100, lastStrum.BassNote))
		lines = append(lines, "[Tutor Instruction]: If the student asks what chord they played, answer using [Most Recent Strum Fact].")
	}
	lines = append(lines, fmt.Sprintf("[Student says]: %s", transcript))

	enriched := strings.Join(lines, "\n")
	log.Printf("[Orchestrator] Routing speech to brain: %q", transcript)

	if err := o.brain.ProcessTurn(ctx, sessionID, enriched); err != nil {
		log.Printf("[Orchestrator] Brain error: %v", err)
	}
}

// ── Tuning Calibration ─────────────────────────────────────────────────────

// HandleTuningCalibration receives a single-string F0 measurement during
// the session-start tuning check and evaluates deviation from A=440 Hz.
func (o *Orchestrator) HandleTuningCalibration(sessionID string, evt dsp.TuningCalibrationEvent) {
	sess := o.GetOrCreateSession(sessionID)

	// Open low E string reference: E2 = 82.41 Hz in standard A=440 tuning.
	const referenceE2Hz = 82.41
	offsetCents := 1200 * math.Log2(evt.Hz/referenceE2Hz)

	sess.mu.Lock()
	sess.TuningOffsetCents = offsetCents
	sess.TuningCalibrated = true
	sess.mu.Unlock()

	log.Printf("[Tuning] %s at %.1f Hz → offset %.1f cents", evt.Note, evt.Hz, offsetCents)

	if math.Abs(offsetCents) > 20 {
		direction := "flat"
		if offsetCents > 0 {
			direction = "sharp"
		}
		o.speak(fmt.Sprintf("Your guitar is about %.0f cents %s. Tune it before we start.", math.Abs(offsetCents), direction))
	} else {
		o.speak("Tuning looks good. Let's begin with E minor.")
	}
}

// ── Internal Helpers ───────────────────────────────────────────────────────

func (o *Orchestrator) speak(text string) {
	log.Printf("[TTS] %q", text)
	if o.tts == nil {
		return
	}
	o.tts.Speak(text)
}

// speakCooled emits speech only if the feedback text differs from the last
// utterance, or the cooldown period has elapsed.
func (o *Orchestrator) speakCooled(text string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if text == o.lastFeedbackText && time.Since(o.lastFeedbackAt) < o.feedbackCooldown {
		return
	}
	o.lastFeedbackText = text
	o.lastFeedbackAt = time.Now()
	go o.speak(text) // non-blocking to avoid holding the mutex during TTS I/O
}

// PublishFeedback pushes conversational text or tip to the UI.
func (o *Orchestrator) PublishFeedback(sessionID string, feedback string) {
	sess := o.GetOrCreateSession(sessionID)
	o.publishState(sess, dsp.ChordEvent{}, feedback)
}

func (o *Orchestrator) publishState(sess *LessonSession, evt dsp.ChordEvent, feedback string) {
	if o.publisher == nil {
		return
	}
	activeTarget := sess.ActiveTarget()
	update := dsp.UIStateUpdate{
		Event:         "lesson_state_update",
		TargetChord:   activeTarget,
		CurrentStreak: sess.SuccessStreak,
		FeedbackText:  feedback,
		StringStatus:  make([]string, 6),
	}
	targetDef := ChordByName(activeTarget)
	for i := 0; i < 6; i++ {
		if targetDef != nil && containsInt(targetDef.MutedStrings, i) {
			update.StringStatus[i] = "muted"
		} else {
			update.StringStatus[i] = "ok"
		}
	}
	for _, mutedIdx := range findMutedIssues(evt, targetDef) {
		if mutedIdx >= 0 && mutedIdx < 6 {
			update.StringStatus[mutedIdx] = "muted"
		}
	}
	o.publisher(update)
}

func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// findMutedIssues heuristically identifies muted string problems.
// A chord matched correctly but with confidence < 0.78 suggests that at
// least one string is muted or buzzing, reducing harmonic completeness.
func findMutedIssues(evt dsp.ChordEvent, def *ChordDef) []int {
	if def == nil || evt.DetectedChord == "" {
		return nil
	}
	if evt.Confidence < 0.78 {
		return []int{4} // B string (index 4) is the most common culprit for beginners
	}
	return nil
}

func stringIndexToName(idx int) string {
	names := []string{"low E", "A", "D", "G", "B", "high e"}
	if idx >= 0 && idx < len(names) {
		return names[idx]
	}
	return "unknown"
}

func displayName(chordName string) string {
	if def := ChordByName(chordName); def != nil {
		return def.DisplayName
	}
	return strings.ReplaceAll(chordName, "_", " ")
}

func formatMistakes(m map[MistakeType]int) string {
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s×%d", k, v))
	}
	return strings.Join(parts, ", ")
}
