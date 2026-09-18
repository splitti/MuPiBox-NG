# Architektur – Grundgerüst

> Deutsch. English version: [architecture.en.md](architecture.en.md)

Die Schnittstellen orientieren sich an einer einzelnen Box mit einem gemeinsam genutzten Player.

| Paket | Verantwortung |
| --- | --- |
| `cmd/mupibox` | Bootstrap-Konfiguration, Adapterwahl, HTTP-Lifecycle, SIGTERM/SIGINT |
| `internal/library` | Scan lokaler Ordner, stabile IDs, Titelreihenfolge, Pfadprüfung |
| `internal/core` | Serialisierte Befehle, Queue, Status, Lautstärkengrenze, Titelwechsel |
| `internal/audio` | Adaptervertrag; mpv und eindeutig gekennzeichnete Simulation |
| `internal/server` | JSON-API, Home-/Kategorie-Modell, Box-Status, Input-Zuordnungen, Cover-Ausgabe |
| `webui` | In die Binärdatei eingebettetes HTML/CSS/JS |
| künftig `internal/store` | SQLite, Migrationen, Einstellungen, Kategorien/Reihen, persistenter Zustand |
| künftig `internal/tts` | Einheitlicher TTS-Vertrag und Provideradapter |
| künftig `internal/power` | Idle-Erkennung, Zeitpläne, kontrollierter Shutdown |
| künftig `internal/providers` | Lokale Quellen, Spotify, Radio, RSS; normalisierte Inhalte und Cache |
| künftig `internal/hardware` | MuPiHat, Batterieprofile, GPIO, Lüfter und Betriebs-LED |
| künftig `internal/display` | Helligkeit, Display-Idle, Splashscreen und Displayadapter |
| künftig `internal/network` | WLAN-Status, Scan/Verbindung und Captive-Portal-Erkennung |
| künftig `internal/homeassistant` | versionierte API, Token und native Home-Assistant-Entitäten |

Alle Bedienmodule rufen denselben Controller auf. Er serialisiert Befehle und Audioabfragen mit einem Mutex; Statuskopien enthalten keine gemeinsam veränderbaren Queue-Slices. Der Backendstatus wird derzeit alle 500 ms abgefragt, Browser lesen alle 750 ms. Polling ist die bewusst einfache M1-Lösung.

mpv wird ohne Shell und Benutzerkonfiguration gestartet. Ein privates 0700-Tempverzeichnis enthält den Unix-Socket. Der Controller übergibt nur gescannte, erneut validierte Dateipfade. Decoderfehler führen zum Fehlerzustand, EOF zum nächsten Queue-Titel.

Pro Titel wird derzeit ein neuer mpv-Prozess erzeugt. Das vereinfacht die Abgrenzung verspäteter Ereignisse; es ist nicht gapless. Ressourcenbedarf und Ladezeit am Pi 3 müssen noch gemessen werden. Die IPC-Schnittstelle wird nicht ins Netzwerk gestellt.

## UI-Architektur

Das Zielgerät ist primär ein 5-Zoll-Touchdisplay mit 800 × 480 Pixeln im Querformat. Die Oberfläche wird deshalb vom kleinen Display aus entworfen und erst danach für größere Browseransichten erweitert. Hover-Zustände dürfen keine Voraussetzung für Bedienung oder Informationszugang sein.

Die UI besteht konzeptionell aus vier Ebenen:

1. schmale Systemstatusleiste,
2. frei konfigurierbare textbasierte Kategorien,
3. bildorientierte Inhaltsreihen innerhalb der Kategorien,
4. kompakter Now-Playing-/Player-Bereich.

Die Statusleiste soll normalisierte Systeminformationen anzeigen: WLAN, Signalstärke, Akku-/Ladezustand, Uhrzeit und später weitere Zustände. Die UI darf nicht direkt von MuPiHAT-, NetworkManager- oder Betriebssystemdetails abhängen. Hardware-/Systemadapter liefern einen gemeinsamen Status. Unbekannte Werte bleiben unbekannt und werden nicht als 0 % dargestellt.

## Datengetriebene Startseite

