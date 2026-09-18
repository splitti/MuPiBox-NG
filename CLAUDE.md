# MuPiBox-NG – Projektanweisung für Claude Code

> Diese Datei ist die dauerhafte Arbeitsgrundlage für Claude Code in diesem Repository.
> Sie beschreibt den tatsächlich vorgefundenen Stand, keine erfundenen Funktionen.

## Was ist MuPiBox-NG

Neuentwicklung eines modularen Musikplayers für Kinder/Familien: **DietPi auf Raspberry Pi
(ARM64, Pi 3 oder neuer)**, Bedienung über Touch, Hardwaretasten, später RFID und
Heimnetz-Browser – alle Bedienarten steuern denselben Player, Status und dieselbe Queue.

Entwicklungsstand: **0.1.0-dev**, experimentell. Arbeit findet im Checkout `/opt/mupibox-ng`
statt, aktueller Branch `rebuild/go-foundation` (nicht `main`). Der unveränderte
Go-Prototyp vom 10.02.2026 liegt vollständig (bytegleich) in `legacy/prototype/` – das ist
historische Referenz, **kein neuer Code gehört dorthin**. Dokumentation liegt zweisprachig in
`docs/` (Deutsch verbindlich, Englisch parallel als `*.en.md`).

Repository: https://github.com/splitti/MuPiBox-NG · Referenz (alter Stack, keine Pflicht):
https://github.com/splitti/MuPiBox

## DEV/TEST-Rollenverteilung (seit 2026-09-18)

- **Diese Debian-13-LXC** ist die einzige **DEV**-Umgebung: Claude Code läuft ausschließlich
  hier, hier wird entwickelt, gebaut, getestet, committet und gepusht (amd64, kein
  `mpv`/Audio/Display nötig).
- **Raspberry Pi/DietPi (`mupibox-test`, 192.168.2.114, arm64)** ist ausschließlich
  **TEST-/Hardware-Ziel**: echte Laufzeit, DSI-Display, Touch, Audio, MuPiHat, systemd. Kein
  Dauerbetrieb von Entwicklungs-/AI-Werkzeugen dort (Claude Code/Anthropic-Reste wurden am
  2026-09-18 vom Pi entfernt).
- **GitHub (`rebuild/go-foundation`)** ist die maßgebliche Source of Truth für dauerhaften
  Code. Deployment auf den Test-Pi ist auch mit uncommitteten Änderungen erlaubt (siehe unten);
  committet/gepusht wird erst nach erfolgreichem Test.
- SSH vom LXC zum Pi läuft über einen dedizierten Schlüssel (`~/.ssh/id_ed25519` auf dem LXC,
  Alias `mupibox-test` in `~/.ssh/config`, `mupibox-pi` als Alt-Alias). Keine privaten
  Schlüssel im Repository.
- **Lokale KI (Ollama/`qwen3:8b`, `http://192.168.3.45:11434`)** ist bevorzugter Worker für
  Fleißarbeit: Datei-/Diff-/Log-Analyse, Fehler-Voranalyse, Testvorschläge, Codesuche,
  einfache Implementierungsvorschläge. Eingebunden als Projekt-MCP-Server `local-ai`
  (`.claude/tools/local-ai-mcp/`, Tool `local_ai`). Claude/Sonnet bleibt Orchestrator,
  Reviewer und trifft alle Architektur-, Sicherheits- und Deployment-Entscheidungen; Qwen
  committet/pusht nie und führt keine destruktiven Aktionen aus. Ergebnisse von `local_ai`
  nicht routinemäßig ein zweites Mal vollständig selbst prüfen – nur kritische Punkte
  verifizieren. Niemals Secrets/Zugangsdaten an `local_ai` übergeben; nur gezielten Kontext
  (konkrete Datei/Diff/Log-Ausschnitt), keine Repository-Vollanalyse.
