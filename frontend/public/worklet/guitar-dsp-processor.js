'use strict';
/**
 * guitar-dsp-processor.js
 *
 * A fully self-contained AudioWorklet processor for real-time guitar chord
 * detection. NO imports, NO require() — all DSP logic is inline to satisfy
 * the AudioWorkletGlobalScope's strict module isolation requirement.
 *
 * Pipeline per audio block (128 samples @ 48kHz):
 *   1. Ring buffer accumulation
 *   2. Spectral flux onset detection
 *   3. 40ms strum-roll guard wait
 *   4. Gate 1: RMS energy threshold  (rejects dead strums, palm mutes)
 *   5. Gate 2: Spectral centroid      (rejects body knocks, pick clacks)
 *   6. Gate 3: Harmonic ratio         (rejects inharmonic noise bursts)
 *   7. YIN pitch detector on low-passed signal → bass F0
 *   8. 12-bin chromagram with tuning offset compensation
 *   9. Cosine similarity chord template matcher
 *  10. postMessage(ChordEvent) → main thread → LiveKit DataChannel
 */

// ─── Constants ───────────────────────────────────────────────────────────────
const SAMPLE_RATE = 48000;
const FFT_SIZE    = 2048;   // ~42.6 ms analysis window at 48kHz
const HOP_SIZE    = 512;    // ~10.7 ms hop between onset checks

// Onset detection
const ONSET_FLUX_MULTIPLIER = 1.5; // spike threshold = mean + N × std
const ONSET_GUARD_SAMPLES   = Math.round(0.040 * SAMPLE_RATE); // 40ms guard
const DEBOUNCE_SAMPLES      = Math.round(0.300 * SAMPLE_RATE); // 300ms debounce

// Gate thresholds (tunable via control messages from main thread)
let RMS_THRESHOLD_DBFS      = -45;  // Gate 1 (sensitive to acoustic guitars at 2-3ft)
let CENTROID_HZ_THRESHOLD   = 180;  // Gate 2 (rejects sub-bass body thuds)
let HARMONIC_RATIO_THRESHOLD = 0.35; // Gate 3 (polyphonic chromatic concentration)

// Chord match
const COSINE_MIN_CONFIDENCE = 0.50; // below this → ambiguous, discard

// ─── Chord Templates ─────────────────────────────────────────────────────────
// 12-bin binary chroma vectors. Pitch class indices: C=0 C#=1 D=2 D#=3 E=4
// F=5 F#=6 G=7 G#=8 A=9 A#=10 B=11
const CHORD_TEMPLATES_RAW = {
  'C_Major': [1,0,0,0,1,0,0,1,0,0,0,0], // C  E  G
  'G_Major': [0,0,1,0,0,0,0,1,0,0,0,1], // G  B  D
  'D_Major': [0,0,1,0,0,0,1,0,0,1,0,0], // D  F# A
  'E_Minor': [0,0,0,0,1,0,0,1,0,0,0,1], // E  G  B
  'A_Minor': [1,0,0,0,1,0,0,0,0,1,0,0], // A  C  E
  'E_Major': [0,0,0,0,1,0,0,0,1,0,0,1], // E  G# B
  'A_Major': [0,1,0,0,1,0,0,0,0,1,0,0], // A  C# E
  'D_Minor': [0,0,1,0,0,1,0,0,0,1,0,0], // D  F  A
};

// Chord root pitch classes for inversion detection
const CHORD_ROOTS = {
  'C_Major': 0, 'G_Major': 7, 'D_Major': 2,
  'E_Minor': 4, 'A_Minor': 9, 'E_Major': 4,
  'A_Major': 9, 'D_Minor': 2,
};

// Pre-normalize templates to unit vectors for cosine similarity
const TEMPLATES = {};
for (const [name, raw] of Object.entries(CHORD_TEMPLATES_RAW)) {
  const mag = Math.sqrt(raw.reduce((s, v) => s + v * v, 0));
  TEMPLATES[name] = new Float32Array(raw.map(v => v / mag));
}

const F_C0 = 16.3516; // frequency of MIDI note 0 (C0) in Hz

