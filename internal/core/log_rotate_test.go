package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotateLogIfNeeded(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mihomo.log")

	rotateLogIfNeeded(path, 100) // missing file: no-op
	if _, err := os.Stat(path + ".old"); !os.IsNotExist(err) {
		t.Fatalf("missing file should not create .old: %v", err)
	}

	if err := os.WriteFile(path, []byte("small"), 0o600); err != nil {
		t.Fatal(err)
	}
	rotateLogIfNeeded(path, 100)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("small log should remain: %v", err)
	}

	big := make([]byte, 150)
	if err := os.WriteFile(path, big, 0o600); err != nil {
		t.Fatal(err)
	}
	rotateLogIfNeeded(path, 100)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("oversize log should be renamed away")
	}
	if info, err := os.Stat(path + ".old"); err != nil || info.Size() != 150 {
		t.Fatalf("expected .old size 150, got %v err=%v", info, err)
	}
}
