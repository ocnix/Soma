package source

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Resolve takes a CLI arg (a local file path or a YouTube URL) and returns a
// playable local file path. cleanup removes any temp files created.
func Resolve(arg string) (path string, cleanup func(), err error) {
	if isYouTubeURL(arg) {
		return downloadYouTube(arg)
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
	s = strings.ToLower(s)
	return strings.Contains(s, "youtube.com/") || strings.Contains(s, "youtu.be/")
}

func downloadYouTube(url string) (string, func(), error) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", noop, fmt.Errorf("yt-dlp not found on PATH — install with `brew install yt-dlp`")
	}

	dir, err := os.MkdirTemp("", "soma-*")
	if err != nil {
		return "", noop, fmt.Errorf("temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	out := filepath.Join(dir, "track.%(ext)s")
	cmd := exec.Command("yt-dlp",
		"-x", "--audio-format", "mp3",
		"--no-progress",
		"-o", out,
		url,
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	fmt.Fprintln(os.Stderr, "fetching audio from youtube...")
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", noop, fmt.Errorf("yt-dlp failed: %w", err)
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "track.*"))
	if len(matches) == 0 {
		cleanup()
		return "", noop, fmt.Errorf("yt-dlp produced no output")
	}
	return matches[0], cleanup, nil
}