// ─── Radix-2 Cooley-Tukey FFT ─────────────────────────────────────────────
// In-place FFT on paired real/imaginary Float32Arrays.
// Input: real samples (imaginary zeroed). Output: complex spectrum in-place.
function fft(re, im) {
  const N = re.length;

  // Bit-reversal permutation
  for (let i = 1, j = 0; i < N; i++) {
    let bit = N >> 1;
    for (; j & bit; bit >>= 1) j ^= bit;
    j ^= bit;
    if (i < j) {
      let t = re[i]; re[i] = re[j]; re[j] = t;
          t = im[i]; im[i] = im[j]; im[j] = t;
    }
  }

  // Butterfly stages
  for (let len = 2; len <= N; len <<= 1) {
    const ang = (-2 * Math.PI) / len;
    const wRe = Math.cos(ang);
    const wIm = Math.sin(ang);
    for (let i = 0; i < N; i += len) {
      let curRe = 1, curIm = 0;
      const half = len >> 1;
      for (let j = 0; j < half; j++) {
        const uRe = re[i + j];
        const uIm = im[i + j];
        const vRe = re[i + j + half] * curRe - im[i + j + half] * curIm;
        const vIm = re[i + j + half] * curIm + im[i + j + half] * curRe;
        re[i + j]        = uRe + vRe;
        im[i + j]        = uIm + vIm;
        re[i + j + half] = uRe - vRe;
        im[i + j + half] = uIm - vIm;
        const newRe = curRe * wRe - curIm * wIm;
        curIm = curRe * wIm + curIm * wRe;
        curRe = newRe;
      }
    }
  }
}

// Compute magnitude spectrum from a real sample window.
// Applies a Hann window before FFT to reduce spectral leakage.
function magnitudeSpectrum(samples) {
  const N = FFT_SIZE;
  const re = new Float32Array(N);
  const im = new Float32Array(N);
  for (let i = 0; i < N; i++) {
    const w = 0.5 * (1 - Math.cos((2 * Math.PI * i) / (N - 1)));
    re[i] = samples[i] * w;
    // im[i] already 0
  }
  fft(re, im);
  const mag = new Float32Array(N >> 1);
  for (let i = 0; i < mag.length; i++) {
    mag[i] = Math.sqrt(re[i] * re[i] + im[i] * im[i]);
  }
  return mag;
}

// ─── Gate 1: RMS Energy ───────────────────────────────────────────────────
function computeRMS(samples) {
  let sum = 0;
  for (let i = 0; i < samples.length; i++) sum += samples[i] * samples[i];
  return Math.sqrt(sum / samples.length);
}
function toDBFS(rms) {
  return rms < 1e-10 ? -120 : 20 * Math.log10(rms);
}

// ─── Gate 2: Spectral Centroid ────────────────────────────────────────────
function computeSpectralCentroid(mag) {
  const binHz = SAMPLE_RATE / FFT_SIZE;
  let weightedSum = 0, totalMag = 0;
  for (let i = 0; i < mag.length; i++) {
    weightedSum += i * binHz * mag[i];
    totalMag    += mag[i];
  }
  return totalMag > 1e-10 ? weightedSum / totalMag : 0;
}

// ─── Gate 3: Polyphonic Chromatic Concentration & Harmonicity ─────────────
// For polyphonic guitar chords (C, G, D, Am, Em), energy is distributed across
// multiple fundamentals. We measure the concentration in the top 3 semitone bins
// (a clean triad concentrates >= 35% of chromatic energy, whereas noise is flat ~25%).
function computeHarmonicRatio(chroma) {
  const sorted = Array.from(chroma).sort((a, b) => b - a);
  const top3 = sorted[0] + sorted[1] + sorted[2];
  const total = sorted.reduce((s, v) => s + v, 0);
  return total > 1e-6 ? top3 / total : 0;
}

// ─── Low-Pass Filter (FIR moving average, ~350Hz cutoff) ─────────────────
function lowPassFilter(samples, cutoffHz) {
  const M   = Math.max(1, Math.round(SAMPLE_RATE / (2 * cutoffHz)));
  const out = new Float32Array(samples.length);
  let   sum = 0;
  for (let i = 0; i < samples.length; i++) {
    sum += samples[i];
    if (i >= M) sum -= samples[i - M];
    out[i] = sum / Math.min(i + 1, M);
  }
  return out;
}