- **Local-first bei Fleißarbeit:** Vor eigener Analyse längerer Logs (`journalctl`,
  `systemctl`, `dmesg`, Netzwerk-/Hardwareausgaben, Pi-Inventarisierung), Code-Reviews
  einzelner Dateien/Diffs, Suche nach Bugs/Dead Code/Inkonsistenzen, Testvorschlägen oder
  Zusammenfassungen großer Ausgaben prüfen, ob `local_ai` das übernehmen kann (Pi/Log →
  Qwen → kompakte Findings → Sonnet). Architektur, sicherheitsrelevante/komplexe
  Implementierungen, Deployment- und Git-Entscheidungen bleiben bei Sonnet. Qwen-Antworten
  kompakt anfordern (Findings, Datei/Zeile, Schweregrad, Empfehlung); bei „keine Findings“
  nur kurz bestätigen, keine langen Ausgaben ungefiltert übernehmen. Für kleine, lokal
  begrenzte Aufgaben weiterhin nur die betroffenen Dateien betrachten, keine
  Repository-Vollanalyse auslösen, außer explizit gewünscht.
- Workflow (siehe `scripts/deploy-pi.sh`, `scripts/test-pi.sh`, `scripts/status-pi.sh`,
  `scripts/logs-pi.sh`): lokal bauen/testen → `deploy-pi` synct den Arbeitsbaum (auch
  uncommittet) auf den Pi, baut dort nativ (arm64/CGO) und startet den Dienst neu → `test-pi`
  führt `go vet`/`go test` nativ auf dem Pi aus → `status-pi`/`logs-pi` zur Kontrolle. Bei
  Fehlern: Qwen für Log-/Fehler-Voranalyse nutzen, Sonnet entscheidet über die Korrektur,
  danach erneut deployen/testen. Erst bei erfolgreichem Teststand auf dem Pi committen und
  pushen. Zielhost über `MUPIBOX_PI_HOST` bzw. SSH-Config-Alias konfigurierbar, keine feste
  IP im Code.
- Keine destruktiven Git-Aktionen (Force-Push, History-Rewrite, Branch-Löschung) ohne
  ausdrückliche Freigabe. Keine Secrets/API-Keys im Repository.

## Zielplattform

- **OS:** DietPi (Debian-basiert), 64-Bit, ohne Desktop. Gegen DietPi-Verhalten entwickeln und
  prüfen, nicht gegen Desktop-Debian, Raspberry Pi OS oder generisches Ubuntu.
- **Hardware:** offiziell unterstützt sind Raspberry Pi 3, Pi 4 und Pi 5, jeweils **64-Bit
  (ARM64) only**. Pi 2, ARM32/armhf sind keine Zielplattform mehr; der Installer muss
  Nicht-ARM64-Zielhardware verständlich ablehnen statt es stillschweigend zu versuchen. Knappe
  RAM-/CPU-Budgets (besonders Pi 3) ernst nehmen. Testhardware: Pi 3, Pi 4, ein MuPiHAT V3.x.
  MuPiHAT V3.x auf Pi 5 gilt wegen bekannter Einschränkungen beim Power-Up/Voltage-Ramp der
  Hardware als **experimentell** und muss im Installer/Adminbereich so gekennzeichnet werden,
  nicht als vollständig freigegeben dargestellt werden.
- **Display:** primäres Testdisplay 800×480 DSI, Querformat, Touch. Andere Auflösungen/Panels
  werden unterstützt – die native UI hat ein logisches 800×480-Layout und skaliert
  seitenverhältnistreu auf die tatsächliche Kernel-Bildschirmgröße. Displaymodus über DietPis
  `dietpi-display` (KMS/DRM); keine festen Panel-Overlays in der generischen Installation,
  Hardwareprofile nur explizit (z. B. Waveshare 5″ DSI via `--waveshare-5-dsi`).
- **UI-Technik:** native Qt-Quick-UI über EGLFS/KMS (`ui/qtquick/Main.qml`, `qmlscene`). Kein
  Chromium-Kiosk, kein X11/Wayland als Annahme. Parallel dazu eine Browser-/Admin-Oberfläche im
  Netzwerk (eingebettetes HTML/CSS/JS unter `webui/`).
