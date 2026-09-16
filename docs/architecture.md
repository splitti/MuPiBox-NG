# Architektur – Grundgerüst

Die Schnittstellen orientieren sich an einer einzigen Box mit einem gemeinsam genutzten Player.

| Paket | Verantwortung |
| --- | --- |
| `cmd/mupibox` | Konfiguration, Adapterwahl, HTTP-Lifecycle, SIGTERM/SIGINT |
| `internal/library` | Scan lokaler Ordner, stabile IDs, Titelreihenfolge, Pfadprüfung |
| `internal/core` | Serialisierte Befehle, Queue, Status, Lautstärkengrenze, Titelwechsel |
| `internal/audio` | Adaptervertrag; mpv und eindeutig gekennzeichnete Simulation |
| `internal/server` | JSON-API, Input-Zuordnungen, Cover-Ausgabe, Browser-Originprüfung |
| `webui` | In die Binärdatei eingebettetes HTML/CSS/JS |

Alle Module rufen den Controller auf. Er serialisiert Befehle und Audioabfragen mit einem Mutex;
Statuskopien enthalten keine gemeinsam veränderbaren Queue-Slices. Der Backendstatus wird alle
500 ms abgefragt. Browser lesen alle 750 ms. Polling ist die bewusst einfache M1-Lösung.

mpv wird ohne Shell und Benutzerkonfiguration gestartet. Ein privates 0700-Tempverzeichnis enthält
den Unix-Socket. Der Controller übergibt nur gescannte, erneut validierte Dateipfade, keine vom
Browser gelieferten Shellbefehle oder beliebigen Medien-URLs. JSON-Anfragen haben Request-IDs,
Zeitlimits und werden erst nach Antwort als erfolgreich betrachtet; load wartet zusätzlich auf
`file-loaded`. Decoderfehler führen zum Fehlerzustand, EOF zum nächsten Queue-Titel.

Pro Titel wird ein neuer mpv-Prozess erzeugt: vereinfacht zunächst die Abgrenzung verspäteter
Ereignisse; nicht gapless, Ressourcenbedarf/Ladezeit am Pi 3 noch messen. Der Adapter kann später
ausgetauscht werden. Die IPC-Schnittstelle wird nicht ins Netzwerk gestellt.
Referenz: https://mpv.io/manual/stable/#json-ipc

## Datentrennung

| Art | Zielverzeichnis im Dienstbetrieb |
| --- | --- |
| Programm | `/usr/local/lib/mupibox-ng/mupibox` |
| Einstellungen | `/etc/mupibox-ng/config.json` |
| Künftiger Status / Zugangsdaten | `/var/lib/mupibox-ng/` (systemd StateDirectory, restriktive Rechte) |
| Lokale Musik | `/srv/mupibox/music` oder konfigurierbarer anderer Pfad |

M1 schreibt noch keine Fortschritts-/Kontodaten. Geplante Persistenz muss atomare Speicherung,
Dateifehler, Schemaversionen und Migrationen berücksichtigen. Geheimnisse nie in HTTP-Status/Logs/Git.

## Ausbau

- Bibliotheksquellen über Provider-Vertrag ergänzen, die normalisierte spielbare Inhalte liefern.
  Keine Spotify/Amazon-URLs durch den lokalen Adapter schmuggeln.
- Spotify braucht einen eigenen geprüften Auth-/Katalog-/Playback-Ablauf mit Kontotrennung je Box.
- RSS: Feedverwaltung, Episoden, Zeit-/Titelreferenzen, dauerhafter Fortschritt.
- Input-Router aus HTTP-Simulation in eigenständigen Dienst verschieben, sobald echte GPIO/RFID-
  Adapter hinzukommen. Entprellen und verlässliche Kartenanwesenheit im jeweiligen Adapter.
- Hardware-Fähigkeiten explizit melden; unbekannte Akkudaten nicht mit 0 % verwechseln.
- Elternverwaltung und Anmeldung getrennt vom Kindermodus; M1-Info ist kein geschützter Adminbereich.
- Update: versionierte Binärverzeichnisse, geprüfte Migrationen, atomare Aktivierung und Rollback;
  Einstellungen und Medien bleiben außerhalb. Der Installationshelfer ist noch kein Updater.

## Abnahme am Gerät

Pi 3 ARM64 / DietPi booten; ALSA-/MuPiHAT-Gerät identifizieren, Mono/Stereo und Lautstärke prüfen,
800×480-DSI-Touch und Kiosk testen. Erst nach bestätigter Revision GPIO und Stromsteuerung
implementieren. Shutdown unter Akku/Laden, Restlaufzeitverhalten und Wiederanlauf tatsächlich testen.
