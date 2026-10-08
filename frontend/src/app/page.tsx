'use client';

import React, { useState } from 'react';
import { CHORD_LIBRARY, LESSON_SEQUENCE } from '@/lib/chords';
import { ChordDiagram } from '@/components/ChordDiagram';
import { StrummingPattern } from '@/components/StrummingPattern';
import { VoiceAssistantCard } from '@/components/VoiceAssistantCard';
import { useGuitarAudio } from '@/lib/useGuitarAudio';
import {
  ChevronRight,
  Headphones,
  Pause,
  Play,
  ArrowRight,
  Mic,
  HelpCircle,
  Lightbulb,
} from 'lucide-react';

export default function GuitarCoPilotPage() {
  const { state, startSession, endSession, playChordAudio, setState } = useGuitarAudio();
  const [selectedChordIndex, setSelectedChordIndex] = useState(0);

  const activeChordId = LESSON_SEQUENCE[selectedChordIndex] || 'G_Major';
  const chordData = CHORD_LIBRARY[activeChordId] || CHORD_LIBRARY['G_Major'];

  const handleNextChord = () => {
    const nextIdx = (selectedChordIndex + 1) % LESSON_SEQUENCE.length;
    setSelectedChordIndex(nextIdx);
    const nextChordId = LESSON_SEQUENCE[nextIdx];
    setState((prev) => ({ ...prev, targetChord: nextChordId }));
  };

  const handleSelectChord = (index: number) => {
    setSelectedChordIndex(index);
    const nextChordId = LESSON_SEQUENCE[index];
    setState((prev) => ({ ...prev, targetChord: nextChordId }));
  };

  return (
    <div className="min-h-screen bg-[#F6F5EE] text-[#1A231F] flex flex-col justify-between p-4 md:p-8 selection:bg-[#E3ECE6]">
      {/* ── TOP HEADER ────────────────────────────────────────────────────────── */}
      <header className="max-w-5xl mx-auto w-full flex items-center justify-between pb-6 border-b border-[#E8E4DA]/60">
        {/* Brand Logo */}
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-full bg-[#2E4638] flex items-center justify-center text-white">
            {/* Guitar headstock / fret logo mark */}
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="9" />
              <line x1="8" y1="12" x2="16" y2="12" />
              <line x1="12" y1="8" x2="12" y2="16" />
            </svg>
          </div>
          <div className="flex items-baseline gap-2">
            <span className="font-extrabold text-lg tracking-tight text-[#1C2520]">fret</span>
            <span className="text-xs text-[#707872] font-medium hidden sm:inline">
              | Your guitar co-pilot
            </span>
          </div>
        </div>

        {/* Status & Session Control */}
        <div className="flex items-center gap-4 sm:gap-6 text-xs font-semibold text-[#576059]">
          <div className="flex items-center gap-1.5">
            <span className={`w-2 h-2 rounded-full ${state.connected ? 'bg-[#2E7D32] animate-pulse' : 'bg-[#9E9B91]'}`} />
            <span>{state.connected ? 'Practice in progress' : 'Ready to begin'}</span>
          </div>

          <button
            onClick={state.connected ? endSession : startSession}
            className="hover:text-[#1A231F] transition-colors"
          >
            {state.connected ? 'End session' : 'Connect microphone'}
          </button>

          {/* User Avatar */}
          <div className="w-7 h-7 rounded-full bg-[#E5DFD3] text-[#2E4638] font-bold flex items-center justify-center text-xs shadow-inner">
            A
          </div>
        </div>
      </header>

      {/* ── MAIN LESSON STAGE ─────────────────────────────────────────────────── */}
      <main className="max-w-5xl mx-auto w-full flex-1 my-6 flex flex-col justify-center">
        {/* Lesson Title Header */}
        <div className="flex flex-col sm:flex-row sm:items-end justify-between mb-6 gap-3">
          <div>
            <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-[#1A231F]">
              Let’s play your first chords.
            </h1>
            <p className="text-sm text-[#6C756E] mt-1 font-normal">
              One shape at a time. No rush, just you and your guitar.
            </p>
          </div>

          <div className="text-left sm:text-right">
            <span className="text-xs font-semibold text-[#1C2520] block">
              The essentials · Lesson 01
            </span>
            <span className="text-xs text-[#707872]">
              3 chords · About 5 minutes
            </span>
          </div>
        </div>

        {/* Breadcrumb Steps Pill Bar */}
        <div className="flex items-center justify-between mb-5">
          <div className="flex items-center gap-2 sm:gap-3 flex-wrap">
            {LESSON_SEQUENCE.map((chordId, idx) => {
              const chord = CHORD_LIBRARY[chordId];
              const isActive = idx === selectedChordIndex;
              return (
                <React.Fragment key={chordId}>
                  <button
                    onClick={() => handleSelectChord(idx)}
                    className={`flex items-center gap-2 px-3.5 py-1.5 rounded-full text-xs font-semibold transition-all ${
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

          <span className="text-[11px] font-bold uppercase tracking-wider text-[#8A928B] hidden sm:block">
            CHORD {selectedChordIndex + 1} OF {LESSON_SEQUENCE.length}
          </span>
        </div>

        {/* ── HERO CHORD CARD ─────────────────────────────────────────────────── */}
        <div className="bg-white border border-[#E8E4DA] rounded-2xl p-6 sm:p-8 shadow-[0_2px_12px_rgba(0,0,0,0.03)]">
          {/* Card Top Title */}
          <div className="text-center mb-6">
            <span className="text-[11px] font-bold uppercase tracking-widest text-[#7C857E]">
              YOUR CURRENT CHORD
            </span>
            <h2 className="text-3xl sm:text-4xl font-extrabold text-[#1A231F] mt-1 tracking-tight">
              {chordData.name}
            </h2>
            <p className="text-sm text-[#6C756E] mt-1 font-medium">
              {chordData.subtitle}
            </p>
          </div>

          {/* 3-Column Layout: Fingers | Diagram & Mic | Reminder */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 items-start pt-2">
            {/* Left Column: Finger Placements */}
            <div className="flex flex-col gap-3">
              <h3 className="text-xs font-bold text-[#1C2520] mb-1">
                Where your fingers go
              </h3>

              {chordData.fingerPlacements.map((fp) => (
                <div key={fp.fingerNumber} className="flex items-center gap-3">
                  <div className="w-6 h-6 rounded-full bg-[#EDEAE1] text-[#2E4638] text-xs font-bold flex items-center justify-center shrink-0">
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

            {/* Center Column: Chord Diagram & Voice Assistant */}
            <div className="flex flex-col items-center">
              <ChordDiagram
                chord={chordData}
                stringStatus={state.stringStatus}
              />

              <VoiceAssistantCard
                agentState={state.agentState}
                detectedChord={state.detectedChord}
                confidence={state.confidence}
                inversion={state.inversion}
                feedbackText={state.feedbackText}
                onMicClick={state.connected ? endSession : startSession}
              />
            </div>

            {/* Right Column: Pedagogical Reminder & Audio Preview */}
            <div className="flex flex-col gap-4 md:pl-4">
              <div className="bg-[#FAF9F5] border border-[#E8E4DA] rounded-xl p-3.5">
                <div className="flex items-center gap-1.5 text-xs font-bold text-[#1C2520] mb-1.5">
                  <Lightbulb size={14} className="text-[#7C857E]" />
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

          {/* Strumming Pattern Arrows (User-specified requirement) */}
          <StrummingPattern
            name={chordData.strummingPattern.name}
            meter={chordData.strummingPattern.meter}
            beats={chordData.strummingPattern.beats}
          />
        </div>
      </main>

      {/* ── BOTTOM STAGE BAR ──────────────────────────────────────────────────── */}
      <footer className="max-w-5xl mx-auto w-full pt-4">
        <div className="flex flex-col sm:flex-row items-center justify-between gap-4 py-3">
          {/* Left: Reassuring Teacher Encouragement */}
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-full bg-[#EDEAE1] flex items-center justify-center text-[#525B54]">
              <Headphones size={16} />
            </div>
            <div>
              <h4 className="text-xs font-bold text-[#1C2520]">Progress, not perfection.</h4>
              <p className="text-[11px] text-[#6C756E]">Stay with this chord as long as you like.</p>
            </div>
          </div>

          {/* Right: Pause & Next Chord Actions */}
          <div className="flex items-center gap-3">
            <button
              onClick={() => setState((p) => ({ ...p, agentState: p.agentState === 'listening' ? 'disconnected' : 'listening' }))}
              className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-semibold border border-[#DCD7CA] bg-white text-[#2B352E] hover:bg-[#F2EFE7] transition-colors"
            >
              <Pause size={13} />
              <span>Pause</span>
            </button>

            <button
              onClick={handleNextChord}
              className="flex items-center gap-2 px-5 py-2 rounded-lg text-xs font-bold bg-[#2E4638] text-white hover:bg-[#24392D] transition-all shadow-sm"
            >
              <span>Next chord</span>
              <ArrowRight size={14} />
            </button>
          </div>
        </div>

        {/* Micro Sub-footer */}
        <div className="flex items-center justify-between text-[11px] text-[#8C948D] pt-4 border-t border-[#E8E4DA]/60 mt-1">
          <div className="flex items-center gap-1.5">
            <Mic size={12} />
            <span>Built-in microphone · Standard 440Hz acoustic tuning</span>
          </div>
          <button className="flex items-center gap-1 hover:text-[#1A231F] transition-colors">
            <HelpCircle size={12} />
            <span>Need a hand?</span>
          </button>
        </div>
      </footer>
    </div>
  );
}
