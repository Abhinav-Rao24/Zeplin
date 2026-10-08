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
    feedbackText: "Give it a gentle strum when you're ready.",
    stringStatus: ['ok', 'ok', 'ok', 'ok', 'ok', 'ok'],
  });

  const roomRef = useRef<LiveKit.Room | null>(null);
  const audioCtxRef = useRef<AudioContext | null>(null);
  const dspNodeRef = useRef<AudioWorkletNode | null>(null);

  const startSession = useCallback(async () => {
    try {
      // 1. Microphone access without noise suppression (preserves guitar harmonics)
      const micStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: false,
          autoGainControl: false,
          sampleRate: 48000,
        },
      });

      // 2. AudioContext at native 48kHz
      const audioCtx = new AudioContext({ sampleRate: 48000, latencyHint: 'interactive' });
      audioCtxRef.current = audioCtx;
      if (audioCtx.state === 'suspended') await audioCtx.resume();

      // 3. Register AudioWorklet
      await audioCtx.audioWorklet.addModule('/worklet/guitar-dsp-processor.js');

      const dspNode = new AudioWorkletNode(audioCtx, 'guitar-dsp-processor', {
        numberOfInputs: 1,
        numberOfOutputs: 0,
        channelCount: 1,
      });
      dspNodeRef.current = dspNode;

      const source = audioCtx.createMediaStreamSource(micStream);
      source.connect(dspNode);

      // 4. Fetch token from Go backend
      let token = '';
      let livekitUrl = 'ws://localhost:7880';
      try {
        const res = await fetch('http://localhost:8080/api/token?identity=student-1');
        if (res.ok) {
          const data = await res.json();
          token = data.token;
          livekitUrl = data.url || livekitUrl;
        }
      } catch (e) {
        console.warn('Backend /api/token not reached, running in standalone mode:', e);
      }

      if (token) {
        const room = new LiveKit.Room({ adaptiveStream: true, dynacast: true });
        roomRef.current = room;

        room.on(LiveKit.RoomEvent.Connected, () => {
          setState((prev) => ({
            ...prev,
            connected: true,
            agentState: 'listening',
          }));
          // Publish mic audio to room
          const audioTrack = micStream.getAudioTracks()[0];
          room.localParticipant.publishTrack(audioTrack, { name: 'student-audio' });

          // Send student_connected handshake so Zeplin greets the student immediately with voice
          setTimeout(() => {
            try {
              room.localParticipant.publishData(
                new TextEncoder().encode(JSON.stringify({ event: 'student_connected' })),
                { reliable: true }
              );
            } catch (e) {
              console.error('Handshake publish error:', e);
            }
          }, 300);
        });

        // CRITICAL: Attach agent voice track to DOM so student hears Zeplin speaking
        room.on(LiveKit.RoomEvent.TrackSubscribed, (track: LiveKit.RemoteTrack) => {
          if (track.kind === LiveKit.Track.Kind.Audio) {
            const el = track.attach();
            el.autoplay = true;
            document.body.appendChild(el);
            console.log('[LiveKit] Agent audio track attached to DOM and active.');
          }
        });

        room.on(LiveKit.RoomEvent.Disconnected, () => {
          setState((prev) => ({ ...prev, connected: false, agentState: 'disconnected' }));
        });

        // Listen for Go orchestrator updates via DataChannel
        room.on(LiveKit.RoomEvent.DataReceived, (payload: Uint8Array) => {
          try {
            const text = new TextDecoder().decode(payload);
            const msg = JSON.parse(text);
            if (msg.event === 'lesson_state_update') {
              setState((prev) => ({
                ...prev,
                targetChord: msg.target_chord || prev.targetChord,
                streak: msg.current_streak ?? prev.streak,
                feedbackText: msg.feedback_text || prev.feedbackText,
                stringStatus: msg.string_status || prev.stringStatus,
                agentState: msg.feedback_text ? 'speaking' : 'listening',
              }));
              // Reset speaking state back to listening after short delay
              if (msg.feedback_text) {
                setTimeout(() => {
                  setState((prev) => ({ ...prev, agentState: 'listening' }));
                }, 4000);
              }
            }
          } catch (err) {
            console.error('DataChannel parse error:', err);
          }
        });

        await room.connect(livekitUrl, token);
      } else {
        // Standalone mode
        setState((prev) => ({
          ...prev,
          connected: true,
          agentState: 'listening',
        }));
      }

      // 5. Route DSP chord detections
      dspNode.port.onmessage = (e) => {
        const msg = e.data;
        if (msg.event === 'chord_detected') {
          setState((prev) => ({
            ...prev,
            detectedChord: msg.detected_chord,
            confidence: msg.confidence,
            inversion: msg.inversion,
          }));

          // Forward to Go backend over LiveKit DataChannel
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
    } catch (err) {
      console.error('Failed to start guitar session:', err);
    }
  }, []);

  const endSession = useCallback(() => {
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
    }));
  }, []);

  // Web Audio Chord Synthesizer ("Hear this chord")
  const playChordAudio = useCallback((notes: string[]) => {
    try {
      const ctx = new (window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext)();
      const noteFreqs: Record<string, number> = {
        E2: 82.41, A2: 110.00, D3: 146.83, G3: 196.00, B3: 246.94, E4: 329.63,
        G2: 98.00, B2: 123.47, C3: 130.81, C4: 261.63, G4: 392.00,
        F4: 349.23, 'F#4': 369.99, 'C#4': 277.18, 'G#3': 207.65,
      };

      notes.forEach((note, i) => {
        const freq = noteFreqs[note] || 220;
        const osc = ctx.createOscillator();
        const gain = ctx.createGain();

        osc.type = 'triangle'; // Warm acoustic guitar harmonic approximation
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

  const changeTargetChord = useCallback((chordId: string) => {
    setState((prev) => ({ ...prev, targetChord: chordId, streak: 0 }));
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
  }, []);

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