// ─── YIN Pitch Detector ───────────────────────────────────────────────────
// Detects monophonic fundamental frequency. Used on the low-passed (< 400Hz)
// signal to extract the bass string's F0 for root/inversion identification.
function yinPitch(samples, minHz, maxHz) {
  const minPeriod = Math.floor(SAMPLE_RATE / maxHz);
  const maxPeriod = Math.min(Math.floor(SAMPLE_RATE / minHz), samples.length >> 1);

  // Step 1 — Difference function
  const diff = new Float32Array(maxPeriod + 1);
  for (let tau = 1; tau <= maxPeriod; tau++) {
    let s = 0;
    const limit = samples.length - maxPeriod;
    for (let i = 0; i < limit; i++) {
      const delta = samples[i] - samples[i + tau];
      s += delta * delta;
    }
    diff[tau] = s;
  }

  // Step 2 — Cumulative mean normalized difference
  const cmnd = new Float32Array(maxPeriod + 1);
  cmnd[0] = 1;
  let runSum = 0;
  for (let tau = 1; tau <= maxPeriod; tau++) {
    runSum   += diff[tau];
    cmnd[tau] = runSum > 0 ? (diff[tau] * tau) / runSum : 1;
  }

  // Step 3 — First dip below threshold
  const YIN_THRESHOLD = 0.15;
  for (let tau = minPeriod; tau <= maxPeriod; tau++) {
    if (cmnd[tau] < YIN_THRESHOLD) {
      // Step 4 — Parabolic interpolation
      if (tau + 1 <= maxPeriod) {
        const s0 = cmnd[tau - 1], s1 = cmnd[tau], s2 = cmnd[tau + 1];
        const denom = 2 * (2 * s1 - s2 - s0);
        const frac  = denom !== 0 ? (s2 - s0) / denom : 0;
        return SAMPLE_RATE / (tau + frac);
      }
      return SAMPLE_RATE / tau;
    }
  }
  return -1; // no pitch found
}

// ─── Note Naming ──────────────────────────────────────────────────────────
const NOTE_NAMES = ['C','C#','D','D#','E','F','F#','G','G#','A','A#','B'];

function hzToNoteName(hz) {
  if (hz <= 0) return 'unknown';
  const midi   = Math.round(12 * Math.log2(hz / 440) + 69);
  const octave = Math.floor(midi / 12) - 1;
  const note   = NOTE_NAMES[((midi % 12) + 12) % 12];
  return `${note}${octave}`;
}

function hzToPitchClass(hz) {
  if (hz <= 0) return -1;
  const midi = Math.round(12 * Math.log2(hz / 440) + 69);
  return ((midi % 12) + 12) % 12;
}

// ─── 12-Bin Chromagram ────────────────────────────────────────────────────
// Maps FFT magnitude spectrum to 12 pitch classes with tuning correction.
function computeChroma(mag, tuningOffsetCents) {
  const chroma = new Float32Array(12);
  const binHz  = SAMPLE_RATE / FFT_SIZE;
  const minBin = Math.ceil(70   / binHz);  // ~70 Hz (low E string is 82.4 Hz)
  const maxBin = Math.floor(1400 / binHz); // ~1400 Hz (core fundamentals & musical overtones)

  // Calculate local noise floor in analysis band
  let sumMag = 0;
  for (let i = minBin; i < maxBin && i < mag.length; i++) sumMag += mag[i];
  const noiseFloor = (sumMag / (maxBin - minBin)) * 0.6;

  for (let i = minBin; i < maxBin && i < mag.length; i++) {
    const m = mag[i];
    if (m < noiseFloor) continue; // suppress diffuse room noise floor
    const rawHz  = i * binHz;
    // Apply tuning offset: shift frequency as if the guitar were in standard pitch
    const adjHz  = rawHz * Math.pow(2, -tuningOffsetCents / 1200);
    if (adjHz <= 0) continue;
    const pc     = Math.round(12 * Math.log2(adjHz / F_C0)) % 12;
    const pcSafe = ((pc % 12) + 12) % 12;
    chroma[pcSafe] += m * m; // energy accumulation
  }

  // L2 normalize
  let norm = 0;
  for (let i = 0; i < 12; i++) norm += chroma[i] * chroma[i];
  norm = Math.sqrt(norm);
  if (norm > 1e-10) for (let i = 0; i < 12; i++) chroma[i] /= norm;
  return chroma;
}

