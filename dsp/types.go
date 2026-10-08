package dsp

// ChordEvent is the structured telemetry payload emitted by the browser
// AudioWorklet after a confirmed chord detection passes all three DSP gates
// (RMS energy, spectral centroid, harmonic ratio). It flows to the Go core
// via the LiveKit WebRTC DataChannel.
type ChordEvent struct {
	// Event is always "chord_detected" for chord events.
	Event string `json:"event"`

	// TimestampMs is the AudioWorklet's currentTime (in ms) when the chord was confirmed.
	TimestampMs int64 `json:"timestamp_ms"`

	// DetectedChord is the matched chord name from the template library,
	// e.g. "G_Major", "A_Minor". Empty if confidence is below threshold.
	DetectedChord string `json:"detected_chord"`

	// BassNote is the bass fundamental note name derived from YIN pitch detection
	// on the low-passed signal (< 400 Hz), e.g. "E2", "C3".
	BassNote string `json:"bass_note"`

	// BassHz is the raw fundamental frequency of the bass note in Hz.
	BassHz float64 `json:"bass_hz"`

	// Confidence is the cosine similarity score between the measured chroma
	// vector and the winning chord template (0.0 – 1.0).
	Confidence float64 `json:"confidence"`

	// Inversion is true when the bass note's pitch class doesn't match the
	// chord's expected root — indicating a first or second inversion voicing
	// (e.g., C/E where E2 rings as the lowest note instead of C2).
	Inversion bool `json:"inversion"`

	// TuningOffsetCents is the estimated deviation from A=440 Hz tuning
	// in cents, calibrated at session start. Applied before chroma folding.
	TuningOffsetCents float64 `json:"tuning_offset_cents"`

	// RmsDBFS is the RMS signal level of the analysis window in dBFS.
	// Used internally by Gate 1; included for telemetry/debugging.
	RmsDBFS float64 `json:"rms_dbfs"`

	// SpectralCentroidHz is the energy-weighted mean frequency of the spectrum.
	// Used internally by Gate 2.
	SpectralCentroidHz float64 `json:"spectral_centroid_hz"`

	// HarmonicRatio is the fraction of energy concentrated at harmonic partials.
	// Used internally by Gate 3.
	HarmonicRatio float64 `json:"harmonic_ratio"`
}

// TuningCalibrationEvent is emitted by the worklet during tuning calibration
// mode when a stable F0 is detected on a single open string.
type TuningCalibrationEvent struct {
	Event string  `json:"event"` // "calibration_f0"
	Hz    float64 `json:"hz"`
	Note  string  `json:"note"`
}

// UIStateUpdate is sent Go→Browser via DataChannel to drive the fretboard UI.
type UIStateUpdate struct {
	Event         string `json:"event"` // "lesson_state_update"
	TargetChord   string `json:"target_chord"`
	CurrentStreak int    `json:"current_streak"`
	FeedbackText  string `json:"feedback_text"`
	// StringStatus is a 6-element slice: "ok" | "muted" | "silent"
	// Index 0 = low E string, Index 5 = high e string.
	StringStatus []string `json:"string_status"`
}
