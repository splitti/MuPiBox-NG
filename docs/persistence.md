# Persistenz

> Deutsch. English version: [persistence.en.md](persistence.en.md)

## Entscheidung

Dynamische MuPiBox-Daten werden in SQLite gespeichert. JSON bleibt auf eine kleine Bootstrap-/Deployment-Konfiguration begrenzt.

Gründe für SQLite:

- atomare Änderungen und Transaktionen,
- robuste gleichzeitige Lese-/Schreibzugriffe innerhalb des Go-Dienstes,
- einfache Sortierung und Filterung für Admin-Oberfläche und Player,
- klare Schema-Versionierung und Migrationen,
- eine einzelne lokale Datei ohne zusätzlichen Datenbankserver,
- Backup/Restore ist einfach,
- sehr kleine Datenmengen bei Navigation und Einstellungen.

Eine große zur Laufzeit umgeschriebene JSON-Konfiguration wird ausdrücklich nicht als langfristige Admin-Persistenz verwendet.

## Dateiaufteilung

Vorgesehene Pfade im Dienstbetrieb:

- `/etc/mupibox-ng/config.json` – Bootstrap/Deployment,
- `/var/lib/mupibox-ng/mupibox.db` – SQLite-Datenbank,
- `/srv/mupibox/music` – lokale Medien,
- weitere Secrets später unter restriktiven Rechten außerhalb von Git und öffentlichen APIs.

Die Bootstrap-Konfiguration enthält aktuell ausschließlich vier Startwerte: Listen-Adresse, Datenbankpfad, lokaler Medien-Grundpfad und Audio-Backend. Lautstärke, TTS, Power, Navigation, Medienzuordnungen und Adminsprache werden ausschließlich aus SQLite geladen. Bei einer leeren Datenbank legt der Dienst definierte Initialdaten direkt in SQLite an; die JSON-Datei ist keine zweite Konfigurationsquelle.

## Geplante Datenbereiche

### Globale Box-Einstellungen

- UI-/Box-Sprache,
- maximale Lautstärke,
- TTS aktiviert/deaktiviert,
- TTS-Sprache,
- TTS-Provider und nicht geheime Provideroptionen,
- Idle-Abschaltung in Minuten,
- spätere Zeitpläne und weitere Geräteoptionen.

### Navigation und Inhalte

Die Navigation wird nicht dauerhaft als starre `Kategorie -> Reihe`-Struktur modelliert, sondern als frei konfigurierbarer Baum. Ein Navigationsknoten kann ein reiner Container sein oder direkt eine Inhaltsquelle binden.

Gespeichert werden mindestens:

- stabile Knoten-ID,
- optionale Parent-ID,
- Sortierposition,
- sichtbar/aktiv,
- lokalisierte Namen,
- Auswahlverhalten (`inline`, `drilldown`, später optional `play`),
- Darstellungsmodus,
- optionale Providerbindung,
- Provider, Quelltyp und Providerreferenz,
- gewünschter Inhaltsmodus (`auto`, `children`, `albums`, `tracks`, `items`),
- optionale manuell gepflegte Inhalte.

Damit kann beispielsweise `Die drei ??? Kids` entweder unter `Hörbücher` liegen und beim Antippen eine Albumansicht öffnen oder selbst als Hauptkategorie direkt Spotify-Alben bzw. Titel anzeigen.

Details: [Dynamisches Navigations- und Inhaltsmodell](navigation-model.md).

Lokalisierte Texte werden nicht als feste `de`-/`en`-Spalten modelliert, damit später weitere Sprachen ohne Schemaänderung ergänzt werden können.

### Wiedergabefortschritt

Fortschritt wird unter einer stabilen, providerneutralen Medien-ID gespeichert. Position und Dauer werden als 64-Bit-Millisekundenwerte abgelegt, damit auch einzelne sehr lange Dateien sicher unterstützt werden. Weitere Felder sind Aktualisierungszeit, Beendet-Status sowie optional Kontext-/Queue-ID und Titelindex. Schreibvorgänge erfolgen begrenzt periodisch und bei allen relevanten Zustandswechseln.