// ─── Chord Template Matcher ───────────────────────────────────────────────
function matchChord(chroma) {
  let bestName = 'Unknown', bestScore = -1;
  for (const [name, template] of Object.entries(TEMPLATES)) {
    let dot = 0;
    for (let i = 0; i < 12; i++) dot += chroma[i] * template[i];
    if (dot > bestScore) { bestScore = dot; bestName = name; }
  }
  return { chord: bestName, confidence: bestScore };
}

// ─── Main AudioWorklet Processor ─────────────────────────────────────────
class GuitarDSPProcessor extends AudioWorkletProcessor {
  constructor(options) {
    super(options);

    // Ring buffer — 4× FFT window for onset + analysis without reallocation
    this._buf     = new Float32Array(FFT_SIZE * 4);
    this._head    = 0;  // write position in ring buffer
    this._filled  = 0;  // total samples written (capped at buf.length)

    // Onset state
    this._prevMag       = null;
    this._fluxHistory   = [];          // rolling flux values for adaptive threshold
    this._onsetPending  = false;
    this._samplesAfterOnset = 0;

    // Debounce: ignore chord events within 300ms of the last one
    this._samplesSinceLastChord = DEBOUNCE_SAMPLES; // start allowing

    // Tuning calibration
    this._tuningOffsetCents = 0;
    this._isCalibrating     = false;

    // Agent state awareness (Speaker bleed prevention)
    this._isAgentSpeaking   = false;

    // Listen for control messages from the main thread
    this.port.onmessage = (e) => {
      const msg = e.data;
      switch (msg.type) {
        case 'set_agent_state':
          this._isAgentSpeaking = (msg.state === 'speaking');
          break;
        case 'set_tuning_offset':
          this._tuningOffsetCents = msg.offsetCents || 0;
          break;
        case 'set_rms_threshold':
          RMS_THRESHOLD_DBFS = msg.value;
          break;
        case 'start_calibration':
          this._isCalibrating = true;
          break;
        case 'stop_calibration':
          this._isCalibrating = false;
          break;
      }
    };
  }

  // process() is called every 128 samples (~2.67ms at 48kHz).
  // Must return true to keep the processor alive.
  process(inputs, outputs) {
    const channel = inputs[0]?.[0];
    if (!channel || channel.length === 0) return true;

    const outChannel = outputs?.[0]?.[0];
    if (outChannel) {
      outChannel.set(channel);
    }

    const n = channel.length; // typically 128

    // Write samples into the ring buffer
    for (let i = 0; i < n; i++) {
      this._buf[this._head % this._buf.length] = channel[i];
      this._head++;
    }
    this._filled = Math.min(this._filled + n, this._buf.length);

    // Wait until we have a full FFT window before doing any analysis
    if (this._filled < FFT_SIZE) return true;

    this._samplesSinceLastChord += n;

    // Extract current analysis window from the ring buffer
    const win = this._extractWindow(FFT_SIZE);
    const mag = magnitudeSpectrum(win);

    // ── Onset Detection (Spectral Flux, half-wave rectified) ─────────────
    if (this._prevMag !== null && !this._onsetPending) {
      let flux = 0;
      for (let i = 0; i < mag.length; i++) {
        const d = mag[i] - this._prevMag[i];
        if (d > 0) flux += d;
      }

      this._fluxHistory.push(flux);
      if (this._fluxHistory.length > 60) this._fluxHistory.shift();

      if (this._fluxHistory.length >= 10) {
        const mean = this._fluxHistory.reduce((s, v) => s + v, 0) / this._fluxHistory.length;
        const vari = this._fluxHistory.reduce((s, v) => s + (v - mean) ** 2, 0) / this._fluxHistory.length;
        const threshold = mean + ONSET_FLUX_MULTIPLIER * Math.sqrt(vari);

        if (flux > threshold) {
          this._onsetPending      = true;
          this._samplesAfterOnset = 0;
        }
      }
    }

    // ── Guard Period ──────────────────────────────────────────────────────
    if (this._onsetPending) {
      this._samplesAfterOnset += n;
      if (this._samplesAfterOnset >= ONSET_GUARD_SAMPLES) {
        this._onsetPending = false;
        // Re-extract window at the guard-elapsed point for a clean analysis
        const analysisWin = this._extractWindow(FFT_SIZE);
        const analysisMag = magnitudeSpectrum(analysisWin);
        this._runAnalysis(analysisWin, analysisMag);
      }
    }

    this._prevMag = mag;
    return true;
  }

