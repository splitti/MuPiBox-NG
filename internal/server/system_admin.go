package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"mupibox/internal/connectivity"
	"mupibox/internal/store"
)

type bootUnit struct {
	Unit       string `json:"unit"`
	DurationMS int64  `json:"duration_ms"`
}

type bootAnalysis struct {
	TotalMS     int64      `json:"total_ms"`
	FirmwareMS  int64      `json:"firmware_ms,omitempty"`
	LoaderMS    int64      `json:"loader_ms,omitempty"`
	KernelMS    int64      `json:"kernel_ms,omitempty"`
	InitrdMS    int64      `json:"initrd_ms,omitempty"`
	UserspaceMS int64      `json:"userspace_ms,omitempty"`
	TopUnits    []bootUnit `json:"top_units"`
	Raw         string     `json:"raw,omitempty"`
}

type interfaceStatus struct {
	Name      string   `json:"name"`
	Up        bool     `json:"up"`
	Addresses []string `json:"addresses"`
}

type adminSystemStatus struct {
	Boot                 bootAnalysis      `json:"boot"`
	SwapActive           bool              `json:"swap_active"`
	WaitOnline           string            `json:"wait_online"`
	CPUGovernors         []string          `json:"cpu_governors"`
	InitialTurboSeconds  int               `json:"initial_turbo_seconds"`
	Model                string            `json:"model,omitempty"`
	NetworkBackend       string            `json:"network_backend"`
	Interfaces           []interfaceStatus `json:"interfaces"`
	StaticApplySupported bool              `json:"static_apply_supported"`
	StaticApplyNotice    string            `json:"static_apply_notice,omitempty"`
	CollectedAt          time.Time         `json:"collected_at"`
}

