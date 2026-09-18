// Package store owns MuPiBox's SQLite persistence and schema migrations.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db   *sql.DB
	path string
}

type TTSSettings struct {
	Enabled  bool   `json:"enabled"`
	Language string `json:"language"`
	Provider string `json:"provider"`
}

type PowerSettings struct {
	IdleShutdownMinutes int `json:"idle_shutdown_minutes"`
}

type AudioSettings struct {
	StartupVolume        int  `json:"startup_volume"`
	MaxVolume            int  `json:"max_volume"`
	StartSoundEnabled    bool `json:"start_sound_enabled"`
	ShutdownSoundEnabled bool `json:"shutdown_sound_enabled"`
}

type DisplaySettings struct {
	IdleOffMinutes int    `json:"idle_off_minutes"`
	Brightness     int    `json:"brightness"`
	UISize         string `json:"ui_size"`
}

type BluetoothSettings struct {
	Enabled bool `json:"enabled"`
}

type WiFiSettings struct {
	PrimaryInterface   string       `json:"primary_interface,omitempty"`
	DisabledInterfaces []string     `json:"disabled_interfaces,omitempty"`
	IPv4               IPv4Settings `json:"ipv4"`
}

type IPv4Settings struct {
	Mode      string   `json:"mode"`
	Interface string   `json:"interface,omitempty"`
	Address   string   `json:"address,omitempty"`
	Gateway   string   `json:"gateway,omitempty"`
	DNS       []string `json:"dns,omitempty"`
}

type ProviderAccountSettings struct {
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"client_id,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	Country      string `json:"country,omitempty"`
}

type ProviderSettings struct {
	Spotify     ProviderAccountSettings `json:"spotify"`
	AmazonMusic ProviderAccountSettings `json:"amazon_music"`
}

