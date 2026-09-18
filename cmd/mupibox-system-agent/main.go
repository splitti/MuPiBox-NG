// mupibox-system-agent performs a very small set of privileged host operations.
// It has no network listener; only the mupibox group can access its Unix socket.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const socketPath = "/run/mupibox-system-agent/control.sock"
const networkStateDir = "/var/lib/mupibox-ng/network"

var interfaceName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
var releaseName = regexp.MustCompile(`^(rollback|v?[0-9][A-Za-z0-9._-]{0,63})$`)

type request struct {
	Action              string `json:"action"`
	Interface           string `json:"interface"`
	Enabled             bool   `json:"enabled"`
	Target              string `json:"target"`
	SwapEnabled         *bool  `json:"swap_enabled,omitempty"`
	WaitOnlineEnabled   *bool  `json:"wait_online_enabled,omitempty"`
	PerformanceMode     string `json:"performance_mode,omitempty"`
	InitialTurboSeconds *int   `json:"initial_turbo_seconds,omitempty"`
	PowerAction         string `json:"power_action,omitempty"`
}
type response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func runCommand(ctx context.Context, name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s is not installed", name)
	}
	command := exec.CommandContext(ctx, path, args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func wifiInterface(name string) error {
	if !interfaceName.MatchString(name) {
		return errors.New("invalid Wi-Fi interface")
	}
	if _, err := os.Stat(filepath.Join("/sys/class/net", name, "wireless")); err != nil {
		return fmt.Errorf("unknown Wi-Fi adapter %s", name)
	}
	return nil
}

func findWPAConfig(name string) (string, error) {
	if err := os.MkdirAll(networkStateDir, 0750); err != nil {
		return "", err
	}
	target := filepath.Join(networkStateDir, "wpa_supplicant-"+name+".conf")
	if info, err := os.Stat(target); err == nil && info.Mode().IsRegular() {
		return target, nil
	}
	content := []byte("ctrl_interface=DIR=/run/wpa_supplicant GROUP=netdev\nupdate_config=1\n")
	candidates := []string{filepath.Join("/etc/wpa_supplicant", "wpa_supplicant-"+name+".conf"), "/etc/wpa_supplicant/wpa_supplicant.conf"}
	for _, candidate := range candidates {
		if raw, err := os.ReadFile(candidate); err == nil && len(raw) > 0 {
			content = raw
			break
		}
	}
	if err := os.WriteFile(target, content, 0600); err != nil {
		return "", err
	}
	return target, nil
}

func waitForControl(ctx context.Context, name string) error {
	path := filepath.Join("/run/wpa_supplicant", name)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(path); err == nil {
			if group, lookupErr := user.LookupGroup("netdev"); lookupErr == nil {
				if gid, parseErr := strconv.Atoi(group.Gid); parseErr == nil {
					_ = os.Chown(path, -1, gid)
				}
			}
			_ = os.Chmod(path, 0660)
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("wpa_supplicant control socket did not appear")
		case <-ticker.C:
		}
	}
}

func setWiFiState(ctx context.Context, name string, enabled bool) error {
	if err := wifiInterface(name); err != nil {
		return err
	}
	control := filepath.Join("/run/wpa_supplicant", name)
	pidFile := filepath.Join("/run/mupibox-system-agent", "wpa-"+name+".pid")
	if !enabled {
		if raw, err := os.ReadFile(pidFile); err == nil {
			if pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw))); parseErr == nil && pid > 1 {
				if process, findErr := os.FindProcess(pid); findErr == nil {
					_ = process.Signal(syscall.SIGTERM)
				}
			}
			_ = os.Remove(pidFile)
			_ = os.Remove(control)
		}
		return runCommand(ctx, "ip", "link", "set", "dev", name, "down")
	}
	_ = runCommand(ctx, "rfkill", "unblock", "wifi")
	if err := runCommand(ctx, "ip", "link", "set", "dev", name, "up"); err != nil {
		return err
	}
	if _, err := os.Stat(control); err == nil {
		return nil
	}
	config, err := findWPAConfig(name)
	if err != nil {
		return err
	}
	if err = os.MkdirAll("/run/wpa_supplicant", 0775); err != nil {
		return err
	}
	path, err := exec.LookPath("wpa_supplicant")
	if err != nil {
		return errors.New("wpa_supplicant is not installed")
	}
	command := exec.CommandContext(ctx, path, "-B", "-P", pidFile, "-i", name, "-c", config, "-C", "/run/wpa_supplicant")
	if output, startErr := command.CombinedOutput(); startErr != nil {
		return fmt.Errorf("start wpa_supplicant: %w: %s", startErr, strings.TrimSpace(string(output)))
	}
	waitCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	return waitForControl(waitCtx, name)
}

func optionalSystemdUnit(ctx context.Context, action string, unit string) error {
	args := []string{action}
	if action == "mask" {
		args = append(args, "--now")
	}
	args = append(args, unit)
	err := runCommand(ctx, "systemctl", args...)
	if err != nil && (strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not found")) {
		return nil
	}
	return err
}

func setSwap(ctx context.Context, enabled bool) error {
	if enabled {
		if err := runCommand(ctx, "systemctl", "unmask", "swap.target"); err != nil {
			return err
		}
		return runCommand(ctx, "systemctl", "start", "swap.target")
	}
	active, err := activeSwap("/proc/swaps")
	if err != nil {
		return fmt.Errorf("read active swap: %w", err)
	}
	if active {
		if err = runCommand(ctx, "swapoff", "-a"); err != nil {
			// Some swap implementations return a non-zero exit after the last
			// device has already disappeared. Only fail when swap is still active.
			if stillActive, readErr := activeSwap("/proc/swaps"); readErr != nil || stillActive {
				return err
			}
		}
	}
	return runCommand(ctx, "systemctl", "mask", "--now", "swap.target")
}