var systemdTimePart = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?(?:ms|s|min)) \((firmware|loader|kernel|initrd|userspace)\)`)
var systemdTotalPart = regexp.MustCompile(`= ([0-9]+(?:\.[0-9]+)?(?:ms|s|min))`)
var blameLine = regexp.MustCompile(`^\s*([0-9]+(?:\.[0-9]+)?(?:ms|s|min))\s+(.+?)\s*$`)

func durationMillis(value string) int64 {
	value = strings.TrimSpace(value)
	multiplier := float64(1000)
	switch {
	case strings.HasSuffix(value, "ms"):
		multiplier = 1
		value = strings.TrimSuffix(value, "ms")
	case strings.HasSuffix(value, "min"):
		multiplier = 60000
		value = strings.TrimSuffix(value, "min")
	case strings.HasSuffix(value, "s"):
		value = strings.TrimSuffix(value, "s")
	default:
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int64(parsed*multiplier + 0.5)
}

func parseBootTime(raw string) bootAnalysis {
	result := bootAnalysis{Raw: strings.TrimSpace(raw), TopUnits: []bootUnit{}}
	for _, match := range systemdTimePart.FindAllStringSubmatch(raw, -1) {
		value := durationMillis(match[1])
		switch match[2] {
		case "firmware":
			result.FirmwareMS = value
		case "loader":
			result.LoaderMS = value
		case "kernel":
			result.KernelMS = value
		case "initrd":
			result.InitrdMS = value
		case "userspace":
			result.UserspaceMS = value
		}
	}
	if match := systemdTotalPart.FindStringSubmatch(raw); len(match) == 2 {
		result.TotalMS = durationMillis(match[1])
	}
	if result.TotalMS == 0 {
		result.TotalMS = result.FirmwareMS + result.LoaderMS + result.KernelMS + result.InitrdMS + result.UserspaceMS
	}
	return result
}

func parseBootBlame(raw string, limit int) []bootUnit {
	result := []bootUnit{}
	for _, line := range strings.Split(raw, "\n") {
		match := blameLine.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		result = append(result, bootUnit{Unit: strings.TrimSpace(match[2]), DurationMS: durationMillis(match[1])})
		if len(result) >= limit {
			break
		}
	}
	return result
}

func commandText(ctx context.Context, name string, args ...string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	output, _ := exec.CommandContext(ctx, path, args...).CombinedOutput()
	return strings.TrimSpace(string(output))
}

func readFirst(paths ...string) string {
	for _, path := range paths {
		if value, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(value)) != "" {
			return strings.TrimSpace(string(value))
		}
	}
	return ""
}

func currentGovernors() []string {
	paths, _ := filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*/cpufreq/scaling_governor")
	seen := map[string]bool{}
	for _, path := range paths {
		if value := readFirst(path); value != "" {
			seen[value] = true
		}
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func parseInitialTurbo(raw string) int {
	value := 0
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.HasPrefix(line, "initial_turbo=") {
			continue
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "initial_turbo=")))
		if err == nil && parsed >= 1 && parsed <= 60 {
			value = parsed
		}
	}
	return value
}

func currentInitialTurbo() int {
	return parseInitialTurbo(readFirst("/boot/firmware/config.txt", "/boot/config.txt"))
}

func currentInterfaces() []interfaceStatus {
	interfaces, _ := net.Interfaces()
	result := []interfaceStatus{}
	for _, item := range interfaces {
		if item.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := item.Addrs()
		values := make([]string, 0, len(addresses))
		for _, address := range addresses {
			values = append(values, address.String())
		}
		result = append(result, interfaceStatus{Name: item.Name, Up: item.Flags&net.FlagUp != 0, Addresses: values})
	}
	return result
}

func detectNetworkBackend(ctx context.Context) (string, bool, string) {
	if _, err := exec.LookPath("nmcli"); err == nil {
		return "NetworkManager", false, "networkmanager"
	}
	if strings.TrimSpace(commandText(ctx, "systemctl", "is-active", "systemd-networkd.service")) == "active" {
		return "systemd-networkd", false, "networkd"
	}
	if _, err := os.Stat("/etc/network/interfaces"); err == nil {
		_, dietPiErr := os.Stat("/boot/dietpi/dietpi-network")
		return "ifupdown/DietPi", dietPiErr == nil, "ifupdown"
	}
	return "unknown", false, "unknown"
}

func collectAdminSystemStatus(ctx context.Context) adminSystemStatus {
	boot := parseBootTime(commandText(ctx, "systemd-analyze", "time"))
	boot.TopUnits = parseBootBlame(commandText(ctx, "systemd-analyze", "blame", "--no-pager"), 15)
	backend, staticSupported, notice := detectNetworkBackend(ctx)
	waitState := "disabled"
	for _, unit := range []string{"systemd-networkd-wait-online.service", "NetworkManager-wait-online.service"} {
		state := commandText(ctx, "systemctl", "is-enabled", unit)
		if state == "enabled" || state == "static" {
			waitState = "enabled"
			break
		}
		if state == "masked" && waitState == "disabled" {
			waitState = "masked"
		}
	}
	return adminSystemStatus{
		Boot:                 boot,
		SwapActive:           len(strings.Split(strings.TrimSpace(readFirst("/proc/swaps")), "\n")) > 1,
		WaitOnline:           waitState,
		CPUGovernors:         currentGovernors(),
		InitialTurboSeconds:  currentInitialTurbo(),
		Model:                readFirst("/proc/device-tree/model", "/sys/firmware/devicetree/base/model"),
		NetworkBackend:       backend,
		Interfaces:           currentInterfaces(),
		StaticApplySupported: staticSupported,
		StaticApplyNotice:    notice,
		CollectedAt:          time.Now().UTC(),
	}
}

func sambaRuntimeStatus(ctx context.Context) map[string]any {
	_, installedErr := exec.LookPath("smbd")
	active := commandText(ctx, "systemctl", "is-active", "smbd.service") == "active"
	enabled := commandText(ctx, "systemctl", "is-enabled", "smbd.service") == "enabled"
	return map[string]any{"installed": installedErr == nil, "active": active, "enabled_at_boot": enabled}
}

func (a *API) registerSystemRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/system", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		jsonResponse(w, http.StatusOK, collectAdminSystemStatus(ctx))
	})
	mux.HandleFunc("POST /api/admin/system/power", func(w http.ResponseWriter, r *http.Request) {
		if a.Connectivity == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("system agent unavailable"))
			return
		}
		var input struct {
			Action string `json:"action"`
		}
		if err := decode(w, r, &input); err != nil {
			problem(w, http.StatusBadRequest, err)
			return
		}
		if input.Action != "reboot" && input.Action != "poweroff" {
			problem(w, http.StatusBadRequest, errors.New("action must be reboot or poweroff"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		if err := a.Connectivity.SchedulePower(ctx, input.Action); err != nil {
			problem(w, http.StatusBadGateway, err)
			return
		}
		jsonResponse(w, http.StatusAccepted, map[string]any{"scheduled": true, "action": input.Action})
	})
	mux.HandleFunc("GET /api/admin/screenshot", func(w http.ResponseWriter, r *http.Request) {
		if a.Connectivity == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("system agent unavailable"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		image, err := a.Connectivity.CaptureScreenshot(ctx)
		if err != nil {
			problem(w, http.StatusBadGateway, err)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(image)
	})
	mux.HandleFunc("GET /api/admin/samba", func(w http.ResponseWriter, r *http.Request) {
		settings, err := a.currentSettings()
		if err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		jsonResponse(w, http.StatusOK, map[string]any{"settings": settings.Samba, "runtime": sambaRuntimeStatus(ctx)})
	})
	mux.HandleFunc("PUT /api/admin/samba", func(w http.ResponseWriter, r *http.Request) {
		if a.Store == nil || a.Connectivity == nil {
			problem(w, http.StatusServiceUnavailable, errors.New("Samba management unavailable"))
			return
		}
		var input struct {
			Enabled   bool   `json:"enabled"`
			Mode      string `json:"mode"`
			ShareName string `json:"share_name"`
			Workgroup string `json:"workgroup"`
			Password  string `json:"password"`
		}
		if err := decode(w, r, &input); err != nil {
			problem(w, http.StatusBadRequest, err)
			return
		}
		settings, ok, err := a.Store.LoadBoxSettings()
		if err != nil || !ok {
			if err == nil {
				err = errors.New("settings not initialized")
			}
			problem(w, http.StatusInternalServerError, err)
			return
		}
		settings.Samba = store.SambaSettings{Enabled: input.Enabled, Mode: input.Mode, ShareName: strings.TrimSpace(input.ShareName), Workgroup: strings.ToUpper(strings.TrimSpace(input.Workgroup))}
		settings = store.NormalizeBoxSettings(settings)
		if err = store.ValidateBoxSettings(settings); err != nil {
			problem(w, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 65*time.Second)
		err = a.Connectivity.ApplySamba(ctx, connectivity.SambaConfig{Enabled: settings.Samba.Enabled, Mode: settings.Samba.Mode, ShareName: settings.Samba.ShareName, Workgroup: settings.Samba.Workgroup, Password: input.Password})
		cancel()
		if err != nil {
			problem(w, http.StatusBadGateway, err)
			return
		}
		if err = a.Store.SaveBoxSettings(settings); err != nil {
			problem(w, http.StatusInternalServerError, err)
			return
		}
		ctx, cancel = context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		jsonResponse(w, http.StatusOK, map[string]any{"settings": settings.Samba, "runtime": sambaRuntimeStatus(ctx)})
	})
}
