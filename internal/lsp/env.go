package lsp

import "github.com/sspzoa/goppi/internal/config"

func cleanEnv(env []string) []string { return config.ScrubEnv(env) }
