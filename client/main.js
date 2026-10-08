/**
 * main.js — Browser glue for the Zeplin Guitar Co-Pilot
 *
 * Responsibilities:
 *   1. Request microphone with noise suppression OFF (preserves guitar harmonics)
 *   2. Set up AudioContext + AudioWorklet (guitar-dsp-processor)
 *   3. Connect to LiveKit room and subscribe/publish tracks
 *   4. Forward AudioWorklet events → LiveKit DataChannel → Go Orchestrator
 *   5. Receive Go UIStateUpdate events from DataChannel → update UI state object
 *
 * NOTE: LiveKit JS SDK must be loaded separately via a <script> tag in index.html.
 * This file uses window.LivekitClient which is provided by the UMD bundle.
 */

'use strict';

// ── Configuration (read from the Go-served /config endpoint or window globals) ──
const LIVEKIT_URL        = window.ZEPLIN_LIVEKIT_URL    || 'ws://localhost:7880';
const LIVEKIT_TOKEN      = window.ZEPLIN_LIVEKIT_TOKEN  || '';
const WORKLET_PATH       = '/worklet/guitar-dsp-processor.js';

// ── UI State ──────────────────────────────────────────────────────────────────
// Minimal state object — the full fretboard UI is Phase 4.
// For now, we expose this on window so the console can inspect it.
window.ZeplinState = {
  connected:      false,
  calibrated:     false,
  targetChord:    '',
  detectedChord:  '',
  confidence:     0,
  streak:         0,
  lastFeedback:   '',
  lastChordEvent: null,
};

// ── Main Entry Point ──────────────────────────────────────────────────────────
async function startZeplin() {
  log('Zeplin Guitar Co-Pilot starting...');

  // 1. Microphone access — CRITICAL: disable noise suppression & AGC
  //    Standard speech denoisers erase guitar harmonics and sustain.
  let micStream;
  try {
    micStream = await navigator.mediaDevices.getUserMedia({
      audio: {
        echoCancellation:  true,  // keeps agent TTS out of the analysis path
        noiseSuppression:  false, // MUST be false — would destroy guitar harmonics
        autoGainControl:   false, // MUST be false — would compress strumming dynamics
        channelCount:      1,
        sampleRate:        48000,
      },
    });
    log('Microphone acquired (48kHz, no noise suppression)');
  } catch (err) {
    logErr('Microphone access denied:', err);
    return;
  }

  // 2. AudioContext at native 48kHz
  const audioCtx = new AudioContext({ sampleRate: 48000, latencyHint: 'interactive' });
  if (audioCtx.state === 'suspended') await audioCtx.resume();

  // 3. Load the AudioWorklet processor (self-contained, no imports needed)
  try {
    await audioCtx.audioWorklet.addModule(WORKLET_PATH);
    log('AudioWorklet module loaded');
  } catch (err) {
    logErr('Failed to load AudioWorklet module:', err);
    return;
  }

  // 4. Create the DSP worklet node and connect the microphone
  const dspNode = new AudioWorkletNode(audioCtx, 'guitar-dsp-processor', {
    numberOfInputs:  1,
    numberOfOutputs: 0, // no audio output needed from DSP node
    channelCount:    1,
    channelCountMode: 'explicit',
  });

  const micSource = audioCtx.createMediaStreamSource(micStream);
  micSource.connect(dspNode);
  log('Microphone → AudioWorklet pipeline active');

  // 5. Connect to LiveKit
  let token = LIVEKIT_TOKEN;
  let serverUrl = LIVEKIT_URL;

  if (!token) {
    try {
      log('Fetching LiveKit session token from /api/token...');
      const resp = await fetch('/api/token');
      if (resp.ok) {
        const data = await resp.json();
        token = data.token;
        serverUrl = data.url || serverUrl;
        log('Token minted successfully for room:', data.room);
      } else {
        log('Token endpoint returned status ' + resp.status + '. Running in local DSP-only mode.');
      }
    } catch (e) {
      log('Could not fetch token from /api/token. Running in local DSP-only mode.');
    }
  }

  if (!token) {
    log('No token available. Running in standalone DSP-only mode.');
    attachWorkletHandler(dspNode, null);
    return;
  }

  const room = new window.LivekitClient.Room({
    adaptiveStream: true,
    dynacast:       true,
  });

  room.on(window.LivekitClient.RoomEvent.Connected, () => {
    log('Connected to LiveKit room:', room.name);
    window.ZeplinState.connected = true;
  });

  room.on(window.LivekitClient.RoomEvent.Disconnected, () => {
    log('Disconnected from LiveKit room');
    window.ZeplinState.connected = false;
  });

  // 6. Handle incoming DataChannel messages (Go → Browser UI state updates)
  room.on(window.LivekitClient.RoomEvent.DataReceived, (payload, participant) => {
    try {
      const text = new TextDecoder().decode(payload);
      const msg  = JSON.parse(text);
      if (msg.event === 'lesson_state_update') {
        window.ZeplinState.targetChord  = msg.target_chord    || '';
        window.ZeplinState.streak       = msg.current_streak  || 0;
        window.ZeplinState.lastFeedback = msg.feedback_text   || '';
        // Phase 4: update visual fretboard here
        log(`[UI] Target: ${msg.target_chord} | Streak: ${msg.current_streak} | ${msg.feedback_text}`);
      }
    } catch (e) {
      logErr('Failed to parse DataChannel message:', e);
    }
  });

  try {
    await room.connect(serverUrl, token);
  } catch (err) {
    logErr('LiveKit connection failed:', err);
    return;
  }

  // 7. Wire AudioWorklet events → LiveKit DataChannel
  attachWorkletHandler(dspNode, room);

  // 8. Start tuning calibration sequence
  runTuningCalibration(dspNode, room);
}

