package main

import (
	"flag"
	"fmt"
	"os"

	"soma/internal/player"
	"soma/internal/source"
	"soma/internal/ui"
)

func main() {
	rate := flag.Float64("rate", 0.15, "8D pan rate in Hz (one full sweep per 1/rate seconds)")
	dry := flag.Bool("dry", false, "disable the 8D effect (play normally)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "soma — 8D audio for focus ☕\n\nusage: %s [flags] <file-or-youtube-url>\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	path, cleanup, err := source.Resolve(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "source error:", err)
		os.Exit(1)
	}
	defer cleanup()

	sess, err := player.Start(path, *rate, *dry)
	if err != nil {
		fmt.Fprintln(os.Stderr, "playback error:", err)
		os.Exit(1)
	}
	defer sess.Close()

	if err := ui.Run(sess.State, sess.Done); err != nil {
		fmt.Fprintln(os.Stderr, "ui error:", err)
		os.Exit(1)
	}
}
