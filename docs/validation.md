# Prüfstand – 16.09.2026

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

## Nicht nachgewiesen / ausstehend

- `TestMPVRealDecode` ist optional und wird ohne mpv übersprungen. Der kurze Audiopaket-Testlauf
  in dieser LXC belegt keine echte Dekodierung. Explizite Prüfung nach mpv-Installation:
  `go test -v ./internal/audio`.
- Hörbare Audioausgabe, ALSA/MuPiHAT, Mono/Stereo, Last/Temperatur und Ladezeit am Pi 3.
- Visuelle Browserprüfung bei 800×480 und Handyauflösung: versucht, aber die Browserumgebung
  blockierte `http://127.0.0.1:8765/preview` mit `ERR_BLOCKED_BY_CLIENT`.
  Das responsive Layout ist implementiert, ein erfolgreicher Screenshot-/Interaktionstest wird nicht behauptet.
- Kein laufender LXC-Webdienst gestartet: MuPiBox Dev bietet keine Start-/Shell-Aktion.
- systemd-Vorlage/Installationshelfer noch nicht auf DietPi ausgeführt.
- `go vet` und `go test -race` über `scripts/check.sh` für die lokale Shell vorbereitet,
  von der begrenzten MuPiBox-Dev-Aktion noch nicht gesondert ausgeführt.
- LXC-Git-Verknüpfung vorbereitet, nicht ausgeführt. Erst `sh scripts/link-lxc.sh` durch Olli macht
  die vorhandene Arbeitskopie zum verknüpften Checkout.

Damit ist das Grundgerüst gebaut und getestet; der vollständige erste Hardware-/Audio-Meilenstein
ist erst nach echter Audio- und Displayabnahme abgeschlossen.