Kategorien wie `Hörbücher`, `Musik`, `Radio` oder `Podcasts` sind keine fest verdrahteten Frontend-Seiten. Das Backend liefert unter `/api/home` ein generisches Modell aus Kategorien, Reihen und Medienobjekten. Das Frontend kennt keine spezielle Kategorie-Logik und rendert nur dieses Modell.

Der aktuelle Zwischenschritt liest Kategorien noch aus JSON. Sobald die Persistenzschicht steht, kommen diese Daten aus SQLite. Das spätere Admin-Interface pflegt dasselbe Modell persistent. Dadurch können Kategorien und Reihen angelegt, gelöscht, sortiert, übersetzt, aktiviert/deaktiviert und mit Providern verknüpft werden, ohne Frontend-Code zu ändern.

Eine Kategorie besitzt mindestens eine stabile ID, lokalisierte Anzeigenamen, Sortierung und Sichtbarkeit. Eine Inhaltsreihe besitzt ebenfalls eine stabile ID, lokalisierte Beschriftungen, Sortierung, Sichtbarkeit und einen Provider. Der Provider erzeugt normalisierte Medienobjekte. Aktuell existiert `local-library`; später kommen zum Beispiel Spotify, Radio, RSS-Podcasts und manuell gepflegte Inhalte hinzu.

Ein normalisiertes Medienobjekt kann stabile ID, Typ, Titel, Untertitel, Cover, Providerreferenz und ausführbare Aktion enthalten. Dadurch kann dieselbe horizontale Reihe lokale Ordner, Künstler, Spotify-Playlists, Radiosender, Hörbücher oder Podcasts darstellen.

Vertikales Scrollen wechselt zwischen Kategorien/Reihen, horizontales Scrollen durchsucht eine Inhaltsgruppe.

## Globale Box-Einstellungen

TTS, Sprache, Idle-Timer und weitere Geräteoptionen sind **boxweite Einstellungen** und nicht Teil einer einzelnen Kategorie. Das Admin-Interface schreibt sie in die persistente Einstellungsablage.

Aktuell exponiert `/api/info` die Entwicklungswerte für TTS und Power bereits getrennt vom Home-/Kategorie-Modell. Das ist bewusst die Richtung für die spätere persistente Architektur.

Audio, Display, Netzwerk, MuPiHat/Batterie, Lüfter, LED, Home Assistant und Themes werden ebenfalls als boxweite Einstellungen bzw. Adapterfähigkeiten behandelt. Display-Idle und Box-Shutdown bleiben getrennte Zustandsautomaten. Details: [System-, Hardware- und Admin-Einstellungen](system-settings.md).

## TTS

TTS wird als eigener Adapter behandelt. Der globale TTS-Zustand umfasst mindestens:

- `enabled`,
- Sprache/Locale,
- Provider,
- später Voice-ID und providerspezifische Einstellungen.

Wenn TTS aktiviert ist, kann ein Kategorie-Tap den lokalisierten Kategorienamen sprechen. Die Kategorie selbst besitzt keine eigene Sprache oder Engine.

Der TTS-Vertrag soll einen Text plus Sprache/Locale an einen Provider übergeben. Provider können lokal/offline oder extern sein. Eine lokale Engine ist wichtig, damit eine Box mit lokalen Medien auch ohne Internet vollständig nutzbar bleibt. Externe Provider sind optional und müssen ihre Zugangsdaten getrennt und sicher speichern.

`browser-dev` ist nur ein Entwicklungsprovider, um Interaktion und Sprache früh zu testen. Er ist keine Produktionslösung.

## Fortschritt und Resume

Fortschritt wird anhand einer stabilen normalisierten Medien-ID persistiert, nicht anhand der aktuellen Queueposition. Position und Dauer verwenden 64-Bit-Millisekundenwerte. Dadurch kann auch eine einzelne 23-Stunden-Datei zuverlässig fortgesetzt werden. Gespeichert wird periodisch sowie bei Pause, Seek, Quellenwechsel und kontrolliertem Shutdown. Gesprochene Inhalte verwenden standardmäßig gut erkennbare 10-Sekunden-Sprünge.

