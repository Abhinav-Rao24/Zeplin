/**
 * main.js — Browser Client for the Zeplin Guitar Co-Pilot
 *
 * Responsibilities:
 *   1. Request microphone with noise suppression OFF (preserves guitar harmonics)
 *   2. Set up AudioContext + AudioWorklet (guitar-dsp-processor)
 *   3. Connect to LiveKit room and subscribe/publish tracks
 *   4. Forward AudioWorklet events → LiveKit DataChannel → Go Orchestrator
 *   5. Receive Go UIStateUpdate events from DataChannel → update Fretboard and UI state
 *   6. Fetch and render practice statistics from /api/summary
 */

'use strict';

// ── Configuration ─────────────────────────────────────────────────────────────
const LIVEKIT_URL        = window.ZEPLIN_LIVEKIT_URL    || 'ws://localhost:7880';
const LIVEKIT_TOKEN      = window.ZEPLIN_LIVEKIT_TOKEN  || '';
const WORKLET_PATH       = '/worklet/guitar-dsp-processor.js';

// ── Global State ──────────────────────────────────────────────────────────────
window.ZeplinState = {
  connected:      false,
  calibrated:     false,
  targetChord:    'E_Minor',
  detectedChord:  '',
  confidence:     0,
  streak:         0,
  lastFeedback:   'Welcome to Zeplin. Press Connect & Start Lesson to begin.',
  lastChordEvent: null,
  stringStatus:   ['ok', 'ok', 'ok', 'ok', 'ok', 'ok'],
};