func activeSwap(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	return len(lines) > 1, nil
}

func setWaitOnline(ctx context.Context, enabled bool) error {
	units := []string{"systemd-networkd-wait-online.service", "NetworkManager-wait-online.service"}
	action := "mask"
	if enabled {
		action = "unmask"
	}
	for _, unit := range units {
		if err := optionalSystemdUnit(ctx, action, unit); err != nil {
			return err
		}
	}
	return nil
}

func setPerformanceMode(mode string) error {
	if mode == "" {
		return nil
	}
	if mode != "balanced" && mode != "performance" && mode != "powersave" {
		return errors.New("invalid performance mode")
	}
	paths, err := filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_governor")
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}
	for _, path := range paths {
		available, _ := os.ReadFile(filepath.Join(filepath.Dir(path), "scaling_available_governors"))
		availableSet := map[string]bool{}
		for _, value := range strings.Fields(string(available)) {
			availableSet[value] = true
		}
		governor := mode
		if mode == "balanced" {
			for _, candidate := range []string{"schedutil", "ondemand", "powersave", "performance"} {
				if availableSet[candidate] {
					governor = candidate
					break
				}
			}
		}
		if !availableSet[governor] {
			return fmt.Errorf("CPU governor %s is unavailable", governor)
		}
		if err = os.WriteFile(path, []byte(governor), 0644); err != nil {
			return fmt.Errorf("set CPU governor: %w", err)
		}
	}
	return nil
}

func setInitialTurbo(path string, seconds int) error {
	if seconds < 0 || seconds > 60 {
		return errors.New("initial turbo must be 0..60 seconds")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read Raspberry Pi boot config: %w", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	replacement := fmt.Sprintf("initial_turbo=%d", seconds)
	replaced := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		if strings.HasPrefix(trimmed, "initial_turbo=") {
			lines[index] = replacement
			replaced = true
			break
		}
	}
	if !replaced {
		lines = append(lines, replacement)
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".mupibox-config-*")
	if err != nil {
		return fmt.Errorf("prepare Raspberry Pi boot config: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.WriteString(strings.Join(lines, "\n") + "\n"); err == nil {
		err = temporary.Chmod(info.Mode().Perm())
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace Raspberry Pi boot config: %w", err)
	}
	return nil
}

func setInitialTurboOnHost(seconds int) error {
	for _, path := range []string{"/boot/firmware/config.txt", "/boot/config.txt"} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return setInitialTurbo(path, seconds)
		}
	}
	return errors.New("Raspberry Pi boot config was not found")
}

func applySystemTuning(ctx context.Context, input request) error {
	if input.SwapEnabled != nil {
		if err := setSwap(ctx, *input.SwapEnabled); err != nil {
			return err
		}
	}
	if input.WaitOnlineEnabled != nil {
		if err := setWaitOnline(ctx, *input.WaitOnlineEnabled); err != nil {
			return err
		}
	}
	if err := setPerformanceMode(input.PerformanceMode); err != nil {
		return err
	}
	if input.InitialTurboSeconds != nil {
		return setInitialTurboOnHost(*input.InitialTurboSeconds)
	}
	return nil
}

func schedulePower(action string) error {
	if action != "reboot" && action != "poweroff" {
		return errors.New("invalid power action")
	}
	path, err := exec.LookPath("systemctl")
	if err != nil {
		return errors.New("systemctl is not installed")
	}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		if output, runErr := exec.Command(path, action).CombinedOutput(); runErr != nil {
			log.Printf("systemctl %s failed: %v: %s", action, runErr, strings.TrimSpace(string(output)))
		}
	}()
	return nil
}

func handle(connection net.Conn) {
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(25 * time.Second))
	decoder := json.NewDecoder(bufio.NewReader(connection))
	decoder.DisallowUnknownFields()
	var input request
	if err := decoder.Decode(&input); err != nil {
		_ = json.NewEncoder(connection).Encode(response{Error: "invalid request"})
		return
	}
	var err error
	switch input.Action {
	case "wifi-state":
		err = setWiFiState(context.Background(), input.Interface, input.Enabled)
	case "release-update":
		if !releaseName.MatchString(input.Target) {
			err = errors.New("invalid release target")
		} else {
			err = runCommand(context.Background(), "systemctl", "start", "--no-block", "mupibox-update@"+input.Target+".service")
		}
	case "system-tuning":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err = applySystemTuning(ctx, input)
		cancel()
	case "power":
		err = schedulePower(input.PowerAction)
	default:
		err = errors.New("unsupported action")
	}
	result := response{OK: err == nil}
	if err != nil {
		result.Error = err.Error()
	}
	_ = json.NewEncoder(connection).Encode(result)
}

func run() error {
	if os.Geteuid() != 0 {
		return errors.New("mupibox-system-agent must run as root")
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0770); err != nil {
		return err
	}
	_ = os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	defer listener.Close()
	defer os.Remove(socketPath)
	if err = os.Chmod(socketPath, 0660); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() { <-ctx.Done(); _ = listener.Close() }()
	for {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			return acceptErr
		}
		go handle(connection)
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
