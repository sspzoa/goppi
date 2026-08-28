package session

import (
	"testing"

	"github.com/sspzoa/goppi/internal/config"
	"github.com/sspzoa/goppi/internal/provider"
)

func TestPersistListDelete(t *testing.T) {
	t.Setenv("GOPPI_DATA_DIR", t.TempDir())
	cfg := config.Default()
	cfg.WorkDir = "/tmp/proj"
	cfg.Model = "solar-pro4"
	id, err := Persist(cfg, "", []provider.Message{
		{Role: provider.RoleUser, Content: "hello world"},
	})
	if err != nil {
		t.Fatal(err)
	}
	items, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != id {
		t.Fatalf("list = %+v", items)
	}
	if items[0].Title != "hello world" {
		t.Fatalf("title = %q", items[0].Title)
	}
	last, err := LoadLast()
	if err != nil {
		t.Fatal(err)
	}
	if last.ID != id {
		t.Fatalf("last = %s", last.ID)
	}
	if err := Delete(id); err != nil {
		t.Fatal(err)
	}
	items, err = List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty, got %+v", items)
	}
}
