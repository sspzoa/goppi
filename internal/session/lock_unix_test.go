//go:build unix

package session

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

func TestHoldExclusive(t *testing.T) {
	if os.Getenv("GOPPI_LOCK_CHILD") == "1" {
		lk, err := Hold(os.Getenv("GOPPI_LOCK_ID"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		defer lk.Release()
		fmt.Println("held")
		_, _ = os.Stdin.Read(make([]byte, 1))
		os.Exit(0)
	}
	dir := t.TempDir()
	t.Setenv("GOPPI_DATA_DIR", dir)
	id := NewID()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHoldExclusive$")
	cmd.Env = append(os.Environ(),
		"GOPPI_DATA_DIR="+dir,
		"GOPPI_LOCK_CHILD=1",
		"GOPPI_LOCK_ID="+id,
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 8)
	if _, err := stdout.Read(buf); err != nil {
		t.Fatal(err)
	}
	if _, err := Hold(id); err == nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		t.Fatal("second hold should fail")
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	lk, err := Hold(id)
	if err != nil {
		t.Fatal(err)
	}
	lk.Release()
}
