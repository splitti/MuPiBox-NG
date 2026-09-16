# Änderungen

> **Deutsch** · [English](CHANGELOG.en.md)

## 0.1.0-dev – 16.09.2026

- Native Qt-Quick-Oberfläche an das Web-Frontend angeglichen (Navigation, Medienkarten und Playerleiste).
- Reversible Quiet-Boot-Konfiguration blendet Kernel-/DietPi-Konsole aus und zeigt den Startscreen früher.
- Neustart auf `rebuild/go-foundation`, Prototyp vollständig unter `legacy/prototype` erhalten.
- Vorgaben, Bestandsprüfung, Architektur und LXC-Abgleich dokumentiert.
- Go-Dienst mit eingebetteter Weboberfläche und lokalen Ordnersammlungen.
- Gemeinsame Steuerung/Queue, Lautstärkelimit, mpv-Adapter und gekennzeichnete Simulation.
- Optionale simulierte Tasten/RFID, systemd-Vorlage und Tests inklusive ARM64-Crossbuild.
- HTTP-Port für die aktive NextGen-Entwicklung auf `8090` vereinheitlicht.
- Startseite auf ein datengetriebenes Kategorie-/Reihenmodell umgestellt (`/api/home`).
- Kategorien und Reihen unterstützen lokalisierte Beschriftungen; Dev-Konfiguration enthält Hörbücher, Musik, Radio und Podcasts auf Deutsch/Englisch.
- TTS-Modell korrigiert: TTS ist eine globale Box-Einstellung mit Ein/Aus, Sprache und Provider; keine TTS-Konfiguration mehr pro Kategorie.
- `browser-dev` als klar begrenzter TTS-Entwicklungsfallback ergänzt; Produktions-TTS soll über austauschbare lokale/externe Provider erfolgen.
- Globale Power-Konfiguration mit `idle_shutdown_minutes` vorbereitet; kontrollierter Shutdown selbst ist noch nicht implementiert.
- SQLite als Zielpersistenz für Admin-Daten, Kategorien/Reihen, globale Einstellungen und spätere Statusdaten festgelegt; JSON bleibt Bootstrap/Übergang.
- Persistenz- und Migrationsarchitektur auf Deutsch/Englisch dokumentiert.
- Zielbild für das spätere Admin-Interface erweitert: Kategorien, Reihen, Übersetzungen, Provider, globale TTS-/Power-Einstellungen und weitere Box-Optionen.
- Projektdokumentation auf deutsch/englische Partnerdateien umgestellt.
- Player-Policy um 10-Sekunden-Sprünge für Hörspiele/Hörbücher und persistenten Resume auch für einzelne, viele Stunden lange Dateien erweitert.
- Zielkonfiguration für Start-/Shutdownsound, Start-/Maximallautstärke, Audioausgabe, Splashscreen, Display-Idle/Helligkeit, Adminpasswort und Themes dokumentiert.
- MuPiHat-/Batterieprofile einschließlich Custom-Profil, Lüfterstufen, Legacy-OnOff-Pins, Betriebs-LED sowie MQTT/Home Assistant als Hardware-/Systemadapter spezifiziert.
- WLAN-Scan/Verbindung per langem Druck auf das Netzwerksymbol und Best-Effort-Captive-Portal-Ablauf vorgesehen.
- Lokale Medienwurzeln und Spotify-Provider mit getrenntem Katalog/Playback sowie regelkonformem temporärem Metadaten-/Covercache spezifiziert.
- SQLite-Store mit Migration 1 implementiert: Einstellungen, Navigationsknoten/-übersetzungen und providerneutraler Wiedergabefortschritt.
- Erste Adminoberfläche unter `/admin/` mit persistenten Box-Einstellungen und visueller Kategorie-/Reihenpflege ergänzt.
- Player speichert lokalen Fortschritt periodisch und bei Pause/Seek/Wechsel/Shutdown und setzt auch einzelne sehr lange Dateien fort.
- Fortschrittsvertrag unterstützt lokale Medien, Spotify/Podcasts und spätere Videos; Live-Radio wird ausdrücklich nicht gespeichert.
- ARM64-Buildtest verwendet ebenfalls `-buildvcs=false`, damit Git-VCS-Stamping bei abweichendem LXC-Dateibesitz den Test nicht mehr blockiert.
- Bootstrap-JSON auf Listen-Adresse, SQLite-Pfad, lokalen Medien-Grundpfad und Audio-Backend reduziert; alle dynamischen Einstellungen und Inhalte kommen aus SQLite.
- SQLite-Migration 2 ergänzt Quelltyp und Quellenreferenz für Medieneinträge.
- Adminsprache ist getrennt von Box-/TTS-Sprache konfigurierbar; Deutsch und Englisch werden aus Sprachdateien geladen.
- Kategorien und Medieneinträge bearbeiten nur noch ein Feld „Name“ in der ausgewählten Inhaltssprache; vorhandene Übersetzungen bleiben erhalten.
- Medien lassen sich direkt unter Kategorien mit den Quellen Lokale Medien, Spotify, Amazon Music, Stream und Podcast konfigurieren. Noch nicht implementierte Provider werden nur gespeichert, nicht als abspielbar ausgegeben.
- Zahnrad und Simulationsdialog aus dem Player entfernt; die Administration öffnet nach fünf Sekunden Halten der Uhrzeit.
- Native Qt-Quick-Testoberfläche über EGLFS/KMS, eigener systemd-Dienst sowie DietPi-Installer, Bootstrap- und Displaydiagnose ergänzt.
- Generische Displayinstallation für das offizielle Raspberry-Pi-DSI-Display; Waveshare-Overlay nur über die explizite Option `--waveshare-5-dsi`.
- Vollständiges neues MuPiBox-Logo als 800×480-Startscreen rekonstruiert, mit Prüfsumme abgesichert und zugleich als Default-Cover eingebunden.
- Noch ausstehend: echte MuPiHat-/RFID-/GPIO-Hardwareadapter, Spotify-/Amazon-Music-/Radio-/Podcast-Integration, vollständige native Playerbedienung, produktiver TTS-Adapter, echter Idle-Shutdown und Rollback.
