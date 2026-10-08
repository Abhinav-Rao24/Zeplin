'use client';

import { useState, useEffect, useRef, useCallback } from 'react';
import * as LiveKit from 'livekit-client';

export interface GuitarAudioState {
  connected: boolean;
  agentState: 'disconnected' | 'listening' | 'thinking' | 'speaking';
  targetChord: string;
  detectedChord: string;
  confidence: number;
  inversion: boolean;
  streak: number;
  feedbackText: string;
  stringStatus: string[];
}

export function useGuitarAudio() {
  const [state, setState] = useState<GuitarAudioState>({
    connected: false,
    agentState: 'disconnected',
    targetChord: 'G_Major',
    detectedChord: '',
    confidence: 0,
    inversion: false,
    streak: 0,
    feedbackText: "Click the mic to start your AI guitar lesson. Zeplin will greet you.",
    stringStatus: ['ok', 'ok', 'ok', 'ok', 'ok', 'ok'],
  });

  const roomRef = useRef<LiveKit.Room | null>(null);
  const audioCtxRef = useRef<AudioContext | null>(null);
  const dspNodeRef = useRef<AudioWorkletNode | null>(null);
  const livekitAudioActiveRef = useRef<boolean>(false);

  // Reliable vocalizer: Speaks via browser speech synthesis as immediate verbal feedback
  const speakVoice = useCallback((text: string) => {
    if (typeof window !== 'undefined' && 'speechSynthesis' in window) {
      try {
        window.speechSynthesis.cancel();
        const utterance = new SpeechSynthesisUtterance(text);
        utterance.rate = 1.0;
        utterance.pitch = 1.0;
        const voices = window.speechSynthesis.getVoices();
        const preferred = voices.find(
          (v) =>
            v.lang.startsWith('en') &&
            (v.name.includes('Natural') ||
              v.name.includes('Google') ||
              v.name.includes('Samantha') ||
              v.name.includes('Daniel') ||
              v.name.includes('Alex'))
        ) || voices.find((v) => v.lang.startsWith('en'));
        if (preferred) utterance.voice = preferred;

        utterance.onend = () => {
          setState((prev) => ({
            ...prev,
            agentState: prev.connected ? 'listening' : 'disconnected',
          }));
        };

        window.speechSynthesis.speak(utterance);
      } catch (err) {
        console.warn('SpeechSynthesis error:', err);
      }
    }
  }, []);

  const startSession = useCallback(async () => {
    try {
      // 1. Microphone access without destructive noise suppression
      const micStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: false,
          autoGainControl: false,
          sampleRate: 48000,
        },
      });

      // 2. AudioContext at 48kHz
      const audioCtx = new AudioContext({ sampleRate: 48000, latencyHint: 'interactive' });
      audioCtxRef.current = audioCtx;
      if (audioCtx.state === 'suspended') await audioCtx.resume();

      // 3. Register DSP AudioWorklet
      try {
        await audioCtx.audioWorklet.addModule('/worklet/guitar-dsp-processor.js');
        const dspNode = new AudioWorkletNode(audioCtx, 'guitar-dsp-processor', {
          numberOfInputs: 1,
          numberOfOutputs: 0,
          channelCount: 1,
        });
        dspNodeRef.current = dspNode;

        const source = audioCtx.createMediaStreamSource(micStream);
        source.connect(dspNode);

        // DSP message receiver
        dspNode.port.onmessage = (e) => {
          const msg = e.data;
          if (msg.event === 'chord_detected') {
            setState((prev) => ({
              ...prev,
              detectedChord: msg.detected_chord,
              confidence: msg.confidence,
              inversion: msg.inversion,
            }));

            if (roomRef.current && roomRef.current.state === LiveKit.ConnectionState.Connected) {
              try {
                roomRef.current.localParticipant.publishData(
                  new TextEncoder().encode(JSON.stringify(msg)),
                  { reliable: true }
                );
              } catch (err) {
                console.error('DataChannel publish failed:', err);
              }
            }
          }
        };
        dspNode.port.start();
      } catch (dspErr) {
        console.warn('AudioWorklet registration note:', dspErr);
      }

      // 4. Immediately trigger warm verbal greeting so user hears Zeplin speak
      const greeting = "Hey there! Welcome to Zeplin. I'm your guitar co-pilot. Let's start with G major. Give me a strum when you're ready.";
      setState((prev) => ({
        ...prev,
        connected: true,
        agentState: 'speaking',
        feedbackText: greeting,
      }));
      speakVoice(greeting);

      // 5. Connect to LiveKit if Go backend is reachable
      let token = '';
      let livekitUrl = 'ws://localhost:7880';
      try {
        const res = await fetch('/api/token?identity=student-1');
        if (res.ok) {
          const data = await res.json();
          token = data.token;
          livekitUrl = data.url || livekitUrl;
        }
      } catch (e) {
        console.warn('Backend /api/token not reached:', e);
      }

      if (token) {
        const room = new LiveKit.Room({ adaptiveStream: true, dynacast: true });
        roomRef.current = room;

        room.on(LiveKit.RoomEvent.Connected, () => {
          const audioTrack = micStream.getAudioTracks()[0];
          if (audioTrack) {
            room.localParticipant.publishTrack(audioTrack, { name: 'student-audio' });
          }

          // Trigger backend handshake
          setTimeout(() => {
            try {
              room.localParticipant.publishData(
                new TextEncoder().encode(JSON.stringify({ event: 'student_connected' })),
                { reliable: true }
              );
            } catch (e) {
              console.error('Handshake publish error:', e);
            }
          }, 400);
        });

        // Attach agent voice track if backend Deepgram TTS is streaming
        room.on(LiveKit.RoomEvent.TrackSubscribed, (track: LiveKit.RemoteTrack) => {
          if (track.kind === LiveKit.Track.Kind.Audio) {
            livekitAudioActiveRef.current = true;
            const el = track.attach();
            el.autoplay = true;
            document.body.appendChild(el);
          }
        });

        room.on(LiveKit.RoomEvent.Disconnected, () => {
          setState((prev) => ({ ...prev, connected: false, agentState: 'disconnected' }));
        });

        // DataChannel updates from Go orchestrator
        room.on(LiveKit.RoomEvent.DataReceived, (payload: Uint8Array) => {
          try {
            const text = new TextDecoder().decode(payload);
            const msg = JSON.parse(text);
            if (msg.event === 'lesson_state_update') {
              const fb = msg.feedback_text || '';
              setState((prev) => ({
                ...prev,
                targetChord: msg.target_chord || prev.targetChord,
                streak: msg.current_streak ?? prev.streak,
                feedbackText: fb || prev.feedbackText,
                stringStatus: msg.string_status || prev.stringStatus,
                agentState: fb ? 'speaking' : 'listening',
              }));

              if (fb && !livekitAudioActiveRef.current) {
                speakVoice(fb);
              }
            }
          } catch (err) {
            console.error('DataChannel parse error:', err);
          }
        });

        await room.connect(livekitUrl, token);
      }
    } catch (err) {
      console.error('Failed to start guitar session:', err);
      // Fallback greeting even if mic permission prompt has delay
      const fallbackGreeting = "Hey there! Welcome to Zeplin. Let's practice G major together. Click again to enable microphone.";
      setState((prev) => ({
        ...prev,
        connected: false,
        feedbackText: fallbackGreeting,
      }));
      speakVoice(fallbackGreeting);
    }
  }, [speakVoice]);

  const endSession = useCallback(() => {
    if (typeof window !== 'undefined' && 'speechSynthesis' in window) {
      window.speechSynthesis.cancel();
    }
    if (roomRef.current) {
      roomRef.current.disconnect();
      roomRef.current = null;
    }
    if (audioCtxRef.current) {
      audioCtxRef.current.close();
      audioCtxRef.current = null;
    }
    setState((prev) => ({
      ...prev,
      connected: false,
      agentState: 'disconnected',
      feedbackText: 'Session ended. Click the mic to practice again.',
    }));
  }, []);

  // Web Audio Chord Synthesizer ("Hear this chord")
  const playChordAudio = useCallback((notes: string[]) => {
    try {
      const ctx = new (window.AudioContext ||
        (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext)();
      const noteFreqs: Record<string, number> = {
        E2: 82.41,
        A2: 110.0,
        D3: 146.83,
        G3: 196.0,
        B3: 246.94,
        E4: 329.63,
        G2: 98.0,
        B2: 123.47,
        C3: 130.81,
        C4: 261.63,
        G4: 392.0,
        F4: 349.23,
        'F#4': 369.99,
        'C#4': 277.18,
        'G#3': 207.65,
      };

      notes.forEach((note, i) => {
        const freq = noteFreqs[note] || 220;
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();

        osc.type = 'triangle';
        osc.frequency.setValueAtTime(freq, ctx.currentTime);

        const startTime = ctx.currentTime + i * 0.04; // strum sweep
        const duration = 1.4;

        gain.gain.setValueAtTime(0, startTime);
        gain.gain.linearRampToValueAtTime(0.18, startTime + 0.02);
        gain.gain.exponentialRampToValueAtTime(0.001, startTime + duration);

        osc.connect(gain);
        gain.connect(ctx.destination);

        osc.start(startTime);
        osc.stop(startTime + duration);
      });
    } catch (e) {
      console.error('Failed to play chord audio:', e);
    }
  }, []);

  const changeTargetChord = useCallback(
    (chordId: string) => {
      const chordDisplay = chordId.replace('_', ' ');
      const switchMsg = `Switching to ${chordDisplay}. Place your fingers and strum cleanly.`;
      setState((prev) => ({
        ...prev,
        targetChord: chordId,
        streak: 0,
        agentState: 'speaking',
        feedbackText: switchMsg,
      }));
      speakVoice(switchMsg);

      if (roomRef.current && roomRef.current.state === LiveKit.ConnectionState.Connected) {
        try {
          roomRef.current.localParticipant.publishData(
            new TextEncoder().encode(JSON.stringify({ event: 'set_target_chord', chord: chordId })),
            { reliable: true }
          );
        } catch (e) {
          console.error('Failed to set target chord:', e);
        }
      }
    },
    [speakVoice]
  );

  useEffect(() => {
    return () => {
      endSession();
    };
  }, [endSession]);

  return {
    state,
    startSession,
    endSession,
    changeTargetChord,
    playChordAudio,
    setState,
  };
}
