# Development Issues and Solutions Log

This document lists all major errors, issues, and bottlenecks faced during the development of the voice agent framework, along with the solutions applied to resolve them.

---

### 1. Windows CGO Dependency Mismatch on LiveKit Media Track
* **Issue**:
  Importing the standard LiveKit `PCMLocalTrack` API from `github.com/livekit/server-sdk-go/v2/pkg/media` broke compilation on Windows. The compiler raised failures trying to build the C-based Opus bindings (`gopkg.in/hraban/opus.v2`) because `pkg-config` and Opus C headers were missing in the environment.
* **Solution**:
  Avoid importing the `pkg/media` package altogether.
  1. We published the outbound voice track using Pion's pure-Go `lksdk.NewLocalSampleTrack` with Opus capability.
  2. We configured Deepgram's streaming Speak WS options (`interfaces.WSSpeakOptions`) to request native `"opus"` encoding at `48000Hz` generated on Deepgram's servers.
  3. The `Binary(data []byte)` callback intercepts these ready-to-use compressed Opus blocks and writes them straight to the `LocalSampleTrack` via Pion's `media.Sample{}` wrapper. This bypassed local CGO/Opus compilation requirements completely.

---

### 2. Deepgram Speak WS Client Method Mismatch (`SendText` Undefined)
* **Issue**:
  Attempting to stream text using typical SDK macros like `dgClient.SendText("...")` threw compilation errors because `SendText` is undefined on the Go SDK v3's Speak client struct.
* **Solution**:
  Utilized Go reflection to inspect all methods exported by `*websocketv1.WSCallback`. We discovered the correct methods for sending streaming text and signalling chunks:
  - `Speak(text string) error`
  - `Flush() error`
  We mapped these signatures directly inside `tts/deepgram.go` and resolved the compilation failure.

---

### 3. Missing `go.sum` Entries for Media Packages
* **Issue**:
  Initial compiler runs threw errors indicating missing `go.sum` hash entries for indirect dependencies of `github.com/livekit/server-sdk-go/v2`.
* **Solution**:
  Ran `go mod tidy` in the workspace root directory. This fetched, downloaded, and verified all direct and indirect dependencies, cleaning up the package tree.

---

### 4. Deepgram API WebSocket Handshake Drop for Opus Encoding without Container 
* **Issue**:
  When attempting to connect to the Deepgram TTS WebSocket stream with `Encoding: "opus"` set within `WSSpeakOptions` (which lacks a `Container` field), the WebSocket connection fails and drops instantly without triggering the `Error` callback. A quick test confirmed that switching `Encoding` to `"linear16"` successfully establishes the WebSocket connection. This confirms that Deepgram's WS Gateway strictly rejects `opus` unless accompanied by `container=none`. 
* **Solution**:
  Bypassed the official Go SDK TTS implementation and built a custom WebSocket client using `github.com/gorilla/websocket`. We switched the encoding from Opus to `mulaw` at `8000Hz` (PCMU) to natively align with WebRTC's standard `webrtc.MimeTypePCMU` capabilities and avoid LiveKit server-side codec mismatch errors.

---

### 5. Premature Silence Cutoffs during STT Ingestion
* **Issue**:
  The conversational agent cut the user off prematurely when they paused mid-sentence to think. This was caused by the STT endpointing defaults which finalized speech turns too aggressively.
* **Solution**:
  We explicitly tuned the `LiveTranscriptionOptions` in `stt/deepgram.go` by specifying:
  - `Endpointing: "1000"` (waits 1 second of silence to finalize a speech chunk)
  - `UtteranceEndMs: "1000"` (requires 1 second of silence to trigger utterance end events)
  - `InterimResults: true` (ensures real-time streaming results are delivered to the brain)
  - `VadEvents: true` (enables real-time Voice Activity Detection events)

---

### 6. Sluggish Barge-In Cutoff Latency
* **Issue**:
  When a user interrupted the agent, the agent would keep speaking for 1-2 seconds. This lag resulted from:
  1. Relying on transcript onset for interruption, which is slower than basic voice activity detection.
  2. The pacing loop using a blocking `time.Sleep`, delaying the processing of the interruption signal.
  3. Pseudo-random channel selection in the Go pacing `select` statement pulling extra frames from `AudioChan` even when an interrupt was pending.
  4. Stray audio frames trickling in from Deepgram over the WebSocket right after the `Clear` signal was sent.
* **Solution**:
  We implemented an "Instant-Kill" control flow:
  1. Triggered the interruption immediately on Deepgram's `SpeechStarted` callback.
  2. Swapped out the pacing loop's `time.Sleep` for a `time.After` check in a select block alongside `interruptChan`.
  3. Added a prioritized non-blocking check on `interruptChan` at the head of the pacing loop to instantly abort processing before pulling from `AudioChan`.
  4. Used an atomic boolean flag (`clearing`) in `tts/deepgram.go` to block and discard any post-clear trickling packets until the next Speak call.

---

### 7. Ghost Interruptions from Ambient Noise (State Machine Resiliency)
* **Issue**:
  Enabling real-time voice activity detection (VAD) via `SpeechStarted` events and early interim results made barge-in highly responsive, but triggered false-positive "ghost interruptions" from room ambient noise. When the agent was completely silent/listening, this ambient noise would still trigger context cancellation, disrupting Eino/Gemini turn processing when the user was about to speak.
* **Solution**:
  We implemented a thread-safe, resilient State Machine using atomic flags:
  - `isBrainActive` (tracks if Gemini is actively generating a turn, set to `true` at the start of `ProcessTurn` and `false` on completion).
  - `isPacingActive` (tracks if the LiveKit pacing loop is active, set to `true` when pulling audio frames from the buffer and `false` when the buffer goes empty and playback completes).
  - **Interruption Guard**: The barge-in interruption logic inside `ConnectLiveKit` was wrapped in a conditional check: it only fires if `isBrainActive || isPacingActive` is `true`. If the agent is silent/listening, all incoming `SpeechStarted` callbacks or interim transcripts are completely ignored for interruption.

