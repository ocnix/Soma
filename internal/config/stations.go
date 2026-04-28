package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Station is a named live audio stream.
type Station struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Tag  string `json:"tag,omitempty"`
}

// DefaultStations are bundled with soma. SomaFM publishes free, ad-free
// streams in many flavors — these are tuned for focus / chill.
var DefaultStations = []Station{
	{Name: "Groove Salad", URL: "https://ice1.somafm.com/groovesalad-256-mp3", Tag: "ambient · downtempo"},
	{Name: "Drone Zone", URL: "https://ice1.somafm.com/dronezone-256-mp3", Tag: "ambient · drone"},
	{Name: "Deep Space One", URL: "https://ice1.somafm.com/deepspaceone-128-mp3", Tag: "ambient · space"},
	{Name: "Mission Control", URL: "https://ice1.somafm.com/missioncontrol-128-mp3", Tag: "nasa · ambient"},
	{Name: "Lush", URL: "https://ice1.somafm.com/lush-128-mp3", Tag: "vocal · chill"},
	{Name: "Beat Blender", URL: "https://ice1.somafm.com/beatblender-128-mp3", Tag: "downtempo · beats"},
	{Name: "Fluid", URL: "https://ice1.somafm.com/fluid-128-mp3", Tag: "trip-hop · ambient"},
	{Name: "Black Rock FM", URL: "https://ice1.somafm.com/brfm-128-mp3", Tag: "burning man · eclectic"},
}

func stationsPath() string { return filepath.Join(Dir(), "stations.json") }

// LoadStations returns the bundled defaults plus any user-saved stations.
func LoadStations() []Station {
	out := make([]Station, 0, len(DefaultStations)+8)
	out = append(out, DefaultStations...)

	b, err := os.ReadFile(stationsPath())
	if err != nil {
		return out
	}
	var custom []Station
	if err := json.Unmarshal(b, &custom); err == nil {
		out = append(out, custom...)
	}
	return out
}

// AddStation appends a user station to disk.
func AddStation(s Station) error {
	if err := ensureDir(); err != nil {
		return err
	}
	var custom []Station
	if b, err := os.ReadFile(stationsPath()); err == nil {
		_ = json.Unmarshal(b, &custom)
	}
	// Dedupe by URL.
	for _, ex := range custom {
		if ex.URL == s.URL {
			return nil
		}
	}
	custom = append(custom, s)
	b, err := json.MarshalIndent(custom, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(stationsPath(), b)
}
