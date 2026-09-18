// Package connectivity provides narrow adapters for the host Wi-Fi and Bluetooth tools.
package connectivity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Runner interface {
	LookPath(name string) (string, error)
	Run(ctx context.Context, input, name string, args ...string) (string, error)
}

type ExecRunner struct{}

func (ExecRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }
func (ExecRunner) Run(ctx context.Context, input, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if input != "" {
		cmd.Stdin = strings.NewReader(input)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

type Manager struct {
	Runner        Runner
	WiFiInterface string
	NetworkPath   string
	ControlPath   string
	AgentSocket   string
}

func New() *Manager { return &Manager{Runner: ExecRunner{}, WiFiInterface: "wlan0"} }

type WiFiNetwork struct {
	SSID          string `json:"ssid"`
	SignalPercent int    `json:"signal_percent"`
	Security      string `json:"security,omitempty"`
	Connected     bool   `json:"connected"`
	Interface     string `json:"interface,omitempty"`
}

type WiFiAdapter struct {
	Interface string `json:"interface"`
	MAC       string `json:"mac,omitempty"`
	Driver    string `json:"driver,omitempty"`
	State     string `json:"state,omitempty"`
	Managed   bool   `json:"managed"`
	Usable    bool   `json:"usable"`
	Enabled   bool   `json:"enabled"`
	Preferred bool   `json:"preferred"`
	Selected  bool   `json:"selected"`
	Active    bool   `json:"active"`
}

type WiFiConnectRequest struct {
	SSID      string `json:"ssid"`
	Password  string `json:"password,omitempty"`
	Interface string `json:"interface,omitempty"`
}

type SystemTuning struct {
	SwapEnabled         *bool  `json:"swap_enabled,omitempty"`
	WaitOnlineEnabled   *bool  `json:"wait_online_enabled,omitempty"`
	PerformanceMode     string `json:"performance_mode,omitempty"`
	InitialTurboSeconds *int   `json:"initial_turbo_seconds,omitempty"`
}

type SambaConfig struct {
	Enabled   bool   `json:"samba_enabled"`
	Mode      string `json:"samba_mode"`
	ShareName string `json:"samba_share_name"`
	Workgroup string `json:"samba_workgroup"`
	Password  string `json:"samba_password,omitempty"`
}

type IPv4Config struct {
	Mode      string   `json:"ipv4_mode"`
	Interface string   `json:"interface"`
	Address   string   `json:"ipv4_address,omitempty"`
	Gateway   string   `json:"ipv4_gateway,omitempty"`
	DNS       []string `json:"ipv4_dns,omitempty"`
}

func (m *Manager) runner() Runner {
	if m != nil && m.Runner != nil {
		return m.Runner
	}
	return ExecRunner{}
}
func (m *Manager) wifiInterface() string {
	if m != nil && strings.TrimSpace(m.WiFiInterface) != "" {
		return m.WiFiInterface
	}
	return "wlan0"
}
func (m *Manager) networkPath() string {
	if m != nil && strings.TrimSpace(m.NetworkPath) != "" {
		return m.NetworkPath
	}
	return "/sys/class/net"
}
func (m *Manager) controlPath() string {
	if m != nil && strings.TrimSpace(m.ControlPath) != "" {
		return m.ControlPath
	}
	return "/run/wpa_supplicant"
}
func (m *Manager) agentSocket() string {
	if m != nil && strings.TrimSpace(m.AgentSocket) != "" {
		return m.AgentSocket
	}
	return "/run/mupibox-system-agent/control.sock"
}

var wifiInterfaceName = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func readTrimmed(path string) string {
	value, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(value))
}

func (m *Manager) ListWiFiAdapters() ([]WiFiAdapter, error) {
	entries, err := os.ReadDir(m.networkPath())
	if err != nil {
		return nil, err
	}
	adapters := []WiFiAdapter{}
	for _, entry := range entries {
		name := entry.Name()
		if !wifiInterfaceName.MatchString(name) {
			continue
		}
		base := filepath.Join(m.networkPath(), name)
		if _, err = os.Stat(filepath.Join(base, "wireless")); err != nil {
			continue
		}
		driver := ""
		if target, linkErr := filepath.EvalSymlinks(filepath.Join(base, "device", "driver")); linkErr == nil {
			driver = filepath.Base(target)
		}
		if driver == "" {
			for _, line := range strings.Split(readTrimmed(filepath.Join(base, "device", "uevent")), "\n") {
				if strings.HasPrefix(line, "DRIVER=") {
					driver = strings.TrimPrefix(line, "DRIVER=")
					break
				}
			}
		}
		_, controlErr := os.Stat(filepath.Join(m.controlPath(), name))
		managed := controlErr == nil
		state := readTrimmed(filepath.Join(base, "operstate"))
		flags, parseErr := strconv.ParseUint(strings.TrimPrefix(readTrimmed(filepath.Join(base, "flags")), "0x"), 16, 64)
		active := parseErr == nil && flags&1 == 1
		adapters = append(adapters, WiFiAdapter{Interface: name, MAC: readTrimmed(filepath.Join(base, "address")), Driver: driver, State: state, Managed: managed, Usable: active && (managed || state == "up"), Enabled: true, Active: active})
	}
	sort.Slice(adapters, func(i, j int) bool { return adapters[i].Interface < adapters[j].Interface })
	return adapters, nil
}

func (m *Manager) SetWiFiAdapterState(ctx context.Context, iface string, enabled bool) error {
	iface = strings.TrimSpace(iface)
	if !wifiInterfaceName.MatchString(iface) {
		return errors.New("invalid Wi-Fi interface")
	}
	adapters, err := m.ListWiFiAdapters()
	if err != nil {
		return err
	}
	found := false
	for _, adapter := range adapters {
		if adapter.Interface == iface {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown Wi-Fi adapter %s", iface)
	}
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", m.agentSocket())
	if err != nil {
		return fmt.Errorf("system agent unavailable: %w", err)
	}
	defer connection.Close()
	deadline := time.Now().Add(20 * time.Second)
	_ = connection.SetDeadline(deadline)
	request := struct {
		Action    string `json:"action"`
		Interface string `json:"interface"`
		Enabled   bool   `json:"enabled"`
	}{Action: "wifi-state", Interface: iface, Enabled: enabled}
	if err = json.NewEncoder(connection).Encode(request); err != nil {
		return err
	}
	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err = json.NewDecoder(connection).Decode(&response); err != nil {
		return fmt.Errorf("system agent response: %w", err)
	}
	if !response.OK {
		if response.Error == "" {
			response.Error = "adapter state change failed"
		}
		return errors.New(response.Error)
	}
	return nil
}

func (m *Manager) SelectWiFiAdapter(ctx context.Context, iface string) error {
	iface = strings.TrimSpace(iface)
	if !wifiInterfaceName.MatchString(iface) {
		return errors.New("invalid Wi-Fi interface")
	}
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", m.agentSocket())
	if err != nil {
		return fmt.Errorf("system agent unavailable: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(35 * time.Second))
	request := struct {
		Action    string `json:"action"`
		Interface string `json:"interface"`
	}{Action: "wifi-select", Interface: iface}
	if err = json.NewEncoder(connection).Encode(request); err != nil {
		return err
	}
	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err = json.NewDecoder(connection).Decode(&response); err != nil {
		return fmt.Errorf("system agent response: %w", err)
	}
	if !response.OK {
		if response.Error == "" {
			response.Error = "Wi-Fi adapter switch failed"
		}
		return errors.New(response.Error)
	}
	return nil
}

func (m *Manager) SetOnboardWiFiDisabled(ctx context.Context, disabled bool) error {
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", m.agentSocket())
	if err != nil {
		return fmt.Errorf("system agent unavailable: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	if err = json.NewEncoder(connection).Encode(struct {
		Action  string `json:"action"`
		Enabled bool   `json:"enabled"`
	}{Action: "wifi-onboard", Enabled: !disabled}); err != nil {
		return err
	}
	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err = json.NewDecoder(connection).Decode(&response); err != nil {
		return fmt.Errorf("system agent response: %w", err)
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	return nil
}

func (m *Manager) ApplyIPv4(ctx context.Context, config IPv4Config) error {
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", m.agentSocket())
	if err != nil {
		return fmt.Errorf("system agent unavailable: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(70 * time.Second))
	payload := struct {
		Action string `json:"action"`
		IPv4Config
	}{Action: "ipv4-config", IPv4Config: config}
	if err = json.NewEncoder(connection).Encode(payload); err != nil {
		return err
	}
	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err = json.NewDecoder(connection).Decode(&response); err != nil {
		return fmt.Errorf("system agent response: %w", err)
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	return nil
}

func (m *Manager) StartReleaseUpdate(ctx context.Context, target string) error {
	target = strings.TrimSpace(target)
	if target == "" || len(target) > 64 {
		return errors.New("invalid release target")
	}
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", m.agentSocket())
	if err != nil {
		return fmt.Errorf("system agent unavailable: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	request := struct {
		Action string `json:"action"`
		Target string `json:"target"`
	}{Action: "release-update", Target: target}
	if err = json.NewEncoder(connection).Encode(request); err != nil {
		return err
	}
	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err = json.NewDecoder(connection).Decode(&response); err != nil {
		return err
	}
	if !response.OK {
		if response.Error == "" {
			response.Error = "update could not be started"
		}
		return errors.New(response.Error)
	}
	return nil
}

func (m *Manager) ApplySystemTuning(ctx context.Context, tuning SystemTuning) error {
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", m.agentSocket())
	if err != nil {
		return fmt.Errorf("system agent unavailable: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(35 * time.Second))
	request := struct {
		Action string `json:"action"`
		SystemTuning
	}{Action: "system-tuning", SystemTuning: tuning}
	if err = json.NewEncoder(connection).Encode(request); err != nil {
		return err
	}
	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err = json.NewDecoder(connection).Decode(&response); err != nil {
		return fmt.Errorf("system agent response: %w", err)
	}
	if !response.OK {
		if response.Error == "" {
			response.Error = "system tuning failed"
		}
		return errors.New(response.Error)
	}
	return nil
}

// CaptureScreenshot asks the system agent for a single PNG frame of the
// active display output. The image is held in memory only; neither side
// writes it to disk.
func (m *Manager) CaptureScreenshot(ctx context.Context) ([]byte, error) {
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", m.agentSocket())
	if err != nil {
		return nil, fmt.Errorf("system agent unavailable: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(15 * time.Second))
	if err = json.NewEncoder(connection).Encode(struct {
		Action string `json:"action"`
	}{Action: "screenshot"}); err != nil {
		return nil, err
	}
	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Data  string `json:"data"`
	}
	if err = json.NewDecoder(connection).Decode(&response); err != nil {
		return nil, fmt.Errorf("system agent response: %w", err)
	}
	if !response.OK {
		if response.Error == "" {
			response.Error = "screenshot capture failed"
		}
		return nil, errors.New(response.Error)
	}
	image, err := base64.StdEncoding.DecodeString(response.Data)
	if err != nil {
		return nil, fmt.Errorf("decode screenshot: %w", err)
	}
	return image, nil
}

func (m *Manager) ApplySamba(ctx context.Context, config SambaConfig) error {
	if config.Mode != "guest" && config.Mode != "password" {
		return errors.New("invalid Samba mode")
	}
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", m.agentSocket())
	if err != nil {
		return fmt.Errorf("system agent unavailable: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(65 * time.Second))
	request := struct {
		Action string `json:"action"`
		SambaConfig
	}{Action: "samba-config", SambaConfig: config}
	if err = json.NewEncoder(connection).Encode(request); err != nil {
		return err
	}
	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err = json.NewDecoder(connection).Decode(&response); err != nil {
		return fmt.Errorf("system agent response: %w", err)
	}
	if !response.OK {
		if response.Error == "" {
			response.Error = "Samba configuration failed"
		}
		return errors.New(response.Error)
	}
	return nil
}

func (m *Manager) SchedulePower(ctx context.Context, action string) error {
	if action != "reboot" && action != "poweroff" {
		return errors.New("invalid power action")
	}
	dialer := net.Dialer{}
	connection, err := dialer.DialContext(ctx, "unix", m.agentSocket())
	if err != nil {
		return fmt.Errorf("system agent unavailable: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
	if err = json.NewEncoder(connection).Encode(struct {
		Action      string `json:"action"`
		PowerAction string `json:"power_action"`
	}{Action: "power", PowerAction: action}); err != nil {
		return err
	}
	var response struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err = json.NewDecoder(connection).Decode(&response); err != nil {
		return fmt.Errorf("system agent response: %w", err)
	}
	if !response.OK {
		if response.Error == "" {
			response.Error = "power action could not be scheduled"
		}
		return errors.New(response.Error)
	}
	return nil
}

func (m *Manager) ScanWiFi(ctx context.Context) ([]WiFiNetwork, error) {
	adapters, _ := m.ListWiFiAdapters()
	interfaces := []string{}
	for _, adapter := range adapters {
		if adapter.State == "up" {
			interfaces = []string{adapter.Interface}
			break
		}
	}
	if len(interfaces) == 0 {
		for _, adapter := range adapters {
			if adapter.Managed {
				interfaces = []string{adapter.Interface}
				break
			}
		}
	}
	if len(interfaces) == 0 {
		interfaces = []string{m.wifiInterface()}
	}
	networks := []WiFiNetwork{}
	scanErrors := []error{}
	successful := false
	for _, iface := range interfaces {
		found, err := m.ScanWiFiOn(ctx, iface)
		if err != nil {
			scanErrors = append(scanErrors, fmt.Errorf("%s: %w", iface, err))
			continue
		}
		successful = true
		networks = append(networks, found...)
	}
	if len(networks) > 0 {
		return deduplicateWiFi(networks), nil
	}
	if successful {
		return []WiFiNetwork{}, nil
	}
	if len(scanErrors) > 0 {
		return nil, errors.Join(scanErrors...)
	}
	return []WiFiNetwork{}, nil
}

func (m *Manager) ScanWiFiOn(ctx context.Context, iface string) ([]WiFiNetwork, error) {
	iface = strings.TrimSpace(iface)
	if iface == "" {
		iface = m.wifiInterface()
	}
	if !wifiInterfaceName.MatchString(iface) {
		return nil, errors.New("invalid Wi-Fi interface")
	}
	if adapters, err := m.ListWiFiAdapters(); err == nil {
		for _, adapter := range adapters {
			if adapter.Interface == iface && !adapter.Usable {
				return nil, fmt.Errorf("Wi-Fi adapter %s is not ready (state=%s, no wpa_supplicant control)", iface, adapter.State)
			}
		}
	}
	r := m.runner()
	available := false
	successful := false
	scanErrors := []error{}
	if _, err := r.LookPath("nmcli"); err == nil {
		available = true
		scanCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		output, err := r.Run(scanCtx, "", "nmcli", "-t", "--escape", "yes", "-f", "IN-USE,SIGNAL,SECURITY,SSID", "device", "wifi", "list", "--rescan", "yes", "ifname", iface)
		cancel()
		if err == nil {
			successful = true
			if networks := tagWiFiInterface(parseNMCLI(output), iface); len(networks) > 0 {
				return networks, nil
			}
		} else {
			scanErrors = append(scanErrors, err)
		}
	}
	if _, err := r.LookPath("wpa_cli"); err == nil {
		available = true
		cacheCtx, cacheCancel := context.WithTimeout(ctx, 2*time.Second)
		output, cacheErr := r.Run(cacheCtx, "", "wpa_cli", "-i", iface, "scan_results")
		cacheCancel()
		cachedNetworks := []WiFiNetwork{}
		if cacheErr == nil {
			successful = true
			cachedNetworks = tagWiFiInterface(parseWPAScan(output), iface)
		}
		// scan_results is a cache. Trigger a real scan before returning it, or
		// an associated adapter often exposes only the currently connected SSID.
		activeCtx, activeCancel := context.WithTimeout(ctx, 4*time.Second)
		output, activeErr := r.Run(activeCtx, "", "wpa_cli", "-i", iface, "scan")
		activeCancel()
		if activeErr == nil && !strings.Contains(strings.ToUpper(output), "FAIL") {
			successful = true
		} else if activeErr != nil {
			scanErrors = append(scanErrors, activeErr)
		} else {
			scanErrors = append(scanErrors, fmt.Errorf("wpa_cli rejected scan: %s", strings.TrimSpace(output)))
		}
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			resultCtx, resultCancel := context.WithTimeout(ctx, 2*time.Second)
			output, err = r.Run(resultCtx, "", "wpa_cli", "-i", iface, "scan_results")
			resultCancel()
			if err == nil {
				successful = true
				if networks := tagWiFiInterface(parseWPAScan(output), iface); len(networks) > 0 {
					if len(cachedNetworks) == 0 || wifiNetworkSignature(networks) != wifiNetworkSignature(cachedNetworks) {
						return networks, nil
					}
				}
			}
			timer := time.NewTimer(500 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		if len(cachedNetworks) > 0 {
			return cachedNetworks, nil
		}
	}
	if successful {
		return []WiFiNetwork{}, nil
	}
	if len(scanErrors) > 0 {
		return nil, fmt.Errorf("Wi-Fi scan failed: %w", errors.Join(scanErrors...))
	}
	if !available {
		return nil, errors.New("no supported Wi-Fi manager found (nmcli or wpa_cli)")
	}
	return []WiFiNetwork{}, nil
}

func wifiNetworkSignature(networks []WiFiNetwork) string {
	parts := make([]string, 0, len(networks))
	for _, network := range deduplicateWiFi(networks) {
		parts = append(parts, fmt.Sprintf("%s|%d|%s|%t", network.SSID, network.SignalPercent, network.Security, network.Connected))
	}
	return strings.Join(parts, "\n")
}

func (m *Manager) ConnectWiFi(ctx context.Context, request WiFiConnectRequest) error {
	request.SSID = strings.TrimSpace(request.SSID)
	if request.SSID == "" || len([]byte(request.SSID)) > 32 {
		return errors.New("SSID must contain 1..32 bytes")
	}
	if len(request.Password) > 63 {
		return errors.New("Wi-Fi password must not exceed 63 characters")
	}
	if request.Password != "" && len(request.Password) < 8 {
		return errors.New("secured Wi-Fi passwords must contain at least 8 characters")
	}
	iface := strings.TrimSpace(request.Interface)
	if iface == "" {
		iface = m.wifiInterface()
	}
	if !wifiInterfaceName.MatchString(iface) {
		return errors.New("invalid Wi-Fi interface")
	}
	r := m.runner()
	connectCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	connectErrors := []error{}
	if _, err := r.LookPath("nmcli"); err == nil {
		args := []string{"--wait", "35", "device", "wifi", "connect", request.SSID, "ifname", iface}
		input := ""
		if request.Password != "" {
			args = append([]string{"--ask"}, args...)
			input = request.Password + "\n"
		}
		if _, err = r.Run(connectCtx, input, "nmcli", args...); err == nil {
			return nil
		}
		connectErrors = append(connectErrors, err)
	}
	if _, err := r.LookPath("wpa_cli"); err == nil {
		if err = m.connectWPA(connectCtx, request, iface); err == nil {
			return nil
		}
		connectErrors = append(connectErrors, err)
	}
	if len(connectErrors) > 0 {
		return fmt.Errorf("Wi-Fi connection failed: %w", errors.Join(connectErrors...))
	}
	return errors.New("no supported Wi-Fi manager found (nmcli or wpa_cli)")
}

func (m *Manager) connectWPA(ctx context.Context, request WiFiConnectRequest, iface string) error {
	r := m.runner()
	output, err := r.Run(ctx, "", "wpa_cli", "-i", iface, "add_network")
	if err != nil {
		return err
	}
	id := strings.TrimSpace(output)
	if _, err = strconv.Atoi(id); err != nil {
		return fmt.Errorf("wpa_cli returned invalid network id")
	}
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = r.Run(cleanupCtx, "", "wpa_cli", "-i", iface, "remove_network", id)
	}
	commands := []string{fmt.Sprintf("set_network %s ssid %s", id, strconv.Quote(request.SSID))}
	if request.Password == "" {
		commands = append(commands, fmt.Sprintf("set_network %s key_mgmt NONE", id))
	} else {
		commands = append(commands, fmt.Sprintf("set_network %s psk %s", id, strconv.Quote(request.Password)))
	}
	commands = append(commands, fmt.Sprintf("enable_network %s", id), fmt.Sprintf("select_network %s", id), "save_config", "quit")
	output, err = r.Run(ctx, strings.Join(commands, "\n")+"\n", "wpa_cli", "-i", iface)
	if err != nil {
		cleanup()
		return errors.New("wpa_supplicant could not apply the Wi-Fi configuration")
	}
	if strings.Contains(output, "FAIL") {
		cleanup()
		return errors.New("wpa_supplicant rejected the Wi-Fi configuration")
	}
	return nil
}

func tagWiFiInterface(networks []WiFiNetwork, iface string) []WiFiNetwork {
	for index := range networks {
		networks[index].Interface = iface
	}
	return networks
}

func splitEscaped(line string, separator rune) []string {
	fields := []string{}
	var b strings.Builder
	escaped := false
	for _, r := range line {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == separator {
			fields = append(fields, b.String())
			b.Reset()
			continue
		}
		b.WriteRune(r)
	}
	fields = append(fields, b.String())
	return fields
}

func parseNMCLI(output string) []WiFiNetwork {
	networks := []WiFiNetwork{}
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := splitEscaped(line, ':')
		if len(fields) < 4 {
			continue
		}
		signal, _ := strconv.Atoi(fields[1])
		ssid := strings.TrimSpace(strings.Join(fields[3:], ":"))
		if ssid == "" {
			continue
		}
		networks = append(networks, WiFiNetwork{SSID: ssid, SignalPercent: clamp(signal), Security: strings.TrimSpace(fields[2]), Connected: strings.TrimSpace(fields[0]) == "*"})
	}
	return deduplicateWiFi(networks)
}

func parseWPAScan(output string) []WiFiNetwork {
	networks := []WiFiNetwork{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 5 || strings.EqualFold(fields[0], "bssid / frequency / signal level / flags / ssid") {
			continue
		}
		dbm, err := strconv.Atoi(strings.TrimSpace(fields[2]))
		if err != nil {
			continue
		}
		ssid := strings.TrimSpace(strings.Join(fields[4:], "\t"))
		if ssid == "" {
			continue
		}
		flags := strings.TrimSpace(fields[3])
		security := "Open"
		if strings.Contains(flags, "WPA") {
			security = "WPA/WPA2"
		} else if strings.Contains(flags, "WEP") {
			security = "WEP"
		}
		networks = append(networks, WiFiNetwork{SSID: ssid, SignalPercent: clamp(2 * (dbm + 100)), Security: security, Connected: strings.Contains(flags, "[CURRENT]")})
	}
	return deduplicateWiFi(networks)
}

func deduplicateWiFi(input []WiFiNetwork) []WiFiNetwork {
	bySSID := map[string]WiFiNetwork{}
	for _, network := range input {
		current, ok := bySSID[network.SSID]
		if !ok || network.Connected || network.SignalPercent > current.SignalPercent {
			bySSID[network.SSID] = network
		}
	}
	out := make([]WiFiNetwork, 0, len(bySSID))
	for _, network := range bySSID {
		out = append(out, network)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Connected != out[j].Connected {
			return out[i].Connected
		}
		if out[i].SignalPercent != out[j].SignalPercent {
			return out[i].SignalPercent > out[j].SignalPercent
		}
		return strings.ToLower(out[i].SSID) < strings.ToLower(out[j].SSID)
	})
	return out
}

func clamp(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

type BluetoothDevice struct {
	Address   string `json:"address"`
	Name      string `json:"name"`
	Paired    bool   `json:"paired"`
	Trusted   bool   `json:"trusted"`
	Connected bool   `json:"connected"`
}

var bluetoothAddress = regexp.MustCompile(`(?i)^[0-9a-f]{2}(:[0-9a-f]{2}){5}$`)

func (m *Manager) SetBluetoothPower(ctx context.Context, enabled bool) error {
	r := m.runner()
	if _, err := r.LookPath("bluetoothctl"); err != nil {
		return errors.New("bluetoothctl is not installed")
	}
	value := "off"
	if enabled {
		value = "on"
	}
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, err := r.Run(runCtx, "", "bluetoothctl", "power", value)
	if err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(output), "failed") {
		return errors.New(strings.TrimSpace(output))
	}
	return nil
}

func (m *Manager) ScanBluetooth(ctx context.Context) ([]BluetoothDevice, error) {
	r := m.runner()
	if _, err := r.LookPath("bluetoothctl"); err != nil {
		return nil, errors.New("bluetoothctl is not installed")
	}
	scanCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()
	_, _ = r.Run(scanCtx, "", "bluetoothctl", "--timeout", "8", "scan", "on")
	output, err := r.Run(scanCtx, "", "bluetoothctl", "devices")
	if err != nil {
		return nil, err
	}
	devices := []BluetoothDevice{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "Device" || !bluetoothAddress.MatchString(fields[1]) {
			continue
		}
		device := BluetoothDevice{Address: strings.ToUpper(fields[1]), Name: strings.Join(fields[2:], " ")}
		info, infoErr := r.Run(scanCtx, "", "bluetoothctl", "info", device.Address)
		if infoErr == nil {
			device.Paired = parseBluetoothBool(info, "Paired")
			device.Trusted = parseBluetoothBool(info, "Trusted")
			device.Connected = parseBluetoothBool(info, "Connected")
		}
		devices = append(devices, device)
	}
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Connected != devices[j].Connected {
			return devices[i].Connected
		}
		if devices[i].Paired != devices[j].Paired {
			return devices[i].Paired
		}
		return strings.ToLower(devices[i].Name) < strings.ToLower(devices[j].Name)
	})
	return devices, nil
}

func parseBluetoothBool(info, key string) bool {
	prefix := strings.ToLower(key) + ":"
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			return strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line, key+":")), "yes")
		}
	}
	return false
}

func (m *Manager) BluetoothCommand(ctx context.Context, action, address string) error {
	address = strings.ToUpper(strings.TrimSpace(address))
	if !bluetoothAddress.MatchString(address) {
		return errors.New("invalid Bluetooth address")
	}
	allowed := map[string]bool{"pair": true, "connect": true, "disconnect": true, "remove": true}
	if !allowed[action] {
		return errors.New("unsupported Bluetooth action")
	}
	r := m.runner()
	if _, err := r.LookPath("bluetoothctl"); err != nil {
		return errors.New("bluetoothctl is not installed")
	}
	timeout := 15 * time.Second
	if action == "pair" {
		timeout = 40 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if action == "pair" {
		output, err := r.Run(runCtx, "", "bluetoothctl", "--timeout", "30", "pair", address)
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(output), "failed") {
			return errors.New(strings.TrimSpace(output))
		}
		_, _ = r.Run(runCtx, "", "bluetoothctl", "trust", address)
		_, err = r.Run(runCtx, "", "bluetoothctl", "connect", address)
		return err
	}
	_, err := r.Run(runCtx, "", "bluetoothctl", action, address)
	return err
}
