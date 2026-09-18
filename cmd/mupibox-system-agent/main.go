// mupibox-system-agent performs a very small set of privileged host operations.
// It has no network listener; only the mupibox group can access its Unix socket.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
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
	Action              string   `json:"action"`
	Interface           string   `json:"interface"`
	Enabled             bool     `json:"enabled"`
	Target              string   `json:"target"`
	SwapEnabled         *bool    `json:"swap_enabled,omitempty"`
	WaitOnlineEnabled   *bool    `json:"wait_online_enabled,omitempty"`
	PerformanceMode     string   `json:"performance_mode,omitempty"`
	InitialTurboSeconds *int     `json:"initial_turbo_seconds,omitempty"`
	PowerAction         string   `json:"power_action,omitempty"`
	SambaEnabled        bool     `json:"samba_enabled,omitempty"`
	SambaMode           string   `json:"samba_mode,omitempty"`
	SambaShareName      string   `json:"samba_share_name,omitempty"`
	SambaWorkgroup      string   `json:"samba_workgroup,omitempty"`
	SambaPassword       string   `json:"samba_password,omitempty"`
	IPv4Mode            string   `json:"ipv4_mode,omitempty"`
	IPv4Address         string   `json:"ipv4_address,omitempty"`
	IPv4Gateway         string   `json:"ipv4_gateway,omitempty"`
	IPv4DNS             []string `json:"ipv4_dns,omitempty"`
}

func applyIPv4(ctx context.Context, input request) error {
	if !interfaceName.MatchString(input.Interface) {
		return errors.New("invalid network interface")
	}
	if _, err := os.Stat(filepath.Join("/sys/class/net", input.Interface)); err != nil {
		return errors.New("network interface not found")
	}
	path := "/boot/dietpi/dietpi-network"
	if _, err := os.Stat(path); err != nil {
		return errors.New("DietPi network tool not found")
	}
	args := []string{"apply", input.Interface, "--enable", "--hotplug"}
	if input.IPv4Mode == "dhcp" {
		args = append(args, "--dhcp")
	} else if input.IPv4Mode == "static" {
		if _, _, err := net.ParseCIDR(input.IPv4Address); err != nil {
			return errors.New("invalid static IPv4 address")
		}
		if ip := net.ParseIP(input.IPv4Gateway); ip == nil || ip.To4() == nil {
			return errors.New("invalid IPv4 gateway")
		}
		args = append(args, "--static", "--ip", input.IPv4Address, "--gateway", input.IPv4Gateway)
		if len(input.IPv4DNS) > 0 {
			args = append(args, "--dns", strings.Join(input.IPv4DNS, " "))
		}
	} else {
		return errors.New("invalid IPv4 mode")
	}
	return runCommand(ctx, path, args...)
}

type response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  string `json:"data,omitempty"`
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

func runCommandInput(ctx context.Context, input, name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%s is not installed", name)
	}
	command := exec.CommandContext(ctx, path, args...)
	command.Stdin = strings.NewReader(input)
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

func selectWiFiAdapter(ctx context.Context, name string) error {
	if err := wifiInterface(name); err != nil {
		return err
	}
	if err := setWiFiState(ctx, name, true); err != nil {
		return fmt.Errorf("enable selected Wi-Fi adapter: %w", err)
	}
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		other := entry.Name()
		if other == name || !interfaceName.MatchString(other) {
			continue
		}
		if _, statErr := os.Stat(filepath.Join("/sys/class/net", other, "wireless")); statErr != nil {
			continue
		}
		if err = setWiFiState(ctx, other, false); err != nil {
			return fmt.Errorf("disable Wi-Fi adapter %s: %w", other, err)
		}
	}
	return nil
}

func setOnboardWiFi(enabled bool) error {
	for _, path := range []string{"/boot/firmware/config.txt", "/boot/config.txt"} {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(string(data), "\n")
		found := false
		for index, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "dtoverlay=disable-wifi" && trimmed != "#dtoverlay=disable-wifi" {
				continue
			}
			found = true
			if enabled {
				lines[index] = "#dtoverlay=disable-wifi"
			} else {
				lines[index] = "dtoverlay=disable-wifi"
			}
		}
		if !found && !enabled {
			lines = append(lines, "dtoverlay=disable-wifi")
		}
		return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
	}
	return errors.New("Raspberry Pi boot config not found")
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
	if info, err := os.Stat("/boot/dietpi/func/dietpi-set_swapfile"); err == nil && info.Mode().IsRegular() {
		size := "0"
		if enabled {
			size = "1"
		}
		unit := fmt.Sprintf("mupibox-swap-%d", time.Now().UnixNano())
		return runCommand(ctx, "systemd-run", "--quiet", "--wait", "--collect", "--pipe", "--unit", unit, "/boot/dietpi/func/dietpi-set_swapfile", size)
	}
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

func sambaConfig(mode, shareName, workgroup string) (string, error) {
	validShare := regexp.MustCompile(`^[A-Za-z0-9 _-]{1,32}$`)
	validWorkgroup := regexp.MustCompile(`^[A-Za-z0-9_-]{1,15}$`)
	if mode != "guest" && mode != "password" {
		return "", errors.New("invalid Samba mode")
	}
	if !validShare.MatchString(shareName) || !validWorkgroup.MatchString(workgroup) {
		return "", errors.New("invalid Samba share name or workgroup")
	}
	access := "guest ok = yes\n"
	if mode == "password" {
		access = "guest ok = no\nvalid users = mupibox\n"
	}
	return fmt.Sprintf(`# Managed by MuPiBox-NG. Changes are overwritten by the admin interface.
[global]
workgroup = %s
server role = standalone server
server string = MuPiBox
map to guest = Bad User
server min protocol = SMB2
load printers = no
printing = bsd
printcap name = /dev/null
disable spoolss = yes

[%s]
path = /srv/mupibox
browseable = yes
read only = no
force user = mupibox
create mask = 0664
directory mask = 0775
%s`, workgroup, shareName, access), nil
}

