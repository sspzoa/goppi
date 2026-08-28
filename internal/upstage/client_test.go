package upstage

import "testing"

func TestSupportsReasoning(t *testing.T) {
	if !SupportsReasoning("solar-pro4") {
		t.Fatal("solar-pro4 should support reasoning")
	}
	if SupportsReasoning("solar-mini") {
		t.Fatal("solar-mini must not send reasoning_effort")
	}
}

func TestKnownModel(t *testing.T) {
	if !KnownModel("solar-pro4") {
		t.Fatal("expected solar-pro4")
	}
	if KnownModel("gpt-4.1") {
		t.Fatal("openai models are not in catalog")
	}
}
