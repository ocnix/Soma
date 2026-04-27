package main

import (
	"flag"
	"fmt"
	"os"

	"soma/internal/config"
	"soma/internal/ui"
)

func main() {
	rate := flag.Float64("rate", 0, "8D pan rate in Hz (overrides saved profile; press [/] in-app)")
	resetProfile := flag.Bool("reset", false, "reset the saved profile to defaults")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "soma — 8D audio for focus ☕\n\nusage: %s [flags] [file-or-youtube-url]\n\nWith no argument, the home screen opens. Profile is saved to ~/.config/soma/.\n\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	var p config.Profile
	if *resetProfile {
		p = config.Default
		_ = config.SaveProfile(p)
	} else {
		p = config.LoadProfile()
	}

	if *rate > 0 {
		p.Rate = *rate
	}

	var initialPath string
	if flag.NArg() > 0 {
		initialPath = flag.Arg(0)
	}

	if err := ui.Run(initialPath, p); err != nil {
		fmt.Fprintln(os.Stderr, "ui error:", err)
		os.Exit(1)
	}
}
