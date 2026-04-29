package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Profile holds user-tunable parameters that persist across runs.
type Profile struct {
	Rate         float64 `json:"rate"`
	Depth        float64 `json:"depth"` // 0..1, scales pan amplitude
	Shape        string  `json:"shape"` // "sine" | "random"
	ReverbMix    float64 `json:"reverb_mix"`
	ReverbOn     bool    `json:"reverb_on"`
	NoiseMode    string  `json:"noise_mode"` // "off" | "white" | "pink" | "brown"
	NoiseVolume  float64 `json:"noise_volume"`
	BinauralOn   bool    `json:"binaural_on"`
	BinauralBeat float64 `json:"binaural_beat"` // Hz
	Crossfeed    float64 `json:"crossfeed"`     // 0..1
	Theme        string  `json:"theme"`
	Viz          string  `json:"viz"` // "sphere" | "lissajous" | "tunnel" | "orbit"
	VolumeDb     float64 `json:"volume_db"`
}

var Default = Profile{
	Rate:         0.15,
	Depth:        1.0,
	Shape:        "sine",
	ReverbMix:    0.18,
	ReverbOn:     true,
	NoiseMode:    "off",
	NoiseVolume:  0.20,
	BinauralOn:   false,
	BinauralBeat: 8.0,
	Crossfeed:    0.30, // gentle default — felt by IEM listeners, near-inaudible on speakers
	Theme:        "coffee",
	Viz:          "sphere",
	VolumeDb:     0,
}

// ── Paths ────────────────────────────────────────────────────────────────────

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "soma")
}

func ensureDir() error {
	return os.MkdirAll(Dir(), 0o755)
}

func profilePath() string  { return filepath.Join(Dir(), "profile.json") }
func recentsPath() string  { return filepath.Join(Dir(), "recents.json") }
func sessionsPath() string { return filepath.Join(Dir(), "sessions.json") }
func inboxPath() string    { return filepath.Join(Dir(), "inbox.md") }

var fsMu sync.Mutex

// ── Profile ──────────────────────────────────────────────────────────────────

func LoadProfile() Profile {
	fsMu.Lock()
	defer fsMu.Unlock()
	p := Default
	b, err := os.ReadFile(profilePath())
	if err != nil {
		return p
	}
	_ = json.Unmarshal(b, &p)
	return p
}

func SaveProfile(p Profile) error {
	fsMu.Lock()
	defer fsMu.Unlock()
	if err := ensureDir(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(profilePath(), b)
}

// ── Recents ──────────────────────────────────────────────────────────────────

type RecentItem struct {
	Source string    `json:"source"`
	Title  string    `json:"title"`
	At     time.Time `json:"at"`
}

const maxRecents = 10

func LoadRecents() []RecentItem {
	fsMu.Lock()
	defer fsMu.Unlock()
	b, err := os.ReadFile(recentsPath())
	if err != nil {
		return nil
	}
	var items []RecentItem
	_ = json.Unmarshal(b, &items)
	return items
}

func AddRecent(item RecentItem) error {
	fsMu.Lock()
	defer fsMu.Unlock()
	if err := ensureDir(); err != nil {
		return err
	}
	var items []RecentItem
	if b, err := os.ReadFile(recentsPath()); err == nil {
		_ = json.Unmarshal(b, &items)
	}
	// dedupe by Source
	out := items[:0]
	for _, it := range items {
		if it.Source != item.Source {
			out = append(out, it)
		}
	}
	items = append([]RecentItem{item}, out...)
	if len(items) > maxRecents {
		items = items[:maxRecents]
	}
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(recentsPath(), b)
}

func RandomRecent() (RecentItem, bool) {
	items := LoadRecents()
	if len(items) == 0 {
		return RecentItem{}, false
	}
	return items[rand.Intn(len(items))], true
}

// ── Sessions ─────────────────────────────────────────────────────────────────

type Session struct {
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
	Length string    `json:"length"` // pretty form, ie "27m14s"
}

func AppendSession(start, end time.Time) error {
	fsMu.Lock()
	defer fsMu.Unlock()
	if err := ensureDir(); err != nil {
		return err
	}
	var sessions []Session
	if b, err := os.ReadFile(sessionsPath()); err == nil {
		_ = json.Unmarshal(b, &sessions)
	}
	dur := end.Sub(start).Round(time.Second)
	if dur < 5*time.Second {
		// Don't log accidental clicks.
		return nil
	}
	sessions = append(sessions, Session{Start: start, End: end, Length: dur.String()})
	b, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(sessionsPath(), b)
}

func loadSessions() []Session {
	b, err := os.ReadFile(sessionsPath())
	if err != nil {
		return nil
	}
	var s []Session
	_ = json.Unmarshal(b, &s)
	return s
}

func TodayTotal() time.Duration {
	fsMu.Lock()
	defer fsMu.Unlock()
	sessions := loadSessions()
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var total time.Duration
	for _, s := range sessions {
		if s.Start.After(startOfDay) {
			d, err := time.ParseDuration(s.Length)
			if err == nil {
				total += d
			}
		}
	}
	return total
}

// Streak counts consecutive days (including today) with at least one session.
func Streak() int {
	fsMu.Lock()
	defer fsMu.Unlock()
	sessions := loadSessions()
	if len(sessions) == 0 {
		return 0
	}
	days := make(map[string]bool)
	for _, s := range sessions {
		d := s.Start.Format("2006-01-02")
		days[d] = true
	}
	keys := make([]string, 0, len(days))
	for k := range days {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	streak := 0
	cursor := time.Now()
	for {
		key := cursor.Format("2006-01-02")
		if days[key] {
			streak++
			cursor = cursor.AddDate(0, 0, -1)
			continue
		}
		// Allow one grace miss only for today (if no session yet today).
		if streak == 0 && key == time.Now().Format("2006-01-02") {
			cursor = cursor.AddDate(0, 0, -1)
			continue
		}
		break
	}
	return streak
}

// ── Inbox ────────────────────────────────────────────────────────────────────

func AppendInbox(thought string) error {
	fsMu.Lock()
	defer fsMu.Unlock()
	if err := ensureDir(); err != nil {
		return err
	}
	if thought == "" {
		return errors.New("empty thought")
	}
	line := fmt.Sprintf("- [%s] %s\n", time.Now().Format("2006-01-02 15:04"), thought)
	f, err := os.OpenFile(inboxPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

// ── Internals ────────────────────────────────────────────────────────────────

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
