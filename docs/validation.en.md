# Validation status – 2026-09-16

> English. German version: [validation.md](validation.md)

## Actually executed

- Read the GitHub repository and branch base; connector push permission confirmed.
- Reviewed the project tree and all Go files, schema, configuration and prototype installers.
- MuPiBox Dev: Go 1.24.4 linux/amd64; original Git status was outside a repository.
- Wrote the new sources directly to `/opt/mupibox-ng` through MuPiBox Dev.
- `go_check build`: successful.
- `go_check test` / `go test ./...`: successful for cmd, audio, core, library, server and webui.
- `TestARM64Build` performs a real `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build`: successful.
- Library tests cover natural sorting, covers, empty/missing directories, stable IDs and symlink escape protection.
- Core tests cover empty queue, play/pause, EOF/automatic track advance, end of queue, volume limits, concurrent inputs and decoder failures without false playing state.
- API tests cover RFID start → button pause → browser resume, shared status, denied origin, invalid JSON, disabled input simulation and no filesystem paths in status responses.
- JavaScript syntax: `node --check webui/static/app.js` successful.
- Shell syntax of the three new scripts: `sh -n` successful.
- Git diff checked for whitespace errors.

An HTTP route conflict was found during the first run and corrected before the successful rerun. SIGTERM shutdown waits for active HTTP requests before closing audio.

A manual build as `root` in the LXC produced `error obtaining VCS status: exit status 128`. The previous binary remained in place and could therefore be started accidentally. Development builds in this environment may use `go build -buildvcs=false ...`; release builds should later embed commit/version information in a controlled way.

## Not yet proven / outstanding

- `TestMPVRealDecode` is optional and is skipped without mpv. The short audio-package test run in this LXC does not prove real decoding. Explicit validation after installing mpv: `go test -v ./internal/audio`.
- Audible audio output, ALSA/MuPiHAT, mono/stereo, Pi 3 load/temperature and load times.
- Complete visual browser validation at 800×480 and phone resolutions.
- Production TTS output on the Pi. Current Web Speech usage is only a development fallback.
- Persistent admin interface for categories/rows; these data currently come from JSON configuration.
- MuPiBox Dev cannot start/stop the running LXC web service because no shell action is exposed.
- systemd template/installer helper has not yet been executed on DietPi.
- `go vet` and `go test -race` are prepared through `scripts/check.sh` but have not been separately executed through the limited MuPiBox Dev action.
- LXC Git linking is prepared. If it is needed again, splitti must run `sh scripts/link-lxc.sh`.

The foundation is built and tested; the first complete hardware/audio milestone is only complete after real audio, display and hardware acceptance testing.