// ── Tuning Calibration Sequence ───────────────────────────────────────────────
async function runTuningCalibration(dspNode, room) {
  log('[Calibration] Starting tuning check — pluck your low E string when prompted...');
  // Signal the worklet to enter calibration mode
  dspNode.port.postMessage({ type: 'start_calibration' });

  // Listen for the first calibration F0 event
  const calibrationPromise = new Promise((resolve) => {
    const handler = (e) => {
      if (e.data.event === 'calibration_f0') {
        dspNode.port.removeEventListener('message', handler);
        resolve(e.data);
      }
    };
    dspNode.port.addEventListener('message', handler);
    dspNode.port.start(); // Required to receive messages from the worklet
  });

  // Wait up to 15 seconds for the student to pluck the E string
  const timeout    = new Promise((_, rej) => setTimeout(() => rej(new Error('timeout')), 15000));
  let calEvt;
  try {
    calEvt = await Promise.race([calibrationPromise, timeout]);
  } catch {
    log('[Calibration] No string detected within 15s — proceeding with A=440 standard tuning.');
    dspNode.port.postMessage({ type: 'stop_calibration' });
    return;
  }

  dspNode.port.postMessage({ type: 'stop_calibration' });

  // Compute offset from E2 = 82.41 Hz
  const referenceE2 = 82.41;
  const offsetCents = 1200 * Math.log2(calEvt.hz / referenceE2);
  log(`[Calibration] Detected ${calEvt.note} at ${calEvt.hz} Hz → offset: ${offsetCents.toFixed(1)} cents`);
  window.ZeplinState.calibrated = true;

  // Push offset to worklet for all subsequent chroma calculations
  dspNode.port.postMessage({ type: 'set_tuning_offset', offsetCents });

  // Forward to Go orchestrator via DataChannel
  if (room) {
    const payload = JSON.stringify({ event: 'calibration_f0', hz: calEvt.hz, note: calEvt.note });
    room.localParticipant.publishData(
      new TextEncoder().encode(payload),
      { reliable: true }
    );
  }
}

// ── Worklet Event → DataChannel ───────────────────────────────────────────────
function attachWorkletHandler(dspNode, room) {
  dspNode.port.onmessage = (e) => {
    const msg = e.data;

    if (msg.event === 'chord_detected') {
      // Update local state for Phase 4 UI
      window.ZeplinState.detectedChord  = msg.detected_chord;
      window.ZeplinState.confidence     = msg.confidence;
      window.ZeplinState.lastChordEvent = msg;

      log(
        `[Chord] ${msg.detected_chord} | bass=${msg.bass_note} | conf=${msg.confidence.toFixed(2)}` +
        ` | inv=${msg.inversion} | rms=${msg.rms_dbfs}dBFS`
      );

      // Forward to Go Orchestrator via LiveKit DataChannel
      if (room && window.ZeplinState.connected) {
        try {
          room.localParticipant.publishData(
            new TextEncoder().encode(JSON.stringify(msg)),
            { reliable: true }
          );
        } catch (err) {
          logErr('DataChannel publish failed:', err);
        }
      }
    }
  };

  // Start the port to enable message events
  dspNode.port.start();
}

// ── Logging ───────────────────────────────────────────────────────────────────
function log(...args) {
  console.log('[Zeplin]', ...args);
  appendToLogPanel(args.join(' '));
}
function logErr(...args) {
  console.error('[Zeplin ERROR]', ...args);
  appendToLogPanel('ERROR: ' + args.join(' '));
}
function appendToLogPanel(text) {
  const el = document.getElementById('log-panel');
  if (!el) return;
  const line = document.createElement('div');
  line.textContent = `[${new Date().toLocaleTimeString()}] ${text}`;
  el.appendChild(line);
  el.scrollTop = el.scrollHeight;
}

// ── Auto-start on page load ───────────────────────────────────────────────────
window.addEventListener('DOMContentLoaded', () => {
  const btn = document.getElementById('start-btn');
  if (btn) {
    btn.addEventListener('click', () => {
      btn.disabled = true;
      btn.textContent = 'Connecting...';
      startZeplin().catch((err) => {
        logErr('startZeplin failed:', err);
        btn.disabled = false;
        btn.textContent = 'Retry';
      });
    });
  }
});