Erwartetes Kernobjekt: `playback_progress`. Ein Fortschrittsdatensatz darf nicht an einen Dateinamen oder eine vergängliche Queue-Position allein gebunden sein.

### Spätere Daten

- RFID-/Tasten-Zuordnungen,
- Podcast-Abonnements und Fortschritt,
- Hörbuchfortschritt,
- Provider-/Account-Metadaten,
- Player-/Queue-Wiederherstellung,
- Admin-/Geräteeinstellungen,
- Batterieprofile, Lüfter-/LED-/Displayeinstellungen,
- Providerkonten und zeitlich begrenzter Providercache.

Geheimnisse wie API-Schlüssel oder Refresh-Tokens werden nicht unverschlüsselt über öffentliche APIs ausgegeben. Die genaue Secret-Ablage wird getrennt festgelegt.

## Schema-Grundidee

Die konkreten Tabellen werden migrationsbasiert definiert. Erwartete Kernobjekte:

- `schema_migrations`
- `settings`
- `navigation_nodes`
- `navigation_node_labels`
- `content_bindings`
- optionale manuelle Items/Zuordnungen
- `playback_progress`
- `battery_profiles`
- `provider_accounts` und `provider_cache`

Ein möglicher `content_bindings`-Datensatz verweist providerneutral auf beispielsweise einen Spotify-Künstler, eine Playlist, ein Album, einen lokalen Ordner, einen RSS-Feed oder einen Radiosender.

IDs sind stabil und unabhängig von Anzeigenamen. Reihenfolge wird explizit gespeichert und nicht aus Namen oder Erstellungszeit abgeleitet.

Die momentan im Entwicklungsprototyp vorhandenen `categories`/`content_rows` sind ausdrücklich ein Zwischenschritt und kein festgeschriebenes Persistenzschema.

## Migrationen

Jede Schemaänderung bekommt eine eindeutige, monotone Migration. Beim Start wird die Datenbankversion geprüft und ausstehende Migrationen werden in definierter Reihenfolge ausgeführt.

Vor migrationskritischen Releases muss Backup/Rollback berücksichtigt werden. Ein Binär-Rollback darf nicht stillschweigend eine neuere, inkompatible Datenbank beschädigen.

## SQLite-Konfiguration

Die Anwendung verwendet genau eine lokale Datenbankdatei. Fremdschlüssel werden aktiviert. Schreiboperationen sollen kurz bleiben und über Transaktionen erfolgen. WAL kann nach Messung auf der Zielhardware aktiviert werden, ist aber keine Voraussetzung für das Datenmodell.

Das MVP verwendet aktuell `github.com/mattn/go-sqlite3`. Dieser Treiber benötigt CGO für einen funktionsfähigen Datenbankbetrieb; der Dienst wird deshalb auf der Zielplattform nativ mit aktivem CGO gebaut. Die Auswahl bleibt bis zum Pi-3-Test vorläufig.

Der Go-Treiber wird erst endgültig ausgewählt, nachdem folgende Punkte auf Pi 3/ARM64 geprüft wurden:

- `CGO_ENABLED=0`-/Crossbuild-Kompatibilität oder vertretbarer CGO-Buildweg,
- Binärgröße und Speicherverbrauch,
- Start- und Migrationszeit,
- Zuverlässigkeit auf DietPi,
- Lizenz und Wartungszustand.

Die Store-Schicht verwendet möglichst `database/sql` bzw. eine schmale interne Schnittstelle, damit der konkrete Treiber nicht in UI-, Player- oder Providerlogik durchsickert.

## Backup

Die Datenbank enthält Benutzereinstellungen und muss updatefest behandelt werden. Geplant sind später:

- konsistentes lokales Backup,
- Import/Restore über Admin,
- optional Export einer menschenlesbaren Konfiguration für Diagnose/Migration,
- keine Medienkopie in der Konfigurationsdatenbank.
