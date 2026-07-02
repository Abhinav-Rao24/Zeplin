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
  Pending user guidance. We will likely need to write a custom `gorilla/websocket` client to append `container=none` to the dial string directly.
