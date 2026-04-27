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
# interactive — paste a path or url into the home screen
./soma

# launch straight into a track
./soma ~/Music/track.mp3
./soma "https://www.youtube.com/watch?v=..."

# tweak the rotation speed (default 0.15 Hz ≈ 6.7s per full sweep)
./soma -rate 0.25

# disable the effect, play normally
./soma -dry
```

### Keys

| screen   | key        | action                |
| -------- | ---------- | --------------------- |
| home     | `enter`    | play the input        |
| home     | `esc`      | quit                  |
| loading  | `esc`      | cancel, back to home  |
| playing  | `space`    | pause / resume        |
| playing  | `← / →`    | seek ±5 s             |
| playing  | `↑ / ↓`    | volume ±2 dB          |
| playing  | `[` / `]`  | 8D rate −/+ 0.05 Hz   |
| playing  | `r`        | toggle reverb         |
| playing  | `d`        | toggle 8D / dry       |
| playing  | `esc`      | back to home          |
| playing  | `q`        | quit                  |
| any      | `ctrl+c`   | hard quit             |

Supported formats: mp3, wav, flac.

## How it works

```
file/yt-dlp ─▶ decoder ─▶ 8D pan ─▶ reverb ─▶ meter ─▶ volume ─▶ speaker
                              │        │        │
                              ▼        ▼        ▼
                            state (snapshotted by TUI @ 30 Hz)
```

- **Source** (`internal/source`) — local path or YouTube URL. YouTube goes through `yt-dlp -x --audio-format mp3`, preserving the title in the filename.
- **Decode** (`internal/player`) — `gopxl/beep` decoders (mp3/wav/flac).
- **8D effect** (`internal/effect/eightd.go`) — mono-sums then equal-power-pans across the stereo field driven by a sine LFO. Equal-power keeps perceived loudness flat as the sound moves.
- **Reverb** (`internal/effect/reverb.go`) — tiny Schroeder reverb (4 combs → 2 allpasses) for room/depth. Mono wet, mixed equally into both channels, so the dry-panned source moves while the room stays still.
- **Meter** (`internal/effect/meter.go`) — peak L/R levels with fast attack / slow release.
- **Volume** — beep `effects.Volume` (log2 base) for the master fader.
- **TUI** (`internal/ui`) — full-screen Bubble Tea + lipgloss. Orbital visualizer (sound orbiting your head as a 2D ellipse with a comet trail), VU meters, full-width progress bar with scrubber, status pane with all live values + keybindings. Coffee-toned palette.
- **Output** — `gopxl/beep/speaker` (CoreAudio on macOS via `oto`).

## Roadmap

- Recents / queue persisted to `~/.cache/soma`
- Streaming YouTube (skip the full download)
- HRTF convolution (true binaural) as an opt-in mode
- Crossfade between tracks
