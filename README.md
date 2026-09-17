# MuPiBox-NG

> **Deutsch** · [English](README.en.md)

Neuentwicklung eines modularen Musikplayers für **DietPi ARM64, Raspberry Pi 3 oder neuer**. Touch, Browser, Tasten und RFID sollen dieselbe Wiedergabe und Warteschlange steuern.

**Entwicklungsstand: 0.1.0-dev.** Noch keine produktionsfertige Box; ein erster experimenteller DietPi-Installer für den Hardwaretest ist vorhanden. Der unveränderte Go-Prototyp vom 10.02.2026 liegt in `legacy/prototype/`. Die komplette Git-Historie bleibt erhalten. Entwicklung auf `rebuild/go-foundation`.

## Was jetzt funktioniert

- Go-HTTP-Dienst mit eingebetteter Weboberfläche.
- Kleine Touch-Oberfläche, primär für 800 × 480 quer entworfen, mit eigener großflächiger 5-Zoll-Wiedergabeebene.
- Auswählbares Commodore-inspiriertes 8-Bit-Theme mit Terminus-/Terminalschrift neben der modernen Oberfläche.
- WLAN-Suche und Verbindung am Touchdisplay (Haltedruck auf das WLAN-Symbol) sowie im Adminbereich; vorhandenes `wpa_cli` oder `nmcli` wird verwendet.
- Bluetooth global ein-/ausschalten sowie Geräte im Adminbereich suchen, koppeln, verbinden, trennen und entfernen.
- Datengetriebene Kategorien und Inhaltsreihen über `/api/home`; Medien verschiedener Provider erscheinen innerhalb einer Kategorie gemischt.
- Bei fehlender Netzwerkverbindung zeigt der Player nur als offline verfügbar markierte lokale Inhalte.
- Kategorien, Medienquellen und lokalisierte Namen werden in SQLite gespeichert und über die Admin-Weboberfläche gepflegt.
- Globale TTS-Entwicklungseinstellung: Ein/Aus, Sprache und Provider; `browser-dev` dient nur als Testfallback.
- Globale Power-Entwicklungseinstellung mit `idle_shutdown_minutes`; der echte Shutdown ist noch nicht implementiert.
- Lokale Ordnerbibliothek mit Unterverzeichnissen, natürlicher Sortierung, ganzen Ordnern und Coverbildern.
- Gemeinsame Warteschlange für alle Clients mit Start, Pause, Weiter/Zurück, Spulen und Lautstärke.
- Austauschbarer Audioadapter: `mpv` für Audio an der Box, `simulated` für Tests ohne Audiogerät.
- Simulierbare Tasten- und RFID-Zuordnungen. Noch keine echten Hardwaretreiber.
- SQLite-Datenbank mit automatischer Migration für Box-Einstellungen, Navigation und Wiedergabefortschritt.
- Erste echte Admin-Weboberfläche unter `/admin/` für globale Einstellungen sowie Kategorien/Reihen.
- Optionaler Admin-Passwortschutz mit sicherem Hash, Login-Sitzung, Abmeldung und begrenzten Fehlversuchen.
- Persistentes Fortsetzen lokaler Audiodateien, auch bei einzelnen sehr langen Dateien; Fortschrittsvertrag für spätere Provider wie Spotify, Live-Radio ausgeschlossen.
- Frei platzierbare Medienquelle „Fortsetzen / Resume-Liste“ mit 1–100 zuletzt begonnenen, noch nicht abgeschlossenen Medien.

Die Zieloberfläche ist keine fest verdrahtete Musikseite. Kategorien wie Hörbücher, Musik, Radio oder Podcasts werden als Daten geliefert. Das spätere Admin-Interface soll dieses Modell und die globalen Box-Einstellungen verwalten.

**Persistenzentscheidung:** Dynamische Admin-Daten liegen in SQLite. Die JSON-Datei enthält ausschließlich Listen-Adresse, SQLite-Pfad, lokalen Medien-Grundpfad und Audio-Backend.

