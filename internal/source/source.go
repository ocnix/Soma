package source

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"soma/internal/config"
)

// Resolve takes a CLI arg (a local file path or a YouTube URL) and returns a
// playable local file path. For YouTube URLs, the result is cached at
// ~/.config/soma/yt-cache/<videoID>.mp3 so subsequent plays of the same
// video are instant. cleanup is a no-op for cached/local files.
func Resolve(arg string) (path string, cleanup func(), err error) {
	if isYouTubeURL(arg) {
		return resolveYouTube(arg)
	}

	abs, err := filepath.Abs(arg)
	if err != nil {
		return "", noop, fmt.Errorf("resolve path: %w", err)
	}
	if _, err := os.Stat(abs); err != nil {
		return "", noop, fmt.Errorf("file not found: %w", err)
	}
	return abs, noop, nil
}

func noop() {}

func isYouTubeURL(s string) bool {
	low := strings.ToLower(s)
	return strings.Contains(low, "youtube.com/") || strings.Contains(low, "youtu.be/")
}

// videoID parses a YouTube URL and returns the canonical video ID, or "" if
// none could be extracted (in which case we fall back to a non-cached temp
// download).
func videoID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Host)
	switch {
	case strings.Contains(host, "youtu.be"):
		return strings.Trim(u.Path, "/")
	case strings.Contains(host, "youtube.com"):
		if v := u.Query().Get("v"); v != "" {
			return v
		}
		// Shorts: /shorts/<id>
		if strings.HasPrefix(u.Path, "/shorts/") {
			return strings.TrimPrefix(u.Path, "/shorts/")
		}
	}
	return ""
}

func cacheDir() string {
	return filepath.Join(config.Dir(), "yt-cache")
}

type cachedMeta struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	ID    string `json:"id"`
}

// CachedTitle returns the saved title for a cached video, or empty if not
// cached. Useful for recents UI.
func CachedTitle(videoID string) string {
	if videoID == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(cacheDir(), videoID+".json"))
	if err != nil {
		return ""
	}
	var m cachedMeta
	if json.Unmarshal(b, &m) == nil {
		return m.Title
	}
	return ""
}

// DisplayTitle returns the human-readable title for a source. For YouTube
// URLs it consults the sidecar metadata captured at download time. For
// local files it strips directory + extension. Falls back to the raw arg.
func DisplayTitle(arg, path string) string {
	if isYouTubeURL(arg) {
		if id := videoID(arg); id != "" {
			if t := CachedTitle(id); t != "" {
				return t
			}
		}
	}
	if path != "" {
		base := filepath.Base(path)
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	return arg
}

// LooksLikeYouTubeID reports whether s could be a raw YouTube ID (11 chars,
// alphanumeric + - _). Used when migrating old recents whose Title was
// stored as the cached filename.
func LooksLikeYouTubeID(s string) bool {
	if len(s) != 11 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

func resolveYouTube(rawURL string) (string, func(), error) {
	id := videoID(rawURL)
	if id != "" {
		cached := filepath.Join(cacheDir(), id+".mp3")
		if _, err := os.Stat(cached); err == nil {
			fmt.Fprintf(os.Stderr, "▸ using cached %s\n", id)
			return cached, noop, nil
		}
	}
	return downloadYouTube(rawURL, id)
}

func downloadYouTube(rawURL, id string) (string, func(), error) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", noop, fmt.Errorf("yt-dlp not found on PATH — install with `brew install yt-dlp`")
	}

	if err := os.MkdirAll(cacheDir(), 0o755); err != nil {
		return "", noop, fmt.Errorf("cache dir: %w", err)
	}

	var outFile, finalPath string
	if id != "" {
		// Deterministic cached path.
		finalPath = filepath.Join(cacheDir(), id+".mp3")
		outFile = finalPath
	} else {
		// Couldn't extract ID — fall back to temp dir, no caching.
		tmp, err := os.MkdirTemp("", "soma-yt-*")
		if err != nil {
			return "", noop, err
		}
		outFile = filepath.Join(tmp, "track.%(ext)s")
		finalPath = "" // discovered after download
	}

	args := []string{
		"-x", "--audio-format", "mp3",
		"--no-progress",
		"--no-playlist",
		"-o", outFile,
	}
	// We also want the title, so request a print line we can capture.
	args = append(args, "--print", "after_move:%(title)s")
	args = append(args, rawURL)

	cmd := exec.Command("yt-dlp", args...)
	var stdout strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	fmt.Fprintln(os.Stderr, "fetching audio from youtube...")
	if err := cmd.Run(); err != nil {
		return "", noop, fmt.Errorf("yt-dlp failed: %w", err)
	}

	title := strings.TrimSpace(stdout.String())
	if i := strings.Index(title, "\n"); i >= 0 {
		title = strings.TrimSpace(title[:i])
	}

	if id != "" {
		// The output was written to finalPath directly.
		if _, err := os.Stat(finalPath); err != nil {
			return "", noop, fmt.Errorf("expected cached file not found: %s", finalPath)
		}
		// Save metadata sidecar for nicer recents.
		meta := cachedMeta{Title: title, URL: rawURL, ID: id}
		if b, err := json.MarshalIndent(meta, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(cacheDir(), id+".json"), b, 0o644)
		}
		return finalPath, noop, nil
	}

	// No ID branch: glob the temp dir.
	tmpDir := filepath.Dir(outFile)
	matches, _ := filepath.Glob(filepath.Join(tmpDir, "*.mp3"))
	if len(matches) == 0 {
		_ = os.RemoveAll(tmpDir)
		return "", noop, fmt.Errorf("yt-dlp produced no output")
	}
	cleanup := func() { _ = os.RemoveAll(tmpDir) }
	return matches[0], cleanup, nil
}
