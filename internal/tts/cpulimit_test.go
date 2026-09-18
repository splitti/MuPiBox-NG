package tts

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCgroupCPULimiterWritesQuotaAndMovesPID(t *testing.T) {
	base := t.TempDir()
	l, err := NewCgroupCPULimiter(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Prepare(4242, 30); err != nil {
		t.Fatal(err)
	}
	quota, err := os.ReadFile(filepath.Join(base, "tts", "cpu.max"))
	if err != nil {
		t.Fatal(err)
	}
	if string(quota) != "30000 100000" {
		t.Fatalf("expected 30%% quota, got %q", quota)
	}
	procs, err := os.ReadFile(filepath.Join(base, "tts", "cgroup.procs"))
	if err != nil {
		t.Fatal(err)
	}
	if string(procs) != strconv.Itoa(4242) {
		t.Fatalf("expected pid written to cgroup.procs, got %q", procs)
	}
}

func TestCgroupCPULimiterFullPercentMeansNoLimit(t *testing.T) {
	l, err := NewCgroupCPULimiter(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Prepare(1, 100); err != nil {
		t.Fatal(err)
	}
	quota, err := os.ReadFile(filepath.Join(l.groupPath, "cpu.max"))
	if err != nil {
		t.Fatal(err)
	}
	if string(quota) != "max" {
		t.Fatalf("expected unlimited quota for 100%%, got %q", quota)
	}
}

func TestNicePriorityLimiterRejectsInvalidPID(t *testing.T) {
	l := NewNicePriorityLimiter(nil)
	if err := l.Prepare(-1, 30); err == nil {
		t.Fatal("expected an error for an invalid pid")
	}
}

func TestDetectOwnCgroupPathReturnsAbsolutePathUnderSysFsCgroup(t *testing.T) {
	path, err := DetectOwnCgroupPath()
	if err != nil {
		t.Skipf("cgroup v2 not available in this test environment: %v", err)
	}
	if !filepath.IsAbs(path) || !strings.HasPrefix(path, "/sys/fs/cgroup/") {
		t.Fatalf("expected an absolute path under /sys/fs/cgroup, got %q", path)
	}
}

func TestNewAutoCPULimiterFallsBackWhenCgroupUnavailable(t *testing.T) {
	// A path that cannot exist as a cgroup (regular file blocking mkdir) forces the fallback.
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	limiter := NewAutoCPULimiter(filepath.Join(blocked, "cgroup-child"), nil)
	if _, ok := limiter.(*NicePriorityLimiter); !ok {
		t.Fatalf("expected fallback to NicePriorityLimiter, got %T", limiter)
	}
}
