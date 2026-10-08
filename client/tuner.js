/**
 * tuner.js — Real-Time Tuning Gauge Component
 *
 * Renders an interactive cents meter (-50 to +50 cents) with:
 *   - Current frequency in Hz
 *   - Note name (e.g. E2 = 82.4 Hz)
 *   - Dynamic needle and color-coded indicator:
 *       🟢 Green (within ±10 cents) — in tune
 *       🟡 Yellow / Red (flat or sharp)
 */

'use strict';

class TunerGauge {
  constructor(containerId) {
    this.container = document.getElementById(containerId);
    this.offsetCents = 0;
    this.hz = 0;
    this.note = '—';
    this.calibrated = false;
  }

  setReading(note, hz, offsetCents, calibrated) {
    this.note = note || '—';
    this.hz = hz || 0;
    this.offsetCents = offsetCents || 0;
    this.calibrated = calibrated;
    this.render();
  }

  render() {
    if (!this.container) return;

    // Clamp cents to [-50, +50]
    const clampedCents = Math.max(-50, Math.min(50, this.offsetCents));
    const percent = ((clampedCents + 50) / 100) * 100;
    const isTune = Math.abs(this.offsetCents) <= 10;
    const needleColor = isTune ? '#22c55e' : (Math.abs(this.offsetCents) < 25 ? '#eab308' : '#ef4444');

    const html = `
      <div class="tuner-card">
        <div class="tuner-header">
          <span class="tuner-title">🎸 Guitar Tuner Calibration</span>
          <span class="tuner-badge ${this.calibrated ? 'calibrated' : ''}">
            ${this.calibrated ? '✓ Calibrated' : 'Listening for E2 (82.4Hz)'}
          </span>
        </div>

        <div class="tuner-display">
          <div class="tuner-note">${this.note}</div>
          <div class="tuner-freq">${this.hz ? this.hz.toFixed(1) + ' Hz' : '—'}</div>
          <div class="tuner-cents" style="color: ${needleColor}">
            ${this.offsetCents > 0 ? '+' : ''}${this.offsetCents.toFixed(1)} cents
          </div>
        </div>

        <div class="tuner-meter-track">
          <div class="tuner-center-mark"></div>
          <div class="tuner-sweet-spot"></div>
          <div class="tuner-needle" style="left: ${percent}%; background-color: ${needleColor};"></div>
        </div>

        <div class="tuner-labels">
          <span>-50♭ Flat</span>
          <span class="target-label">Target: E2 (82.4Hz)</span>
          <span>+50♯ Sharp</span>
        </div>
      </div>
    `;

    this.container.innerHTML = html;
  }
}

window.TunerGauge = TunerGauge;