## Entwicklung in der LXC

Voraussetzung: Go ab 1.24, Git. Bereits geprüft: Debian 13 / Go 1.24.4, Projektpfad `/opt/mupibox-ng`.

```sh
cd /opt/mupibox-ng
mkdir -p music

go test ./...
rm -f bin/mupibox && \
go build -buildvcs=false -o bin/mupibox ./cmd/mupibox && \
./bin/mupibox -config deploy/config.dev.json
```

Aufruf im Heimnetz: `http://<LXC-IP>:8090`.

`-buildvcs=false` verhindert beim lokalen Root-Build den bekannten Go/Git-VCS-Stamping-Fehler bei abweichendem Checkout-Besitzer. Durch `rm` und `&&` wird nach einem fehlgeschlagenen Build keine alte Binärdatei gestartet.

## Dynamische Startseite

Bei einer leeren SQLite-Datenbank legt der Dienst folgende Startkategorien an:

- Hörbücher / Audiobooks
- Musik / Music
- Radio
- Podcasts

Eine Kategorie besitzt eine stabile ID und lokalisierte Labels. Darunter liegen beliebig viele Reihen. Eine Reihe besitzt ebenfalls lokalisierte Labels und einen Provider. `local-library` erzeugt bereits Cover-Kacheln aus der lokalen Musikbibliothek.

TTS ist **nicht pro Kategorie** konfiguriert. Die Box besitzt global `enabled`, `language` und `provider`. Ist TTS aktiv, kann ein Kategorie-Tap den Namen in der gewählten Boxsprache sprechen. Für die Entwicklung verwendet `browser-dev` die Browser-Sprachausgabe; die Produktionsarchitektur erhält einen eigenen lokalen/externen TTS-Adapter.

## Admin und SQLite

Das Admin-Interface ist unter `/admin/` erreichbar. Im aktuellen ersten Stand pflegt es bereits globale Grundeinstellungen sowie Kategorien, Reihenfolge, Übersetzungen und Reihen/Provider. Weiter vorgesehen sind:

- Kategorien und deren Reihenfolge/Sichtbarkeit/Übersetzungen,
- Inhaltsreihen und Provider-Zuordnung,
- globale TTS-Einstellungen und Provider-Konfiguration,
- maximale Lautstärke,
- Idle-Abschaltung und spätere Zeitpläne,
- später RFID/Tasten, Provider-Konten und weitere Geräteeinstellungen.

Diese Daten liegen in der unter `database_path` konfigurierten SQLite-Datei; im Dienstbetrieb ist `/var/lib/mupibox-ng/mupibox.db` vorgesehen. JSON bleibt nur für Bootstrap-/Deployment-Werte. Im Bereich „Sicherheit“ kann ein Admin-Passwort gesetzt werden; danach sind Verwaltung und externe Konnektivitäts-APIs nur mit angemeldeter Sitzung erreichbar.

## Echte lokale Wiedergabe

`mpv` über die Debian-/DietPi-Paketverwaltung installieren. Die LXC braucht für hörbaren Ton zusätzlich ein verfügbares Audiogerät; sonst am Pi testen. Simulation ist kein Audiotest.

```sh
sudo apt-get install mpv
cp deploy/config.example.json config.local.json
# backend in config.local.json auf "mpv" setzen.
./bin/mupibox -config config.local.json
```

Unterstützte Endungen: MP3, FLAC, OGG, OPUS, WAV, M4A, AAC. Tatsächlich lesbare Codecs hängen von mpv ab. Cover: `cover.jpg`, `cover.png`, `folder.jpg`, `folder.png`.

## Bedienung und API