type MQTTSettings struct {
	Enabled     bool   `json:"enabled"`
	Broker      string `json:"broker,omitempty"`
	Port        int    `json:"port"`
	Topic       string `json:"topic,omitempty"`
	ClientID    string `json:"client_id,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Refresh     int    `json:"refresh_seconds"`
	RefreshIdle int    `json:"refresh_idle_seconds"`
	Timeout     int    `json:"timeout_seconds"`
	Debug       bool   `json:"debug"`
	HAEnabled   bool   `json:"home_assistant_enabled"`
	HATopic     string `json:"home_assistant_topic,omitempty"`
}

type BatteryProfile struct {
	Name     string `json:"name"`
	V100     int    `json:"v_100"`
	V75      int    `json:"v_75"`
	V50      int    `json:"v_50"`
	V25      int    `json:"v_25"`
	V0       int    `json:"v_0"`
	Warning  int    `json:"warning"`
	Shutdown int    `json:"shutdown"`
}

type MuPiHATSettings struct {
	Enabled         bool             `json:"enabled"`
	SelectedBattery string           `json:"selected_battery"`
	CurrentLimitMA  int              `json:"current_limit_ma"`
	Profiles        []BatteryProfile `json:"battery_profiles"`
}

type SystemSettings struct {
	SwapPolicy          string `json:"swap_policy"`
	WaitOnlinePolicy    string `json:"wait_online_policy"`
	PerformanceMode     string `json:"performance_mode"`
	InitialTurboSeconds int    `json:"initial_turbo_seconds"`
}

type BoxSettings struct {
	Language      string            `json:"language"`
	AdminLanguage string            `json:"admin_language"`
	TTS           TTSSettings       `json:"tts"`
	Power         PowerSettings     `json:"power"`
	Audio         AudioSettings     `json:"audio"`
	Display       DisplaySettings   `json:"display"`
	WiFi          WiFiSettings      `json:"wifi"`
	Bluetooth     BluetoothSettings `json:"bluetooth"`
	Providers     ProviderSettings  `json:"providers"`
	MQTT          MQTTSettings      `json:"mqtt"`
	MuPiHAT       MuPiHATSettings   `json:"mupihat"`
	System        SystemSettings    `json:"system"`
	Theme         string            `json:"theme"`
}

type Navigation struct {
	Categories []Category `json:"categories"`
}
type Category struct {
	ID     string            `json:"id"`
	Labels map[string]string `json:"labels"`
	Rows   []Row             `json:"rows,omitempty"`
}
type Row struct {
	ID         string            `json:"id"`
	Labels     map[string]string `json:"labels"`
	Provider   string            `json:"provider"`
	SourceType string            `json:"source_type,omitempty"`
	SourceRef  string            `json:"source_ref,omitempty"`
}

type Progress struct {
	Provider   string    `json:"provider"`
	AccountID  string    `json:"account_id,omitempty"`
	MediaID    string    `json:"media_id"`
	PositionMS int64     `json:"position_ms"`
	DurationMS int64     `json:"duration_ms"`
	ContextID  string    `json:"context_id,omitempty"`
	ItemIndex  int       `json:"item_index,omitempty"`
	Completed  bool      `json:"completed"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path is required")
	}
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, path: path}
	if err = s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Snapshot(target string) error {
	if strings.TrimSpace(target) == "" {
		return errors.New("snapshot target is required")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		return errors.New("snapshot target already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := s.db.Exec(`PRAGMA wal_checkpoint(FULL)`); err != nil {
		return fmt.Errorf("checkpoint database: %w", err)
	}
	if _, err := s.db.Exec(`VACUUM INTO ?`, target); err != nil {
		return fmt.Errorf("create database snapshot: %w", err)
	}
	return nil
}

func (s *Store) RestoreSnapshot(source string) error {
	if strings.TrimSpace(source) == "" {
		return errors.New("snapshot source is required")
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("snapshot source is not a regular file")
	}
	if _, err = s.db.Exec(`ATTACH DATABASE ? AS backup`, source); err != nil {
		return fmt.Errorf("open backup database: %w", err)
	}
	defer s.db.Exec(`DETACH DATABASE backup`)
	var integrity string
	if err = s.db.QueryRow(`PRAGMA backup.integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		if err == nil {
			err = fmt.Errorf("integrity result: %s", integrity)
		}
		return fmt.Errorf("invalid backup database: %w", err)
	}
	required := []string{"settings", "navigation_nodes", "navigation_labels", "playback_progress"}
	for _, table := range required {
		var count int
		if err = s.db.QueryRow(`SELECT COUNT(*) FROM backup.sqlite_master WHERE type='table' AND name=?`, table).Scan(&count); err != nil || count != 1 {
			if err == nil {
				err = fmt.Errorf("missing table %s", table)
			}
			return fmt.Errorf("invalid backup database: %w", err)
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		`DELETE FROM navigation_labels`, `DELETE FROM navigation_nodes`, `DELETE FROM playback_progress`, `DELETE FROM settings`,
		`INSERT INTO settings(key,value,updated_at) SELECT key,value,updated_at FROM backup.settings`,
		`INSERT INTO navigation_nodes(id,parent_id,node_type,sort_order,enabled,provider,source_type,source_ref) SELECT id,parent_id,node_type,sort_order,enabled,provider,source_type,source_ref FROM backup.navigation_nodes WHERE parent_id IS NULL`,
		`INSERT INTO navigation_nodes(id,parent_id,node_type,sort_order,enabled,provider,source_type,source_ref) SELECT id,parent_id,node_type,sort_order,enabled,provider,source_type,source_ref FROM backup.navigation_nodes WHERE parent_id IS NOT NULL`,
		`INSERT INTO navigation_labels(node_id,language,label) SELECT node_id,language,label FROM backup.navigation_labels`,
		`INSERT INTO playback_progress(provider,account_id,media_id,position_ms,duration_ms,context_id,item_index,completed,updated_at) SELECT provider,account_id,media_id,position_ms,duration_ms,context_id,item_index,completed,updated_at FROM backup.playback_progress`,
	}
	for _, statement := range statements {
		if _, err = tx.Exec(statement); err != nil {
			return fmt.Errorf("restore database: %w", err)
		}
	}
	return tx.Commit()
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return err
	}
	var version int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}
	if version < 1 {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		statements := []string{
			`CREATE TABLE settings(key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL)`,
			`CREATE TABLE navigation_nodes(
     id TEXT PRIMARY KEY,
     parent_id TEXT REFERENCES navigation_nodes(id) ON DELETE CASCADE,
     node_type TEXT NOT NULL CHECK(node_type IN ('category','row')),
     sort_order INTEGER NOT NULL,
     enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
     provider TEXT NOT NULL DEFAULT ''
   )`,
			`CREATE INDEX navigation_parent_sort ON navigation_nodes(parent_id,sort_order)`,
			`CREATE TABLE navigation_labels(
     node_id TEXT NOT NULL REFERENCES navigation_nodes(id) ON DELETE CASCADE,
     language TEXT NOT NULL,
     label TEXT NOT NULL,
     PRIMARY KEY(node_id,language)
   )`,
			`CREATE TABLE playback_progress(
     provider TEXT NOT NULL,
     account_id TEXT NOT NULL DEFAULT '',
     media_id TEXT NOT NULL,
     position_ms INTEGER NOT NULL CHECK(position_ms>=0),
     duration_ms INTEGER NOT NULL CHECK(duration_ms>=0),
     context_id TEXT NOT NULL DEFAULT '',
     item_index INTEGER NOT NULL DEFAULT 0,
     completed INTEGER NOT NULL DEFAULT 0 CHECK(completed IN (0,1)),
     updated_at TEXT NOT NULL,
     PRIMARY KEY(provider,account_id,media_id)
   )`,
		}
		for _, q := range statements {
			if _, err = tx.Exec(q); err != nil {
				return fmt.Errorf("migration 1: %w", err)
			}
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(1,?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		version = 1
	}
	if version < 2 {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err = tx.Exec(`ALTER TABLE navigation_nodes ADD COLUMN source_type TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migration 2 source_type: %w", err)
		}
		if _, err = tx.Exec(`ALTER TABLE navigation_nodes ADD COLUMN source_ref TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migration 2 source_ref: %w", err)
		}
		if _, err = tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(2,?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func ValidateBoxSettings(v BoxSettings) error {
	if strings.TrimSpace(v.Language) == "" {
		return errors.New("language is required")
	}
	if strings.TrimSpace(v.AdminLanguage) == "" {
		return errors.New("admin_language is required")
	}
	if v.Audio.MaxVolume < 1 || v.Audio.MaxVolume > 100 {
		return errors.New("audio.max_volume must be 1..100")
	}
	if v.Audio.StartupVolume < 0 || v.Audio.StartupVolume > v.Audio.MaxVolume {
		return errors.New("audio.startup_volume must be between 0 and max_volume")
	}
	if v.Power.IdleShutdownMinutes < 0 {
		return errors.New("power.idle_shutdown_minutes must be >= 0")
	}
	if v.Display.IdleOffMinutes < 0 {
		return errors.New("display.idle_off_minutes must be >= 0")
	}
	if v.Display.Brightness < 1 || v.Display.Brightness > 100 {
		return errors.New("display.brightness must be 1..100")
	}
	if v.Display.UISize != "normal" && v.Display.UISize != "large" {
		return errors.New("display.ui_size must be normal or large")
	}
	if v.TTS.Enabled && (strings.TrimSpace(v.TTS.Language) == "" || strings.TrimSpace(v.TTS.Provider) == "") {
		return errors.New("enabled TTS requires language and provider")
	}
	interfaceName := regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	if v.WiFi.PrimaryInterface != "" && !interfaceName.MatchString(v.WiFi.PrimaryInterface) {
		return errors.New("wifi.primary_interface is invalid")
	}
	disabled := map[string]bool{}
	for _, name := range v.WiFi.DisabledInterfaces {
		if !interfaceName.MatchString(name) {
			return errors.New("wifi.disabled_interfaces contains an invalid interface")
		}
		if disabled[name] {
			return errors.New("wifi.disabled_interfaces contains duplicates")
		}
		disabled[name] = true
	}
	if disabled[v.WiFi.PrimaryInterface] && v.WiFi.PrimaryInterface != "" {
		return errors.New("wifi.primary_interface cannot be disabled")
	}
	if v.WiFi.IPv4.Mode != "dhcp" && v.WiFi.IPv4.Mode != "static" {
		return errors.New("wifi.ipv4.mode must be dhcp or static")
	}
	if v.WiFi.IPv4.Interface != "" && !interfaceName.MatchString(v.WiFi.IPv4.Interface) {
		return errors.New("wifi.ipv4.interface is invalid")
	}
	if v.WiFi.IPv4.Mode == "static" {
		if ip, _, err := net.ParseCIDR(v.WiFi.IPv4.Address); err != nil || ip.To4() == nil {
			return errors.New("wifi.ipv4.address must be a valid IPv4 CIDR")
		}
		if ip := net.ParseIP(v.WiFi.IPv4.Gateway); ip == nil || ip.To4() == nil {
			return errors.New("wifi.ipv4.gateway must be a valid IPv4 address")
		}
		for _, value := range v.WiFi.IPv4.DNS {
			if ip := net.ParseIP(value); ip == nil || ip.To4() == nil {
				return errors.New("wifi.ipv4.dns contains an invalid IPv4 address")
			}
		}
	}
	if v.MQTT.Port < 1 || v.MQTT.Port > 65535 {
		return errors.New("mqtt.port must be 1..65535")
	}
	if v.MQTT.Refresh < 1 || v.MQTT.RefreshIdle < 1 || v.MQTT.Timeout < 1 {
		return errors.New("mqtt intervals must be positive")
	}
	if v.MQTT.Enabled && strings.TrimSpace(v.MQTT.Broker) == "" {
		return errors.New("enabled MQTT requires a broker")
	}
	if v.MQTT.HAEnabled && !v.MQTT.Enabled {
		return errors.New("Home Assistant discovery requires MQTT")
	}
	if v.MuPiHAT.CurrentLimitMA != 1790 && v.MuPiHAT.CurrentLimitMA != 2200 && v.MuPiHAT.CurrentLimitMA != 2700 {
		return errors.New("mupihat.current_limit_ma must be 1790, 2200 or 2700")
	}
	profileNames := map[string]bool{}
	for _, profile := range v.MuPiHAT.Profiles {
		if strings.TrimSpace(profile.Name) == "" || profileNames[profile.Name] {
			return errors.New("mupihat battery profile names must be unique")
		}
		profileNames[profile.Name] = true
		if profile.V100 < profile.V75 || profile.V75 < profile.V50 || profile.V50 < profile.V25 || profile.V25 < profile.V0 {
			return fmt.Errorf("mupihat battery profile %s has invalid voltage order", profile.Name)
		}
	}
	if v.MuPiHAT.Enabled && !profileNames[v.MuPiHAT.SelectedBattery] {
		return errors.New("enabled MuPiHAT requires a selected battery profile")
	}
	if v.System.SwapPolicy != "keep" && v.System.SwapPolicy != "enabled" && v.System.SwapPolicy != "disabled" {
		return errors.New("system.swap_policy must be keep, enabled or disabled")
	}
	if v.System.WaitOnlinePolicy != "keep" && v.System.WaitOnlinePolicy != "enabled" && v.System.WaitOnlinePolicy != "disabled" {
		return errors.New("system.wait_online_policy must be keep, enabled or disabled")
	}
	if v.System.PerformanceMode != "balanced" && v.System.PerformanceMode != "performance" && v.System.PerformanceMode != "powersave" {
		return errors.New("system.performance_mode must be balanced, performance or powersave")
	}
	if v.System.InitialTurboSeconds < 0 || v.System.InitialTurboSeconds > 60 {
		return errors.New("system.initial_turbo_seconds must be 0..60")
	}
	if strings.TrimSpace(v.Theme) == "" {
		return errors.New("theme is required")
	}
	return nil
}

func defaultBatteryProfiles() []BatteryProfile {
	return []BatteryProfile{
		{Name: "Ansmann 2S1P", V100: 8100, V75: 7800, V50: 7400, V25: 7000, V0: 6700, Warning: 7000, Shutdown: 6800},
		{Name: "ENERpower 2S2P 10.000mAh", V100: 8000, V75: 7700, V50: 7300, V25: 6900, V0: 6000, Warning: 6500, Shutdown: 6150},
		{Name: "USB-C mode (no battery)", V100: 1, V75: 1, V50: 1, V25: 1, V0: 1, Warning: 0, Shutdown: 0},
		{Name: "Custom", V100: 8100, V75: 7800, V50: 7400, V25: 7000, V0: 6700, Warning: 7000, Shutdown: 6800},
	}
}

func normalizeBoxSettings(v *BoxSettings) {
	if strings.TrimSpace(v.AdminLanguage) == "" {
		v.AdminLanguage = v.Language
		if strings.TrimSpace(v.AdminLanguage) == "" {
			v.AdminLanguage = "de"
		}
	}
	if strings.TrimSpace(v.Display.UISize) == "" {
		v.Display.UISize = "normal"
	}
	if v.WiFi.IPv4.Mode == "" {
		v.WiFi.IPv4.Mode = "dhcp"
	}
	if v.MQTT.Port == 0 {
		v.MQTT.Port = 1883
	}
	if v.MQTT.Refresh == 0 {
		v.MQTT.Refresh = 5
	}
	if v.MQTT.RefreshIdle == 0 {
		v.MQTT.RefreshIdle = 30
	}
	if v.MQTT.Timeout == 0 {
		v.MQTT.Timeout = 60
	}
	if v.MQTT.ClientID == "" {
		v.MQTT.ClientID = "MuPiBox"
	}
	if v.MQTT.Topic == "" {
		v.MQTT.Topic = "MuPiBox/Boxname"
	}
	if v.MQTT.HATopic == "" {
		v.MQTT.HATopic = "homeassistant"
	}
	if len(v.MuPiHAT.Profiles) == 0 {
		v.MuPiHAT.Profiles = defaultBatteryProfiles()
	}
	if v.MuPiHAT.SelectedBattery == "" {
		v.MuPiHAT.SelectedBattery = "USB-C mode (no battery)"
	}
	if v.MuPiHAT.CurrentLimitMA == 0 {
		v.MuPiHAT.CurrentLimitMA = 1790
	}
	if v.System.SwapPolicy == "" {
		v.System.SwapPolicy = "keep"
	}
	if v.System.WaitOnlinePolicy == "" {
		v.System.WaitOnlinePolicy = "keep"
	}
	if v.System.PerformanceMode == "" {
		v.System.PerformanceMode = "balanced"
	}
}

func NormalizeBoxSettings(v BoxSettings) BoxSettings {
	normalizeBoxSettings(&v)
	return v
}
func (s *Store) LoadBoxSettings() (BoxSettings, bool, error) {
	var raw string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key='box.settings'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return BoxSettings{}, false, nil
	}
	if err != nil {
		return BoxSettings{}, false, err
	}
	var v BoxSettings
	if err = json.Unmarshal([]byte(raw), &v); err != nil {
		return BoxSettings{}, false, fmt.Errorf("decode box settings: %w", err)
	}
	normalizeBoxSettings(&v)
	return v, true, nil
}
func (s *Store) SaveBoxSettings(v BoxSettings) error {
	normalizeBoxSettings(&v)
	if err := ValidateBoxSettings(v); err != nil {
		return err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO settings(key,value,updated_at) VALUES('box.settings',?,?)
 ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`, string(raw), time.Now().UTC().Format(time.RFC3339Nano))
	return err
}
func (s *Store) EnsureBoxSettings(defaults BoxSettings) (BoxSettings, error) {
	normalizeBoxSettings(&defaults)
	v, ok, err := s.LoadBoxSettings()
	if err != nil {
		return BoxSettings{}, err
	}
	if ok {
		return v, nil
	}
	if err = s.SaveBoxSettings(defaults); err != nil {
		return BoxSettings{}, err
	}
	return defaults, nil
}

func (s *Store) LoadAdminPasswordHash() (string, bool, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key='admin.password_hash'`).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, value != "", nil
}

func (s *Store) SaveAdminPasswordHash(value string) error {
	if strings.TrimSpace(value) == "" {
		_, err := s.db.Exec(`DELETE FROM settings WHERE key='admin.password_hash'`)
		return err
	}
	_, err := s.db.Exec(`INSERT INTO settings(key,value,updated_at) VALUES('admin.password_hash',?,?)
 ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`, value, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

var validID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func validateLabels(labels map[string]string) error {
	if len(labels) == 0 {
		return errors.New("at least one label is required")
	}
	for lang, label := range labels {
		if !validID.MatchString(lang) {
			return fmt.Errorf("invalid language %q", lang)
		}
		if strings.TrimSpace(label) == "" || len(label) > 160 {
			return fmt.Errorf("invalid label for %s", lang)
		}
	}
	return nil
}
func ValidateNavigation(v Navigation) error {
	seen := map[string]bool{}
	for _, c := range v.Categories {
		if !validID.MatchString(c.ID) {
			return fmt.Errorf("invalid category id %q", c.ID)
		}
		if seen[c.ID] {
			return fmt.Errorf("duplicate node id %q", c.ID)
		}
		seen[c.ID] = true
		if err := validateLabels(c.Labels); err != nil {
			return fmt.Errorf("category %s: %w", c.ID, err)
		}
		for _, r := range c.Rows {
			if !validID.MatchString(r.ID) {
				return fmt.Errorf("invalid media id %q", r.ID)
			}
			if seen[r.ID] {
				return fmt.Errorf("duplicate node id %q", r.ID)
			}
			seen[r.ID] = true
			if err := validateLabels(r.Labels); err != nil {
				return fmt.Errorf("media %s: %w", r.ID, err)
			}
			if strings.TrimSpace(r.Provider) == "" {
				return fmt.Errorf("media %s: provider is required", r.ID)
			}
			if r.SourceType != "" && !validID.MatchString(r.SourceType) {
				return fmt.Errorf("media %s: invalid source_type", r.ID)
			}
			if len(r.SourceRef) > 2048 {
				return fmt.Errorf("media %s: source_ref is too long", r.ID)
			}
			if r.Provider == "local-library" && r.SourceType == "path" {
				ref := filepath.Clean(filepath.FromSlash(strings.TrimSpace(r.SourceRef)))
				if strings.TrimSpace(r.SourceRef) == "" || filepath.IsAbs(ref) || ref == ".." || strings.HasPrefix(ref, ".."+string(filepath.Separator)) {
					return fmt.Errorf("media %s: local path must be below the media root", r.ID)
				}
			}
			if r.Provider == "resume-list" {
				limit, err := strconv.Atoi(strings.TrimSpace(r.SourceRef))
				if r.SourceType != "limit" || err != nil || limit < 1 || limit > 100 {
					return fmt.Errorf("media %s: resume limit must be 1..100", r.ID)
				}
			}
		}
	}
	return nil
}
func (s *Store) HasNavigation() (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM navigation_nodes`).Scan(&n)
	return n > 0, err
}
func (s *Store) SaveNavigation(v Navigation) error {
	if err := ValidateNavigation(v); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM navigation_nodes`); err != nil {
		return err
	}
	insertNode := func(id string, parent any, kind string, order int, provider, sourceType, sourceRef string) error {
		_, err := tx.Exec(`INSERT INTO navigation_nodes(id,parent_id,node_type,sort_order,enabled,provider,source_type,source_ref) VALUES(?,?,?,?,1,?,?,?)`, id, parent, kind, order, provider, sourceType, sourceRef)
		return err
	}
	insertLabels := func(id string, labels map[string]string) error {
		for lang, label := range labels {
			if _, err := tx.Exec(`INSERT INTO navigation_labels(node_id,language,label) VALUES(?,?,?)`, id, lang, label); err != nil {
				return err
			}
		}
		return nil
	}
	for ci, c := range v.Categories {
		if err = insertNode(c.ID, nil, "category", ci, "", "", ""); err != nil {
			return err
		}
		if err = insertLabels(c.ID, c.Labels); err != nil {
			return err
		}
		for ri, r := range c.Rows {
			if err = insertNode(r.ID, c.ID, "row", ri, r.Provider, r.SourceType, r.SourceRef); err != nil {
				return err
			}
			if err = insertLabels(r.ID, r.Labels); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
func (s *Store) EnsureNavigation(defaults Navigation) error {
	ok, err := s.HasNavigation()
	if err != nil || ok {
		return err
	}
	return s.SaveNavigation(defaults)
}
func (s *Store) LoadNavigation() (Navigation, error) {
	rows, err := s.db.Query(`SELECT id,parent_id,node_type,provider,source_type,source_ref FROM navigation_nodes WHERE enabled=1 ORDER BY CASE WHEN parent_id IS NULL THEN 0 ELSE 1 END,sort_order,id`)
	if err != nil {
		return Navigation{}, err
	}
	defer rows.Close()
	type node struct {
		id                                    string
		parent                                sql.NullString
		kind, provider, sourceType, sourceRef string
	}
	nodes := []node{}
	for rows.Next() {
		var n node
		if err = rows.Scan(&n.id, &n.parent, &n.kind, &n.provider, &n.sourceType, &n.sourceRef); err != nil {
			return Navigation{}, err
		}
		nodes = append(nodes, n)
	}
	if err = rows.Err(); err != nil {
		return Navigation{}, err
	}
	labels := map[string]map[string]string{}
	lr, err := s.db.Query(`SELECT node_id,language,label FROM navigation_labels`)
	if err != nil {
		return Navigation{}, err
	}
	defer lr.Close()
	for lr.Next() {
		var id, lang, label string
		if err = lr.Scan(&id, &lang, &label); err != nil {
			return Navigation{}, err
		}
		if labels[id] == nil {
			labels[id] = map[string]string{}
		}
		labels[id][lang] = label
	}
	out := Navigation{Categories: []Category{}}
	indexes := map[string]int{}
	for _, n := range nodes {
		if n.kind == "category" {
			indexes[n.id] = len(out.Categories)
			out.Categories = append(out.Categories, Category{ID: n.id, Labels: labels[n.id], Rows: []Row{}})
		}
	}
	for _, n := range nodes {
		if n.kind != "row" || !n.parent.Valid {
			continue
		}
		i, ok := indexes[n.parent.String]
		if ok {
			out.Categories[i].Rows = append(out.Categories[i].Rows, Row{ID: n.id, Labels: labels[n.id], Provider: n.provider, SourceType: n.sourceType, SourceRef: n.sourceRef})
		}
	}
	return out, lr.Err()
}

func (s *Store) SaveProgress(v Progress) error {
	if strings.TrimSpace(v.Provider) == "" || strings.TrimSpace(v.MediaID) == "" {
		return errors.New("provider and media_id are required")
	}
	if v.Provider == "radio" {
		return nil
	}
	if v.PositionMS < 0 || v.DurationMS < 0 {
		return errors.New("progress values must be >= 0")
	}
	now := time.Now().UTC()
	if !v.UpdatedAt.IsZero() {
		now = v.UpdatedAt.UTC()
	}
	_, err := s.db.Exec(`INSERT INTO playback_progress(provider,account_id,media_id,position_ms,duration_ms,context_id,item_index,completed,updated_at)
 VALUES(?,?,?,?,?,?,?,?,?)
 ON CONFLICT(provider,account_id,media_id) DO UPDATE SET position_ms=excluded.position_ms,duration_ms=excluded.duration_ms,
 context_id=excluded.context_id,item_index=excluded.item_index,completed=excluded.completed,updated_at=excluded.updated_at`,
		v.Provider, v.AccountID, v.MediaID, v.PositionMS, v.DurationMS, v.ContextID, v.ItemIndex, v.Completed, now.Format(time.RFC3339Nano))
	return err
}
func (s *Store) LoadProgress(provider, accountID, mediaID string) (Progress, bool, error) {
	var v Progress
	var completed int
	var updated string
	err := s.db.QueryRow(`SELECT position_ms,duration_ms,context_id,item_index,completed,updated_at FROM playback_progress WHERE provider=? AND account_id=? AND media_id=?`, provider, accountID, mediaID).Scan(&v.PositionMS, &v.DurationMS, &v.ContextID, &v.ItemIndex, &completed, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Progress{}, false, nil
	}
	if err != nil {
		return Progress{}, false, err
	}
	v.Provider = provider
	v.AccountID = accountID
	v.MediaID = mediaID
	v.Completed = completed == 1
	v.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return v, true, nil
}

func (s *Store) ListRecentProgress(limit int) ([]Progress, error) {
	if limit < 1 {
		limit = 1
	} else if limit > 1000 {
		limit = 1000
	}
	rows, err := s.db.Query(`SELECT provider,account_id,media_id,position_ms,duration_ms,context_id,item_index,completed,updated_at
  FROM playback_progress WHERE completed=0 AND position_ms>=5000 ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Progress{}
	for rows.Next() {
		var v Progress
		var completed int
		var updated string
		if err = rows.Scan(&v.Provider, &v.AccountID, &v.MediaID, &v.PositionMS, &v.DurationMS, &v.ContextID, &v.ItemIndex, &completed, &updated); err != nil {
			return nil, err
		}
		v.Completed = completed == 1
		v.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) PutProgress(provider, accountID, mediaID string, positionMS, durationMS int64, contextID string, itemIndex int, completed bool) error {
	return s.SaveProgress(Progress{Provider: provider, AccountID: accountID, MediaID: mediaID, PositionMS: positionMS, DurationMS: durationMS, ContextID: contextID, ItemIndex: itemIndex, Completed: completed})
}
func (s *Store) GetProgress(provider, accountID, mediaID string) (positionMS, durationMS int64, completed, found bool, err error) {
	v, found, err := s.LoadProgress(provider, accountID, mediaID)
	if err != nil || !found {
		return 0, 0, false, found, err
	}
	return v.PositionMS, v.DurationMS, v.Completed, true, nil
}