// ── Main Entry Point ──────────────────────────────────────────────────────────
async function startZeplin() {
  log('Starting Zeplin Guitar Co-Pilot...');

  // 1. Microphone access — CRITICAL: disable noise suppression & AGC
  //    Standard speech denoisers erase guitar harmonics and sustain.
  let micStream;
  try {
    micStream = await navigator.mediaDevices.getUserMedia({
      audio: {
        echoCancellation:  true,  // keeps agent TTS out of the analysis path
        noiseSuppression:  false, // MUST be false — preserves guitar string overtones
        autoGainControl:   false, // MUST be false — preserves strumming dynamics
        channelCount:      1,
        sampleRate:        48000,
      },
    });
    log('Microphone active (48kHz studio input, no speech filter)');
  } catch (err) {
    logErr('Microphone access denied:', err);
    alert('Please allow microphone access to use the Guitar Co-Pilot.');
    return;
  }

  // 2. AudioContext at native 48kHz
  const audioCtx = new AudioContext({ sampleRate: 48000, latencyHint: 'interactive' });
  if (audioCtx.state === 'suspended') await audioCtx.resume();

  // 3. Load the AudioWorklet processor (self-contained, no external imports)
  try {
    await audioCtx.audioWorklet.addModule(WORKLET_PATH);
    log('AudioWorklet DSP module registered');
  } catch (err) {
    logErr('Failed to load AudioWorklet module:', err);
    return;
  }

  // 4. Create the DSP worklet node and connect the microphone
  const dspNode = new AudioWorkletNode(audioCtx, 'guitar-dsp-processor', {
    numberOfInputs:   1,
    numberOfOutputs:  0,
    channelCount:     1,
    channelCountMode: 'explicit',
  });

  const micSource = audioCtx.createMediaStreamSource(micStream);
  micSource.connect(dspNode);
  log('DSP analysis pipeline active (FFT + 3-Gate + Chromagram)');

  // 5. Mint or fetch LiveKit session token
  let token = LIVEKIT_TOKEN;
  let serverUrl = LIVEKIT_URL;

  if (!token) {
    try {
      log('Requesting LiveKit session token from /api/token...');
      const resp = await fetch('/api/token?identity=student-1');
      if (resp.ok) {
        const data = await resp.json();
        token = data.token;
        serverUrl = data.url || serverUrl;
        log('Session token issued for room: ' + data.room);
      } else {
        log('Token API returned ' + resp.status + '. Running in local standalone DSP mode.');
      }
    } catch (e) {
      log('Could not reach /api/token. Running in local standalone DSP mode.');
    }
  }

  if (!token) {
    log('Running in local DSP-only mode without LiveKit agent.');
    attachWorkletHandler(dspNode, null);
    return;
  }

  // 6. Connect to LiveKit Room
  const room = new window.LivekitClient.Room({
    adaptiveStream: true,
    dynacast:       true,
  });

  room.on(window.LivekitClient.RoomEvent.Connected, () => {
    log('Connected to LiveKit classroom room: ' + room.name);
    window.ZeplinState.connected = true;

    // Publish local mic track to room so agent STT/VAD can hear the student
    room.localParticipant.publishTrack(micStream.getAudioTracks()[0], {
      name: 'student-audio',
    }).then(() => {
      log('Published student audio track to LiveKit room.');
    }).catch(err => {
      logErr('Failed to publish audio track:', err);
    });
  });

  room.on(window.LivekitClient.RoomEvent.Disconnected, () => {
    log('Disconnected from LiveKit room.');
    window.ZeplinState.connected = false;
  });

  // Handle incoming DataChannel messages (Go Orchestrator → Browser UI)
  room.on(window.LivekitClient.RoomEvent.DataReceived, (payload, participant) => {
    try {
      const text = new TextDecoder().decode(payload);
      const msg  = JSON.parse(text);
      if (msg.event === 'lesson_state_update') {
        window.ZeplinState.targetChord  = msg.target_chord    || '';
        window.ZeplinState.streak       = msg.current_streak  || 0;
        window.ZeplinState.lastFeedback = msg.feedback_text   || '';
        window.ZeplinState.stringStatus = msg.string_status   || [];

        // Update SVG fretboard with current target chord and string statuses
        if (window.fretboard && msg.target_chord) {
          window.fretboard.setChord(msg.target_chord, msg.string_status);
        }

        log(`[Co-Pilot] Target: ${msg.target_chord} | Streak: ${msg.current_streak} | ${msg.feedback_text}`);
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
  log('[Tuning] Starting tuning calibration — pluck your open low E string (82.4 Hz)...');
  dspNode.port.postMessage({ type: 'start_calibration' });

  if (window.tuner) {
    window.tuner.setReading('E2', 82.4, 0, false);
  }

  const calibrationPromise = new Promise((resolve) => {
    const handler = (e) => {
      if (e.data.event === 'calibration_f0') {
        dspNode.port.removeEventListener('message', handler);
        resolve(e.data);
      }
    };
    dspNode.port.addEventListener('message', handler);
    dspNode.port.start();
  });

  // Wait up to 15s for the student to pluck the open string
  const timeout = new Promise((_, rej) => setTimeout(() => rej(new Error('timeout')), 15000));
  let calEvt;
  try {
    calEvt = await Promise.race([calibrationPromise, timeout]);
  } catch {
    log('[Tuning] Pluck not detected within 15s — proceeding with standard A=440 tuning.');
    dspNode.port.postMessage({ type: 'stop_calibration' });
    if (window.tuner) {
      window.tuner.setReading('E2', 82.41, 0, true);
    }
    return;
  }

  dspNode.port.postMessage({ type: 'stop_calibration' });

  // Calculate deviation in cents from E2 = 82.41 Hz
  const referenceE2 = 82.41;
  const offsetCents = 1200 * Math.log2(calEvt.hz / referenceE2);
  log(`[Tuning] Detected ${calEvt.note} at ${calEvt.hz.toFixed(1)} Hz (deviation: ${offsetCents.toFixed(1)} cents)`);
  window.ZeplinState.calibrated = true;

  if (window.tuner) {
    window.tuner.setReading(calEvt.note, calEvt.hz, offsetCents, true);
  }

  // Send offset to worklet for subsequent chromagram angle rotation
  dspNode.port.postMessage({ type: 'set_tuning_offset', offsetCents });

  // Send calibration F0 to Go orchestrator via LiveKit DataChannel
  if (room && window.ZeplinState.connected) {
    const payload = JSON.stringify({ event: 'calibration_f0', hz: calEvt.hz, note: calEvt.note });
    room.localParticipant.publishData(
      new TextEncoder().encode(payload),
      { reliable: true }
    );
  }
}

// ── Worklet Event → DataChannel Bridge ─────────────────────────────────────────
function attachWorkletHandler(dspNode, room) {
  dspNode.port.onmessage = (e) => {
    const msg = e.data;

    if (msg.event === 'chord_detected') {
      window.ZeplinState.detectedChord  = msg.detected_chord;
      window.ZeplinState.confidence     = msg.confidence;
      window.ZeplinState.lastChordEvent = msg;

      // Visual pulse animation on detected card
      const card = document.getElementById('detected-chord');
      if (card) {
        card.classList.remove('strum-pulse');
        void card.offsetWidth;
        card.classList.add('strum-pulse');
      }

      log(
        `[Strum Detected] ${msg.detected_chord} | Bass: ${msg.bass_note} (${msg.bass_hz}Hz)` +
        ` | Conf: ${(msg.confidence * 100).toFixed(0)}% | Inv: ${msg.inversion} | Level: ${msg.rms_dbfs}dBFS`
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

  dspNode.port.start();
}

// ── Practice Summary Modal ────────────────────────────────────────────────────
async function openPracticeStats() {
  const modal = document.getElementById('stats-modal');
  const content = document.getElementById('stats-modal-body');
  if (!modal || !content) return;

  modal.classList.add('visible');
  content.innerHTML = '<div style="color: #64748b; padding: 1rem;">Loading practice statistics from SQLite...</div>';

  try {
    const res = await fetch('/api/summary?student=student-1');
    if (!res.ok) throw new Error('API returned ' + res.status);
    const data = await res.json();

    const accuracyPercent = data.total_strums > 0 ? ((data.clean_strums / data.total_strums) * 100).toFixed(0) : 0;
    const minutes = Math.floor((data.total_practice_sec || 0) / 60);

    let mistakesHtml = '';
    if (data.top_mistakes && data.top_mistakes.length > 0) {
      mistakesHtml = data.top_mistakes.map(m => `
        <div class="mistake-item">
          <div>
            <strong>${m.chord_name.replace('_', ' ')}</strong>
            <span class="mistake-type">(${m.mistake_type.replace('_', ' ')})</span>
            <div style="font-size: 0.8rem; color: #94a3b8;">${m.details}</div>
          </div>
          <div class="mistake-count">${m.occurrence_count}×</div>
        </div>
      `).join('');
    } else {
      mistakesHtml = '<div style="color: #64748b; font-size: 0.85rem;">No recurring mistakes recorded yet. Clean playing!</div>';
    }

    content.innerHTML = `
      <div class="stats-grid">
        <div class="stat-box">
          <div class="stat-number">${data.total_sessions || 0}</div>
          <div class="stat-label">Total Sessions</div>
        </div>
        <div class="stat-box">
          <div class="stat-number">${minutes}m</div>
          <div class="stat-label">Practice Time</div>
        </div>
        <div class="stat-box">
          <div class="stat-number">${data.total_strums || 0}</div>
          <div class="stat-label">Total Strums</div>
        </div>
        <div class="stat-box">
          <div class="stat-number" style="color: #22c55e;">${accuracyPercent}%</div>
          <div class="stat-label">Accuracy Rate</div>
        </div>
      </div>

      <div style="margin-top: 1.5rem;">
        <h4 style="font-size: 0.9rem; color: #94a3b8; text-transform: uppercase; margin-bottom: 0.75rem;">
          ⚠️ Top Recurring Mistakes (Trained Tendencies)
        </h4>
        <div class="mistakes-list">
          ${mistakesHtml}
        </div>
      </div>
    `;
  } catch (err) {
    content.innerHTML = `<div style="color: #ef4444; padding: 1rem;">Failed to load statistics: ${err.message}</div>`;
  }
}

function closePracticeStats() {
  const modal = document.getElementById('stats-modal');
  if (modal) modal.classList.remove('visible');
}

// ── Logging Utilities ─────────────────────────────────────────────────────────
function log(...args) {
  console.log('[Zeplin]', ...args);
  appendToLogPanel(args.join(' '));
}
function logErr(...args) {
  console.error('[Zeplin ERROR]', ...args);
  appendToLogPanel('⚠️ ' + args.join(' '));
}
function appendToLogPanel(text) {
  const el = document.getElementById('log-panel');
  if (!el) return;
  const line = document.createElement('div');
  line.textContent = `[${new Date().toLocaleTimeString()}] ${text}`;
  el.appendChild(line);
  el.scrollTop = el.scrollHeight;
}

// ── Page Initialization ───────────────────────────────────────────────────────
window.addEventListener('DOMContentLoaded', () => {
  // Initialize SVG fretboard and tuning gauge
  window.fretboard = new FretboardRenderer('fretboard-container');
  window.fretboard.setChord('E_Minor', ['ok', 'ok', 'ok', 'ok', 'ok', 'ok']);

  window.tuner = new TunerGauge('tuner-container');
  window.tuner.setReading('E2', 82.41, 0, false);

  // Connect button
  const btn = document.getElementById('start-btn');
  if (btn) {
    btn.addEventListener('click', () => {
      btn.disabled = true;
      btn.textContent = 'Connecting...';
      startZeplin().catch((err) => {
        logErr('startZeplin error:', err);
        btn.disabled = false;
        btn.textContent = 'Retry Connection';
      });
    });
  }

  // Stats modal listeners
  const statsBtn = document.getElementById('stats-btn');
  if (statsBtn) statsBtn.addEventListener('click', openPracticeStats);

  const closeBtn = document.getElementById('close-stats-btn');
  if (closeBtn) closeBtn.addEventListener('click', closePracticeStats);
});