  _extractWindow(size) {
    const win   = new Float32Array(size);
    const bufLen = this._buf.length;
    const start  = this._head - size;
    for (let i = 0; i < size; i++) {
      win[i] = this._buf[((start + i) % bufLen + bufLen) % bufLen];
    }
    return win;
  }

  _runAnalysis(win, mag) {
    // Debounce: skip if a chord event fired too recently
    if (this._samplesSinceLastChord < DEBOUNCE_SAMPLES) return;

    // ── Gate 1: RMS Energy & Speaker Bleed Rejection ─────────────────────
    const rms  = computeRMS(win);
    const dbfs = toDBFS(rms);

    // If agent is speaking, elevate minimum RMS threshold by +12dB to reject
    // synthesized speech acoustics from laptop speakers leaking into the microphone.
    // Genuine guitar pick attacks easily produce > -26 dBFS, whereas speaker bleed is typically < -32 dBFS.
    const effectiveRmsThreshold = this._isAgentSpeaking ? Math.max(RMS_THRESHOLD_DBFS + 12, -26) : RMS_THRESHOLD_DBFS;
    if (dbfs < effectiveRmsThreshold) return; // speaker bleed / dead strum / finger lift

    // ── Gate 2: Spectral Centroid ─────────────────────────────────────────
    const centroid = computeSpectralCentroid(mag);
    if (centroid < CENTROID_HZ_THRESHOLD) return; // body knock / thud

    // ── Bass F0 via YIN on low-passed signal ─────────────────────────────
    const lowPassed = lowPassFilter(win, 400);
    const bassHz    = yinPitch(lowPassed, 70, 420);

    // ── Chromagram ────────────────────────────────────────────────────────
    const chroma = computeChroma(mag, this._tuningOffsetCents);

    // ── Gate 3: Polyphonic Chromatic Concentration ─────────────────────────
    const harmRatio = computeHarmonicRatio(chroma);
    // While agent speaks, require slightly stronger concentration to reject speech leakage
    const effectiveHarmonicThreshold = this._isAgentSpeaking ? 0.45 : HARMONIC_RATIO_THRESHOLD;
    if (harmRatio < effectiveHarmonicThreshold) return; // inharmonic noise burst / speech plosive

    // ── Chord Match ───────────────────────────────────────────────────────
    const { chord, confidence } = matchChord(chroma);
    if (confidence < COSINE_MIN_CONFIDENCE) return; // ambiguous

    // ── Inversion Detection ───────────────────────────────────────────────
    const bassNote      = hzToNoteName(bassHz);
    const bassPC        = hzToPitchClass(bassHz);
    const chordRootPC   = CHORD_ROOTS[chord] ?? -1;
    const isInversion   = bassHz > 0 && chordRootPC >= 0 && bassPC !== chordRootPC;

    // ── Calibration Mode ──────────────────────────────────────────────────
    if (this._isCalibrating && bassHz > 0) {
      this.port.postMessage({
        event: 'calibration_f0',
        hz:    Math.round(bassHz * 10) / 10,
        note:  bassNote,
      });
      this._samplesSinceLastChord = 0;
      return;
    }

    // ── Emit Chord Event ──────────────────────────────────────────────────
    this._samplesSinceLastChord = 0;

    this.port.postMessage({
      event:                'chord_detected',
      timestamp_ms:         Math.round(currentTime * 1000),
      detected_chord:       chord,
      bass_note:            bassNote,
      bass_hz:              Math.round(bassHz * 10) / 10,
      confidence:           Math.round(confidence * 1000) / 1000,
      inversion:            isInversion,
      tuning_offset_cents:  Math.round(this._tuningOffsetCents * 10) / 10,
      rms_dbfs:             Math.round(dbfs * 10) / 10,
      spectral_centroid_hz: Math.round(centroid),
      harmonic_ratio:       Math.round(harmRatio * 100) / 100,
    });
  }
}

registerProcessor('guitar-dsp-processor', GuitarDSPProcessor);
