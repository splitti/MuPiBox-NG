package connectivity

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type fakeRunner struct {
	paths   map[string]bool
	outputs map[string]string
	errors  map[string]error
	calls   []string
	inputs  []string
}

func TestSetWiFiAdapterStateUsesSystemAgent(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "wlan1")
	if err := os.MkdirAll(filepath.Join(base, "wireless"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "flags"), []byte("0x1002\n"), 0600); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "agent.sock")
	listener, err := net.Listen("unix", socket)
	if errors.Is(err, syscall.EPERM) {
		t.Skip("Unix sockets are unavailable in this test sandbox")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	received := make(chan map[string]any, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		var request map[string]any
		_ = json.NewDecoder(connection).Decode(&request)
		received <- request
		_ = json.NewEncoder(connection).Encode(map[string]any{"ok": true})
	}()
	manager := &Manager{NetworkPath: root, AgentSocket: socket}
	if err = manager.SetWiFiAdapterState(context.Background(), "wlan1", true); err != nil {
		t.Fatal(err)
	}
	request := <-received
	if request["action"] != "wifi-state" || request["interface"] != "wlan1" || request["enabled"] != true {
		t.Fatalf("unexpected agent request: %#v", request)
	}
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if f.paths[name] {
		return "/usr/bin/" + name, nil
	}
	return "", errors.New("missing")
}
func (f *fakeRunner) Run(_ context.Context, input, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, key)
	f.inputs = append(f.inputs, input)
	if err, ok := f.errors[key]; ok {
		return "", err
	}
	if output, ok := f.outputs[key]; ok {
		return output, nil
	}
	return "", nil
}

func TestParseWiFiScans(t *testing.T) {
	nm := parseNMCLI("*:79:WPA2:Home\n:35:WPA2:Guest\\:West\n:62:WPA2:Home\n")
	if len(nm) != 2 || !nm[0].Connected || nm[0].SSID != "Home" || nm[1].SSID != "Guest:West" {
		t.Fatalf("unexpected nmcli scan: %#v", nm)
	}
	wpa := parseWPAScan("bssid / frequency / signal level / flags / ssid\naa:bb:cc:dd:ee:ff\t2412\t-61\t[WPA2-PSK-CCMP][ESS]\thocuspocus\n")
	if len(wpa) != 1 || wpa[0].SSID != "hocuspocus" || wpa[0].SignalPercent != 78 || wpa[0].Security != "WPA/WPA2" {
		t.Fatalf("unexpected wpa scan: %#v", wpa)
	}
}

func TestScanWiFiFallsBackFromEmptyNMCLIToWPA(t *testing.T) {
	runner := &fakeRunner{paths: map[string]bool{"nmcli": true, "wpa_cli": true}, outputs: map[string]string{
		"nmcli -t --escape yes -f IN-USE,SIGNAL,SECURITY,SSID device wifi list --rescan yes ifname wlan0": "",
		"wpa_cli -i wlan0 scan":         "OK\n",
		"wpa_cli -i wlan0 scan_results": "bssid / frequency / signal level / flags / ssid\naa:bb:cc:dd:ee:ff\t2412\t-55\t[WPA2-PSK-CCMP][ESS]\thocuspocus\n",
	}}
	networks, err := (&Manager{Runner: runner, WiFiInterface: "wlan0"}).ScanWiFi(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(networks) != 1 || networks[0].SSID != "hocuspocus" {
		t.Fatalf("unexpected networks: %#v", networks)
	}
	if !strings.Contains(strings.Join(runner.calls, "\n"), "wpa_cli -i wlan0 scan_results") {
		t.Fatalf("wpa_cli fallback was not used: %#v", runner.calls)
	}
}

func TestListWiFiAdapters(t *testing.T) {
	root := t.TempDir()
	for _, adapter := range []struct{ name, mac, driver, state string }{{"wlan0", "00:11:22:33:44:55", "brcmfmac", "up"}, {"wlan1", "66:77:88:99:aa:bb", "rtl8192cu", "down"}} {
		base := filepath.Join(root, adapter.name)
		if err := os.MkdirAll(filepath.Join(base, "wireless"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(base, "device"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, "address"), []byte(adapter.mac+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, "operstate"), []byte(adapter.state+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		flags := "0x1002\n"
		if adapter.state == "up" {
			flags = "0x1003\n"
		}
		if err := os.WriteFile(filepath.Join(base, "flags"), []byte(flags), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, "device", "uevent"), []byte("DRIVER="+adapter.driver+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	adapters, err := (&Manager{NetworkPath: root}).ListWiFiAdapters()
	if err != nil {
		t.Fatal(err)
	}
	if len(adapters) != 2 || adapters[0].Interface != "wlan0" || adapters[0].Driver != "brcmfmac" || !adapters[0].Active || adapters[1].Active || adapters[1].Interface != "wlan1" || adapters[1].MAC != "66:77:88:99:aa:bb" {
		t.Fatalf("unexpected adapters: %#v", adapters)
	}
}

func TestConnectWiFiUsesArgumentSafeNMCLI(t *testing.T) {
	runner := &fakeRunner{paths: map[string]bool{"nmcli": true}, outputs: map[string]string{}}
	manager := &Manager{Runner: runner, WiFiInterface: "wlan0"}
	if err := manager.ConnectWiFi(context.Background(), WiFiConnectRequest{SSID: "Kids; network", Password: "safe password"}); err != nil {
		t.Fatal(err)
	}
	call := runner.calls[len(runner.calls)-1]
	if !strings.Contains(call, "nmcli --ask --wait 35 device wifi connect Kids; network ifname wlan0") {
		t.Fatalf("unexpected command: %s", call)
	}
	if runner.inputs[len(runner.inputs)-1] != "safe password\n" {
		t.Fatal("password was not supplied through stdin")
	}
	if err := manager.ConnectWiFi(context.Background(), WiFiConnectRequest{SSID: "x", Password: "short"}); err == nil {
		t.Fatal("short secured password accepted")
	}
}

func TestConnectWiFiFallsBackFromNMCLIToWPA(t *testing.T) {
	nmCall := "nmcli --ask --wait 35 device wifi connect Home ifname wlan0"
	runner := &fakeRunner{paths: map[string]bool{"nmcli": true, "wpa_cli": true}, errors: map[string]error{nmCall: errors.New("device is not managed")}, outputs: map[string]string{"wpa_cli -i wlan0 add_network": "4\n"}}
	manager := &Manager{Runner: runner, WiFiInterface: "wlan0"}
	if err := manager.ConnectWiFi(context.Background(), WiFiConnectRequest{SSID: "Home", Password: "safe password"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(runner.calls, "\n"), "wpa_cli -i wlan0 add_network") {
		t.Fatalf("wpa_cli fallback was not used: %#v", runner.calls)
	}
}

func TestBluetoothScanAndCommandValidation(t *testing.T) {
	runner := &fakeRunner{paths: map[string]bool{"bluetoothctl": true}, outputs: map[string]string{
		"bluetoothctl devices":                "Device AA:BB:CC:DD:EE:FF Kids Headphones\n",
		"bluetoothctl info AA:BB:CC:DD:EE:FF": "Name: Kids Headphones\nPaired: yes\nTrusted: yes\nConnected: no\n",
	}}
	manager := &Manager{Runner: runner}
	devices, err := manager.ScanBluetooth(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || !devices[0].Paired || !devices[0].Trusted || devices[0].Connected {
		t.Fatalf("unexpected devices: %#v", devices)
	}
	if err = manager.BluetoothCommand(context.Background(), "connect", "not-a-mac"); err == nil {
		t.Fatal("invalid address accepted")
	}
}
