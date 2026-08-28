package config

import "testing"

func TestNormalizeSolarMiniDropsEffort(t *testing.T) {
	cfg := Default()
	cfg.Model = "solar-mini"
	cfg.ReasoningEffort = "high"
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if cfg.ReasoningEffort != "" {
		t.Fatalf("solar-mini must omit reasoning_effort, got %q", cfg.ReasoningEffort)
	}
}

func TestNormalizeDefaultEffortIsMedium(t *testing.T) {
	cfg := Default()
	cfg.ReasoningEffort = ""
	if err := cfg.Normalize(); err != nil {
		t.Fatal(err)
	}
	if cfg.ReasoningEffort != "medium" {
		t.Fatalf("got %q", cfg.ReasoningEffort)
	}
}

func TestNormalizeRejectsBadEffort(t *testing.T) {
	cfg := Default()
	cfg.ReasoningEffort = "super"
	if err := cfg.Normalize(); err == nil {
		t.Fatal("expected error")
	}
}
