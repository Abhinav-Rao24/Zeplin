'use client';

import React, { useState } from 'react';
import { CHORD_LIBRARY, LESSON_SEQUENCE } from '@/lib/chords';
import { ChordDiagram } from '@/components/ChordDiagram';
import { StrummingPattern } from '@/components/StrummingPattern';
import { useGuitarAudio } from '@/lib/useGuitarAudio';
import {
  Mic,
  Volume2,
  X,
  Sparkles,
} from 'lucide-react';

export default function GuitarCoPilotPage() {
  const { state, startSession, endSession, changeTargetChord, playChordAudio } = useGuitarAudio();
  // Default to Chord 3 of 6 (G_Major) to match reference screenshot
  const [selectedChordIndex, setSelectedChordIndex] = useState(2);

  const activeChordId = LESSON_SEQUENCE[selectedChordIndex] || 'G_Major';
  const chordData = CHORD_LIBRARY[activeChordId] || CHORD_LIBRARY['G_Major'];

  const handleNextChord = () => {
    const nextIdx = (selectedChordIndex + 1) % LESSON_SEQUENCE.length;
    setSelectedChordIndex(nextIdx);
    const nextChordId = LESSON_SEQUENCE[nextIdx];
    changeTargetChord(nextChordId);
  };

  return (
    <div className="min-h-screen bg-[#F6F5EE] text-[#1A231F] flex flex-col items-center justify-center p-3 sm:p-6 font-sans selection:bg-[#E3ECE6]">
      {/* ── MAIN LESSON CARD ────────────────────────────────────────── */}
      <div className="max-w-[430px] w-full bg-white rounded-[32px] p-6 sm:p-7 shadow-[0_10px_35px_rgba(0,0,0,0.03)] border border-[#EBE7DF] flex flex-col justify-between min-h-[660px]">
        {/* Card Header */}
        <div>
          <div className="flex items-start justify-between">
            <div>
              <p className="text-xs text-[#717671] font-medium leading-none">
                Lesson 2 · Open chords
              </p>
              <h3 className="text-xs font-bold text-[#1B231E] mt-1.5 leading-none">
                Chord {selectedChordIndex + 1} of {LESSON_SEQUENCE.length}
              </h3>
            </div>

            <button
              onClick={() => setSelectedChordIndex(2)}
              className="text-[#1B231E] p-1 -mr-1 hover:opacity-60 transition-opacity"
              aria-label="Close"
            >
              <X size={19} className="stroke-[1.8]" />
            </button>
          </div>

          {/* 6-Segment Progress Bar */}
          <div className="flex items-center gap-1.5 mt-3.5 mb-5">
            {LESSON_SEQUENCE.map((_, idx) => (
              <div
                key={idx}
                className={`h-1 rounded-full flex-1 transition-all duration-300 ${
                  idx <= selectedChordIndex ? 'bg-[#244230]' : 'bg-[#E5E2DA]'
                }`}
              />
            ))}
          </div>

          {/* Hero Chord Title & Subtitle */}
          <div className="text-center mt-2 mb-3">
            <h1 className="text-4xl font-extrabold text-[#161E1A] tracking-tight">
              {chordData.name}
            </h1>
            <p className="text-sm font-normal text-[#606862] mt-1">
              {chordData.subtitle}
            </p>
          </div>

          {/* Fretboard SVG Diagram */}
          <div className="my-2 flex justify-center">
            <ChordDiagram
              chord={chordData}
              stringStatus={state.stringStatus}
            />
          </div>
        </div>

        {/* Middle Section: Dynamic Coaching Tip & Finger Positions Guide */}
        <div className="flex flex-col gap-2.5 my-3">
          {/* Dynamic Coach Tip Box (Matches Reference Screenshot) */}
          <div className="bg-[#EBF1EC] border border-[#DEE7DD] rounded-2xl p-4 text-[#1E2922] transition-all">
            <p className="text-[13px] leading-relaxed font-normal">
              {state.feedbackText ||
                'Almost there. Your B string sounds muted, so lift your ring finger slightly and strum again.'}
            </p>

            {/* Live Student Speech (Visual confirmation of real-time communication) */}
            {state.userSpeech && (
              <div className="mt-2 pt-2 border-t border-[#D5E1D4] flex items-center gap-1.5 text-xs text-[#244230] font-medium">
                <Sparkles size={12} className="shrink-0" />
                <span className="truncate italic">You: &ldquo;{state.userSpeech}&rdquo;</span>
              </div>
            )}
          </div>

          {/* Where Your Fingers Go (Finger Placement Guide) */}
          <div className="bg-[#FAF9F5] border border-[#EBE7DF] rounded-2xl p-3.5 flex flex-col gap-1.5">
            <span className="text-[10px] font-bold uppercase tracking-wider text-[#79817B]">
              Where your fingers go
            </span>
            <div className="grid grid-cols-1 gap-1.5">
              {chordData.fingerPlacements.map((fp) => (
                <div key={fp.fingerNumber} className="flex items-center gap-2.5">
                  <span className="w-4 h-4 rounded-full bg-[#244230] text-white text-[9.5px] font-bold flex items-center justify-center shrink-0">
                    {fp.fingerNumber}
                  </span>
                  <span className="text-xs font-semibold text-[#1C2520]">
                    {fp.fingerName}
                  </span>
                  <span className="text-xs text-[#6F7771]">
                    on {fp.stringName}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Strumming Pattern Rhythm Guide */}
          {chordData.strummingPattern && (
            <StrummingPattern
              name={chordData.strummingPattern.name}
              meter={chordData.strummingPattern.meter}
              beats={chordData.strummingPattern.beats}
            />
          )}
        </div>

        {/* ── BOTTOM INTERACTION CONTROLS ──────────────────────────── */}
        <div className="flex items-center justify-between pt-2 px-1">
          {/* Left: Repeat Button */}
          <button
            onClick={() => playChordAudio(chordData.notes)}
            className="px-5 py-2.5 rounded-full bg-white border border-[#DDD9CE] text-xs font-bold text-[#244230] shadow-sm hover:bg-[#F8F6F0] active:scale-95 transition-all cursor-pointer"
          >
            Repeat
          </button>

          {/* Center: Microphone Voice Assistant Button */}
          <div className="flex flex-col items-center">
            <button
              onClick={state.connected ? endSession : startSession}
              className={`w-14 h-14 rounded-full flex items-center justify-center text-white shadow-md hover:scale-105 active:scale-95 transition-all cursor-pointer ${
                state.agentState === 'speaking'
                  ? 'bg-[#244230] ring-4 ring-[#244230]/20 scale-105'
                  : state.connected
                  ? 'bg-[#244230] ring-4 ring-[#244230]/15'
                  : 'bg-[#244230]'
              }`}
              title={state.connected ? 'Click to stop listening' : 'Click to talk with Zeplin'}
            >
              {state.agentState === 'speaking' ? (
                <Volume2 size={23} className="animate-bounce" />
              ) : (
                <Mic size={23} className={state.connected ? 'animate-pulse' : ''} />
              )}
            </button>

            <span className="text-[11px] text-[#717872] font-medium mt-1.5 select-none">
              {state.connected
                ? state.agentState === 'speaking'
                  ? 'Zeplin speaking…'
                  : 'Listening…'
                : 'Listening…'}
            </span>
          </div>

          {/* Right: Skip Button */}
          <button
            onClick={handleNextChord}
            className="px-5 py-2.5 rounded-full bg-white border border-[#DDD9CE] text-xs font-bold text-[#244230] shadow-sm hover:bg-[#F8F6F0] active:scale-95 transition-all cursor-pointer"
          >
            Skip
          </button>
        </div>
      </div>
    </div>
  );
}
