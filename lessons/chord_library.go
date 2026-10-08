package lessons

// ChordDef describes the musical properties of a chord for matching and
// pedagogical feedback. PitchClasses uses standard MIDI semitone numbering:
// C=0, C#=1, D=2, D#=3, E=4, F=5, F#=6, G=7, G#=8, A=9, A#=10, B=11.
type ChordDef struct {
	// Name is the canonical chord identifier used in ChordEvent payloads.
	Name string
	// DisplayName is the human-readable name spoken by the agent.
	DisplayName string
	// Root is the pitch class of the chord root (0–11).
	Root int
	// PitchClasses lists the three (or more) pitch classes that define the chord.
	PitchClasses []int
	// OpenStringFingering encodes the standard beginner open-position fingering.
	// Each element represents one string [low E → high e], value is:
	//   -1 = string not played (muted / x)
	//    0 = open string
	//   N  = fret number
	OpenStringFingering []int
	// MutedStrings is the indices (0-based, low-E=0) of strings that must NOT ring.
	MutedStrings []int
}

// V1ChordLibrary is the v1 beginner chord set: 8 open-position chords.
var V1ChordLibrary = []ChordDef{
	{
		Name:                "C_Major",
		DisplayName:         "C major",
		Root:                0, // C
		PitchClasses:        []int{0, 4, 7},
		OpenStringFingering: []int{-1, 3, 2, 0, 1, 0},
		MutedStrings:        []int{0}, // low E muted
	},
	{
		Name:                "G_Major",
		DisplayName:         "G major",
		Root:                7, // G
		PitchClasses:        []int{7, 11, 2},
		OpenStringFingering: []int{3, 2, 0, 0, 3, 3},
		MutedStrings:        []int{},
	},
	{
		Name:                "D_Major",
		DisplayName:         "D major",
		Root:                2, // D
		PitchClasses:        []int{2, 6, 9},
		OpenStringFingering: []int{-1, -1, 0, 2, 3, 2},
		MutedStrings:        []int{0, 1}, // low E and A muted
	},
	{
		Name:                "E_Minor",
		DisplayName:         "E minor",
		Root:                4, // E
		PitchClasses:        []int{4, 7, 11},
		OpenStringFingering: []int{0, 2, 2, 0, 0, 0},
		MutedStrings:        []int{},
	},
	{
		Name:                "A_Minor",
		DisplayName:         "A minor",
		Root:                9, // A
		PitchClasses:        []int{9, 0, 4},
		OpenStringFingering: []int{-1, 0, 2, 2, 1, 0},
		MutedStrings:        []int{0}, // low E muted
	},
	{
		Name:                "E_Major",
		DisplayName:         "E major",
		Root:                4, // E
		PitchClasses:        []int{4, 8, 11},
		OpenStringFingering: []int{0, 2, 2, 1, 0, 0},
		MutedStrings:        []int{},
	},
	{
		Name:                "A_Major",
		DisplayName:         "A major",
		Root:                9, // A
		PitchClasses:        []int{9, 1, 4},
		OpenStringFingering: []int{-1, 0, 2, 2, 2, 0},
		MutedStrings:        []int{0}, // low E muted
	},
	{
		Name:                "D_Minor",
		DisplayName:         "D minor",
		Root:                2, // D
		PitchClasses:        []int{2, 5, 9},
		OpenStringFingering: []int{-1, -1, 0, 2, 3, 1},
		MutedStrings:        []int{0, 1}, // low E and A muted
	},
}

// ChordByName returns the ChordDef for a given chord name, or nil if not found.
func ChordByName(name string) *ChordDef {
	for i := range V1ChordLibrary {
		if V1ChordLibrary[i].Name == name {
			return &V1ChordLibrary[i]
		}
	}
	return nil
}
