/**
 * fretboard.js — Interactive SVG Guitar Fretboard Renderer
 *
 * Renders an accurate, responsive SVG guitar neck with:
 *   - 6 strings (String 6/Low-E to String 1/High-e with proportional wire gauges)
 *   - Nut and Frets 1 through 4 (standard beginner open chord territory)
 *   - Fret marker inlay dots (e.g. Fret 3)
 *   - Finger dots with fret numbers and finger numbers
 *   - String status indicators above the nut:
 *       🟢 Green 'O'  = open string ringing correctly
 *       ⚪ Grey 'X'   = string muted / not played intentionally
 *       🔴 Red alert  = string muted or buzzing by mistake
 */

'use strict';

const CHORD_FINGERINGS = {
  'C_Major': {
    name: 'C Major',
    // [string6, string5, string4, string3, string2, string1]
    // { fret: -1..4, finger: 0..4 }
    strings: [
      { fret: -1, finger: 0 }, // E (X)
      { fret: 3,  finger: 3 }, // A (Ring)
      { fret: 2,  finger: 2 }, // D (Middle)
      { fret: 0,  finger: 0 }, // G (Open)
      { fret: 1,  finger: 1 }, // B (Index)
      { fret: 0,  finger: 0 }, // e (Open)
    ]
  },
  'G_Major': {
    name: 'G Major',
    strings: [
      { fret: 3,  finger: 2 }, // E (Middle)
      { fret: 2,  finger: 1 }, // A (Index)
      { fret: 0,  finger: 0 }, // D (Open)
      { fret: 0,  finger: 0 }, // G (Open)
      { fret: 3,  finger: 3 }, // B (Ring)
      { fret: 3,  finger: 4 }, // e (Pinky)
    ]
  },
  'D_Major': {
    name: 'D Major',
    strings: [
      { fret: -1, finger: 0 }, // E (X)
      { fret: -1, finger: 0 }, // A (X)
      { fret: 0,  finger: 0 }, // D (Open)
      { fret: 2,  finger: 1 }, // G (Index)
      { fret: 3,  finger: 3 }, // B (Ring)
      { fret: 2,  finger: 2 }, // e (Middle)
    ]
  },
  'E_Minor': {
    name: 'E Minor',
    strings: [
      { fret: 0,  finger: 0 }, // E (Open)
      { fret: 2,  finger: 2 }, // A (Middle)
      { fret: 2,  finger: 3 }, // D (Ring)
      { fret: 0,  finger: 0 }, // G (Open)
      { fret: 0,  finger: 0 }, // B (Open)
      { fret: 0,  finger: 0 }, // e (Open)
    ]
  },
  'A_Minor': {
    name: 'A Minor',
    strings: [
      { fret: -1, finger: 0 }, // E (X)
      { fret: 0,  finger: 0 }, // A (Open)
      { fret: 2,  finger: 2 }, // D (Middle)
      { fret: 2,  finger: 3 }, // G (Ring)
      { fret: 1,  finger: 1 }, // B (Index)
      { fret: 0,  finger: 0 }, // e (Open)
    ]
  },
  'E_Major': {
    name: 'E Major',
    strings: [
      { fret: 0,  finger: 0 }, // E (Open)
      { fret: 2,  finger: 2 }, // A (Middle)
      { fret: 2,  finger: 3 }, // D (Ring)
      { fret: 1,  finger: 1 }, // G (Index)
      { fret: 0,  finger: 0 }, // B (Open)
      { fret: 0,  finger: 0 }, // e (Open)
    ]
  },
  'A_Major': {
    name: 'A Major',
    strings: [
      { fret: -1, finger: 0 }, // E (X)
      { fret: 0,  finger: 0 }, // A (Open)
      { fret: 2,  finger: 1 }, // D (Index)
      { fret: 2,  finger: 2 }, // G (Middle)
      { fret: 2,  finger: 3 }, // B (Ring)
      { fret: 0,  finger: 0 }, // e (Open)
    ]
  },
  'D_Minor': {
    name: 'D Minor',
    strings: [
      { fret: -1, finger: 0 }, // E (X)
      { fret: -1, finger: 0 }, // A (X)
      { fret: 0,  finger: 0 }, // D (Open)
      { fret: 2,  finger: 2 }, // G (Middle)
      { fret: 3,  finger: 3 }, // B (Ring)
      { fret: 1,  finger: 1 }, // e (Index)
    ]
  }
};

const STRING_NAMES = ['6:E', '5:A', '4:D', '3:G', '2:B', '1:e'];
const STRING_GAUGES = [3.4, 2.8, 2.2, 1.8, 1.3, 1.0]; // Low-E to High-e

class FretboardRenderer {
  constructor(containerId) {
    this.container = document.getElementById(containerId);
    this.currentChord = null;
    this.stringStatus = ['ok', 'ok', 'ok', 'ok', 'ok', 'ok'];
  }

