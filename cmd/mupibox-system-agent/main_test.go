package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActiveSwap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "swaps")
	header := "Filename\tType\tSize\tUsed\tPriority\n"
	if err := os.WriteFile(path, []byte(header), 0600); err != nil {
		t.Fatal(err)
	}
	active, err := activeSwap(path)
	if err != nil || active {
		t.Fatalf("header-only swap table: active=%t err=%v", active, err)
	}
	if err = os.WriteFile(path, []byte(header+"/swapfile file 1024 0 -2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	active, err = activeSwap(path)
	if err != nil || !active {
		t.Fatalf("active swap table: active=%t err=%v", active, err)
	}
}

func TestSetInitialTurbo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.txt")
	if err := os.WriteFile(path, []byte("# Raspberry Pi\n#initial_turbo=20\ndtoverlay=vc4-kms-v3d\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := setInitialTurbo(path, 30); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(raw), "initial_turbo=") != 1 || !strings.Contains(string(raw), "initial_turbo=30") {
		t.Fatalf("unexpected config: %s", raw)
	}
	if err = setInitialTurbo(path, 61); err == nil {
		t.Fatal("accepted invalid initial turbo duration")
	}
}
