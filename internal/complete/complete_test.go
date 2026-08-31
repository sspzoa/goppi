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

func TestQueryCopy(t *testing.T) {
	got := Query("/cop")
	if len(got) != 1 || got[0].Name != "/copy" {
		t.Fatalf("got %+v", got)
	}
}

func TestQueryRetry(t *testing.T) {
	got := Query("/re")
	if len(got) != 1 || got[0].Name != "/retry" {
		t.Fatalf("got %+v", got)
	}
}

func TestQueryExport(t *testing.T) {
	got := Query("/exp")
	if len(got) != 1 || got[0].Name != "/export" {
		t.Fatalf("got %+v", got)
	}
	if !Ready("/export") {
		t.Fatal("/export should run without an id")
	}
}

func TestQueryDiff(t *testing.T) {
	got := Query("/di")
	if len(got) != 1 || got[0].Name != "/diff" {
		t.Fatalf("got %+v", got)
	}
}

func TestQueryJobs(t *testing.T) {
	got := Query("/jo")
	if len(got) != 1 || got[0].Name != "/jobs" {
		t.Fatalf("got %+v", got)
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

func TestQueryDelete(t *testing.T) {
	got := Query("/del")
	if len(got) != 1 || got[0].Name != "/delete" {
		t.Fatalf("got %+v", got)
	}
	if !Ready("/delete") {
		t.Fatal("/delete should run without an id")
	}
}

func TestReadySessionsNeedsID(t *testing.T) {
	if Ready("/sessions") {
		t.Fatal("/sessions should wait for an id")
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
		if !strings.Contains(s, "mcp") {
			t.Fatalf("%s script missing mcp", shell)
		}
		if !strings.Contains(s, "sandbox") || !strings.Contains(s, "strict") {
			t.Fatalf("%s script missing sandbox/strict", shell)
		}
		if !strings.Contains(s, "worktree") {
			t.Fatalf("%s script missing worktree", shell)
		}
		if !strings.Contains(s, "complete") {
			t.Fatalf("%s script missing complete", shell)
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
