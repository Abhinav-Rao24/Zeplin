'use client';

import React, { useState } from 'react';
import { CHORD_LIBRARY, LESSON_SEQUENCE } from '@/lib/chords';
import { ChordDiagram } from '@/components/ChordDiagram';
import { StrummingPattern } from '@/components/StrummingPattern';
import { useGuitarAudio } from '@/lib/useGuitarAudio';
import {
  ChevronRight,
  Headphones,
  Pause,
  Play,
  ArrowRight,
  Mic,
  MicOff,
  Sparkles,
  Volume2,
  AlertCircle,
  HelpCircle,
  Radio,
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
    <div className="min-h-screen bg-[#F6F5EE] text-[#1A231F] flex flex-col justify-between p-4 md:p-8 selection:bg-[#E3ECE6]">
      {/* ── TOP HEADER ────────────────────────────────────────────────────────── */}
      <header className="max-w-5xl mx-auto w-full flex items-center justify-between pb-5 border-b border-[#E8E4DA]/70">
        {/* Brand Logo */}
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-full bg-[#2E4638] flex items-center justify-center text-white shadow-sm">
            <Radio size={18} className="stroke-[2.2] animate-pulse" />
          </div>
          <div className="flex flex-col">
            <div className="flex items-baseline gap-2">
              <span className="font-extrabold text-lg tracking-tight text-[#1C2520]">Zeplin</span>
              <span className="text-xs text-[#6F7771] font-medium hidden sm:inline">
                | AI Guitar Voice Assistant
              </span>
            </div>
            <span className="text-[11px] text-[#2E4638] font-bold flex items-center gap-1">
              <Sparkles size={11} /> Real-Time DSP &amp; Speech Co-Pilot
            </span>
          </div>
        </div>

        {/* Status & Mic Session Toggle */}
        <div className="flex items-center gap-3 sm:gap-5">
          {/* Main Voice Assistant Activation Pill Button */}
          <button
            onClick={state.connected ? endSession : startSession}
            className={`flex items-center gap-2 px-3.5 py-1.5 rounded-full text-xs font-bold transition-all shadow-sm ${
              state.connected
                ? 'bg-[#2E4638] text-white hover:bg-[#243A2E]'
                : 'bg-white border border-[#D5D0C2] text-[#2E4638] hover:bg-[#F2EFE6]'
            }`}
          >
            {state.connected ? (
              <>
                <span className="w-2 h-2 rounded-full bg-[#4ADE80] animate-ping" />
                <Mic size={14} />
                <span>Voice Agent Active</span>
              </>
            ) : (
              <>
                <MicOff size={14} className="text-[#8C948E]" />
                <span>Turn On Mic &amp; Start</span>
              </>
            )}
          </button>

          {/* User Avatar */}
          <div className="w-8 h-8 rounded-full bg-[#E5DFD3] text-[#2E4638] font-bold flex items-center justify-center text-xs shadow-inner">
            A
          </div>
        </div>
      </header>

      {/* ── MAIN STAGE ────────────────────────────────────────────────────────── */}
      <main className="max-w-5xl mx-auto w-full flex-1 my-5 flex flex-col justify-center">
        {/* Lesson Title & Sequence Nav */}
        <div className="flex flex-col sm:flex-row sm:items-end justify-between mb-5 gap-3">
          <div>
            <h1 className="text-2xl sm:text-3xl font-extrabold tracking-tight text-[#1A231F]">
              Let’s play your first chords.
            </h1>
            <p className="text-sm text-[#6C756E] mt-0.5 font-normal">
              One shape at a time. Zeplin listens as you play and guides your technique.
            </p>
          </div>

          <div className="text-left sm:text-right">
            <span className="text-xs font-bold text-[#1C2520] block">
              The Essentials · Lesson 01
            </span>
            <span className="text-xs text-[#707872]">
              3 chords · Interactive Voice Mode
            </span>
          </div>
        </div>

        {/* Chord Step Pills Bar */}
        <div className="flex items-center justify-between mb-4">
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

          {/* Clean Streak Indicator */}
          <div className="flex items-center gap-2">
            <span className="text-[11px] font-bold uppercase tracking-wider text-[#7A837C]">
              Clean Streak:
            </span>
            <div className="flex items-center gap-1">
              {[0, 1, 2, 3, 4].map((i) => (
                <div
                  key={i}
                  className={`w-2.5 h-2.5 rounded-full border transition-all ${
                    i < state.streak
                      ? 'bg-[#2E7D32] border-[#2E7D32] shadow-sm'
                      : 'bg-transparent border-[#B5B0A4]'
                  }`}
                />
              ))}
            </div>
          </div>
        </div>

        {/* ── VOICE ASSISTANT HERO CARD ──────────────────────────────────────── */}
        <div className="bg-white border border-[#E8E4DA] rounded-2xl p-6 sm:p-7 shadow-[0_2px_16px_rgba(0,0,0,0.03)]">
          {/* Prominent Voice Assistant Live Speech Banner */}
          <div className="mb-6 p-4 rounded-xl bg-[#FAF9F5] border border-[#EAE6DC] flex flex-col sm:flex-row items-center gap-4">
            {/* Reactive Voice Orb / Audio Visualizer */}
            <div className="relative shrink-0 flex items-center justify-center">
              <div
                className={`w-14 h-14 rounded-full flex items-center justify-center transition-all duration-300 shadow-sm ${
                  state.agentState === 'speaking'
                    ? 'bg-[#2E4638] text-white scale-105'
                    : state.connected
                    ? 'bg-[#E2ECE5] text-[#2E4638]'
                    : 'bg-[#EAE5D9] text-[#787E79]'
                }`}
              >
                {state.connected && (
                  <span className="absolute inset-0 rounded-full border-2 border-[#2E4638] opacity-50 animate-ping pointer-events-none" />
                )}

                {state.agentState === 'speaking' ? (
                  <Volume2 size={24} className="stroke-[2.2] animate-bounce" />
                ) : (
                  <Mic size={24} className="stroke-[2.2]" />
                )}
              </div>
            </div>

            {/* Voice Dialogue & Live Spoken Transcript */}
            <div className="flex-1 text-center sm:text-left">
              <div className="flex items-center justify-center sm:justify-start gap-2 mb-1">
                <span className="text-xs font-extrabold uppercase tracking-wider text-[#2E4638] flex items-center gap-1">
                  <Sparkles size={12} />
                  {state.agentState === 'speaking'
                    ? 'Zeplin Speaking'
                    : state.connected
                    ? 'Zeplin Listening to Your Guitar'
                    : 'Voice Assistant Offline'}
                </span>

                {state.connected && (
                  <span className="text-[10px] font-bold px-2 py-0.5 rounded-full bg-[#E5ECE7] text-[#2B4337]">
                    Active 48kHz Stream
                  </span>
                )}
              </div>

              {/* Dynamic Spoken Feedback Subtitle */}
              <p className="text-sm font-semibold text-[#1A231F] italic leading-snug">
                {state.connected
                  ? `"${state.feedbackText}"`
                  : 'Click "Turn On Mic & Start" to activate your AI guitar teacher. Zeplin will greet you and guide your playing.'}
              </p>

              {/* Live Audio Detection Telemetry Pill */}
              {state.detectedChord && (
                <div className="inline-flex items-center gap-2 mt-2 px-2.5 py-0.5 rounded-full text-xs bg-[#EAF0EC] text-[#243A2E] font-medium">
                  <span>Detected Chord:</span>
                  <strong className="text-[#1A231F]">{state.detectedChord.replace('_', ' ')}</strong>
                  <span className="text-[#59665E]">({Math.round(state.confidence * 100)}% confidence)</span>
                  {state.inversion && (
                    <span className="flex items-center gap-0.5 text-red-600 font-bold">
                      <AlertCircle size={11} /> Low string ringing
                    </span>
                  )}
                </div>
              )}
            </div>

            {/* Quick Action Button */}
            {!state.connected && (
              <button
                onClick={startSession}
                className="shrink-0 px-4 py-2 rounded-lg text-xs font-bold bg-[#2E4638] text-white hover:bg-[#24392D] transition-colors shadow-sm"
              >
                Start Lesson &amp; Greet
              </button>
            )}
          </div>

          {/* Card Title Bar */}
          <div className="text-center mb-6">
            <span className="text-[11px] font-bold uppercase tracking-widest text-[#7C857E]">
              YOUR CURRENT CHORD
            </span>
            <h2 className="text-3xl sm:text-4xl font-extrabold text-[#1A231F] mt-0.5 tracking-tight">
              {chordData.name}
            </h2>
            <p className="text-sm text-[#6C756E] mt-1 font-medium">
              {chordData.subtitle}
            </p>
          </div>

          {/* 3-Column Core: Where your fingers go | Chord Box | Reminder */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 items-start pt-1">
            {/* Left Column: Numbered Finger Placements */}
            <div className="flex flex-col gap-3">
              <h3 className="text-xs font-bold uppercase tracking-wider text-[#6B756E] mb-0.5">
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

            {/* Center Column: Interactive Vertical Chord Box Diagram */}
            <div className="flex flex-col items-center">
              <ChordDiagram
                chord={chordData}
                stringStatus={state.stringStatus}
              />
            </div>

            {/* Right Column: Technique Reminder & Audio Sample */}
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

          {/* Strumming Pattern Arrows (Prominently displayed) */}
          <StrummingPattern
            name={chordData.strummingPattern.name}
            meter={chordData.strummingPattern.meter}
            beats={chordData.strummingPattern.beats}
          />
        </div>
      </main>

      {/* ── BOTTOM STAGE BAR ──────────────────────────────────────────────────── */}
      <footer className="max-w-5xl mx-auto w-full pt-3">
        <div className="flex flex-col sm:flex-row items-center justify-between gap-4 py-3">
          {/* Reassurance Encouragement */}
          <div className="flex items-center gap-3">
            <div className="w-8 h-8 rounded-full bg-[#EDEAE1] flex items-center justify-center text-[#525B54]">
              <Headphones size={16} />
            </div>
            <div>
              <h4 className="text-xs font-bold text-[#1C2520]">Progress, not perfection.</h4>
              <p className="text-[11px] text-[#6C756E]">Stay with this chord as long as you like.</p>
            </div>
          </div>

          {/* Navigation & Controls */}
          <div className="flex items-center gap-3">
            <button
              onClick={() => {
                if (state.connected) {
                  endSession();
                } else {
                  startSession();
                }
              }}
              className="flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-semibold border border-[#DCD7CA] bg-white text-[#2B352E] hover:bg-[#F2EFE7] transition-colors shadow-sm"
            >
              {state.connected ? <Pause size={13} /> : <Play size={13} />}
              <span>{state.connected ? 'Pause Session' : 'Resume'}</span>
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
            <span>48kHz microphone · Continuous DSP onset gating</span>
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
