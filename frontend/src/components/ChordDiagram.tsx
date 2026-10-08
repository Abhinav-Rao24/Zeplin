'use strict';

import React from 'react';
import { ChordData } from '@/lib/chords';

interface ChordDiagramProps {
  chord: ChordData;
  stringStatus?: string[]; // 6 strings: "ok" | "muted"
}

export const ChordDiagram: React.FC<ChordDiagramProps> = ({ chord, stringStatus = [] }) => {
  const width = 200;
  const height = 190;
  const margin = { top: 28, left: 24, right: 24, bottom: 22 };

  const gridWidth = width - margin.left - margin.right;
  const gridHeight = height - margin.top - margin.bottom;

  const numStrings = 6;
  const numFrets = 4;
  const stringSpacing = gridWidth / (numStrings - 1);
  const fretSpacing = gridHeight / numFrets;

  const stringLabels = ['E', 'A', 'D', 'G', 'B', 'e'];

  return (
    <div className="flex flex-col items-center">
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className="w-full max-w-[210px] h-auto select-none"
        xmlns="http://www.w3.org/2000/svg"
      >
        {/* Top Nut Bar (Thick horizontal bar representing fret 0) */}
        <rect
          x={margin.left - 1}
          y={margin.top - 3}
          width={gridWidth + 2}
          height={4}
          fill="#1C2420"
          rx={1}
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
              stroke="#D3D0C8"
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
              stroke="#B3B0A6"
              strokeWidth={s === 0 ? '1.8' : s === 5 ? '1.0' : '1.3'}
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
                y={margin.top - 10}
                fill="#8C887E"
                fontSize="12"
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
                  cy={margin.top - 12}
                  r="4.5"
                  fill="none"
                  stroke={isProblemMuted ? '#DC2626' : '#6A726C'}
                  strokeWidth="1.6"
                />
                {isProblemMuted && (
                  <circle cx={x} cy={margin.top - 12} r="2.5" fill="#DC2626" />
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
                r="10"
                fill={isProblem ? '#DC2626' : '#3E5C4E'}
                stroke={isProblem ? '#FCA5A5' : '#4E7262'}
                strokeWidth="1.5"
                filter="drop-shadow(0 1px 2px rgba(0,0,0,0.15))"
              />
              {finger > 0 && (
                <text
                  x={x}
                  y={y + 3.5}
                  fill="#FFFFFF"
                  fontSize="9.5"
                  fontWeight="700"
                  textAnchor="middle"
                >
                  {finger}
                </text>
              )}
            </g>
          );
        })}

        {/* String Labels (E A D G B e below grid) */}
        {stringLabels.map((lbl, s) => {
          const x = margin.left + s * stringSpacing;
          return (
            <text
              key={`label-${s}`}
              x={x}
              y={margin.top + gridHeight + 14}
              fill="#8F8C82"
              fontSize="9"
              fontWeight="600"
              textAnchor="middle"
            >
              {lbl}
            </text>
          );
        })}
      </svg>
    </div>
  );
};
