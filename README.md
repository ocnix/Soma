# soma

8D audio for focus. Plays local audio files or YouTube links and pans the sound around your head with a slow LFO so it stays moving — the goal is to occupy enough of the ADHD background process that the foreground task can actually run.

> *"Coffee is the modern soma."* — first drunk by Yemeni Sufis to stay focused through long nights of devotion.

## Requirements

- Go 1.21+
- `yt-dlp` on PATH (only if you want YouTube support): `brew install yt-dlp`
- Headphones. The effect doesn't work on speakers.

## Install

```sh
cd ~/code/soma
go mod tidy
go build -o soma .
```

## Use

```sh
# local file
./soma ~/Music/track.mp3

# youtube
./soma "https://www.youtube.com/watch?v=..."

# tweak the rotation speed (default 0.15 Hz ≈ 6.7s per full sweep)
./soma -rate 0.25 track.mp3

# disable the effect, play normally
./soma -dry track.mp3
```

Supported formats: mp3, wav, flac.

## How it works

```
file/yt-dlp ─▶ decoder ─▶ 8D pan ─▶ meter ─▶ speaker
                            │         │
                            ▼         ▼
                          state (read by TUI @ 30 Hz)
```

- **Source** (`internal/source`) — local path or YouTube URL. YouTube goes through `yt-dlp` to a temp mp3.
- **Decode** (`internal/player`) — `gopxl/beep` decoders (mp3/wav/flac).
- **8D effect** (`internal/effect/eightd.go`) — wraps the stream, mono-sums each sample, then equal-power-pans it across the stereo field driven by a sine LFO. Equal-power keeps perceived loudness flat as the sound moves.
- **Meter** (`internal/effect/meter.go`) — peak L/R levels with fast attack / slow release, pushed to shared state.
- **TUI** (`internal/ui`) — Bubble Tea + lipgloss, coffee-toned palette. Spatial position dot, VU meters with green/yellow/red zones, elapsed/total time, mode + LFO rate. `q` to quit.
- **Output** — `gopxl/beep/speaker` (CoreAudio on macOS via `oto`).

## Roadmap

- Reverb tail for spatial depth
- Pause / seek / volume keybindings
- Playlist + queue
- HRTF convolution (true binaural) as an opt-in mode
- Crossfade between tracks
