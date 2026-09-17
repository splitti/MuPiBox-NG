package server

import (
	"archive/zip"
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mupibox/internal/library"
	"mupibox/internal/store"
)

func TestBackupAndRestoreWithMedia(t *testing.T) {
	dir := t.TempDir()
	music := filepath.Join(dir, "music")
	if err := os.MkdirAll(filepath.Join(music, "stories"), 0755); err != nil {
		t.Fatal(err)
	}
	track := filepath.Join(music, "stories", "one.mp3")
	if err := os.WriteFile(track, []byte("audio"), 0644); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(filepath.Join(dir, "mupibox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	settings := store.BoxSettings{Language: "de", AdminLanguage: "de", Audio: store.AudioSettings{StartupVolume: 30, MaxVolume: 60}, Display: store.DisplaySettings{Brightness: 80, UISize: "normal"}, Theme: "modern-dark"}
	if err = db.SaveBoxSettings(settings); err != nil {
		t.Fatal(err)
	}
	lib, err := library.Scan(music)
	if err != nil {
		t.Fatal(err)
	}
	api := &API{Store: db, Library: lib, MusicDir: music, BackupDir: filepath.Join(dir, "backups"), Version: "test"}
	handler := api.Handler()
	download := httptest.NewRecorder()
	handler.ServeHTTP(download, httptest.NewRequest(http.MethodGet, "/api/admin/backup?include_media=true", nil))
	if download.Code != 200 {
		t.Fatalf("backup status=%d body=%s", download.Code, download.Body.String())
	}
	archive, err := zip.NewReader(bytes.NewReader(download.Body.Bytes()), int64(download.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, entry := range archive.File {
		names = append(names, entry.Name)
	}
	joined := strings.Join(names, "\n")
	if !strings.Contains(joined, "database/mupibox.db") || !strings.Contains(joined, "media/stories/one.mp3") {
		t.Fatalf("backup entries: %s", joined)
	}
	settings.Audio.StartupVolume = 20
	settings.Audio.MaxVolume = 25
	if err = db.SaveBoxSettings(settings); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(track); err != nil {
		t.Fatal(err)
	}
	var upload bytes.Buffer
	writer := multipart.NewWriter(&upload)
	part, err := writer.CreateFormFile("backup", "backup.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(part, bytes.NewReader(download.Body.Bytes())); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/admin/restore?include_media=true", &upload)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	restore := httptest.NewRecorder()
	handler.ServeHTTP(restore, request)
	if restore.Code != 200 {
		t.Fatalf("restore status=%d body=%s", restore.Code, restore.Body.String())
	}
	restored, ok, err := db.LoadBoxSettings()
	if err != nil || !ok || restored.Audio.MaxVolume != 60 {
		t.Fatalf("restored settings=%#v ok=%v err=%v", restored, ok, err)
	}
	content, err := os.ReadFile(track)
	if err != nil || string(content) != "audio" {
		t.Fatalf("restored media=%q err=%v", content, err)
	}
}
