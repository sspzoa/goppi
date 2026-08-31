package main

import (
	"github.com/sspzoa/goppi/internal/config"
)

func peelWorkdirArgs(args []string) (wd string, rest []string) {
	rest = args
	for len(rest) >= 2 && (rest[0] == "-C" || rest[0] == "--cwd") {
		wd = rest[1]
		rest = rest[2:]
	}
	return wd, rest
}

func loadConfigWithWorkdirArgs(args []string) (config.Config, []string, error) {
	wd, rest := peelWorkdirArgs(args)
	cfg, err := config.Load()
	if err != nil {
		return config.Config{}, rest, err
	}
	if wd != "" {
		cfg.WorkDir = wd
		if err := cfg.Normalize(); err != nil {
			return config.Config{}, rest, err
		}
	}
	return cfg, rest, nil
}