Details: [Dynamisches Player- und Wiedergabeprofil](player-model.md).

## Energieverwaltung

Die Energieverwaltung wird als eigener Dienst/Adapter geplant und nicht direkt in die UI oder den Player eingebaut.

Der erste Anwendungsfall ist `idle_shutdown_minutes`: Nach einer konfigurierten Zeit ohne aktive Wiedergabe und ohne relevante Benutzeraktivität kann die Box kontrolliert herunterfahren. Der Wert `0` deaktiviert die Funktion.

Später können Zeitpläne, Nachtruhe oder feste Abschaltzeiten hinzukommen. Vor einem Shutdown müssen laufende Schreibvorgänge abgeschlossen, Persistenz konsistent gespeichert und Audio sauber beendet werden. Auf der Entwicklungs-LXC darf die Shutdown-Funktion nicht ungeprüft das Hostsystem abschalten.

## Persistenz und Datentrennung

Dynamische Administrationsdaten werden künftig in SQLite gespeichert. JSON bleibt eine kleine Bootstrap-Konfiguration.

| Art | Ziel im Dienstbetrieb |
| --- | --- |
| Programm | `/usr/local/lib/mupibox-ng/mupibox` |
| Bootstrap-Konfiguration | `/etc/mupibox-ng/config.json` |
| SQLite-Datenbank | `/var/lib/mupibox-ng/mupibox.db` |
| Geheimnisse/Provider-Zugangsdaten | restriktiv unter `/var/lib/mupibox-ng/` bzw. spätere Secret-Abstraktion |
| Lokale Musik | `/srv/mupibox/music` oder konfigurierbarer Pfad |

SQLite speichert Kategorien, Reihen, Übersetzungen, globale Box-Einstellungen, Zuordnungen und später Fortschritt/Status. Das Schema wird versioniert und über Migrationen weiterentwickelt. Updates dürfen Einstellungen und Medien nicht überschreiben.

Details: [persistence.md](persistence.md)

## Administration

Das Admin-Interface ist eine getrennte Verwaltungsoberfläche, voraussichtlich unter `/admin`. Es soll mindestens Kategorien/Reihen, Reihenfolge/Sichtbarkeit, Übersetzungen, Provider/Zielquellen, globale TTS-Einstellungen, Idle-Timer, Lautstärkelimit und später weitere Geräte-/Kontoeinstellungen verwalten.

Die Kind-/Player-Oberfläche erhält nur das normalisierte Lesemodell. Schreibzugriffe und Berechtigungen des Admin-Interfaces werden separat abgesichert und nicht mit der heutigen Entwicklungs-API gleichgesetzt.

## Ausbau

- SQLite-Store mit Migrationen und Repository-/Service-Schicht ergänzen.
- Admin-API und Admin-Weboberfläche ergänzen.
- TTS-Adapter mit mindestens einem Offline-Provider implementieren; externe Provider optional ergänzen.
- Idle-Erkennung und kontrollierten Shutdown implementieren.
- Bibliotheksquellen über Provider-Vertrag ergänzen.
- Normalisierten Systemstatus für WLAN, Akku/Laden und weitere Geräteinformationen bereitstellen.
- Spotify mit getrenntem Auth-/Katalog-/Playback-Adapter, regelkonformem temporärem Cache und Kontotrennung je Box integrieren.
- RSS-Feedverwaltung, Episoden und Fortschritt ergänzen.
- Input-Router aus HTTP-Simulation in eigenständige Adapter verschieben.
- Update/Rollback mit versionierten Binärdateien und geprüften Datenbankmigrationen umsetzen.

## Abnahme am Gerät

Pi 3 ARM64 / DietPi booten; ALSA-/MuPiHAT-Gerät identifizieren, Audio prüfen, 800×480-DSI-Touch und Kiosk testen. Dabei insbesondere Touchzielgrößen, vertikales/horizontales Scrollen, Lesbarkeit der Statusleiste, Cover-Performance, TTS-Reaktionszeit, SQLite-Latenz und Idle-/Shutdown-Verhalten prüfen. Erst nach bestätigter Hardware-Revision GPIO und Stromsteuerung endgültig implementieren.
