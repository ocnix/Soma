// Package stream opens HTTP audio streams (icecast / shoutcast / direct mp3
// URLs) for playback. We deliberately do NOT request ICY metadata so the
// response body is pure mp3 frames.
package stream

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Open performs a GET on the given URL and returns the body for streaming
// playback. Caller must close the body. The handshake has a 15s timeout;
// the body itself stays open indefinitely so live streams can run.
func Open(url string) (io.ReadCloser, error) {
	client := &http.Client{Timeout: 0}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("User-Agent", "soma/1.0 (+https://github.com/ocnix/Soma)")
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("stream returned HTTP %d", resp.StatusCode)
	}
	// Wrap so closing the body also cancels the context.
	return &cancelCloser{ReadCloser: resp.Body, cancel: cancel}, nil
}

type cancelCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

// IsURL reports whether s is an http(s) URL.
func IsURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// IsStream reports whether s looks like a live radio stream rather than a
// YouTube link or a direct file URL.
func IsStream(s string) bool {
	if !IsURL(s) {
		return false
	}
	lower := strings.ToLower(s)
	if strings.Contains(lower, "youtube.com") || strings.Contains(lower, "youtu.be") {
		return false
	}
	base := strings.SplitN(lower, "?", 2)[0]
	for _, ext := range []string{".mp3", ".wav", ".flac"} {
		if strings.HasSuffix(base, ext) {
			// Direct file URL — leave it to the file path.
			return false
		}
	}
	return true
}
