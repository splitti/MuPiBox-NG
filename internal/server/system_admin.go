package server

import (
	"context"
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
	Boot                 bootAnalysis     `json:"boot"`
	SwapActive           bool             `json:"swap_active"`
	WaitOnline           string           `json:"wait_online"`
	CPUGovernors         []string         `json:"cpu_governors"`
	Model                string           `json:"model,omitempty"`
	NetworkBackend       string           `json:"network_backend"`
	Interfaces           []interfaceStatus `json:"interfaces"`
	StaticApplySupported bool             `json:"static_apply_supported"`
	StaticApplyNotice    string           `json:"static_apply_notice,omitempty"`
	CollectedAt          time.Time        `json:"collected_at"`
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
		case "firmware": result.FirmwareMS = value
		case "loader": result.LoaderMS = value
		case "kernel": result.KernelMS = value
		case "initrd": result.InitrdMS = value
		case "userspace": result.UserspaceMS = value
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
	if err != nil { return "" }
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
		if value := readFirst(path); value != "" { seen[value] = true }
	}
	values := make([]string, 0, len(seen))
	for value := range seen { values = append(values, value) }
	sort.Strings(values)
	return values
}

func currentInterfaces() []interfaceStatus {
	interfaces, _ := net.Interfaces()
	result := []interfaceStatus{}
	for _, item := range interfaces {
		if item.Flags&net.FlagLoopback != 0 { continue }
		addresses, _ := item.Addrs()
		values := make([]string, 0, len(addresses))
		for _, address := range addresses { values = append(values, address.String()) }
		result = append(result, interfaceStatus{Name: item.Name, Up: item.Flags&net.FlagUp != 0, Addresses: values})
	}
	return result
}

func detectNetworkBackend(ctx context.Context) (string, bool, string) {
	if _, err := exec.LookPath("nmcli"); err == nil { return "NetworkManager", false, "networkmanager" }
	if strings.TrimSpace(commandText(ctx, "systemctl", "is-active", "systemd-networkd.service")) == "active" {
		return "systemd-networkd", false, "networkd"
	}
	if _, err := os.Stat("/etc/network/interfaces"); err == nil {
		return "ifupdown/DietPi", false, "ifupdown"
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
		if state == "enabled" || state == "static" { waitState = "enabled"; break }
		if state == "masked" && waitState == "disabled" { waitState = "masked" }
	}
	return adminSystemStatus{
		Boot: boot,
		SwapActive: len(strings.Split(strings.TrimSpace(readFirst("/proc/swaps")), "\n")) > 1,
		WaitOnline: waitState,
		CPUGovernors: currentGovernors(),
		Model: readFirst("/proc/device-tree/model", "/sys/firmware/devicetree/base/model"),
		NetworkBackend: backend,
		Interfaces: currentInterfaces(),
		StaticApplySupported: staticSupported,
		StaticApplyNotice: notice,
		CollectedAt: time.Now().UTC(),
	}
}

func (a *API) registerSystemRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/system", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		jsonResponse(w, http.StatusOK, collectAdminSystemStatus(ctx))
	})
}
