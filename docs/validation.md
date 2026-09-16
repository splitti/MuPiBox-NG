# Prüfstand – 16.09.2026

> Deutsch. English version: [validation.en.md](validation.en.md)

## Tatsächlich ausgeführt

- GitHub-Repository und Branch-Basis gelesen; Push-Recht vom Connector bestätigt.
- Projektbaum und alle Go-Dateien, Schema, Konfiguration und Installer des Prototyps geprüft.
- MuPiBox Dev: Go 1.24.4 linux/amd64, ursprünglicher Git-Status ohne Repository.
- Neue Quellen direkt in `/opt/mupibox-ng` über MuPiBox Dev geschrieben.
- `go_check build`: erfolgreich.
- `go_check test` / `go test ./...`: erfolgreich für cmd, audio, core, library, server, webui.
- `TestARM64Build` führt einen echten `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build` aus: erfolgreich.
- Bibliothekstests: natürliche Sortierung, Cover, leeres/fehlendes Verzeichnis, stabile IDs, Symlink-Ausbruch.
- Coretests: leere Queue, Play/Pause, EOF/automatischer Titelwechsel, Ende, Lautstärkengrenze,
  gleichzeitige Eingaben und Decoderfehler ohne falschen Playing-Status.
- API-Test: RFID startet → Taste pausiert → Browser setzt fort, gemeinsamer Status,
  unerlaubte Origin, ungültiges JSON, abgeschaltete Input-Simulation, keine Dateipfade im Status.
- JavaScript-Syntax: `node --check webui/static/app.js` erfolgreich.
- Shell-Syntax der drei neuen Skripte: `sh -n` erfolgreich.
- Git-Diff auf Whitespacefehler geprüft.

Beim ersten Lauf wurde ein Konflikt zwischen Go-HTTP-Routen festgestellt und vor dem erfolgreichen
Wiederholungslauf korrigiert. SIGTERM-Shutdown wartet auf aktive HTTP-Anfragen, bevor Audio geschlossen wird.

Beim manuellen Build als `root` in der LXC trat `error obtaining VCS status: exit status 128` auf.
Dabei blieb die vorhandene alte Binärdatei bestehen und konnte versehentlich erneut gestartet werden.
Für Entwicklungsbuilds in dieser Umgebung kann `go build -buildvcs=false ...` verwendet werden;
Release-Builds sollen später Commit-/Versionsinformationen kontrolliert einbetten.

## Nicht nachgewiesen / ausstehend

- `TestMPVRealDecode` ist optional und wird ohne mpv übersprungen. Der kurze Audiopaket-Testlauf
  in dieser LXC belegt keine echte Dekodierung. Explizite Prüfung nach mpv-Installation:
  `go test -v ./internal/audio`.
- Hörbare Audioausgabe, ALSA/MuPiHAT, Mono/Stereo, Last/Temperatur und Ladezeit am Pi 3.
- Vollständige visuelle Browserprüfung bei 800×480 und Handyauflösung.
- Produktive TTS-Ausgabe auf dem Pi. Die derzeitige Web-Speech-Nutzung ist nur Entwicklungsfallback.
- Persistentes Admin-Interface für Kategorien/Reihen; aktuell stammen diese Daten aus JSON-Konfiguration.
- Kein laufender LXC-Webdienst kann über MuPiBox Dev gestartet/gestoppt werden, da keine Shell-Aktion vorhanden ist.
- systemd-Vorlage/Installationshelfer noch nicht auf DietPi ausgeführt.
- `go vet` und `go test -race` über `scripts/check.sh` für die lokale Shell vorbereitet,
  von der begrenzten MuPiBox-Dev-Aktion noch nicht gesondert ausgeführt.
- LXC-Git-Verknüpfung vorbereitet. Falls erneut nötig, muss `sh scripts/link-lxc.sh` durch splitti ausgeführt werden.

Damit ist das Grundgerüst gebaut und getestet; der vollständige erste Hardware-/Audio-Meilenstein
ist erst nach echter Audio-, Display- und Hardwareabnahme abgeschlossen.
