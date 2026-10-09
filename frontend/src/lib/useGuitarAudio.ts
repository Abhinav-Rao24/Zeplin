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
  userSpeech?: string;
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
    feedbackText: 'Almost there. Your B string sounds muted, so lift your ring finger slightly and strum again.',
    userSpeech: '',
    stringStatus: ['ok', 'ok', 'ok', 'ok', 'ok', 'ok'],
  });

  const roomRef = useRef<LiveKit.Room | null>(null);
  const audioCtxRef = useRef<AudioContext | null>(null);
  const dspNodeRef = useRef<AudioWorkletNode | null>(null);
  const localTrackRef = useRef<LiveKit.LocalAudioTrack | null>(null);
  const recognitionRef = useRef<{ stop: () => void } | null>(null);

  const startSession = useCallback(async () => {
    try {
      // 1. Acquire raw mic via getUserMedia so the track stays alive the whole time.
      //    This is the single source of truth — we hand it to both DSP and LiveKit.
      const micStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,   // keep ON on Windows — disabling it drops AGC and kills VAD
          autoGainControl: true,
          sampleRate: 48000,
        },
      });
      const rawTrack = micStream.getAudioTracks()[0];
      // Wrap in a LiveKit LocalAudioTrack (userProvidedTrack=true means LiveKit won't stop it)
      const localAudioTrack = new LiveKit.LocalAudioTrack(rawTrack, undefined, true);
      localTrackRef.current = localAudioTrack;

      // 2. AudioContext at 48kHz for local DSP chord recognition
      const audioCtx = new AudioContext({ sampleRate: 48000, latencyHint: 'interactive' });
      audioCtxRef.current = audioCtx;
      if (audioCtx.state === 'suspended') await audioCtx.resume();

      // 3. Feed the same mic stream to DSP worklet
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
        console.warn('AudioWorklet registration notice:', dspErr);
      }

      // 4. Fetch token from Go backend
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
        const room = new LiveKit.Room({
          adaptiveStream: true,
          dynacast: true,
        });
        roomRef.current = room;

        room.on(LiveKit.RoomEvent.Connected, async () => {
          setState((prev) => ({
            ...prev,
            connected: true,
            agentState: 'listening',
            feedbackText: 'Zeplin connected. Strum your guitar or talk to me!',
          }));

          // Publish the local audio track
          try {
            await room.localParticipant.publishTrack(localAudioTrack, { name: 'student-audio' });
            console.log('[LiveKit] Student audio track published to room.');
          } catch (pubErr) {
            console.error('Failed to publish audio track:', pubErr);
          }

          // Single handshake with target chord
          setTimeout(() => {
            try {
              room.localParticipant.publishData(
                new TextEncoder().encode(
                  JSON.stringify({ event: 'student_connected', chord: 'G_Major' })
                ),
                { reliable: true }
              );
            } catch (e) {
              console.error('Handshake publish error:', e);
            }
          }, 250);
        });

        // Attach agent voice track (Deepgram TTS via LiveKit)
        room.on(LiveKit.RoomEvent.TrackSubscribed, (track: LiveKit.RemoteTrack) => {
          if (track.kind === LiveKit.Track.Kind.Audio) {
            const el = track.attach();
            el.autoplay = true;
            document.body.appendChild(el);
            console.log('[LiveKit] Agent audio track attached to DOM and playing.');
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

              if (fb) {
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
        setState((prev) => ({
          ...prev,
          connected: true,
          agentState: 'listening',
          feedbackText: 'Listening for your chords (standalone mode)...',
        }));
      }

      // Note: Browser SpeechRecognition is intentionally not used here.
      // It conflicts with our getUserMedia mic stream on Chrome/Windows.
      // All speech-to-text is handled server-side by Deepgram STT via the
      // LiveKit audio track, which fires transcript callbacks and routes them
      // through the orchestrator brain via orch.HandleSpeech.
      console.log('[Zeplin] Mic active. Speaking into Deepgram STT via LiveKit RTP pipeline.');
    } catch (err) {
      console.error('Failed to start guitar session:', err);
      setState((prev) => ({
        ...prev,
        connected: false,
        feedbackText: 'Microphone access denied. Please click to allow mic permissions.',
      }));
    }
  }, []);

  const endSession = useCallback(() => {
    if (recognitionRef.current) {
      try {
        recognitionRef.current.stop();
      } catch {}
      recognitionRef.current = null;
    }
    if (localTrackRef.current) {
      localTrackRef.current.stop();
      localTrackRef.current = null;
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
      userSpeech: '',
      feedbackText: 'Session stopped. Click below to start practicing.',
    }));
  }, []);

  // Web Audio Chord Synthesizer ("Repeat" chord audio)
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

  const changeTargetChord = useCallback((chordId: string) => {
    setState((prev) => ({
      ...prev,
      targetChord: chordId,
      streak: 0,
      agentState: 'listening',
      feedbackText: `Target changed to ${chordId.replace('_', ' ')}. Strum cleanly when ready.`,
    }));

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
