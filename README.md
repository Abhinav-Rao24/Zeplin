# Zeplin

Zeplin is a fast, real-time voice assistant framework built in Go. It connects users in a voice room with an AI model that listens, thinks, and speaks back with minimal delay.

Note: This project is currently in active development.

Zeplin is built on top of:
- LiveKit: WebRTC voice rooms for audio input and output
- Deepgram: Fast Speech-to-Text (STT) and streaming Text-to-Speech (TTS)
- Groq: Ultra-fast LLM response generation using Llama 3.1
- Google WebRTC VAD: Local voice detection to detect when the user starts speaking

---

## How It Works

1. Listening: The user speaks in a LiveKit room. The agent receives the audio stream.
2. Fast Voice Detection: Instead of waiting for the cloud, Zeplin checks for voice locally. If you interrupt the bot while it is speaking, it cuts itself off immediately.
3. Transcribing: The user's speech is transcribed into text in real time using Deepgram.
4. Thinking: When the user finishes speaking, the text is sent to Groq (running Llama 3.1 8B Instant), which starts generating a reply token by token.
5. Speaking: As reply words arrive from Groq, they are streamed to Deepgram TTS. The resulting audio is sent back into the LiveKit room at a steady 20 millisecond pace.

```
                      +-------------------+
                      |  LiveKit WebRTC   |
                      |  User in Room     |
                      +---------+---------+
                                | (User Voice)
                                v
               +----------------+----------------+
               |                                 |
               v                                 v
     +-------------------+             +-------------------+
     |  Local Voice VAD  |             |   Deepgram STT    |
     | (Interruption)    |             |  (Transcription)  |
     +---------+---------+             +---------+---------+
               |                                 |
        (Stops bot if user                       | (User message)
         speaks over it)                         v
               |                       +-------------------+
               |                       |    Groq Brain     |
               |                       |    (Llama 3.1)    |
               |                       +---------+---------+
               |                                 |
               |                                 | (Streamed text)
               |                                 v
               |                       +-------------------+
               |                       |   Deepgram TTS    |
               |                       |  (Voice Output)   |
               |                       +---------+---------+
               |                                 |
               +---------------+                 | (Audio frames)
                               |                 v
                               v       +-------------------+
                     [Cancel / Drain]  |   LiveKit Audio   |
                                       |   Pacing (20ms)   |
                                       +---------+---------+
                                                 |
                                                 v
                                       +-------------------+
                                       |  User Hears Bot   |
                                       +-------------------+
```

---

## Performance Targets and Current Benchmarks

A big challenge with voice bots is latency (the delay before the bot answers) and interruption speed (how quickly it stops when you talk over it). 

Here is how Zeplin's targets and current test results compare to typical voice bots:

| Metric | Typical Voice Bot | Zeplin Target | Current Zeplin Benchmark | Status |
| :--- | :--- | :--- | :--- | :--- |
| Interruption Latency (Barge-in) | 800 ms - 1500 ms (waits on cloud transcription) | Under 200 ms | ~60 ms - 200 ms (Local WebRTC VAD) | Better than average |
| Time to First Token (TTFT) | 400 ms - 800 ms (standard cloud LLMs) | Under 250 ms | ~180 ms - 220 ms (via Groq) | Better than average |
| Time to First Audio (TTFA) | 300 ms - 600 ms | Under 250 ms | ~200 ms - 240 ms (Deepgram streaming) | Better than average |
| Total Turn-Around Delay | 1000 ms - 2000 ms | Under 500 ms | ~450 ms - 650 ms | In progress (tuning buffer handoff) |
| Audio Playback Jitter | 5 ms - 15 ms variation | Under 1 ms variation | ~0.4 ms standard deviation | Stable |

Because Zeplin processes voice detection locally and uses Groq's high-speed inference, the bot cuts off almost instantly when interrupted and starts responding much faster than standard cloud setups.

---

## Core Features

- Quick Interruption (Barge-In): You can talk over the agent at any point. When you speak, the bot stops playback and cancels the current response right away.
- Echo Protection: To prevent the bot from hearing its own voice through laptop speakers, Zeplin checks incoming words against what it recently said. If they match, the audio is ignored.
- Pure Go Audio: Sends standard mu-law audio over WebRTC without requiring C compiler tools or external Opus libraries on Windows.
- Automatic Connection Recovery: Reconnects to the speech services automatically if a connection drop occurs.
- Built-in Metrics: Prints a clean performance summary after every turn showing token speed, audio speed, and playback stability.

---

## Project Structure

```
.
├── brain/                   # Groq connection and conversation handling
├── config/                  # Configuration and environment variables
├── memory/                  # In-memory storage for chat history
├── stt/                     # Speech-to-Text integration (Deepgram)
├── transport/               # WebRTC handling, VAD, state machine, telemetry
├── tts/                     # Text-to-Speech integration (Deepgram)
├── development_issues.md    # Issues faced during development and fixes applied
├── go.mod                   # Go dependencies
├── go.sum                   # Go checksums
└── main.go                  # Main entrypoint
```

---

## Prerequisites

- Go 1.22 or newer
- A LiveKit Cloud account (or local LiveKit server)
- A Deepgram API key (for STT and TTS)
- A Groq API key (for Llama 3.1)

---

## Environment Setup

Create a `.env` file in the project root:

```env
LIVEKIT_URL=wss://your-project.livekit.cloud
LIVEKIT_API_KEY=your_livekit_api_key
LIVEKIT_API_SECRET=your_livekit_api_secret
LIVEKIT_ROOM_NAME=voice-agent-room

DEEPGRAM_API_KEY=your_deepgram_api_key
TTS_API_KEY=your_deepgram_api_key
GROQ_API_KEY=your_groq_api_key
```

Variables:
- `LIVEKIT_URL`: URL of your LiveKit server.
- `LIVEKIT_API_KEY`: Your LiveKit API key.
- `LIVEKIT_API_SECRET`: Your LiveKit API secret.
- `LIVEKIT_ROOM_NAME`: Room to join (defaults to `voice-agent-room`).
- `DEEPGRAM_API_KEY`: API key for Deepgram transcription and voice generation.
- `TTS_API_KEY`: API key for TTS service.
- `GROQ_API_KEY`: API key for Groq LLM inference.

---

## How to Run

1. Clone the repository:
   ```bash
   git clone https://github.com/Abhinav-Rao24/Zeplin.git
   cd Zeplin
   ```

2. Download Go dependencies:
   ```bash
   go mod download
   ```

3. Build and run:

   On Linux / macOS:
   ```bash
   go build -o zeplin .
   ./zeplin
   ```

   On Windows:
   ```powershell
   go build -o Zeplin.exe .
   .\Zeplin.exe
   ```

4. Join the room:
   Open the LiveKit Meet web app (or your own LiveKit frontend), join the room named `voice-agent-room`, unmute your microphone, and start speaking.

---

## Development Status

This framework is under active development. Current work includes:
- Further reducing end-to-end turn-around latency.
- Expanding memory storage options beyond in-memory storage.
- Adding support for tool calling and custom agent actions.

---

## License

This project is licensed under the MIT License.