  setChord(chordName, stringStatus) {
    this.currentChord = chordName;
    if (stringStatus && Array.isArray(stringStatus)) {
      this.stringStatus = stringStatus;
    }
    this.render();
  }

  render() {
    if (!this.container) return;

    const chord = CHORD_FINGERINGS[this.currentChord];
    const width = 360;
    const height = 280;

    // Layout constants
    const margin = { top: 40, left: 45, right: 35, bottom: 25 };
    const boardWidth = width - margin.left - margin.right;
    const boardHeight = height - margin.top - margin.bottom;

    const numFrets = 4;
    const numStrings = 6;
    const stringSpacing = boardWidth / (numStrings - 1);
    const fretSpacing = boardHeight / numFrets;

    let svg = `<svg viewBox="0 0 ${width} ${height}" class="fretboard-svg" xmlns="http://www.w3.org/2000/svg">`;

    // 1. Fretboard background wood texture
    svg += `<rect x="${margin.left - 6}" y="${margin.top}" width="${boardWidth + 12}" height="${boardHeight}" rx="4" fill="#181a20" stroke="#2d3342" stroke-width="1.5" />`;

    // 2. Nut (fret 0 bone)
    svg += `<rect x="${margin.left - 6}" y="${margin.top - 6}" width="${boardWidth + 12}" height="6" rx="2" fill="#e2e8f0" stroke="#94a3b8" stroke-width="0.5" />`;

    // 3. Fret lines
    for (let f = 1; f <= numFrets; f++) {
      const y = margin.top + f * fretSpacing;
      svg += `<line x1="${margin.left - 6}" y1="${y}" x2="${margin.left + boardWidth + 6}" y2="${y}" stroke="#64748b" stroke-width="2" />`;
      // Fret number label on left
      svg += `<text x="${margin.left - 18}" y="${y - fretSpacing / 2 + 4}" fill="#64748b" font-size="11" font-weight="600" text-anchor="middle">${f}</text>`;
    }

    // 4. Inlay dot on fret 3
    const fret3Y = margin.top + 2.5 * fretSpacing;
    const fretCenterX = margin.left + boardWidth / 2;
    svg += `<circle cx="${fretCenterX}" cy="${fret3Y}" r="4.5" fill="#334155" opacity="0.8" />`;

    // 5. Strings
    for (let s = 0; s < numStrings; s++) {
      const x = margin.left + s * stringSpacing;
      const gauge = STRING_GAUGES[s];
      svg += `<line x1="${x}" y1="${margin.top}" x2="${x}" y2="${margin.top + boardHeight}" stroke="#94a3b8" stroke-width="${gauge}" stroke-linecap="round" />`;

      // String name label below
      svg += `<text x="${x}" y="${margin.top + boardHeight + 16}" fill="#64748b" font-size="10" font-weight="600" text-anchor="middle">${STRING_NAMES[s].split(':')[1]}</text>`;
    }

    // 6. Nut status indicators & Finger placement dots
    if (chord) {
      chord.strings.forEach((str, s) => {
        const x = margin.left + s * stringSpacing;
        const status = this.stringStatus[s] || 'ok';

        if (str.fret === -1) {
          // Muted string (X) above nut
          svg += `<text x="${x}" y="${margin.top - 12}" fill="#ef4444" font-size="14" font-weight="700" text-anchor="middle">✕</text>`;
        } else if (str.fret === 0) {
          // Open string (O) above nut
          const color = status === 'muted' ? '#ef4444' : '#22c55e';
          svg += `<circle cx="${x}" cy="${margin.top - 16}" r="6" fill="none" stroke="${color}" stroke-width="2" />`;
          if (status === 'muted') {
            svg += `<text x="${x}" y="${margin.top - 26}" fill="#ef4444" font-size="9" font-weight="700" text-anchor="middle">MUTED!</text>`;
          }
        } else {
          // Fretted note dot
          const y = margin.top + (str.fret - 0.5) * fretSpacing;
          const dotColor = status === 'muted' ? '#ef4444' : '#38bdf8';
          const ringColor = status === 'muted' ? '#fca5a5' : '#bae6fd';

          // Outer pulse ring
          svg += `<circle cx="${x}" cy="${y}" r="13" fill="${dotColor}" stroke="${ringColor}" stroke-width="2" filter="drop-shadow(0 2px 4px rgba(0,0,0,0.5))" />`;
          // Finger number inside dot
          if (str.finger > 0) {
            svg += `<text x="${x}" y="${y + 4}" fill="#0f172a" font-size="11" font-weight="800" text-anchor="middle">${str.finger}</text>`;
          }
        }
      });
    }

    svg += '</svg>';
    this.container.innerHTML = svg;
  }
}

window.FretboardRenderer = FretboardRenderer;