func configureSamba(ctx context.Context, input request) error {
	if !input.SambaEnabled {
		if err := runCommand(ctx, "systemctl", "disable", "--now", "smbd.service", "nmbd.service"); err != nil && !strings.Contains(err.Error(), "not found") {
			return err
		}
		return nil
	}
	config, err := sambaConfig(input.SambaMode, input.SambaShareName, input.SambaWorkgroup)
	if err != nil {
		return err
	}
	if _, err = exec.LookPath("smbd"); err != nil {
		return errors.New("Samba is not installed; run the MuPiBox installer again")
	}
	if input.SambaMode == "password" {
		if input.SambaPassword != "" {
			if len(input.SambaPassword) < 8 || len(input.SambaPassword) > 128 {
				return errors.New("Samba password must contain 8..128 characters")
			}
			if err = runCommandInput(ctx, input.SambaPassword+"\n"+input.SambaPassword+"\n", "smbpasswd", "-s", "-a", "mupibox"); err != nil {
				return err
			}
		} else if err = runCommand(ctx, "pdbedit", "-L", "-u", "mupibox"); err != nil {
			return errors.New("set a Samba password when enabling password protection for the first time")
		}
	}
	if err = os.MkdirAll("/srv/mupibox", 0775); err != nil {
		return err
	}
	if account, lookupErr := user.Lookup("mupibox"); lookupErr == nil {
		uid, uidErr := strconv.Atoi(account.Uid)
		gid, gidErr := strconv.Atoi(account.Gid)
		if uidErr == nil && gidErr == nil {
			_ = os.Chown("/srv/mupibox", uid, gid)
		}
	}
	if err = os.MkdirAll("/etc/samba", 0755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp("/etc/samba", ".mupibox-smb-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.WriteString(config); err == nil {
		err = temporary.Chmod(0644)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = runCommand(ctx, "testparm", "-s", temporaryPath); err != nil {
		return fmt.Errorf("validate Samba configuration: %w", err)
	}
	if err = os.Rename(temporaryPath, "/etc/samba/smb.conf"); err != nil {
		return err
	}
	if err = runCommand(ctx, "systemctl", "unmask", "smbd.service", "nmbd.service"); err != nil {
		return err
	}
	return runCommand(ctx, "systemctl", "enable", "--now", "smbd.service", "nmbd.service")
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

var drmConnectorCard = regexp.MustCompile(`^card(\d+)-`)

// activeDRMCard finds the DRM device backing the currently enabled display
// output. The legacy /dev/fb0 framebuffer is not scanned out once the native
// Qt Quick UI takes KMS master, so screenshots must read the real DRM plane.
func activeDRMCard(drmClassPath string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(drmClassPath, "card*-*", "enabled"))
	if err != nil {
		return "", err
	}
	for _, match := range matches {
		raw, readErr := os.ReadFile(match)
		if readErr != nil || strings.TrimSpace(string(raw)) != "enabled" {
			continue
		}
		name := filepath.Base(filepath.Dir(match))
		if sub := drmConnectorCard.FindStringSubmatch(name); sub != nil {
			return "/dev/dri/card" + sub[1], nil
		}
	}
	return "", errors.New("no active display output found")
}

func captureScreenshot(ctx context.Context) ([]byte, error) {
	card, err := activeDRMCard("/sys/class/drm")
	if err != nil {
		return nil, err
	}
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, errors.New("ffmpeg is not installed")
	}
	command := exec.CommandContext(ctx, path,
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "kmsgrab", "-device", card,
		"-i", "-",
		"-vf", "hwmap=derive_device=drm,format=bgr0",
		"-frames:v", "1", "-update", "1",
		"-c:v", "png", "-f", "image2", "-",
	)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err = command.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stdout.Len() == 0 {
		return nil, errors.New("empty screenshot capture")
	}
	return stdout.Bytes(), nil
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
	_ = connection.SetDeadline(time.Now().Add(70 * time.Second))
	decoder := json.NewDecoder(bufio.NewReader(connection))
	decoder.DisallowUnknownFields()
	var input request
	if err := decoder.Decode(&input); err != nil {
		_ = json.NewEncoder(connection).Encode(response{Error: "invalid request"})
		return
	}
	var err error
	var screenshot []byte
	switch input.Action {
	case "screenshot":
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		screenshot, err = captureScreenshot(ctx)
		cancel()
	case "wifi-state":
		err = setWiFiState(context.Background(), input.Interface, input.Enabled)
	case "wifi-select":
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		err = selectWiFiAdapter(ctx, input.Interface)
		cancel()
	case "wifi-onboard":
		err = setOnboardWiFi(input.Enabled)
	case "ipv4-config":
		ctx, cancel := context.WithTimeout(context.Background(), 65*time.Second)
		err = applyIPv4(ctx, input)
		cancel()
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
	case "samba-config":
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		err = configureSamba(ctx, input)
		cancel()
	default:
		err = errors.New("unsupported action")
	}
	result := response{OK: err == nil}
	if err != nil {
		result.Error = err.Error()
	} else if screenshot != nil {
		result.Data = base64.StdEncoding.EncodeToString(screenshot)
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
