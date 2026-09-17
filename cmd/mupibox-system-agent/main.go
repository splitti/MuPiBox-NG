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
	Action    string `json:"action"`
	Interface string `json:"interface"`
	Enabled   bool   `json:"enabled"`
	Target    string `json:"target"`
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