- **Audio:** `mpv` + ALSA, kein PulseAudio als Annahme.
- **Netzwerk auf DietPi:** oft `wpa_supplicant`/`wpa_cli`, nicht zwingend NetworkManager.
  Vorhandenes Tool (`wpa_cli` oder `nmcli`) nutzen, nicht eines erzwingen.
- **Privilegierte Host-Aktionen:** ausschließlich über `mupibox-system-agent` (Unix-Socket, kein
  Netz-Listener). DietPi-Werkzeuge wie `/boot/dietpi/dietpi-network` verwenden, wenn vorhanden –
  keine Raspberry-Pi-OS-only-Pfade hart verdrahten.
- **MuPiHat:** bevorzugte Hardware für Audio, Akku/Laden, Ein/Aus und geordneten Shutdown; andere
  Hardware über gekapselte Adapter. Bestehender Python-Treiber für den BQ25792 bleibt die
  I²C-Quelle (GPLv3, Lizenz/Urheberhinweis erhalten); der Go-Dienst übernimmt Konfiguration,
  Admin-API, Statusanzeige und Schutzlogik. Details: `docs/mupihat.md`.
- **Langfristig:** lokale Medien, Spotify (pro Box eigenes Konto), Webradio/Streams, Podcasts
  (RSS); Bedienung optional über Touch, Hardwaretasten, später RFID; Web-Administration;
  möglichst schneller System-/GUI-Start; später Home-Assistant-Integration.

### Pfadkonvention im Dienstbetrieb

| Art | Pfad |
| --- | --- |
| Programm | `/usr/local/lib/mupibox-ng/mupibox` |
| Bootstrap-JSON | `/etc/mupibox-ng/config.json` |
| SQLite | `/var/lib/mupibox-ng/mupibox.db` |
| Medien | `/srv/mupibox/music` |

JSON enthält **nur Bootstrap**-Werte (Listen-Adresse, DB-Pfad, Medienpfad, Audio-Backend).
Dynamische Admin-Daten (Kategorien, Reihen, Box-Einstellungen, TTS, Idle-Timer, Zuordnungen)
liegen in SQLite mit versionierten Migrationen. Updates dürfen Einstellungen/Medien nicht
überschreiben.

## Architektur

| Paket | Verantwortung |
| --- | --- |
| `cmd/mupibox` | Bootstrap-Konfiguration, Adapterwahl, HTTP-Lifecycle, SIGTERM/SIGINT |
| `cmd/mupibox-system-agent` | Restricted System Agent für privilegierte Host-Aktionen (Unix-Socket) |
| `internal/library` | Scan lokaler Ordner, stabile IDs, Titelreihenfolge, Pfadprüfung |
| `internal/core` | Controller: serialisierte Befehle, Queue, Status, Lautstärkengrenze, Titelwechsel |
| `internal/audio` | Audio-Adaptervertrag; `mpv` und gekennzeichnete `simulated`-Implementierung |
| `internal/server` | JSON-API, Home-/Kategorie-Modell, Auth/Sessions, System-/Admin-Endpunkte |
| `internal/connectivity` | WLAN- (`wpa_cli`/`nmcli`) und Bluetooth-Verwaltung |
| `internal/store` | SQLite, Migrationen, Einstellungen, Kategorien/Reihen, persistenter Zustand |
| `webui` | In die Binärdatei eingebettetes HTML/CSS/JS (Player + `/admin/`) |
| `ui/qtquick` | Native Qt-Quick-Touchoberfläche (`Main.qml`) für EGLFS/KMS |
| `deploy` | systemd-Units und Beispiel-/Dev-Konfigurationen |
| `scripts` | DietPi-Installer, Display-/Boot-Tuning, Release-Update, LXC-Abgleich |
| `legacy/prototype` | Historischer Go-Prototyp, unverändert archiviert – nicht weiterentwickeln |

Geplant, noch nicht vorhanden (siehe `docs/architecture.md` „Ausbau“): `internal/tts`,
`internal/power`, `internal/providers`, `internal/hardware`, `internal/display`,
`internal/network`, `internal/homeassistant`.

