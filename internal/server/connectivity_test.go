package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mupibox/internal/audio"
	"mupibox/internal/connectivity"
	"mupibox/internal/core"
	"mupibox/internal/library"
	"mupibox/internal/store"
)

type connectivityRunner struct {
	paths   map[string]bool
	outputs map[string]string
	calls   []string
}

func (f *connectivityRunner) LookPath(name string) (string, error) {
	if f.paths[name] {
		return "/usr/bin/" + name, nil
	}
	return "", errors.New("missing")
}
func (f *connectivityRunner) Run(_ context.Context, _ string, name string, args ...string) (string, error) {
	key := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, key)
	return f.outputs[key], nil
}

func TestConnectivityEndpoints(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "track.wav"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	networkPath := filepath.Join(dir, "net")
	if err := os.MkdirAll(filepath.Join(networkPath, "wlan0", "wireless"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(networkPath, "wlan0", "address"), []byte("00:11:22:33:44:55\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(networkPath, "wlan0", "operstate"), []byte("up\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(networkPath, "wlan0", "flags"), []byte("0x1003\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(networkPath, "wlan1", "wireless"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(networkPath, "wlan1", "operstate"), []byte("down\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(networkPath, "wlan1", "flags"), []byte("0x1002\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lib, err := library.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	player, err := core.New(lib, &audio.Simulated{}, 60)
	if err != nil {
		t.Fatal(err)
	}
	defer player.Close()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	settings := store.BoxSettings{Language: "de", AdminLanguage: "de", Audio: store.AudioSettings{StartupVolume: 30, MaxVolume: 60}, Display: store.DisplaySettings{Brightness: 100, UISize: "normal"}, Bluetooth: store.BluetoothSettings{Enabled: true}, Theme: "modern-dark"}
	if err = db.SaveBoxSettings(settings); err != nil {
		t.Fatal(err)
	}
	runner := &connectivityRunner{paths: map[string]bool{"nmcli": true, "bluetoothctl": true}, outputs: map[string]string{
		"nmcli -t --escape yes -f IN-USE,SIGNAL,SECURITY,SSID device wifi list --rescan yes ifname wlan0": "*:80:WPA2:Home\n",
		"bluetoothctl devices":                "Device AA:BB:CC:DD:EE:FF Headphones\n",
		"bluetoothctl info AA:BB:CC:DD:EE:FF": "Paired: yes\nConnected: no\nTrusted: yes\n",
	}}
	api := &API{Player: player, Library: lib, Store: db, Connectivity: &connectivity.Manager{Runner: runner, WiFiInterface: "wlan0", NetworkPath: networkPath}}
	handler := api.Handler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/connectivity/wifi/adapters", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"interface":"wlan0"`) || !strings.Contains(response.Body.String(), `"selected_interface":"wlan0"`) || !strings.Contains(response.Body.String(), `"interface":"wlan1"`) {
		t.Fatalf("wifi adapters status=%d body=%s", response.Code, response.Body.String())
	}
	preference := httptest.NewRequest(http.MethodPut, "/api/connectivity/wifi/preferences", strings.NewReader(`{"primary_interface":"wlan0","disabled_interfaces":["wlan1"]}`))
	preference.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, preference)
	if response.Code != http.StatusAccepted {
		t.Fatalf("wifi preferences status=%d body=%s", response.Code, response.Body.String())
	}
	saved, _, err := db.LoadBoxSettings()
	if err != nil || saved.WiFi.PrimaryInterface != "wlan0" || len(saved.WiFi.DisabledInterfaces) != 1 {
		t.Fatalf("wifi preferences not persisted: %#v %v", saved.WiFi, err)
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/connectivity/wifi", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"ssid":"Home"`) {
		t.Fatalf("wifi scan status=%d body=%s", response.Code, response.Body.String())
	}
	request := httptest.NewRequest(http.MethodPost, "/api/connectivity/wifi/connect", strings.NewReader(`{"ssid":"Home","password":"12345678"}`))
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("wifi connect status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/connectivity/bluetooth", nil))
	if response.Code != 200 {
		t.Fatalf("bluetooth scan status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Devices []connectivity.BluetoothDevice `json:"devices"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &payload); err != nil || len(payload.Devices) != 1 || !payload.Devices[0].Paired {
		t.Fatalf("bluetooth payload=%#v err=%v", payload, err)
	}
}
