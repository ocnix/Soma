// Package library scans local audio files into a fast searchable index.
//
// Index is cached to ~/.config/soma/library.json so subsequent runs don't
// rewalk the filesystem. Filename + folder structure provides title / album /
// artist; we don't read ID3 tags (keeps it dep-free and fast).
package library

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"soma/internal/config"
)

// Item is one audio file in the library.
type Item struct {
	Path   string `json:"path"`   // absolute path
	Title  string `json:"title"`  // filename without extension
	Album  string `json:"album"`  // parent directory
	Artist string `json:"artist"` // grandparent directory
}

// Display returns a friendly one-line label for list rendering.
func (it Item) Display() string {
	switch {
	case it.Artist != "" && it.Album != "":
		return it.Artist + " — " + it.Album + " / " + it.Title
	case it.Album != "":
		return it.Album + " / " + it.Title
	default:
		return it.Title
	}
}

// Index is the in-memory library snapshot.
type Index struct {
	mu     sync.RWMutex
	items  []Item
	loaded bool
	root   string
}

func NewIndex() *Index {
	home, _ := os.UserHomeDir()
	return &Index{root: filepath.Join(home, "Music")}
}

func (idx *Index) Root() string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.root
}

func (idx *Index) SetRoot(p string) {
	idx.mu.Lock()
	idx.root = p
	idx.mu.Unlock()
}

// Items returns a snapshot of the current index.
func (idx *Index) Items() []Item {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	out := make([]Item, len(idx.items))
	copy(out, idx.items)
	return out
}

func (idx *Index) IsEmpty() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.items) == 0
}

func (idx *Index) Loaded() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.loaded
}

// LoadCache reads a previously-scanned index from disk. No error if missing.
func (idx *Index) LoadCache() {
	b, err := os.ReadFile(cachePath())
	if err != nil {
		return
	}
	var items []Item
	if err := json.Unmarshal(b, &items); err != nil {
		return
	}
	idx.mu.Lock()
	idx.items = items
	idx.loaded = true
	idx.mu.Unlock()
}

// Scan walks the configured root for supported audio files. Safe to call from
// a goroutine. Replaces the in-memory index and writes a fresh cache.
func (idx *Index) Scan() error {
	idx.mu.RLock()
	root := idx.root
	idx.mu.RUnlock()

	items, err := scan(root)
	if err != nil {
		return err
	}
	idx.mu.Lock()
	idx.items = items
	idx.loaded = true
	idx.mu.Unlock()
	_ = saveCache(items)
	return nil
}

// Filter returns the subset of items whose Display() contains every
// whitespace-separated token of q (case-insensitive). Empty q returns all.
// Results are capped at limit (0 = no cap) to keep rendering fast.
func (idx *Index) Filter(q string, limit int) []Item {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		out := make([]Item, len(idx.items))
		copy(out, idx.items)
		if limit > 0 && len(out) > limit {
			out = out[:limit]
		}
		return out
	}
	tokens := strings.Fields(q)
	var out []Item
	for _, it := range idx.items {
		hay := strings.ToLower(it.Display())
		ok := true
		for _, t := range tokens {
			if !strings.Contains(hay, t) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, it)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out
}

// ── internals ────────────────────────────────────────────────────────────────

func cachePath() string {
	return filepath.Join(config.Dir(), "library.json")
}

func saveCache(items []Item) error {
	if err := os.MkdirAll(config.Dir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	tmp := cachePath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, cachePath())
}

func scan(root string) ([]Item, error) {
	if root == "" {
		return nil, nil
	}
	if _, err := os.Stat(root); err != nil {
		return nil, nil // no music dir → empty library, not an error
	}
	var items []Item
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			// Skip hidden dirs like .DS_Store / .git
			if strings.HasPrefix(info.Name(), ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".mp3", ".wav", ".flac":
		default:
			return nil
		}
		items = append(items, makeItem(path))
		return nil
	})
	// Stable order for deterministic UI.
	sort.Slice(items, func(i, j int) bool {
		if items[i].Artist != items[j].Artist {
			return items[i].Artist < items[j].Artist
		}
		if items[i].Album != items[j].Album {
			return items[i].Album < items[j].Album
		}
		return items[i].Title < items[j].Title
	})
	return items, nil
}

func makeItem(path string) Item {
	dir := filepath.Dir(path)
	parent := filepath.Base(dir)
	grand := filepath.Base(filepath.Dir(dir))
	if parent == "." {
		parent = ""
	}
	if grand == "." || grand == string(filepath.Separator) {
		grand = ""
	}
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return Item{Path: path, Title: title, Album: parent, Artist: grand}
}

// LastScan returns the modtime of the cache file, or zero if no cache.
func LastScan() time.Time {
	info, err := os.Stat(cachePath())
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
