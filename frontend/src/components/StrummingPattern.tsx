'use client';

import React, { useState, useEffect } from 'react';
import { StrumBeat } from '@/lib/chords';
import { ArrowDown, ArrowUp, Play, Square } from 'lucide-react';

interface StrummingPatternProps {
  name: string;
  meter: string;
  beats: StrumBeat[];
}

export const StrummingPattern: React.FC<StrummingPatternProps> = ({
  name,
  meter,
  beats,
}) => {
  const [isPlaying, setIsPlaying] = useState(false);
  const [activeBeatIndex, setActiveBeatIndex] = useState<number>(-1);

  useEffect(() => {
    let interval: NodeJS.Timeout | null = null;
    if (isPlaying) {
      // 80 BPM, 8th notes = 375ms per subdivision
      const msPerBeat = 375;
      interval = setInterval(() => {
        setActiveBeatIndex((prev) => (prev + 1) % beats.length);
      }, msPerBeat);
    } else {
      setActiveBeatIndex(-1);
    }
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [isPlaying, beats.length]);

  return (
    <div className="w-full bg-[#FAF9F5] border border-[#E8E4DA] rounded-xl p-3.5 mt-4">
      <div className="flex items-center justify-between mb-2.5">
        <div className="flex items-center gap-2">
          <span className="text-[11px] font-bold uppercase tracking-wider text-[#737C75]">
            Strumming Pattern ({meter})
          </span>
          <span className="text-xs text-[#2E4638] font-semibold bg-[#E8ECE8] px-2 py-0.5 rounded-full">
            {name}
          </span>
        </div>

        <button
          onClick={() => setIsPlaying(!isPlaying)}
          className="flex items-center gap-1.5 text-xs font-semibold px-2.5 py-1 rounded-md text-[#2B4337] bg-white border border-[#DDD9CE] hover:bg-[#F2EFE7] transition-colors"
        >
          {isPlaying ? (
            <>
              <Square size={12} className="fill-[#2B4337]" />
              <span>Stop Rhythm</span>
            </>
          ) : (
            <>
              <Play size={12} className="fill-[#2B4337]" />
              <span>Practice Rhythm</span>
            </>
          )}
        </button>
      </div>

      {/* Rhythmic Arrows Bar */}
      <div className="grid grid-cols-8 gap-1.5 pt-1">
        {beats.map((beat, idx) => {
          const isActive = idx === activeBeatIndex;
          return (
            <div
              key={`strum-${idx}`}
              className={`flex flex-col items-center justify-center p-1.5 rounded-lg border transition-all duration-150 ${
                isActive
                  ? 'bg-[#2E4638] text-white border-[#2E4638] scale-105 shadow-sm'
                  : 'bg-white border-[#E8E4DA] text-[#2E4638]'
              }`}
            >
              {/* Arrow Indicator */}
              <div className="h-6 flex items-center justify-center">
                {beat.arrow === 'down' ? (
                  <ArrowDown
                    size={18}
                    className={`stroke-[2.5] ${
                      isActive ? 'text-white' : beat.accent ? 'text-[#1E3027]' : 'text-[#3E5C4E]'
                    }`}
                  />
                ) : beat.arrow === 'up' ? (
                  <ArrowUp
                    size={18}
                    className={`stroke-[2.5] ${
                      isActive ? 'text-white' : 'text-[#527967]'
                    }`}
                  />
                ) : (
                  <span className={`text-base font-bold ${isActive ? 'text-white' : 'text-[#B0ACA0]'}`}>
                    ·
                  </span>
                )}
              </div>

              {/* Subdivided Beat Label */}
              <span
                className={`text-[10px] font-bold mt-0.5 ${
                  isActive ? 'text-[#E2ECE5]' : 'text-[#858C86]'
                }`}
              >
                {beat.beat}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
};
