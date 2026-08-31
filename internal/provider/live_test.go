package provider

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sspzoa/goppi/internal/upstage"
)

func TestLiveSolarSmoke(t *testing.T) {
	if os.Getenv("GOPPI_LIVE") != "1" {
		t.Skip("set GOPPI_LIVE=1 and UPSTAGE_API_KEY to run")
	}
	key := os.Getenv("UPSTAGE_API_KEY")
	if key == "" {
		t.Skip("UPSTAGE_API_KEY not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	resp, err := NewSolar(upstage.New(key, "")).Chat(ctx, ChatRequest{
		Model:     "solar-mini",
		Messages:  []Message{{Role: RoleUser, Content: "Reply with exactly: pong"}},
		MaxTokens: 32,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(resp.Message.Content), "pong") {
		t.Fatalf("got %q", resp.Message.Content)
	}
}