Alle Bedienmodule rufen denselben Controller auf; er serialisiert Befehle/Audioabfragen per
Mutex. Backendstatus wird derzeit gepollt (Backend alle 500 ms, Browser alle 750 ms – bewusst
einfache M1-Lösung). `mpv` läuft ohne Shell, mit Argument-Slice, in einem privaten 0700-Tempdir
mit Unix-Socket; pro Titel ein neuer Prozess (nicht gapless). Kategorien/Reihen/Medien sind
**datengetrieben** über `/api/home` – keine feste Frontend-Verdrahtung von Kategorien.

Vollständige Details: [`docs/architecture.md`](docs/architecture.md),
[`docs/requirements.md`](docs/requirements.md) (verbindliche Produktvorgaben),
[`docs/persistence.md`](docs/persistence.md), [`docs/navigation-model.md`](docs/navigation-model.md),
[`docs/player-model.md`](docs/player-model.md), [`docs/system-settings.md`](docs/system-settings.md),
[`docs/connectivity.md`](docs/connectivity.md), [`docs/backup-update.md`](docs/backup-update.md),
[`docs/mupihat.md`](docs/mupihat.md), [`docs/spotify.md`](docs/spotify.md),
[`docs/admin-guide.md`](docs/admin-guide.md), [`docs/repository-audit.md`](docs/repository-audit.md)
(Bewertung des alten Prototyps), [`docs/device-install-dietpi.md`](docs/device-install-dietpi.md).

## Build- und Testkommandos

```sh
cd /opt/mupibox-ng
mkdir -p music                      # einmalig für lokale Dev-Bibliothek

go test ./...
rm -f bin/mupibox && \
go build -buildvcs=false -o bin/mupibox ./cmd/mupibox && \
./bin/mupibox -config deploy/config.dev.json     # http://<host>:8090
```

`-buildvcs=false` ist beim lokalen Root-Build nötig, wenn der Checkout-Besitzer vom Build-User
abweicht (bekannter Go/Git-VCS-Stamping-Fehler). Durch `rm` vor `&&` startet nach einem
fehlgeschlagenen Build keine alte Binärdatei.

Vollständige Prüfung wie in CI (`.github/workflows/ci.yml`) und `scripts/check.sh`:

```sh
go vet ./...
go test -race ./...
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o bin/mupibox-linux-arm64 ./cmd/mupibox
node --check webui/static/admin/admin.js
node --check webui/static/app.js
bash -n scripts/*.sh
```

Echte lokale Wiedergabe (kein Simulationstest):

```sh
sudo apt-get install mpv
cp deploy/config.example.json config.local.json   # backend auf "mpv" setzen
./bin/mupibox -config config.local.json
```

CI läuft auf Push/PR gegen `rebuild/go-foundation` via GitHub Actions (Go 1.24.x, `ubuntu-latest`).

Deployment vom DEV-LXC auf den Test-Pi (auch uncommittete Änderungen, siehe
DEV/TEST-Rollenverteilung):

```sh
./scripts/deploy-pi.sh          # rsync Arbeitsbaum + install-service.sh + Neustart auf dem Pi
./scripts/test-pi.sh            # go vet/go test nativ auf dem Pi (arm64)
./scripts/status-pi.sh          # systemctl status + /api/health
./scripts/logs-pi.sh            # journalctl -u mupibox-ng
```

## Coding-Konventionen

### Go (DietPi-optimiert)

- Go **≥ 1.24**, Modul `mupibox`. CGO ist wegen `mattn/go-sqlite3` nötig – Cross-/Geräte-Builds
  daran ausrichten.
- Standardbibliothek bevorzugen; neue Abhängigkeiten nur mit klarem Nutzen und
  ARM64/DietPi-Tauglichkeit.
- Externe Prozesse (z. B. `mpv`) **ohne Shell** starten, Argumente als Slice, private
  Sockets/Tempdirs mit restriktiven Rechten.
