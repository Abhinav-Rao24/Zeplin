'use client';

import React from 'react';
import { Mic, Volume2, Sparkles, AlertCircle } from 'lucide-react';

interface VoiceAssistantProps {
  agentState: 'disconnected' | 'listening' | 'thinking' | 'speaking';
  detectedChord: string;
  confidence: number;
  inversion: boolean;
  feedbackText: string;
  onMicClick?: () => void;
}

export const VoiceAssistantCard: React.FC<VoiceAssistantProps> = ({
  agentState,
  detectedChord,
  confidence,
  inversion,
  feedbackText,
  onMicClick,
}) => {
  return (
    <div className="flex flex-col items-center justify-center mt-5 w-full">
      {/* Central Circular Voice / Listening Button */}
      <button
        onClick={onMicClick}
        className={`relative w-14 h-14 rounded-full flex items-center justify-center transition-all duration-300 shadow-sm ${
          agentState === 'speaking'
            ? 'bg-[#314B3E] text-white shadow-md'
            : agentState === 'thinking'
            ? 'bg-[#D6E2D9] text-[#243B30]'
            : agentState === 'listening'
            ? 'bg-[#E3EBE5] text-[#2E4638] hover:bg-[#D4E0D7]'
            : 'bg-[#EDE9DF] text-[#787E79]'
        }`}
      >
        {/* Animated Listening Pulse Ring */}
        {agentState === 'listening' && (
          <span className="absolute inset-0 rounded-full border-2 border-[#385B4A] opacity-60 animate-ping pointer-events-none" />
        )}

        {agentState === 'speaking' ? (
          <Volume2 size={24} className="stroke-[2.2]" />
        ) : agentState === 'thinking' ? (
          <Sparkles size={22} className="stroke-[2.2] animate-spin" />
        ) : (
          <Mic size={24} className="stroke-[2.2]" />
        )}
      </button>

      {/* Main Status Text */}
      <div className="text-center mt-3 max-w-sm">
        <h4 className="text-[15px] font-semibold text-[#1A231F]">
          {agentState === 'speaking'
            ? 'Zeplin is speaking...'
            : agentState === 'thinking'
            ? 'Analyzing chord harmonics...'
            : agentState === 'listening'
            ? 'Listening for your chord...'
            : 'Co-pilot offline'}
        </h4>

        {/* Dynamic Instructional Feedback / Quote */}
        <p className="text-[13px] text-[#5A635D] mt-1 leading-snug">
          {feedbackText ? `"${feedbackText}"` : 'Give it a gentle strum when you\'re ready.'}
        </p>

        {/* Live Signal Telemetry Badge */}
        {detectedChord && (
          <div className="inline-flex items-center gap-1.5 mt-2.5 px-2.5 py-1 rounded-full text-xs font-semibold bg-[#E8EFEA] text-[#243B30]">
            <span>Heard:</span>
            <span className="font-bold">{detectedChord.replace('_', ' ')}</span>
            <span className="text-[10px] text-[#55695D]">
              ({Math.round(confidence * 100)}% match)
            </span>
            {inversion && (
              <span className="flex items-center gap-0.5 text-red-600 font-bold ml-1">
                <AlertCircle size={11} /> Wrong bass
              </span>
            )}
          </div>
        )}
      </div>
    </div>
  );
};
