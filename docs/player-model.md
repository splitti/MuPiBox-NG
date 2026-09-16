# Dynamisches Player- und Wiedergabeprofil

> Deutsch. English version: [player-model.en.md](player-model.en.md)

## Ziel

Die MuPiBox verwendet nicht für alle Medien dieselbe Player-Oberfläche. Stattdessen besitzt ein Navigationsknoten,
eine Inhaltsbindung oder ein Medienobjekt ein konfigurierbares Wiedergabeprofil. Dadurch kann ein Hörspiel sehr
reduziert dargestellt werden, während eine Musik-Playlist eine scrollbare Titelliste, Shuffle und Repeat anbietet.

Die Weboberfläche darf nicht aufgrund eines festen Medientyps wie `spotify`, `hörbuch` oder `playlist` entscheiden,
welche Bedienelemente sichtbar sind. Sie rendert die vom Backend gelieferte Player-Policy.

## Grundprofile

### Reduzierter Hörspiel-/Hörbuchmodus

Typische Darstellung auf 800x480:

- großes Cover,
- Titel bzw. Folge und optional Kapitel/Titelnummer,
- Fortschrittsbalken mit Position und Dauer,
- große Play/Pause-Taste,
- große Vor-/Zurück-Funktionen für die erlaubte Navigation,
- Lautstärke,
- Zurück zur vorherigen Ansicht.

In diesem Profil können Titelliste, Shuffle und Repeat vollständig ausgeblendet werden. Das ist für lineare
Hörspiele und Hörbücher der bevorzugte Standard.

### Musik-/Playlistmodus

Für Musik, Playlists oder andere frei auswählbare Titelsammlungen kann statt eines großen Covers eine scrollbare
Queue-/Titelliste angezeigt werden. Die aktuell laufende Zeile wird hervorgehoben. Ein Tipp auf eine Zeile startet
den entsprechenden Titel direkt.

Je nach Policy können zusätzlich erscheinen:

- Shuffle,
- Repeat,
- Repeat-One,
- Queue/Titelliste,
- direkte Titelauswahl,
- Cover als kleine Now-Playing-Kachel,
- Vor/Zurück.

## Player-Policy

Die konkrete Namensgebung wird bei der Implementierung festgelegt. Inhaltlich soll eine Policy mindestens folgende
Entscheidungen ausdrücken können:

- `show_queue` – Queue/Titelliste sichtbar,
- `allow_track_selection` – einzelne Titel direkt auswählbar,
- `allow_shuffle` – Shuffle verfügbar,
- `allow_repeat` – Repeat verfügbar,
- `allow_repeat_one` – einzelnen Titel wiederholen,
- `show_cover` – Cover sichtbar,
- `cover_size` bzw. Darstellungsprofil,
- `allow_previous` / `allow_next`,
- `allow_seek` – innerhalb des laufenden Mediums spulen,
- `allow_volume` – Lautstärkeregelung anzeigen,
- optional `previous_next_behavior` – Titelwechsel, Kapitelwechsel oder Zeit-Sprung.

Die Policy kann vom Navigationsknoten/Content-Binding vorgegeben und später bei Bedarf durch einen Provider sinnvoll
vorbelegt werden. Im Adminbereich kann splitti die erlaubten Funktionen überschreiben.

## Presets im Adminbereich

Damit nicht für jeden Eintrag viele Schalter einzeln gesetzt werden müssen, soll der Adminbereich sinnvolle Presets
anbieten, zum Beispiel:

- `Hörspiel / Hörbuch` – großes Cover, keine Queue, kein Shuffle/Repeat,
- `Musik / Playlist` – Queue, Titelauswahl, Shuffle und Repeat erlaubt,
- `Album` – Queue und Titelauswahl, Shuffle optional,
- `Radio` – minimales Now Playing, keine Queue/Seek wenn vom Stream nicht unterstützt,
- `Benutzerdefiniert` – alle Optionen einzeln konfigurierbar.

Presets sind nur Vorbelegungen. Gespeichert wird die resultierende Policy bzw. eine stabile Preset-Referenz mit
möglichen Overrides.

## Titelliste und Touch

Eine sichtbare Titelliste muss auf dem 800x480-Touchdisplay groß genug bedienbar bleiben. Jede Zeile zeigt mindestens
Titel und optional Untertitel/Interpret/Dauer. Der aktuell laufende Titel wird deutlich markiert.

Standardinteraktion:

- Tipp auf die Titelzeile: Titel direkt abspielen,
- optionaler Lautsprecher-/TTS-Button in der Zeile: Titel vorlesen,
- keine versteckte Hover-Funktion,
- kein Long-Press als einzige Möglichkeit für eine wichtige Funktion.

