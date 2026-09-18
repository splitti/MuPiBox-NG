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

func TestActiveDRMCard(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := activeDRMCard(dir); err == nil {
		t.Fatal("expected error when no connector is present")
	}
	write("card0-HDMI-A-1/enabled", "disabled\n")
	if _, err := activeDRMCard(dir); err == nil {
		t.Fatal("expected error when no connector is enabled")
	}
	write("card1-DSI-1/enabled", "enabled\n")
	card, err := activeDRMCard(dir)
	if err != nil {
		t.Fatal(err)
	}
	if card != "/dev/dri/card1" {
		t.Fatalf("got %s, want /dev/dri/card1", card)
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

func TestSambaConfig(t *testing.T) {
	guest, err := sambaConfig("guest", "MuPiBox", "WORKGROUP")
	if err != nil || !strings.Contains(guest, "path = /srv/mupibox") || !strings.Contains(guest, "guest ok = yes") {
		t.Fatalf("unexpected guest config: %q %v", guest, err)
	}
	protected, err := sambaConfig("password", "Music", "HOME")
	if err != nil || !strings.Contains(protected, "valid users = mupibox") || strings.Contains(protected, "guest ok = yes") {
		t.Fatalf("unexpected password config: %q %v", protected, err)
	}
	if _, err = sambaConfig("guest", "bad/name", "HOME"); err == nil {
		t.Fatal("accepted invalid share name")
	}
}
