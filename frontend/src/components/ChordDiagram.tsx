'use strict';

import React from 'react';
import { ChordData } from '@/lib/chords';

interface ChordDiagramProps {
  chord: ChordData;
  stringStatus?: string[]; // 6 strings: "ok" | "muted"
}

export const ChordDiagram: React.FC<ChordDiagramProps> = ({ chord, stringStatus = [] }) => {
  const width = 230;
  const height = 210;
  const margin = { top: 32, left: 32, right: 32, bottom: 20 };

  const gridWidth = width - margin.left - margin.right;
  const gridHeight = height - margin.top - margin.bottom;

  const numStrings = 6;
  const numFrets = 4;
  const stringSpacing = gridWidth / (numStrings - 1);
  const fretSpacing = gridHeight / numFrets;

  return (
    <div className="flex flex-col items-center select-none">
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className="w-full max-w-[240px] h-auto"
        xmlns="http://www.w3.org/2000/svg"
      >
        {/* Top Nut Bar (Thick horizontal bar representing fret 0) */}
        <rect
          x={margin.left - 1.5}
          y={margin.top - 4}
          width={gridWidth + 3}
          height={5}
          fill="#1C2420"
          rx={1.5}
        />

        {/* Fret Grid Lines (Horizontal frets) */}
        {Array.from({ length: numFrets + 1 }).map((_, f) => {
          if (f === 0) return null; // Nut already drawn
          const y = margin.top + f * fretSpacing;
          return (
            <line
              key={`fret-${f}`}
              x1={margin.left}
              y1={y}
              x2={margin.left + gridWidth}
              y2={y}
              stroke="#2B3630"
              strokeWidth="1.2"
            />
          );
        })}

        {/* String Lines (Vertical strings from low E to high e) */}
        {Array.from({ length: numStrings }).map((_, s) => {
          const x = margin.left + s * stringSpacing;
          return (
            <line
              key={`string-${s}`}
              x1={x}
              y1={margin.top}
              x2={x}
              y2={margin.top + gridHeight}
              stroke="#2B3630"
              strokeWidth={s === 0 ? '1.8' : s === 5 ? '1.1' : '1.3'}
            />
          );
        })}

        {/* Above-Nut Markers (X for muted, O for open strings) */}
        {chord.diagram.frets.map((fret, s) => {
          const x = margin.left + s * stringSpacing;
          const status = stringStatus[s] || 'ok';
          const isProblemMuted = status === 'muted' && fret >= 0;

          if (fret === -1) {
            // Muted string (✕)
            return (
              <text
                key={`mute-${s}`}
                x={x}
                y={margin.top - 12}
                fill="#2B3630"
                fontSize="13"
                fontWeight="700"
                textAnchor="middle"
              >
                ✕
              </text>
            );
          } else if (fret === 0) {
            // Open string (○)
            return (
              <g key={`open-${s}`}>
                <circle
                  cx={x}
                  cy={margin.top - 14}
                  r="5"
                  fill="none"
                  stroke={isProblemMuted ? '#DC2626' : '#2B3630'}
                  strokeWidth="1.8"
                />
                {isProblemMuted && (
                  <circle cx={x} cy={margin.top - 14} r="2.5" fill="#DC2626" />
                )}
              </g>
            );
          }
          return null;
        })}

        {/* Finger Dots (Solid Forest Green Circles with Finger Numbers) */}
        {chord.diagram.frets.map((fret, s) => {
          if (fret <= 0) return null;
          const x = margin.left + s * stringSpacing;
          const y = margin.top + (fret - 0.5) * fretSpacing;
          const finger = chord.diagram.fingers[s];
          const status = stringStatus[s] || 'ok';
          const isProblem = status === 'muted';

          return (
            <g key={`dot-${s}`}>
              <circle
                cx={x}
                cy={y}
                r="11"
                fill={isProblem ? '#DC2626' : '#244230'}
                stroke={isProblem ? '#FCA5A5' : '#2E523C'}
                strokeWidth="1.5"
                filter="drop-shadow(0 2px 3px rgba(0,0,0,0.12))"
              />
              {finger > 0 && (
                <text
                  x={x}
                  y={y + 4}
                  fill="#FFFFFF"
                  fontSize="11"
                  fontWeight="700"
                  textAnchor="middle"
                >
                  {finger}
                </text>
              )}
            </g>
          );
        })}
      </svg>
    </div>
  );
};