Damit bleibt `Antippen = Abspielen` eindeutig, während TTS bei aktivierter globaler TTS-Funktion separat erreichbar ist.
Auf sehr kleinen Layouts kann der TTS-Button nur eingeblendet werden, wenn TTS boxweit aktiviert ist.

## TTS in der Queue

TTS bleibt eine globale Box-Funktion mit global gewählter Sprache und Provider. Die Player-Policy kann lediglich
bestimmen, ob TTS-Aktionen für sichtbare Titel angeboten werden dürfen.

Der vorzulesende Titel stammt aus den normalisierten Medienmetadaten. Die Weboberfläche kennt keine Provider-
Sonderfälle. Bei Spotify, lokalen Dateien oder Podcasts wird derselbe TTS-Ablauf verwendet.

## Fortschritt und Queue

Playerstatus und Queue bleiben serverseitig die gemeinsame Wahrheit für Touchscreen, Browser, RFID und Tasten.
Die Listenansicht zeigt daher dieselbe Queue, die auch durch externe Eingaben verändert wird.

Für Hörbücher/Hörspiele soll später persistenter Fortschritt unterstützt werden. Das Wiedergabeprofil entscheidet nur,
welche Bedienfunktionen sichtbar/erlaubt sind; es entscheidet nicht über die technische Fortschrittsspeicherung.

## Zeitsprünge bei gesprochenen Inhalten

Für Hörspiele und Hörbücher ist der Standard für Zurück/Vor ein Zeitsprung von **10 Sekunden**. Die Player-Policy erhält dafür einen Modus wie `time_skip` und getrennte Werte für Rück- und Vorsprung. Dadurch können andere Profile weiterhin Titel- oder Kapitelwechsel verwenden.

Der Sprung wird immer gegen `0` und die bekannte Mediendauer begrenzt. Die Touchziele zeigen die Sprungweite eindeutig an, beispielsweise `−10` und `+10`, statt Symbole zu verwenden, die mit Titelwechsel verwechselt werden können.

## Persistentes Fortsetzen

Fortschritt wird pro stabilem, normalisiertem Medienobjekt gespeichert – unabhängig davon, ob ein Hörspiel aus 22 Titeln oder aus **einer einzigen 23-Stunden-Datei** besteht. Ein langer Einzeltrack darf nach Pause, Neustart oder Shutdown nicht wieder bei null beginnen.

Ein Fortschrittsdatensatz enthält mindestens:

- stabile Medien-ID einschließlich Provider-/Accountkontext,
- Position in Millisekunden als 64-Bit-Wert,
- bekannte Dauer in Millisekunden,
- Zeitpunkt der letzten Aktualisierung,
- beendet/nicht beendet,
- bei Sammlungen optional Queue-/Titelindex und Kontext-ID.

Gespeichert wird periodisch während der Wiedergabe sowie zwingend bei Pause, Seek, Titel-/Quellenwechsel, sauberem Shutdown und Prozessende. Die Schreibfrequenz wird begrenzt, um unnötige SQLite-Schreiblast zu vermeiden; nach einem harten Stromverlust darf höchstens ein kurzes, definiertes Intervall fehlen.

Beim erneuten Start eines nicht beendeten Mediums wird an der gespeicherten Position fortgesetzt. Nahe am Anfang wird kein sinnloser Resume angeboten; nahe am Ende kann ein Medium nach einer konfigurierbaren Regel als beendet gelten. Ein bewusst neu gestartetes Medium kann seinen Fortschritt zurücksetzen.

Fortschritt ist serverseitig und gilt deshalb identisch für Touch, Tasten, RFID, Browser und unterstützte Provider. Providerspezifische Fortschrittsinformationen können synchronisiert werden, die lokale MuPiBox-Datenbank bleibt aber die verlässliche gemeinsame Abstraktion.

## Resume-Policy nach Quelle

Fortschritt ist grundsätzlich für endliche Medien aktiviert: lokale Audioinhalte, Spotify-Titel/-Episoden, Podcasts und spätere Videodateien bzw. Videos. Live-Radio und andere nicht sinnvoll seekbare Live-Streams verwenden `resume_policy = none` und erzeugen keinen Fortschrittsdatensatz.

Spotify verwendet denselben lokalen Fortschrittsvertrag mit Provider-, Account- und Medien-ID. Eine spätere Synchronisation mit einem Provider ist zusätzlich möglich, ersetzt aber nicht den gemeinsamen MuPiBox-Zustand.

## Persistenz

Wiedergabeprofile werden später in SQLite gespeichert. Erwartete Konzepte sind beispielsweise:

- `playback_profiles`,
- Zuordnung eines Profils zu `navigation_nodes` oder `content_bindings`,
- optional providerseitige Defaultprofile,
- optionale Overrides pro Knoten/Inhalt.

Das Profil wird zusammen mit den aufgelösten Inhalten an das Frontend geliefert, damit die Weboberfläche vollständig
datengetrieben bleibt.