- Keine Root-Rechte im Player-Dienst. Host-Änderungen (WLAN-Adapter, Swap, Samba, Power, IPv4)
  ausschließlich über den System-Agent und geprüfte Aktionen.
- Ressourcenschonend: wenig Goroutine-Wildwuchs, keine unnötigen Timer/Polls, Cover-Scans und
  große Dateien speicherschonend. Pi-3-Latenz (SQLite, Cover, mpv-Start) mitdenken.
- Fehler explizit behandeln; externe Tools (`wpa_cli`, `nmcli`, `bluetoothctl`, `rfkill`,
  `dietpi-network`) dürfen fehlen – dann klarer Fehler, kein Absturz.
- Neuer Code braucht `go test`-Abdeckung. Simulation (`simulated`-Audio, HTTP-Input) klar vom
  echten Gerätepfad trennen.
- Architektur einhalten: `cmd/` schlank, Logik in `internal/*`. Web-UI über `webui` embedden.
  Kategorien/Reihen datengetrieben, nicht im Frontend hart verdrahten.
- Shutdown/Idle auf der Entwicklungs-LXC niemals ungeprüft den Host herunterfahren.

### Shell-Skripte (DietPi-optimiert)

- `#!/usr/bin/env bash`, `set -euo pipefail`, stabile `cd` über `"${BASH_SOURCE[0]}"`.
- Root nur wo nötig; früh `EUID` prüfen und verständlich abbrechen.
- Pakete über `apt-get` mit `DEBIAN_FRONTEND=noninteractive`; nur Pakete, die auf DietPi/Debian
  ARM64 existieren; optionale Pi-Pakete nur installieren, wenn `apt-cache show` sie kennt.
- DietPi-eigene Tools/Pfade bevorzugen (`dietpi-display`, `/boot/dietpi/…`, DietPi-Dienste).
  Keine Raspberry-Pi-OS-only-Annahmen (`raspi-config` als Default, NetworkManager-only,
  Desktop-Session).
- Idempotent und wiederanlauffähig: vorhandene User, Units, Verzeichnisse, Configs nicht
  zerstören. Medien und SQLite bei Updates niemals überschreiben.
- Keine interaktiven Prompts in Installern – Optionen per Flags (wie `scripts/install-dietpi.sh`).
- `systemctl` nur für Units, die wir selbst installieren oder deren Existenz geprüft wurde.
- Architektur über `dpkg --print-architecture` (arm64 bevorzugt).
- Logging nach stderr/stdout, kein `eval` mit untrusted Input, Pfade quoten.

### UI und Produktregeln

- Touch zuerst: große Ziele, kein Hover als einzige Informationsquelle.
- Admin unter `/admin/`, getrennt von der Kinder-/Player-Oberfläche.
- Gemeinsame Queue für alle Clients über denselben Controller.
- Keine Secrets in Git. WLAN-Passwörter nicht in der MuPiBox-DB speichern, wenn der Host das
  Netz verwaltet. Zugangsdaten für externe Provider (Spotify, TTS) getrennt und sicher speichern.

## Arbeitsweise

- Vor größeren Änderungen bestehende Doku in `docs/` und Installer in `scripts/` lesen.
- `legacy/prototype/` ist rein historisch – neuer Code gehört nicht dorthin, keine ungeprüfte
  Löschung alten Codes (Git-Historie bleibt vollständig erhalten).
- Keine unnötigen Refactors. Deutsch in Regeln/Nutzertexten, Englisch parallel in `*.en.md`, wo
  das Projekt das bereits tut (README, CHANGELOG, docs/).
- Anforderungen in `docs/requirements.md` sind verbindliche Vorgaben, keine Behauptung über den
  Ist-Stand – der Ist-Stand steht in README.md und CHANGELOG.md.
- Dieses Repository läuft in einer Debian-13-LXC unter `/opt/mupibox-ng` ohne Pi-Hardware; Audio,
  GPIO, Stromversorgung und Display müssen zusätzlich am echten Pi getestet werden.
