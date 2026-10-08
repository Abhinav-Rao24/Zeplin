'use client';

import React, { useState } from 'react';
import { CHORD_LIBRARY, LESSON_SEQUENCE } from '@/lib/chords';
import { ChordDiagram } from '@/components/ChordDiagram';
import { StrummingPattern } from '@/components/StrummingPattern';
import { useGuitarAudio } from '@/lib/useGuitarAudio';
import {
  ChevronRight,
  Play,
  ArrowRight,
  Mic,
  Volume2,
  AlertCircle,
  Square,
  MessageSquare,
} from 'lucide-react';

export default function GuitarCoPilotPage() {
  const { state, startSession, endSession, changeTargetChord, playChordAudio } = useGuitarAudio();
  const [selectedChordIndex, setSelectedChordIndex] = useState(0);

  const activeChordId = LESSON_SEQUENCE[selectedChordIndex] || 'G_Major';
  const chordData = CHORD_LIBRARY[activeChordId] || CHORD_LIBRARY['G_Major'];

  const handleNextChord = () => {
    const nextIdx = (selectedChordIndex + 1) % LESSON_SEQUENCE.length;
    setSelectedChordIndex(nextIdx);
    const nextChordId = LESSON_SEQUENCE[nextIdx];
    changeTargetChord(nextChordId);
  };

  const handleSelectChord = (index: number) => {
    setSelectedChordIndex(index);
    const nextChordId = LESSON_SEQUENCE[index];
    changeTargetChord(nextChordId);
  };

  return (
    <div className="min-h-screen bg-[#F6F5EE] text-[#1A231F] flex flex-col justify-between p-4 sm:p-8 font-sans selection:bg-[#E3ECE6]">
      {/* ── TOP HEADER ────────────────────────────────────────────────────────── */}
      <header className="max-w-5xl mx-auto w-full flex items-center justify-between pb-5 border-b border-[#E8E4DA]/80">
        {/* Brand Logo & Subtitle */}
        <div className="flex items-center gap-2.5">
          <span className="font-extrabold text-xl tracking-tight text-[#1A231F]">zeplin</span>
          <span className="text-gray-300 font-light">|</span>
          <span className="text-xs sm:text-sm text-[#5D6660] font-medium">Your guitar co-pilot</span>
        </div>

        {/* Status Indicator (NOT A BUTTON) & User Avatar */}
        <div className="flex items-center gap-3">
          <div
            className={`flex items-center gap-2 px-3 py-1 rounded-full text-xs font-semibold transition-all ${
              state.connected
                ? 'bg-[#E3ECE6] text-[#2E4638]'
                : 'bg-[#EDEAE1] text-[#757D77]'
            }`}
          >
            <span
              className={`w-2 h-2 rounded-full ${
                state.connected ? 'bg-[#2E4638] animate-pulse' : 'bg-[#9BA29D]'
              }`}
            />
            <span>{state.connected ? 'Voice Co-Pilot Active' : 'Offline'}</span>
          </div>

          {/* User Avatar */}
          <div className="w-8 h-8 rounded-full bg-[#E5DFD3] text-[#2E4638] font-bold flex items-center justify-center text-xs shadow-inner">
            A
          </div>
        </div>
      </header>

      {/* ── MAIN STAGE ────────────────────────────────────────────────────────── */}
      <main className="max-w-5xl mx-auto w-full flex-1 my-6 flex flex-col justify-center">
        {/* Lesson Subheader Row */}
        <div className="flex flex-col sm:flex-row sm:items-end justify-between mb-5 gap-3">
          <div>
            <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#1A231F]">
              Let’s play your first chords.
            </h1>
            <p className="text-sm text-[#6C756E] mt-0.5 font-normal">
              One shape at a time. Zeplin listens as you play and guides your technique.
            </p>
          </div>

          <div className="text-left sm:text-right shrink-0">
            <span className="text-xs font-bold text-[#1C2520] block">
              The essentials · Lesson 01
            </span>
            <span className="text-xs text-[#707872]">
              3 chords · AI Voice Mode
            </span>
          </div>
        </div>

        {/* Chord Step Pills Bar */}
        <div className="flex items-center justify-between mb-4 flex-wrap gap-2">
          <div className="flex items-center gap-2 sm:gap-3 flex-wrap">
            {LESSON_SEQUENCE.map((chordId, idx) => {
              const chord = CHORD_LIBRARY[chordId];
              const isActive = idx === selectedChordIndex;
              return (
                <React.Fragment key={chordId}>
                  <button
                    onClick={() => handleSelectChord(idx)}
                    className={`flex items-center gap-1.5 px-3.5 py-1.5 rounded-full text-xs font-semibold transition-all ${
                      isActive
                        ? 'bg-[#2E4638] text-white shadow-sm'
                        : 'bg-[#EDEAE1] text-[#69726A] hover:bg-[#E3DFD5]'
                    }`}
                  >
                    <span className="font-bold">{chord.letter}</span>
                    <span>{chord.name}</span>
                    {isActive && <span className="w-1.5 h-1.5 rounded-full bg-white ml-0.5" />}
                  </button>

                  {idx < LESSON_SEQUENCE.length - 1 && (
                    <ChevronRight size={14} className="text-[#A39E92]" />
                  )}
                </React.Fragment>
              );
            })}
          </div>

          {/* Clean Streak Indicator */}
          <div className="flex items-center gap-2">
            <span className="text-[11px] font-bold uppercase tracking-wider text-[#7A837C]">
              Clean streak:
            </span>
            <div className="flex items-center gap-1">
              {[0, 1, 2, 3, 4].map((i) => (
                <div
                  key={i}
                  className={`w-2.5 h-2.5 rounded-full border transition-all ${
                    i < state.streak
                      ? 'bg-[#2E4638] border-[#2E4638] shadow-sm'
                      : 'bg-transparent border-[#B5B0A4]'
                  }`}
                />
              ))}
            </div>
          </div>
        </div>

        {/* ── MAIN WHITE LESSON CARD ──────────────────────────────────────── */}
        <div className="bg-white border border-[#E8E4DA] rounded-2xl p-6 sm:p-8 shadow-[0_2px_12px_rgba(0,0,0,0.03)]">
          {/* Top Title Bar of the Card */}
          <div className="text-center mb-6">
            <span className="text-[11px] font-bold uppercase tracking-widest text-[#7C857E]">
              YOUR CURRENT CHORD
            </span>
            <h2 className="text-3xl sm:text-4xl font-extrabold text-[#1A231F] mt-0.5 tracking-tight">
              {chordData.name}
            </h2>
            <p className="text-xs sm:text-sm text-[#6C756E] mt-1 font-medium">
              {chordData.subtitle}
            </p>
          </div>

          {/* 3-Column Core: Where your fingers go | Chord Box Diagram & Voice Co-Pilot | Reminder */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 items-start">
            {/* Left Column: Numbered Finger Placements */}
            <div className="flex flex-col gap-3">
              <h3 className="text-xs font-bold uppercase tracking-wider text-[#6B756E] mb-1">
                Where your fingers go
              </h3>

              {chordData.fingerPlacements.map((fp) => (
                <div key={fp.fingerNumber} className="flex items-center gap-3">
                  <div className="w-6 h-6 rounded-full bg-[#EDEAE1] text-[#2E4638] text-xs font-extrabold flex items-center justify-center shrink-0">
                    {fp.fingerNumber}
                  </div>
                  <div>
                    <h4 className="text-xs font-bold text-[#1C2520] leading-snug">
                      {fp.fingerName}
                    </h4>
                    <p className="text-[11px] text-[#6C756E] leading-tight">
                      {fp.stringName}
                    </p>
                  </div>
                </div>
              ))}
            </div>

            {/* Center Column: Interactive Vertical Chord Box Diagram & The SINGLE Voice Co-Pilot Control */}
            <div className="flex flex-col items-center">
              {/* Vertical Fretboard SVG */}
              <ChordDiagram
                chord={chordData}
                stringStatus={state.stringStatus}
              />

              {/* ── THE ONLY VOICE ASSISTANT MIC / CO-PILOT BUTTON ──────── */}
              <div className="w-full mt-4 flex flex-col items-center">
                {!state.connected ? (
                  <button
                    onClick={startSession}
                    className="flex items-center gap-2.5 px-6 py-2.5 rounded-full text-xs font-bold bg-[#2E4638] text-white hover:bg-[#23382C] transition-all shadow-sm"
                  >
                    <Mic size={15} />
                    <span>Start Voice Co-Pilot</span>
                  </button>
                ) : (
                  <div className="flex flex-col items-center">
                    <div className="flex items-center gap-2 px-5 py-2 rounded-full text-xs font-bold bg-[#2E4638] text-white shadow-sm ring-4 ring-[#2E4638]/15">
                      {state.agentState === 'speaking' ? (
                        <>
                          <Volume2 size={16} className="animate-bounce" />
                          <span>Zeplin Speaking...</span>
                        </>
                      ) : (
                        <>
                          <div className="flex items-center gap-0.5">
                            <span className="w-1 h-3 bg-white rounded-full animate-pulse" />
                            <span className="w-1 h-4 bg-white rounded-full animate-pulse delay-75" />
                            <span className="w-1 h-2 bg-white rounded-full animate-pulse delay-150" />
                          </div>
                          <Mic size={15} />
                          <span>Listening to You &amp; Guitar</span>
                        </>
                      )}
                    </div>

                    <button
                      onClick={endSession}
                      className="mt-2 text-[11px] font-semibold text-[#8C938E] hover:text-[#2E4638] flex items-center gap-1 transition-colors"
                    >
                      <Square size={9} />
                      <span>Stop Voice Session</span>
                    </button>
                  </div>
                )}

                {/* Live Speech Subtitle Strip */}
                <div className="mt-2.5 w-full max-w-[280px] text-center">
                  <p className="text-xs font-semibold text-[#1A231F] italic leading-tight">
                    &ldquo;{state.feedbackText}&rdquo;
                  </p>

                  {/* Student Spoken Transcript (shows Zeplin hears your speech) */}
                  {state.userSpeech && (
                    <div className="mt-1.5 flex items-center justify-center gap-1 text-[11px] text-[#2E4638] font-medium">
                      <MessageSquare size={11} />
                      <span className="italic">You: &ldquo;{state.userSpeech}&rdquo;</span>
                    </div>
                  )}

                  {/* Chord Detection Telemetry Badge */}
                  {state.detectedChord && (
                    <div className="inline-flex items-center gap-1.5 mt-2 px-2.5 py-0.5 rounded-full text-[11px] bg-[#EAF0EC] text-[#243A2E] font-medium">
                      <span>Detected:</span>
                      <strong className="text-[#1A231F]">{state.detectedChord.replace('_', ' ')}</strong>
                      <span className="text-[#59665E]">({Math.round(state.confidence * 100)}%)</span>
                      {state.inversion && (
                        <span className="flex items-center gap-0.5 text-red-600 font-bold">
                          <AlertCircle size={10} /> Low buzz
                        </span>
                      )}
                    </div>
                  )}
                </div>
              </div>
            </div>

            {/* Right Column: Technique Reminder & Hear Chord Audio */}
            <div className="flex flex-col gap-3 md:pl-2">
              <div className="bg-[#FAF9F5] border border-[#E8E4DA] rounded-xl p-3.5">
                <div className="flex items-center gap-1.5 text-xs font-bold text-[#1C2520] mb-1.5">
                  <span className="text-sm">💡</span>
                  <span>A little reminder</span>
                </div>
                <p className="text-xs text-[#576059] leading-relaxed">
                  {chordData.reminder}
                </p>
              </div>

              <button
                onClick={() => playChordAudio(chordData.notes)}
                className="flex items-center gap-2 text-xs font-semibold text-[#2E4638] hover:text-[#18261F] transition-colors py-1 px-1"
              >
                <div className="w-5 h-5 rounded-full border border-[#2E4638] flex items-center justify-center">
                  <Play size={10} className="fill-[#2E4638] ml-0.5" />
                </div>
                <span>Hear this chord</span>
              </button>
            </div>
          </div>

          {/* Strumming Pattern Arrows (Prominently displayed with Arrow blocks) */}
          <StrummingPattern
            name={chordData.strummingPattern.name}
            meter={chordData.strummingPattern.meter}
            beats={chordData.strummingPattern.beats}
          />
        </div>
      </main>

      {/* ── BOTTOM BAR ────────────────────────────────────────────────────────── */}
      <footer className="max-w-5xl mx-auto w-full flex items-center justify-between pt-4 border-t border-[#E8E4DA]/80">
        <p className="text-xs text-[#707872] italic font-serif">
          Progress, not perfection.
        </p>

        {/* Clean Single Action: Next Chord */}
        <button
          onClick={handleNextChord}
          className="flex items-center gap-1.5 px-4 py-1.5 rounded-full text-xs font-bold bg-[#2E4638] text-white hover:bg-[#23382C] transition-all shadow-sm"
        >
          <span>Next chord</span>
          <ArrowRight size={13} />
        </button>
      </footer>
    </div>
  );
}