| Route | Zweck |
| --- | --- |
| `GET /api/home` | Kategorien, Reihen, lokalisierte Beschriftungen und normalisierte Medienobjekte |
| `GET /api/library` | Lokale Ordner, Titel, stabile IDs, Cover-URLs |
| `GET /api/status` | Gemeinsame Warteschlange und Adapterstatus |
| `GET /api/info` | Version, Backend sowie globale TTS-/Power-Entwicklungswerte |
| `GET /api/health` | HTTP-Dienst erreichbar |
| `POST /api/command` | Wiedergabebefehl |
| `POST /api/input` | Simulierte Taste/RFID im Entwicklungsmodus |
| `GET/PUT /api/admin/settings` | Persistente globale Box-Einstellungen |
| `GET/PUT /api/admin/navigation` | Persistente Kategorien und Reihen |
| `GET /api/admin/auth`, `POST /api/admin/login` | Passwortschutz und Admin-Sitzung |
| `PUT /api/admin/password`, `POST /api/admin/logout` | Passwort setzen/ändern und Sitzung beenden |
| `GET /api/connectivity/wifi/adapters` | WLAN-Adapter, Zustand und aktive Auswahl |
| `PUT /api/connectivity/wifi/preferences` | Freigegebene und bevorzugte WLAN-Adapter speichern |
| `GET /api/connectivity/wifi` | WLAN-Netze suchen |
| `POST /api/connectivity/wifi/connect` | Mit WLAN verbinden (Passwort wird nicht von MuPiBox gespeichert) |
| `GET /api/connectivity/bluetooth` | Bluetooth-Geräte suchen und Status lesen |
| `POST /api/connectivity/bluetooth/command` | Bluetooth koppeln, verbinden, trennen oder entfernen |

Die Admin- und extern aufgerufenen Konnektivitäts-APIs werden geschützt, sobald ein Admin-Passwort gesetzt ist. Trotzdem ist die Entwicklungsbox nur für ein vertrauenswürdiges Heimnetz vorgesehen: kein direktes Internet-Portforwarding.

## Tests und Grenzen

`go test ./...` prüft Bibliothek/Pfadgrenzen, Queue/EOF, Lautstärke, konkurrierende Bedienung, API, datengetriebene Home-Struktur, globale Box-Infos, RFID-/Tasten-Simulation sowie einen Linux-ARM64-Crossbuild.

Noch offen: Admin-Authentifizierung und weitere Hardware-/Provider-Einstellungsseiten, produktiver lokaler TTS-Adapter, echter Idle-Shutdown, Webradio, RSS-Podcasts, Spotify-Katalog/Playback, Video/YouTube, echte RFID/GPIO-Module, MuPiHAT/Shutdown/Akku, vollständige native Playerbedienung, Releases und Rollback.

## Dokumentation

- [Verbindliche Vorgaben](docs/requirements.md) · [Requirements (English)](docs/requirements.en.md)
- [Architektur](docs/architecture.md) · [Architecture (English)](docs/architecture.en.md)
- [Persistenz](docs/persistence.md) · [Persistence (English)](docs/persistence.en.md)
- [Navigation/Inhalte](docs/navigation-model.md) · [Navigation/content (English)](docs/navigation-model.en.md)
- [Player/Wiedergabeprofile](docs/player-model.md) · [Player/playback profiles (English)](docs/player-model.en.md)
- [System-/Hardwareeinstellungen](docs/system-settings.md) · [System/hardware settings (English)](docs/system-settings.en.md)
- [MuPiHat-Integration](docs/mupihat.md) · [MuPiHat integration (English)](docs/mupihat.en.md)
- [WLAN/Bluetooth](docs/connectivity.md) · [Wi-Fi/Bluetooth (English)](docs/connectivity.en.md)
- [Spotify/Cache](docs/spotify.md) · [Spotify/cache (English)](docs/spotify.en.md)
- [Bestandsprüfung](docs/repository-audit.md) · [Repository audit (English)](docs/repository-audit.en.md)
- [DietPi-/Gerätetest](docs/device-install-dietpi.md)
- [Prüfstand](docs/validation.md) · [Validation (English)](docs/validation.en.md)
