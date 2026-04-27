# soma

8D audio for ADHD focus. Plays local audio files or YouTube links and pans the sound around your head with an LFO so it stays moving — the goal is to occupy enough of the ADHD background process that the foreground task can actually run.

> *"Coffee is the modern soma."* — first drunk by Yemeni Sufis to stay focused through long nights of devotion.

## Requirements

- Go 1.21+
- `yt-dlp` on PATH (only if you want YouTube support): `brew install yt-dlp`
- Headphones. The effect doesn't work on speakers.

## Install

```sh
cd ~/code/soma
go build -o soma .
```

## Use

```sh
# interactive — paste a path or url, navigate recents, etc.
./soma

# launch straight into a track
./soma ~/Music/track.mp3
./soma "https://www.youtube.com/watch?v=..."

# override saved rate for this session
./soma -rate 0.25

# wipe the saved profile back to defaults
./soma -reset
```

Supported formats: mp3, wav, flac.

## Keys

### Home

| key | action |
|---|---|
| `enter` | play the input (or selected recent) |
| `↑` / `↓` | navigate recents |
| `?` | random play from recents |
| `t` | cycle theme |
| `esc` | quit |

### Playing

| key | action |
|---|---|
| `space` | pause / resume |
| `← / →` | seek ±5 s |
| `↑ / ↓` | volume ±2 dB |
| `[` / `]` | 8D rate −/+ 0.05 Hz |
| `S` | toggle shape (sine ↔ random walk) |
| `d` | toggle 8D / dry |
| `r` | toggle reverb |
| `n` | cycle ambience (off / white / pink / brown / **rain** / **ocean** / **wind** / **fire**) |
| `N` | noise volume +5% |
| `b` | toggle binaural beats |
| `f` | open Focus Lab |
| `g` | distraction inbox |
| `s` | sleep timer |
| `p` | toggle Pomodoro mode (25 / 5) |
| `m` | minimalist mode |
| `t` | cycle theme |
| `esc` | back to home (logs the session) |
| `q` | quit |

### Mouse (any terminal that supports mouse)

- **Click progress bar** → seek
- **Scroll wheel** → volume
- **Click chips** (8D / reverb / noise / binaural / play-state) → toggle

### Focus Lab

| key | action |
|---|---|
| `↑ / ↓` | select slider |
| `← / →` or `+ / -` | adjust |
| `1` / `2` / `3` / `4` | preset (calm / focus / stim / random walk) |
| `f` or `esc` | close |

## Files written

`~/.config/soma/`:

- `profile.json` — your tunable settings (rate, depth, shape, reverb, noise, binaural, theme, volume)
- `recents.json` — last 10 played items
- `sessions.json` — playback history (used for "today" and "streak" on home)
- `inbox.md` — markdown bullets of distractions captured with `g`

## How it works

```
file/yt-dlp ─▶ decoder ─▶ 8D pan ─▶ reverb ─┐
                                              ├─▶ mixer ─▶ meter ─▶ volume ─▶ ctrl ─▶ speaker
                                  noise ────┤
                              binaural ────┘
```

- **Source** (`internal/source`) — local path or YouTube URL. YouTube goes through `yt-dlp -x --audio-format mp3 --no-playlist`, preserving the title in the filename.
- **Decode** (`internal/player`) — `gopxl/beep` decoders (mp3/wav/flac).
- **8D effect** (`internal/effect/eightd.go`) — mono-sums then equal-power-pans across the stereo field. LFO is either a sine (smooth, predictable) or a smoothed random walk (organic, unpredictable — the brain doesn't tune it out as fast).
- **Reverb** — Schroeder (4 combs → 2 allpasses), mono wet mixed equally into both channels so the dry-panned source moves while the room stays still.
- **Ambience** (`noise.go`) — white / pink (Voss-McCartney) / brown (leaky integrator) / **rain** (pink + tonal droplet bursts with quick decay) / **ocean** (brown + slow LFO waves) / **wind** (brown + double LFO gusts) / **fire** (brown + crackle bursts). Layered under the music via `beep.Mixer`.
- **FFT spectrum** (`spectrum.go` + `internal/dsp/fft.go`) — Cooley-Tukey radix-2 FFT, 1024 samples, Hann-windowed, 32 log-spaced bands, attack/release smoothing per band. Drives the radial spectrum visualizer.
- **Binaural beats** (`binaural.go`) — two slightly-detuned sines (carrier 200 Hz), beat frequency tunable in the Focus Lab.
- **Meter** (`meter.go`) — peak L/R levels with fast attack / slow release.
- **TUI** (`internal/ui`) — Bubble Tea + lipgloss. Full-screen layout with the orbital visualizer (sound orbiting your head as a 2D ellipse with a comet trail), VU meters, full-width progress bar, status pane with all live values + chips. Coffee/Midnight/Forest/Cream themes. Mouse clicks supported.
- **Focus Lab** — `f` opens the personalization panel: 4 presets (Calm/Focus/Stim/Random Walk) + sliders for rate, depth, reverb mix, noise volume, binaural beat. All persisted to `profile.json`.
- **Capture-and-release** — `g` opens the distraction inbox to jot a thought without leaving the player.
- **Pacing** — `s` for a sleep/focus timer (25 / 50 / 90 min); `p` for Pomodoro (25/5 with auto-pause/resume on break).
- **Output** — `gopxl/beep/speaker` (CoreAudio on macOS via `oto`).

## Roadmap

- AirPods head-tracking via `CMHeadphoneMotionManager` (world-locked 8D)
- Streaming YouTube (no full download)
- Spectrum analyzer (FFT)
- Album art via kitty/sixel
- Crossfade between queued tracks
