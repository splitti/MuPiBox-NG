package tts

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// DetectOwnCgroupPath reads /proc/self/cgroup to find this process's own
// cgroup v2 path, e.g. /sys/fs/cgroup/system.slice/mupibox-ng.service. It
// returns an error on cgroup v1/hybrid hosts or containers without cgroup
// namespace access; callers should treat that as "cgroup limiting
// unavailable" and fall back, not as a fatal error (see NewAutoCPULimiter).
func DetectOwnCgroupPath() (string, error) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", fmt.Errorf("read /proc/self/cgroup: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && parts[0] == "0" && parts[1] == "" {
			return filepath.Join("/sys/fs/cgroup", parts[2]), nil
		}
	}
	return "", fmt.Errorf("cgroup v2 unified hierarchy not detected in /proc/self/cgroup")
}

// CgroupCPULimiter places each rendered job's process into a dedicated
// cgroup v2 child ("tts") and caps it with cpu.max, so a single long Piper
// render cannot starve mpv/UI for CPU. This requires the mupibox-ng systemd
// unit to delegate its own cgroup subtree (Delegate=yes in the unit file);
// that needs no root privileges beyond what the unit already has, since the
// service only ever manages a subtree it already owns. See
// deploy/mupibox-ng.service.
type CgroupCPULimiter struct {
	groupPath string
}

// NewCgroupCPULimiter creates (or reuses) baseCgroup/tts and probes that the
// cpu controller is actually writable there. It fails fast with a clear
// error if cgroup delegation is not set up, so callers can fall back to
// NewNicePriorityLimiter instead of silently doing nothing.
func NewCgroupCPULimiter(baseCgroup string) (*CgroupCPULimiter, error) {
	// The cpu controller must be enabled in *our own* subtree_control before
	// a child cgroup exposes cpu.max at all -- Delegate=yes hands us write
	// access to do this, but does not enable it for us, and systemd can
	// clear it again once no child cgroup exists (observed on a real
	// DietPi/Pi 4: subtree_control reset to empty after removing the only
	// child). enableCPUController is therefore called here and again in
	// Prepare, defensively, before every job.
	if err := enableCPUController(baseCgroup); err != nil {
		return nil, err
	}
	groupPath := filepath.Join(baseCgroup, "tts")
	if err := os.MkdirAll(groupPath, 0755); err != nil {
		return nil, fmt.Errorf("create tts cgroup: %w", err)
	}
	if err := os.WriteFile(filepath.Join(groupPath, "cpu.max"), []byte("max"), 0644); err != nil {
		return nil, fmt.Errorf("cgroup v2 cpu controller not writable at %s (missing Delegate=yes?): %w", groupPath, err)
	}
	return &CgroupCPULimiter{groupPath: groupPath}, nil
}

// enableCPUController writes "+cpu" to baseCgroup's own cgroup.subtree_control
// so its children (our "tts" cgroup) get a cpu.max file at all. A no-op error
// (e.g. already enabled, or not delegated) is surfaced to the caller, who
// treats any error here as "cgroup limiting unavailable" and falls back.
func enableCPUController(baseCgroup string) error {
	path := filepath.Join(baseCgroup, "cgroup.subtree_control")
	if err := os.WriteFile(path, []byte("+cpu"), 0644); err != nil {
		return fmt.Errorf("enable cpu controller in %s (missing Delegate=yes?): %w", path, err)
	}
	return nil
}

// Prepare caps the cgroup's CPU quota for the duration of one job and moves
// the given pid into it. percent<=0 or >=100 means "no limit" (cpu.max=max)
// -- this is a coarse, per-job cap, not a precise scheduling guarantee.
func (c *CgroupCPULimiter) Prepare(pid int, percent int) error {
	_ = enableCPUController(filepath.Dir(c.groupPath)) // defensive re-assert, see NewCgroupCPULimiter
	quota := "max"
	if percent > 0 && percent < 100 {
		quota = fmt.Sprintf("%d 100000", percent*1000)
	}
	if err := os.WriteFile(filepath.Join(c.groupPath, "cpu.max"), []byte(quota), 0644); err != nil {
		return fmt.Errorf("set cpu.max: %w", err)
	}
	if err := os.WriteFile(filepath.Join(c.groupPath, "cgroup.procs"), []byte(strconv.Itoa(pid)), 0644); err != nil {
		return fmt.Errorf("move pid into tts cgroup: %w", err)
	}
	return nil
}

// NicePriorityLimiter is the Pi-3/4 fallback when cgroup v2 delegation is
// unavailable (older DietPi image, no unified hierarchy): it just lowers
// the synthesis subprocess's scheduling priority so the kernel scheduler
// favors mpv/UI under contention. It cannot express an exact percentage,
// which matches the requirement that background CPU is a UX knob, not a
// hard guarantee.
type NicePriorityLimiter struct {
	log *slog.Logger
}

func NewNicePriorityLimiter(log *slog.Logger) *NicePriorityLimiter {
	if log == nil {
		log = slog.Default()
	}
	return &NicePriorityLimiter{log: log}
}

func (n *NicePriorityLimiter) Prepare(pid int, _ int) error {
	if err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, 15); err != nil {
		return fmt.Errorf("lower piper process priority: %w", err)
	}
	return nil
}

// NewAutoCPULimiter picks the most robust option available on this host:
// cgroup v2 delegation if usable, otherwise the nice-priority fallback.
// Concurrency is separately fixed at 1 in Manager, which together with
// either limiter keeps a single long render from starving playback/UI.
func NewAutoCPULimiter(baseCgroup string, log *slog.Logger) CPULimiter {
	if log == nil {
		log = slog.Default()
	}
	if baseCgroup != "" {
		if l, err := NewCgroupCPULimiter(baseCgroup); err == nil {
			return l
		} else {
			log.Warn("cgroup CPU limiting unavailable for tts, falling back to process priority", "error", err)
		}
	}
	return NewNicePriorityLimiter(log)
}
