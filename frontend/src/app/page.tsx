'use client';

import React, { useEffect, useState } from 'react';
import { Mic, MicOff } from 'lucide-react';
import { CHORD_LIBRARY, ChordData } from '@/lib/chords';
import { HorizontalFretboard } from '@/components/HorizontalFretboard';
import { AcousticRings } from '@/components/AcousticRings';
import { useGuitarAudio } from '@/lib/useGuitarAudio';

export default function AmbientHUDPage() {
  const { state, isMicMuted, toggleMic, changeTargetChord, startSession, endSession } = useGuitarAudio();

  // Active target chord: defaults to state.targetChord or 'E_Minor'
  const currentChordId = state.targetChord || 'E_Minor';
  const chordData: ChordData | null =
    CHORD_LIBRARY[currentChordId] || CHORD_LIBRARY['E_Minor'] || null;

  const [hasStarted, setHasStarted] = useState(false);

  // Available beginner chords for one-click switching
  const quickChords = [
    { id: 'E_Minor', label: 'Em' },
    { id: 'A_Minor', label: 'Am' },
    { id: 'C_Major', label: 'C' },
    { id: 'G_Major', label: 'G' },
    { id: 'D_Major', label: 'D' },
    { id: 'E_Major', label: 'E' },
    { id: 'A_Major', label: 'A' },
  ];

  // Auto-connect audio session on user's first click or keypress
  const handleUserGesture = () => {
    if (!state.connected && !hasStarted) {
      setHasStarted(true);
      startSession();
    }
  };

  useEffect(() => {
    // Attempt auto-start if permissions allow, or bind to first window click
    const handleFirstClick = () => {
      if (!state.connected && !hasStarted) {
        setHasStarted(true);
        startSession();
      }
    };
    window.addEventListener('click', handleFirstClick, { once: true });
    return () => {
      window.removeEventListener('click', handleFirstClick);
    };
  }, [state.connected, hasStarted, startSession]);

  // Format chord display title: e.g. "A Minor (Am)"
  const chordTitle = chordData
    ? `${chordData.name.replace(/_/g, ' ')} (${chordData.letter})`
    : 'A Minor (Am)';

  return (
    <main
      onClick={handleUserGesture}
      className="min-h-screen w-full bg-[#111317] text-white flex flex-col justify-between p-6 sm:p-10 select-none overflow-hidden font-sans"
    >
      {/* ── 1. MINIMAL TOP STATUS BAR ───────────────────────────────── */}
      <header className="flex items-start justify-between w-full max-w-[1200px] mx-auto pt-2">
        {/* Left: Branding & Connection Status & Dedicated Mic Toggle */}
        <div className="flex items-center gap-3 sm:gap-4 flex-wrap">
          <span className="text-xs sm:text-sm font-semibold tracking-widest text-[#8E949E] uppercase">
            ZEPLIN // GUITAR CO-PILOT
          </span>

          {/* Connection status pill */}
          <button
            onClick={(e) => {
              e.stopPropagation();
              if (state.connected) {
                endSession();
              } else {
                startSession();
              }
            }}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-[#1A1D24] border border-[#2D3139] text-xs font-medium cursor-pointer transition-colors hover:border-[#3E434D]"
            title={state.connected ? 'Click to disconnect' : 'Click to connect'}
          >
            <span
              className={`w-2 h-2 rounded-full transition-colors ${
                state.connected
                  ? 'bg-[#4ADE80] shadow-[0_0_8px_rgba(74,222,128,0.5)]'
                  : 'bg-[#F59E0B]'
              }`}
            />
            <span className={state.connected ? 'text-[#8E949E]' : 'text-[#F59E0B]'}>
              {state.connected ? 'Connected' : 'Connecting...'}
            </span>
          </button>

          {/* Dedicated Microphone Toggle & Status Button */}
          {state.connected && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                toggleMic();
              }}
              className={`flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border transition-colors cursor-pointer ${
                isMicMuted
                  ? 'bg-[#2E1A1A] border-[#EF4444]/40 text-[#EF4444] hover:bg-[#3D1E1E]'
                  : 'bg-[#15271E] border-[#22C55E]/40 text-[#4ADE80] hover:bg-[#1C362A]'
              }`}
              title={isMicMuted ? 'Microphone is muted. Click to unmute' : 'Microphone is live. Click to mute'}
            >
              {isMicMuted ? (
                <>
                  <MicOff size={13} className="text-[#EF4444]" />
                  <span>MIC MUTED</span>
                </>
              ) : (
                <>
                  <Mic size={13} className="text-[#4ADE80] animate-pulse" />
                  <span>MIC ON</span>
                </>
              )}
            </button>
          )}
        </div>

        {/* Right: Low-contrast Monospace Telemetry */}
        <div className="text-right font-mono text-[11px] sm:text-xs text-[#6B7280] leading-snug tracking-tight">
          <p className="text-[10px] text-[#4B5563] uppercase">WebRTC telemetry</p>
          <p>RTT: 42ms</p>
          <p>TTFT: 110ms</p>
        </div>
      </header>

      {/* ── 2. CENTRAL DYNAMIC CHORD TARGET & FRETBOARD GRID ────────── */}
      <section className="flex flex-col items-center justify-center my-auto w-full max-w-[1100px] mx-auto text-center">
        {/* Central Chord Title & Sub-labels */}
        <div className="mb-4 sm:mb-6">
          {chordData ? (
            <>
              <h1 className="text-3xl sm:text-4xl md:text-5xl font-bold tracking-tight text-[#F1F3F5]">
                {chordTitle}
              </h1>
              <p className="text-xs sm:text-sm tracking-[0.2em] font-semibold text-[#8E949E] uppercase mt-2">
                FINGER POSITIONING
              </p>
              <p className="text-xs sm:text-sm font-medium text-[#4ADE80] mt-0.5">
                Target Chord
              </p>
            </>
          ) : (
            <p className="text-lg sm:text-xl text-[#6B7280] font-normal italic">
              Say a chord to begin (e.g. &ldquo;Show me G major&rdquo;)
            </p>
          )}

          {/* Quick Chord Selector Pills */}
          <div className="flex items-center justify-center gap-2 mt-4 flex-wrap">
            {quickChords.map((qc) => {
              const isActive = currentChordId === qc.id;
              return (
                <button
                  key={qc.id}
                  onClick={(e) => {
                    e.stopPropagation();
                    changeTargetChord(qc.id);
                  }}
                  className={`px-3 py-1 rounded-full text-xs font-semibold tracking-wide transition-all ${
                    isActive
                      ? 'bg-[#3B82F6] text-white shadow-[0_0_10px_rgba(59,130,246,0.4)] border border-[#60A5FA]'
                      : 'bg-[#1E222B] text-[#9CA3AF] hover:bg-[#282D37] hover:text-[#E5E7EB] border border-[#2D3139]'
                  }`}
                >
                  {qc.label}
                </button>
              );
            })}
          </div>
        </div>

        {/* 3. Progressive 6-String Horizontal Fretboard Grid */}
        <div className="w-full">
          <HorizontalFretboard
            chord={chordData}
            stringStatus={state.stringStatus}
            isBlank={!chordData}
          />
        </div>

        {/* 4. Acoustic Feedback Rings (Voice Activity Indicator) */}
        <div className="mt-6 mb-2">
          <AcousticRings state={state.agentState} />
        </div>
      </section>

      {/* ── 5. GLANCEABLE CAPTIONS (BOTTOM CENTER) ───────────────────── */}
      <footer className="w-full max-w-[950px] mx-auto text-center pb-4 px-4">
        <p className="text-base sm:text-lg md:text-[20px] font-normal text-[#E5E7EB] leading-relaxed tracking-normal">
          {state.feedbackText ||
            'Great choice! Place your 1st finger on the B string (1st fret), 2nd finger on the D string (2nd fret), and 3rd on the G string (2nd fret). Let me hear it!'}
        </p>

        {/* Real-time speech confirmation (subtle indicator) */}
        {state.userSpeech && (
          <p className="text-xs text-[#9CA3AF] mt-2 italic font-mono">
            You: &ldquo;{state.userSpeech}&rdquo;
          </p>
        )}
      </footer>
    </main>
  );
}
