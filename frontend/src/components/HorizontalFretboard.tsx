'use client';

import React from 'react';
import { ChordData } from '@/lib/chords';

interface HorizontalFretboardProps {
  chord: ChordData | null;
  stringStatus?: string[]; // 6 strings from low E (index 0) to high e (index 5)
  isBlank?: boolean;
}

export const HorizontalFretboard: React.FC<HorizontalFretboardProps> = ({
  chord,
  stringStatus = [],
  isBlank = false,
}) => {
  // SVG Coordinate space
  const svgWidth = 960;
  const svgHeight = 200;

  const leftMargin = 55; // space for nut indicators (O, X)
  const nutWidth = 10;
  const nutX = leftMargin + 10;
  const boardStartX = nutX + nutWidth;
  const boardEndX = svgWidth - 30;
  const boardWidth = boardEndX - boardStartX;

  const topStringY = 32;
  const bottomStringY = 168;
  const stringSpanY = bottomStringY - topStringY;
  const numStrings = 6;
  const stringSpacing = stringSpanY / (numStrings - 1); // 27.2px

  const numFrets = 15;
  const fretWidth = boardWidth / numFrets; // ~58px per fret

  // String gauges (top string is high e, bottom string is low E)
  // Index in array corresponds to row from top (0 = high e, 5 = low E)
  const stringGauges = [1.4, 1.8, 2.2, 2.7, 3.2, 3.8];

  // Inlay marker dots (single at frets 3, 5, 7, 9, 15; double at fret 12)
  const singleDotFrets = [3, 5, 7, 9, 15];
  const doubleDotFrets = [12];
  const middleY = (topStringY + bottomStringY) / 2;

  // Map each string index (0 = low E to 5 = high e) to its row Y position:
  // Row 0 = High e (stringIndex 5)
  // Row 1 = B (stringIndex 4)
  // Row 2 = G (stringIndex 3)
  // Row 3 = D (stringIndex 2)
  // Row 4 = A (stringIndex 1)
  // Row 5 = Low E (stringIndex 0)
  const getStringY = (stringIndex: number) => {
    const row = 5 - stringIndex;
    return topStringY + row * stringSpacing;
  };

  // Nut indicator calculation (open 'O', muted 'X')
  // For each string index (0 to 5):
  const getNutMarker = (stringIndex: number): 'O' | 'X' | null => {
    if (isBlank || !chord) return null;
    const fret = chord.diagram.frets[stringIndex];
    if (fret === -1) return 'X';
    if (fret === 0) return 'O';
    return null; // fretted, so no nut indicator
  };

  return (
    <div className="w-full max-w-[960px] mx-auto select-none px-2">
      <svg
        viewBox={`0 0 ${svgWidth} ${svgHeight}`}
        className="w-full h-auto drop-shadow-sm"
        xmlns="http://www.w3.org/2000/svg"
      >
        {/* Fretboard Wood Background (Subtle dark matte slat) */}
        <rect
          x={boardStartX}
          y={topStringY - 10}
          width={boardWidth}
          height={stringSpanY + 20}
          fill="#1A1D24"
          rx={3}
        />

        {/* Fret Inlay Position Markers (Pearl Dots) */}
        {singleDotFrets.map((fret) => {
          const cx = boardStartX + (fret - 0.5) * fretWidth;
          return (
            <circle
              key={`dot-${fret}`}
              cx={cx}
              cy={middleY}
              r={4}
              fill="#525866"
              opacity={0.4}
            />
          );
        })}

        {doubleDotFrets.map((fret) => {
          const cx = boardStartX + (fret - 0.5) * fretWidth;
          return (
            <g key={`double-dot-${fret}`}>
              <circle
                cx={cx}
                cy={middleY - 20}
                r={3.5}
                fill="#525866"
                opacity={0.4}
              />
              <circle
                cx={cx}
                cy={middleY + 20}
                r={3.5}
                fill="#525866"
                opacity={0.4}
              />
            </g>
          );
        })}

        {/* Vertical Fret Wire Lines (1 to numFrets) */}
        {Array.from({ length: numFrets }).map((_, i) => {
          const fretNum = i + 1;
          const x = boardStartX + fretNum * fretWidth;
          return (
            <line
              key={`fret-wire-${fretNum}`}
              x1={x}
              y1={topStringY - 8}
              x2={x}
              y2={bottomStringY + 8}
              stroke="#5A6170"
              strokeWidth="2"
              strokeLinecap="round"
            />
          );
        })}

        {/* Ivory / Bone Nut Bar (Left boundary) */}
        <rect
          x={nutX}
          y={topStringY - 10}
          width={nutWidth}
          height={stringSpanY + 20}
          fill="#E2DEC9"
          rx={2}
          stroke="#C8C4B0"
          strokeWidth="0.8"
        />

        {/* Horizontal Guitar Strings (6 lines spanning across frets) */}
        {Array.from({ length: numStrings }).map((_, row) => {
          const stringIndex = 5 - row;
          const y = topStringY + row * stringSpacing;
          const gauge = stringGauges[row];
          const isMutedIssue = stringStatus[stringIndex] === 'muted';

          return (
            <line
              key={`string-${row}`}
              x1={leftMargin}
              y1={y}
              x2={boardEndX}
              y2={y}
              stroke={isMutedIssue ? '#F59E0B' : '#B8BCC6'}
              strokeWidth={gauge}
              opacity={isMutedIssue ? 0.95 : 0.85}
            />
          );
        })}

        {/* Nut String Indicators (Left of Nut: 'O' or 'X') */}
        {!isBlank &&
          chord &&
          Array.from({ length: numStrings }).map((_, row) => {
            const stringIndex = 5 - row;
            const marker = getNutMarker(stringIndex);
            if (!marker) return null;

            const y = topStringY + row * stringSpacing;
            const isMutedIssue = stringStatus[stringIndex] === 'muted';

            return (
              <text
                key={`nut-marker-${row}`}
                x={leftMargin - 14}
                y={y + 5}
                textAnchor="middle"
                fontSize={marker === 'O' ? '15' : '14'}
                fontWeight="700"
                fontFamily="system-ui, sans-serif"
                fill={
                  marker === 'X'
                    ? '#9CA3AF'
                    : isMutedIssue
                    ? '#F59E0B'
                    : '#F3F4F6'
                }
              >
                {marker === 'X' ? '✕' : '○'}
              </text>
            );
          })}

        {/* Populated Finger Position Markers (Matte slate-blue circles with finger numbers) */}
        {!isBlank &&
          chord &&
          chord.fingerPlacements.map((fp) => {
            if (fp.fret <= 0) return null;

            // Center of the fret slot: midpoint between fret wire (fp.fret - 1) and (fp.fret)
            const cx = boardStartX + (fp.fret - 0.5) * fretWidth;
            const cy = getStringY(fp.stringIndex);
            const isMutedIssue = stringStatus[fp.stringIndex] === 'muted';

            return (
              <g key={`finger-${fp.fingerNumber}-${fp.stringIndex}-${fp.fret}`}>
                {/* Matte Slate-Blue Marker Circle */}
                <circle
                  cx={cx}
                  cy={cy}
                  r={13.5}
                  fill={isMutedIssue ? '#D97706' : '#4E7399'}
                  stroke={isMutedIssue ? '#B45309' : '#395775'}
                  strokeWidth="1.5"
                />

                {/* Clear Finger Number (1: Index, 2: Middle, 3: Ring, 4: Pinky) */}
                <text
                  x={cx}
                  y={cy}
                  textAnchor="middle"
                  dominantBaseline="central"
                  fill="#FFFFFF"
                  fontSize="13"
                  fontWeight="700"
                  fontFamily="system-ui, -apple-system, sans-serif"
                >
                  {fp.fingerNumber}
                </text>
              </g>
            );
          })}
      </svg>
    </div>
  );
};
