package server

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"mupibox/internal/store"
)

const maxRestoreSize int64 = 250 << 30

type backupManifest struct {
	Format        int       `json:"format"`
	CreatedAt     time.Time `json:"created_at"`
	Version       string    `json:"version"`
	IncludesMedia bool      `json:"includes_media"`
}

func (a *API) registerMaintenanceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/admin/backup", a.downloadBackup)
	mux.HandleFunc("POST /api/admin/restore", a.restoreBackup)
	mux.HandleFunc("GET /api/admin/releases", a.listReleases)
	mux.HandleFunc("POST /api/admin/releases/switch", a.switchRelease)
}

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
}

func (a *API) listReleases(w http.ResponseWriter, r *http.Request) {
	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://api.github.com/repos/splitti/MuPiBox-NG/releases?per_page=30", nil)
	if err != nil {
		problem(w, 500, err)
		return
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "MuPiBox-NG/"+a.Version)
	response, err := (&http.Client{Timeout: 12 * time.Second}).Do(request)
	if err != nil {
		problem(w, 502, fmt.Errorf("load GitHub releases: %w", err))
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		problem(w, 502, fmt.Errorf("GitHub releases returned HTTP %d", response.StatusCode))
		return
	}
	var releases []githubRelease
	if err = json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&releases); err != nil {
		problem(w, 502, err)
		return
	}
	_, rollbackErr := os.Stat("/var/lib/mupibox-ng/updates/previous-ref")
	jsonResponse(w, 200, map[string]any{"current_version": a.Version, "releases": releases, "rollback_available": rollbackErr == nil})
}

func (a *API) switchRelease(w http.ResponseWriter, r *http.Request) {
	if a.Connectivity == nil {
		problem(w, 503, errors.New("system agent unavailable"))
		return
	}
	var request struct {
		Target string `json:"target"`
	}
	if err := decode(w, r, &request); err != nil {
		problem(w, 400, err)
		return
	}
	if err := a.Connectivity.StartReleaseUpdate(r.Context(), request.Target); err != nil {
		problem(w, 502, err)
		return
	}
	jsonResponse(w, http.StatusAccepted, map[string]any{"started": true, "target": request.Target, "notice": "services will restart during the update"})
}

func addZipFile(archive *zip.Writer, name, source string, storeOnly bool) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = name
	header.Method = zip.Deflate
	if storeOnly {
		header.Method = zip.Store
	}
	writer, err := archive.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(writer, file)
	return err
}

func (a *API) downloadBackup(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		problem(w, 503, errors.New("persistent store unavailable"))
		return
	}
	includeMedia := r.URL.Query().Get("include_media") == "true" || r.URL.Query().Get("include_media") == "1"
	tempDir, err := os.MkdirTemp("", "mupibox-backup-")
	if err != nil {
		problem(w, 500, err)
		return
	}
	defer os.RemoveAll(tempDir)
	database := filepath.Join(tempDir, "mupibox.db")
	if err = a.Store.Snapshot(database); err != nil {
		problem(w, 500, err)
		return
	}
	filename := "mupibox-backup-" + time.Now().UTC().Format("20060102-150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "no-store")
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(12 * time.Hour))
	archive := zip.NewWriter(w)
	manifest := backupManifest{Format: 1, CreatedAt: time.Now().UTC(), Version: a.Version, IncludesMedia: includeMedia}
	manifestWriter, createErr := archive.Create("manifest.json")
	if createErr == nil {
		createErr = json.NewEncoder(manifestWriter).Encode(manifest)
	}
	if createErr == nil {
		createErr = addZipFile(archive, "database/mupibox.db", database, false)
	}
	root := filepath.Clean(a.MusicDir)
	if createErr == nil && includeMedia && root != "" && root != "." {
		createErr = filepath.WalkDir(root, func(filePath string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			relative, err := filepath.Rel(root, filePath)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return errors.New("media path escaped root")
			}
			return addZipFile(archive, "media/"+filepath.ToSlash(relative), filePath, true)
		})
	}
	if closeErr := archive.Close(); createErr == nil {
		createErr = closeErr
	}
	_ = createErr
}

func archiveDestination(root, name string) (string, error) {
	clean := path.Clean(strings.TrimPrefix(name, "media/"))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
		return "", errors.New("invalid media path in backup")
	}
	destination := filepath.Join(root, filepath.FromSlash(clean))
	relative, err := filepath.Rel(root, destination)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("media path escaped root")
	}
	return destination, nil
}

