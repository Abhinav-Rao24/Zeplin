export interface FingerPlacement {
  fingerNumber: number;
  fingerName: string; // e.g. "Index finger"
  stringName: string; // e.g. "A string · 2nd fret"
  fret: number;
  stringIndex: number; // 0 = low E, 5 = high e
}

export interface StrumBeat {
  beat: string;       // "1", "&", "2", "&", "3", "&", "4", "&"
  arrow: 'down' | 'up' | 'rest';
  accent?: boolean;
}

export interface ChordData {
  id: string;
  name: string;
  letter: string;
  subtitle: string;
  fingerPlacements: FingerPlacement[];
  diagram: {
    // 6 strings: -1 = X (muted), 0 = O (open), >0 = fret number
    frets: number[];
    // finger to display on that string (0 if open/muted)
    fingers: number[];
  };
  strummingPattern: {
    name: string;
    meter: string; // "4/4"
    beats: StrumBeat[];
  };
  reminder: string;
  notes: string[]; // e.g. ["G2", "B2", "D3", "G3", "B3", "G4"]
}

export const CHORD_LIBRARY: Record<string, ChordData> = {
  G_Major: {
    id: 'G_Major',
    name: 'G major',
    letter: 'G',
    subtitle: 'Strum all six strings',
    fingerPlacements: [
      { fingerNumber: 1, fingerName: 'Index finger', stringName: 'A string · 2nd fret', fret: 2, stringIndex: 1 },
      { fingerNumber: 2, fingerName: 'Middle finger', stringName: 'Low E string · 3rd fret', fret: 3, stringIndex: 0 },
      { fingerNumber: 3, fingerName: 'Ring finger', stringName: 'High E string · 3rd fret', fret: 3, stringIndex: 5 },
    ],
    diagram: {
      frets: [3, 2, 0, 0, 0, 3],
      fingers: [2, 1, 0, 0, 0, 3],
    },
    strummingPattern: {
      name: 'Classic Ballad Strum',
      meter: '4/4',
      beats: [
        { beat: '1', arrow: 'down', accent: true },
        { beat: '&', arrow: 'rest' },
        { beat: '2', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
        { beat: '3', arrow: 'rest' },
        { beat: '&', arrow: 'up' },
        { beat: '4', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
      ],
    },
    reminder: "Keep your fingertips curved and press just behind each fret. It's okay if it buzzes at first.",
    notes: ['G2', 'B2', 'D3', 'G3', 'B3', 'G4'],
  },
  C_Major: {
    id: 'C_Major',
    name: 'C major',
    letter: 'C',
    subtitle: 'Strum five strings · Mute low E',
    fingerPlacements: [
      { fingerNumber: 1, fingerName: 'Index finger', stringName: 'B string · 1st fret', fret: 1, stringIndex: 4 },
      { fingerNumber: 2, fingerName: 'Middle finger', stringName: 'D string · 2nd fret', fret: 2, stringIndex: 2 },
      { fingerNumber: 3, fingerName: 'Ring finger', stringName: 'A string · 3rd fret', fret: 3, stringIndex: 1 },
    ],
    diagram: {
      frets: [-1, 3, 2, 0, 1, 0],
      fingers: [0, 3, 2, 0, 1, 0],
    },
    strummingPattern: {
      name: 'Folk 4/4 Steady Strum',
      meter: '4/4',
      beats: [
        { beat: '1', arrow: 'down', accent: true },
        { beat: '&', arrow: 'rest' },
        { beat: '2', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
        { beat: '3', arrow: 'down', accent: true },
        { beat: '&', arrow: 'rest' },
        { beat: '4', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
      ],
    },
    reminder: "Let the tip of your ring finger lightly touch the low E string to keep it silently muted.",
    notes: ['C3', 'E3', 'G3', 'C4', 'E4'],
  },
  D_Major: {
    id: 'D_Major',
    name: 'D major',
    letter: 'D',
    subtitle: 'Strum bottom four strings',
    fingerPlacements: [
      { fingerNumber: 1, fingerName: 'Index finger', stringName: 'G string · 2nd fret', fret: 2, stringIndex: 3 },
      { fingerNumber: 2, fingerName: 'Middle finger', stringName: 'High E string · 2nd fret', fret: 2, stringIndex: 5 },
      { fingerNumber: 3, fingerName: 'Ring finger', stringName: 'B string · 3rd fret', fret: 3, stringIndex: 4 },
    ],
    diagram: {
      frets: [-1, -1, 0, 2, 3, 2],
      fingers: [0, 0, 0, 1, 3, 2],
    },
    strummingPattern: {
      name: 'Acoustic Pop Rhythm',
      meter: '4/4',
      beats: [
        { beat: '1', arrow: 'down', accent: true },
        { beat: '&', arrow: 'down' },
        { beat: '2', arrow: 'rest' },
        { beat: '&', arrow: 'up' },
        { beat: '3', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
        { beat: '4', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
      ],
    },
    reminder: "Make a tight triangle with your fingers. Pick cleanly from the open 4th D string downward.",
    notes: ['D3', 'A3', 'D4', 'F#4'],
  },
  E_Minor: {
    id: 'E_Minor',
    name: 'E minor',
    letter: 'Em',
    subtitle: 'Strum all six strings',
    fingerPlacements: [
      { fingerNumber: 2, fingerName: 'Middle finger', stringName: 'A string · 2nd fret', fret: 2, stringIndex: 1 },
      { fingerNumber: 3, fingerName: 'Ring finger', stringName: 'D string · 2nd fret', fret: 2, stringIndex: 2 },
    ],
    diagram: {
      frets: [0, 2, 2, 0, 0, 0],
      fingers: [0, 2, 3, 0, 0, 0],
    },
    strummingPattern: {
      name: 'Resonant Acoustic Strum',
      meter: '4/4',
      beats: [
        { beat: '1', arrow: 'down', accent: true },
        { beat: '&', arrow: 'rest' },
        { beat: '2', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
        { beat: '3', arrow: 'rest' },
        { beat: '&', arrow: 'up' },
        { beat: '4', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
      ],
    },
    reminder: "Stand your two fingers up tall on their tips so the open G and B strings ring loud and clear.",
    notes: ['E2', 'B2', 'E3', 'G3', 'B3', 'E4'],
  },
  A_Minor: {
    id: 'A_Minor',
    name: 'A minor',
    letter: 'Am',
    subtitle: 'Strum five strings · Mute low E',
    fingerPlacements: [
      { fingerNumber: 1, fingerName: 'Index finger', stringName: 'B string · 1st fret', fret: 1, stringIndex: 4 },
      { fingerNumber: 2, fingerName: 'Middle finger', stringName: 'D string · 2nd fret', fret: 2, stringIndex: 2 },
      { fingerNumber: 3, fingerName: 'Ring finger', stringName: 'G string · 2nd fret', fret: 2, stringIndex: 3 },
    ],
    diagram: {
      frets: [-1, 0, 2, 2, 1, 0],
      fingers: [0, 0, 2, 3, 1, 0],
    },
    strummingPattern: {
      name: 'Soulful Ballad Strum',
      meter: '4/4',
      beats: [
        { beat: '1', arrow: 'down', accent: true },
        { beat: '&', arrow: 'rest' },
        { beat: '2', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
        { beat: '3', arrow: 'down', accent: true },
        { beat: '&', arrow: 'up' },
        { beat: '4', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
      ],
    },
    reminder: "Same exact shape as E major, just shifted down one string! Keep your wrist relaxed.",
    notes: ['A2', 'E3', 'A3', 'C4', 'E4'],
  },
  E_Major: {
    id: 'E_Major',
    name: 'E major',
    letter: 'E',
    subtitle: 'Strum all six strings',
    fingerPlacements: [
      { fingerNumber: 1, fingerName: 'Index finger', stringName: 'G string · 1st fret', fret: 1, stringIndex: 3 },
      { fingerNumber: 2, fingerName: 'Middle finger', stringName: 'A string · 2nd fret', fret: 2, stringIndex: 1 },
      { fingerNumber: 3, fingerName: 'Ring finger', stringName: 'D string · 2nd fret', fret: 2, stringIndex: 2 },
    ],
    diagram: {
      frets: [0, 2, 2, 1, 0, 0],
      fingers: [0, 2, 3, 1, 0, 0],
    },
    strummingPattern: {
      name: 'Classic Rock Drive',
      meter: '4/4',
      beats: [
        { beat: '1', arrow: 'down', accent: true },
        { beat: '&', arrow: 'rest' },
        { beat: '2', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
        { beat: '3', arrow: 'rest' },
        { beat: '&', arrow: 'up' },
        { beat: '4', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
      ],
    },
    reminder: "Arch your index finger so you don't inadvertently bump into the open B string next to it.",
    notes: ['E2', 'B2', 'E3', 'G#3', 'B3', 'E4'],
  },
  A_Major: {
    id: 'A_Major',
    name: 'A major',
    letter: 'A',
    subtitle: 'Strum five strings · Mute low E',
    fingerPlacements: [
      { fingerNumber: 1, fingerName: 'Index finger', stringName: 'D string · 2nd fret', fret: 2, stringIndex: 2 },
      { fingerNumber: 2, fingerName: 'Middle finger', stringName: 'G string · 2nd fret', fret: 2, stringIndex: 3 },
      { fingerNumber: 3, fingerName: 'Ring finger', stringName: 'B string · 2nd fret', fret: 2, stringIndex: 4 },
    ],
    diagram: {
      frets: [-1, 0, 2, 2, 2, 0],
      fingers: [0, 0, 1, 2, 3, 0],
    },
    strummingPattern: {
      name: 'Crisp Folk Strum',
      meter: '4/4',
      beats: [
        { beat: '1', arrow: 'down', accent: true },
        { beat: '&', arrow: 'rest' },
        { beat: '2', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
        { beat: '3', arrow: 'down', accent: true },
        { beat: '&', arrow: 'rest' },
        { beat: '4', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
      ],
    },
    reminder: "Stack your three fingers snugly inside the 2nd fret like three peas in a pod.",
    notes: ['A2', 'E3', 'A3', 'C#4', 'E4'],
  },
  D_Minor: {
    id: 'D_Minor',
    name: 'D minor',
    letter: 'Dm',
    subtitle: 'Strum bottom four strings',
    fingerPlacements: [
      { fingerNumber: 1, fingerName: 'Index finger', stringName: 'High E string · 1st fret', fret: 1, stringIndex: 5 },
      { fingerNumber: 2, fingerName: 'Middle finger', stringName: 'G string · 2nd fret', fret: 2, stringIndex: 3 },
      { fingerNumber: 3, fingerName: 'Ring finger', stringName: 'B string · 3rd fret', fret: 3, stringIndex: 4 },
    ],
    diagram: {
      frets: [-1, -1, 0, 2, 3, 1],
      fingers: [0, 0, 0, 2, 3, 1],
    },
    strummingPattern: {
      name: 'Melancholy Slow Strum',
      meter: '4/4',
      beats: [
        { beat: '1', arrow: 'down', accent: true },
        { beat: '&', arrow: 'rest' },
        { beat: '2', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
        { beat: '3', arrow: 'down', accent: true },
        { beat: '&', arrow: 'up' },
        { beat: '4', arrow: 'down', accent: false },
        { beat: '&', arrow: 'up' },
      ],
    },
    reminder: "Reach your index finger out to the 1st fret while keeping your ring finger anchored on fret 3.",
    notes: ['D3', 'A3', 'D4', 'F4'],
  },
};

export const LESSON_SEQUENCE = [
  'E_Minor',
  'A_Minor',
  'G_Major',
  'C_Major',
  'D_Major',
  'E_Major',
];
