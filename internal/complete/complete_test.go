package complete

import (
	"strings"
	"testing"
)

func TestQuerySlashFiltersCommands(t *testing.T) {
	got := Query("/mo")
	if len(got) != 1 || got[0].Name != "/model" {
		t.Fatalf("got %+v", got)
	}
}

func TestQuerySlashHidesAliases(t *testing.T) {
	got := Query("/")
	for _, it := range got {
		if it.Alias {
			t.Fatalf("alias %s shown for /", it.Name)
		}
	}
	if len(got) < 6 {
		t.Fatalf("expected primary commands, got %d", len(got))
	}
}

func TestQueryModelArgs(t *testing.T) {
	got := Query("/model so")
	if len(got) == 0 {
		t.Fatal("expected model matches")
	}
	for _, it := range got {
		if !strings.HasPrefix(it.Name, "so") && !strings.HasPrefix(it.Name, "solar") {
			t.Fatalf("unexpected %q", it.Name)
		}
	}
}

func TestApplyCommandAndArg(t *testing.T) {
	if got := Apply("/mo", Item{Name: "/model", Arg: true}); got != "/model " {
		t.Fatalf("got %q", got)
	}
	if got := Apply("/help", Item{Name: "/help"}); got != "/help" {
		t.Fatalf("got %q", got)
	}
	if got := Apply("/model so", Item{Name: "solar-pro4"}); got != "/model solar-pro4" {
		t.Fatalf("got %q", got)
	}
}

func TestReady(t *testing.T) {
	if Ready("/mo") {
		t.Fatal("/mo should not be ready")
	}
	if !Ready("/help") {
		t.Fatal("/help should be ready")
	}
	if Ready("/model") {
		t.Fatal("/model should wait for an argument")
	}
	if Ready("/model so") {
		t.Fatal("/model so should wait for a full id")
	}
	if !Ready("/model solar-pro4") {
		t.Fatal("exact model should be ready")
	}
	if !Ready("안녕") {
		t.Fatal("plain text should be ready")
	}
}

func TestScriptMentionsModels(t *testing.T) {
	for _, shell := range []string{"zsh", "bash", "fish"} {
		s, err := Script(shell)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(s, "solar-pro4") || !strings.Contains(s, "effort") {
			t.Fatalf("%s script missing model or flags", shell)
		}
	}
}

func TestNamesCommands(t *testing.T) {
	got, err := Names("commands", "log")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}