func extractZipEntry(entry *zip.File, target string, limit int64) error {
	if entry.FileInfo().Mode()&os.ModeSymlink != 0 || !entry.FileInfo().Mode().IsRegular() {
		return errors.New("backup contains unsupported file type")
	}
	if entry.FileInfo().Size() > limit {
		return errors.New("backup entry is too large")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(destination, io.LimitReader(source, limit+1))
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > limit {
		return errors.New("backup entry exceeded size limit")
	}
	return nil
}

func (a *API) restoreBackup(w http.ResponseWriter, r *http.Request) {
	if a.Store == nil {
		problem(w, 503, errors.New("persistent store unavailable"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRestoreSize)
	multipartReader, err := r.MultipartReader()
	if err != nil {
		problem(w, 400, errors.New("restore requires multipart/form-data"))
		return
	}
	tempDir, err := os.MkdirTemp("", "mupibox-restore-")
	if err != nil {
		problem(w, 500, err)
		return
	}
	defer os.RemoveAll(tempDir)
	bundle := filepath.Join(tempDir, "backup.zip")
	found := false
	for {
		part, nextErr := multipartReader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			problem(w, 400, nextErr)
			return
		}
		if part.FormName() != "backup" {
			part.Close()
			continue
		}
		destination, openErr := os.OpenFile(bundle, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if openErr != nil {
			part.Close()
			problem(w, 500, openErr)
			return
		}
		written, copyErr := io.Copy(destination, io.LimitReader(part, maxRestoreSize+1))
		closeErr := destination.Close()
		part.Close()
		if copyErr != nil || closeErr != nil || written > maxRestoreSize {
			if copyErr == nil {
				copyErr = closeErr
			}
			if copyErr == nil {
				copyErr = errors.New("backup upload is too large")
			}
			problem(w, 400, copyErr)
			return
		}
		found = true
		break
	}
	if !found {
		problem(w, 400, errors.New("backup file is missing"))
		return
	}
	reader, err := zip.OpenReader(bundle)
	if err != nil {
		problem(w, 400, errors.New("backup is not a valid ZIP archive"))
		return
	}
	defer reader.Close()
	database := filepath.Join(tempDir, "restore.db")
	includeMedia := r.URL.Query().Get("include_media") == "true" || r.URL.Query().Get("include_media") == "1"
	databaseFound := false
	mediaEntries := []*zip.File{}
	for _, entry := range reader.File {
		clean := path.Clean(entry.Name)
		if clean != entry.Name || strings.HasPrefix(clean, "../") || path.IsAbs(clean) {
			problem(w, 400, errors.New("backup contains an invalid path"))
			return
		}
		switch {
		case clean == "database/mupibox.db":
			if databaseFound {
				problem(w, 400, errors.New("backup contains multiple databases"))
				return
			}
			if err = extractZipEntry(entry, database, 2<<30); err != nil {
				problem(w, 400, err)
				return
			}
			databaseFound = true
		case strings.HasPrefix(clean, "media/") && includeMedia:
			mediaEntries = append(mediaEntries, entry)
		}
	}
	if !databaseFound {
		problem(w, 400, errors.New("backup does not contain database/mupibox.db"))
		return
	}
	backupDir := a.BackupDir
	if strings.TrimSpace(backupDir) == "" {
		backupDir = filepath.Join(tempDir, "safety")
	}
	if err = os.MkdirAll(backupDir, 0750); err != nil {
		problem(w, 500, err)
		return
	}
	safety := filepath.Join(backupDir, "pre-restore-"+time.Now().UTC().Format("20060102-150405")+".db")
	if err = a.Store.Snapshot(safety); err != nil {
		problem(w, 500, fmt.Errorf("create safety backup: %w", err))
		return
	}
	if err = a.Store.RestoreSnapshot(database); err != nil {
		problem(w, 400, err)
		return
	}
	settings, ok, settingsErr := a.Store.LoadBoxSettings()
	if settingsErr != nil || !ok {
		_ = a.Store.RestoreSnapshot(safety)
		if settingsErr == nil {
			settingsErr = errors.New("restored backup has no box settings")
		}
		problem(w, 400, settingsErr)
		return
	}
	if settingsErr = store.ValidateBoxSettings(settings); settingsErr != nil {
		_ = a.Store.RestoreSnapshot(safety)
		problem(w, 400, fmt.Errorf("invalid restored settings: %w", settingsErr))
		return
	}
	restoredMedia := 0
	if includeMedia {
		root := filepath.Clean(a.MusicDir)
		for _, entry := range mediaEntries {
			target, pathErr := archiveDestination(root, entry.Name)
			if pathErr != nil {
				problem(w, 400, pathErr)
				return
			}
			if err = extractZipEntry(entry, target, 64<<30); err != nil {
				problem(w, 500, err)
				return
			}
			_ = os.Chmod(target, 0644)
			restoredMedia++
		}
		if err = a.rescanLibrary(); err != nil {
			problem(w, 500, err)
			return
		}
	}
	a.TTS = TTSConfig{Enabled: settings.TTS.Enabled, Language: settings.TTS.Language, Provider: settings.TTS.Provider}
	a.Power = PowerConfig{IdleShutdownMinutes: settings.Power.IdleShutdownMinutes}
	a.clearAdminSessions(w, r)
	jsonResponse(w, 200, map[string]any{"restored": true, "media_files": restoredMedia, "safety_backup": filepath.Base(safety), "login_required": true})
}
