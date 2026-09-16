# MuPiBox-NG

Neuentwicklung eines modularen Musikplayers für **DietPi ARM64, Raspberry Pi 3 oder neuer**.
Touch, Browser, Tasten und RFID sollen dieselbe Wiedergabe und Warteschlange steuern.

**Entwicklungsstand: 0.1.0-dev.** Noch keine produktionsfertige Box und kein fertiger Installer.
Der unveränderte Go-Prototyp vom 10.02.2026 liegt in `legacy/prototype/`.
Die komplette Git-Historie bleibt erhalten. Entwicklung auf `rebuild/go-foundation`.

## Was jetzt funktioniert

- Go-HTTP-Dienst mit eingebetteter Weboberfläche, ohne Node-Laufzeit oder externe Go-Module.
- Deutsche Oberfläche mit großen Bedienelementen; Layout für 800 × 480 quer, responsive für andere Größen.
- Lokale Ordnerbibliothek: Unterverzeichnisse, natürliche Dateinamen-Sortierung (2 vor 10), ganze Ordner, Cover.
- Eine Warteschlange für alle Clients: Start, Pause, Weiter/Zurück, Spulen, Lautstärke und serverseitiges Maximum.
- Austauschbarer Audioadapter: `mpv` für Audio **an der Box**, `simulated` für Tests ohne Audiogerät.
- Konfigurierbare Tasten- und RFID-Zuordnungen über eine optionale **Simulation**. Noch keine echten Hardwaretreiber.
- Fehler werden zurückgemeldet; es gibt keine vorgetäuschte erfolgreiche Audiowiedergabe bei fehlendem Decoder.
- systemd-Vorlage, getrennte Programm-, Konfigurations- und Musikverzeichnisse, sichtbare Version.

`simulated` erzeugt keinen Ton und verwendet pro Titel eine feste Testdauer von 180 Sekunden.
Browser sind Fernbedienungen; es wird kein Audio zum Handy gestreamt.

## Entwicklung in der LXC

Voraussetzung: Go ab 1.24, Git. Bereits geprüft: Debian 13 / Go 1.24.4, Projektpfad `/opt/mupibox-ng`.
Die MuPiBox-Dev-Verbindung kann Dateien bearbeiten und Go bauen/testen, aber kein Git klonen oder Shellbefehle ausführen.
Die ChatGPT-Arbeitsumgebung ist ein anderes System.

Die hier angelegten Quelldateien sind in der LXC zunächst eine Arbeitskopie ohne Git-Metadaten.
Einmalig dort ausführen, nachdem der Branch auf GitHub gespeichert wurde:

```sh
cd /opt/mupibox-ng
sh scripts/link-lxc.sh
```

Das Skript klont in einen temporären Nachbarordner, vergleicht vorhandene Dateien bytegenau und
bricht bei Abweichungen ab. Fehlende Repository-Dateien werden ergänzt, dann erst die Git-Metadaten übernommen.
`dev-access-check.txt`, eigene Konfiguration und Musik bleiben erhalten. Kein `reset --hard`, kein Löschen bestehender Dateien.
Für einen vollständig neuen Arbeitsplatz genügt ein normaler Clone:

```sh
git clone --branch rebuild/go-foundation https://github.com/splitti/MuPiBox-NG.git
cd MuPiBox-NG
```

Danach:

```sh
mkdir -p music
# Eigene Audiodateien in music/ oder Unterordner kopieren.
go test ./...
go build -o bin/mupibox ./cmd/mupibox
./bin/mupibox -config deploy/config.dev.json
```

Aufruf im Heimnetz: `http://<LXC-IP>:8080`. `config.dev.json` aktiviert die Simulation und bindet an alle Interfaces.
Ohne Konfigurationsdatei wird nur `127.0.0.1:8080` verwendet; Standard-Audioadapter ist mpv.

## Echte lokale Wiedergabe

`mpv` über die Debian-/DietPi-Paketverwaltung installieren. Die LXC braucht für hörbaren Ton zusätzlich ein
verfügbares Audiogerät; sonst am Pi testen. Niemals Simulation als Audiotest werten.

```sh
sudo apt-get install mpv
cp deploy/config.dev.json config.local.json
# In config.local.json: backend auf "mpv", inputs.simulate bei Bedarf auf false setzen.
./bin/mupibox -config config.local.json
```

