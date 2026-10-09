'use client';

import React from 'react';

interface AcousticRingsProps {
  state: 'disconnected' | 'listening' | 'thinking' | 'speaking';
}

export const AcousticRings: React.FC<AcousticRingsProps> = ({ state }) => {
  const isSpeaking = state === 'speaking';
  const isListening = state === 'listening';
  const isThinking = state === 'thinking';
  const isActive = isSpeaking || isListening || isThinking;

  return (
    <div className="flex items-center justify-center my-4 py-1 select-none">
      <div className="relative flex items-center justify-center w-24 h-16">
        {/* Subtle Ambient Radial Glow */}
        {isActive && (
          <div
            className={`absolute w-20 h-20 rounded-full blur-xl transition-all duration-700 pointer-events-none ${
              isSpeaking
                ? 'bg-blue-500/15 scale-125'
                : isThinking
                ? 'bg-emerald-500/10 scale-110'
                : 'bg-slate-400/10 scale-100'
            }`}
          />
        )}

        {/* Concentric Acoustic Rings SVG */}
        <svg
          viewBox="0 0 160 80"
          className="w-32 h-16 overflow-visible"
          xmlns="http://www.w3.org/2000/svg"
        >
          {/* Ring 1 (Innermost Center) */}
          <ellipse
            cx="80"
            cy="40"
            rx="10"
            ry="9"
            fill="none"
            stroke={isSpeaking ? '#93C5FD' : isActive ? '#64748B' : '#334155'}
            strokeWidth="1.8"
            className={`transition-all duration-300 ${
              isSpeaking
                ? 'animate-pulse'
                : isListening
                ? 'opacity-80'
                : 'opacity-40'
            }`}
          />

          {/* Ring 2 */}
          <ellipse
            cx="80"
            cy="40"
            rx="20"
            ry="16"
            fill="none"
            stroke={isSpeaking ? '#60A5FA' : isActive ? '#475569' : '#1E293B'}
            strokeWidth="1.6"
            className={`transition-all duration-500 ${
              isSpeaking
                ? 'animate-[ping_2.4s_cubic-bezier(0,0,0.2,1)_infinite]'
                : isListening
                ? 'opacity-70'
                : 'opacity-30'
            }`}
          />

          {/* Ring 3 */}
          <ellipse
            cx="80"
            cy="40"
            rx="32"
            ry="23"
            fill="none"
            stroke={isSpeaking ? '#3B82F6' : isActive ? '#475569' : '#1E293B'}
            strokeWidth="1.4"
            className={`transition-all duration-700 ${
              isSpeaking
                ? 'animate-[pulse_1.8s_ease-in-out_infinite]'
                : isListening
                ? 'opacity-60'
                : 'opacity-25'
            }`}
          />

          {/* Ring 4 */}
          <ellipse
            cx="80"
            cy="40"
            rx="45"
            ry="30"
            fill="none"
            stroke={isSpeaking ? '#2563EB' : isActive ? '#334155' : '#0F172A'}
            strokeWidth="1.2"
            className={`transition-all duration-1000 ${
              isSpeaking
                ? 'opacity-70'
                : isListening
                ? 'opacity-40'
                : 'opacity-15'
            }`}
          />

          {/* Ring 5 (Outermost subtle wave) */}
          <ellipse
            cx="80"
            cy="40"
            rx="58"
            ry="36"
            fill="none"
            stroke={isSpeaking ? '#1D4ED8' : isActive ? '#334155' : '#0F172A'}
            strokeWidth="1.0"
            strokeDasharray="4 4"
            className={`transition-all duration-1000 ${
              isSpeaking
                ? 'opacity-50'
                : isListening
                ? 'opacity-30'
                : 'opacity-10'
            }`}
          />
        </svg>
      </div>
    </div>
  );
};