Unterstützte Endungen: MP3, FLAC, OGG, OPUS, WAV, M4A, AAC; tatsächlich lesbare Codecs hängen von mpv ab.
Jeder Ordner mit Audiodateien wird als Sammlung angezeigt, inklusive relativem Pfad. Ein Klick spielt die direkt darin
enthaltenen Dateien. Unterordner sind eigene Sammlungen. Cover: `cover.jpg`, `cover.png`, `folder.jpg`, `folder.png`.
Versteckte Verzeichnisse und Symlinks werden beim Scan ausgelassen. Ein Neustart aktualisiert die Bibliothek.
Dateien im Musikverzeichnis müssen für den Dienstbenutzer lesbar sein.

## Bedienung und API

Alle Befehle laufen durch `internal/core.Controller`. Der Status wird von Browsern alle 750 ms abgefragt.
Es gibt keine unabhängigen Browser-Warteschlangen. Die Box funktioniert auch ohne geöffneten Browser.

| Route | Zweck |
| --- | --- |
| `GET /api/library` | Ordner, Titel, stabile IDs, Cover-URLs |
| `GET /api/status` | Gemeinsame Warteschlange und tatsächlicher Adapterstatus |
| `GET /api/info` | Version, Adapter, Simulationsfreigabe |
| `GET /api/health` | HTTP-Dienst erreichbar (kein Audiogerätetest) |
| `POST /api/command` | JSON: `action`, optional `folder_id` oder `value` |
| `POST /api/input` | Nur mit `inputs.simulate`: JSON `module` und `id` |

Aktionen: `folder`, `play`, `pause`, `toggle`, `next`, `previous`, `stop`, `seek`, `volume`, `volume_delta`.
`seek` und Positionen sind Sekunden, Lautstärke 0 bis zum konfigurierten Maximum.
`previous` startet den vorherigen Titel; am Anfang den ersten erneut. Am Queue-Ende wird gestoppt.

RFID im ersten Meilenstein: UID in `inputs.rfid` einer Ordner-ID aus `/api/library` zuordnen.
Die Kennung ist keine Zugangskontrolle. Ein erneutes Ereignis startet den Ordner von vorn;
Fortsetzen, Entprellen und Entfernen sind für die echte Hardwareanbindung noch umzusetzen.
Im Info-Dialog kann man Tasten/RFID testen; Standard-Taste `play_pause`.

Die Entwicklungs-API ist für ein vertrauenswürdiges Heimnetz vorgesehen. Sie hat noch keine Benutzeranmeldung.
Kein Internet-Portforwarding. Browser-Schreibzugriffe benötigen JSON und dürfen nicht von fremden Origins kommen.

## Dienst auf DietPi

Optional nach Installation von Go, Git und mpv:

```sh
sudo sh scripts/install-service.sh
# Musik nach /srv/mupibox/music kopieren, /etc/mupibox-ng/config.json prüfen.
sudo systemctl enable --now mupibox-ng
journalctl -u mupibox-ng -f
```

Der Installationshelfer führt kein Systemupgrade aus, überschreibt keine vorhandene Konfiguration und startet den Dienst nicht automatisch.
Er ersetzt bei erneutem Aufruf die Programmdatei. **Noch keine Update-/Rollback-Funktion.**
Dienstnutzer `mupibox`, Audiogruppe `audio`, keine Root-Wiedergabe. Audioauswahl und MuPiHAT-Treiber müssen am Gerät geprüft werden.
Ein Display-Kiosk wird noch nicht eingerichtet.

## Tests und Grenzen

`go test ./...` prüft Bibliothek/Pfadgrenzen, Queue/EOF, Lautstärke, konkurrierende Bedienung, API,
RFID-/Tasten-Simulation sowie einen Linux-ARM64-Crossbuild.
`TestMPVRealDecode` dekodiert eine selbst erzeugte WAV mit Null-Audioausgabe, wenn mpv installiert ist;
sonst wird dieser Test **übersprungen**. `go test -v ./internal/audio` zeigt den Skip ausdrücklich.
Für zusätzliche lokale Prüfung: `sh scripts/check.sh` (vet, race, ARM64-Build).

Noch offen: persistenter Hörfortschritt, Queue-Persistenz, Webradio, RSS-Podcasts, Spotify-Anmeldung und -Wiedergabe,
Amazon-Machbarkeit, echte RFID/GPIO-Module, MuPiHAT/Shutdown/Akku, Elternverwaltung, Kiosk, Releases und Rollback.
Es wird keine Akku-Prozentanzeige angenommen. Die Hardware ist hier noch nicht getestet.

- [Verbindliche Vorgaben](docs/requirements.md)
- [Bestandsprüfung und Übernahmeentscheidung](docs/repository-audit.md)
- [Architektur und nächste Schritte](docs/architecture.md)
- [Durchgeführte Prüfungen](docs/validation.md)
