# Verbindliche Projektvorgaben

> Deutsch. English version: [requirements.en.md](requirements.en.md)

Stand 16.09.2026, übernommen aus splittis Übergabe und den laufenden Projektentscheidungen. Anforderungen sind keine Behauptung, dass diese Funktionen bereits implementiert sind. Der Ist-Stand steht in README und CHANGELOG.

## Produkt und Plattform

MuPiBox-NG wird ein Musikplayer für Kinder/Familien mit Touch, Tasten, RFID und Heimnetz-Browser. Bedienarten müssen einzeln oder frei kombiniert funktionieren; alle steuern denselben Player, denselben Status und dieselbe Warteschlange. Die Anwendung darf ohne optionale Hardware nicht abstürzen.

Ziel: DietPi, ARM64/64 Bit, Raspberry Pi 3 als Mindestplattform. Pi 2 ist kein Ziel mehr. Entwicklung und automatisierte Tests laufen in einer Debian-13-LXC ohne Pi-Hardware. Testhardware: Pi 3, vermutlich Pi 4, ein MuPiHAT. Audio, GPIO, Stromversorgung und Display müssen später am Pi getestet werden.

## Hardware

MuPiHAT bevorzugen, andere Hardware über gekapselte Adapter ermöglichen. Benötigt: Akku und Laden, Ein/Aus und geordneter Shutdown, Audio mono/stereo. Vor Umsetzung prüfen: genaue Revision, Pinbelegung, Treiber, Shutdown-Sequenz und auslesbare Akku-/Ladeinformationen. Keine Akku-Prozentanzeige ohne nachgewiesene Telemetrie.

Hauptdisplay: Waveshare Raspberry Pi 5inch Capacitive 5-Points Touch Display, 800 × 480, DSI, Querformat. Andere Auflösungen weiter unterstützen. Das 5-Zoll-Display ist die primäre Designreferenz; Desktop- und Handyansichten sind sekundär.

## Oberfläche und Navigation

Die Bedienoberfläche soll sich visuell grob an Streaming-Oberflächen wie Netflix orientieren, ohne deren Gestaltung 1:1 zu kopieren. Ziel ist eine dunkle, klar strukturierte, auf sehr kleinen Touch-Displays gut bedienbare Oberfläche mit wenig Text und großen Touchzielen.

Oben befindet sich eine schmale, dauerhaft sichtbare Statusleiste ähnlich einer Android-Statusleiste. Dort sollen später unter anderem WLAN-Verbindung bzw. Signalstärke, Akku-/Ladezustand und weitere relevante Systeminformationen angezeigt werden. Eine herunterziehbare erweiterte Statusansicht ist eine spätere Ausbaustufe.

Der Hauptbereich besteht aus frei konfigurierbaren Kategorien, zum Beispiel Hörbücher, Musik, Radio und Podcasts. Kategorien sind keine fest im Frontend eingebauten Seiten. Sie werden als Daten vom Backend geliefert und vertikal untereinander dargestellt. Die Administration soll Kategorien anlegen, löschen, benennen, übersetzen, sortieren, aktivieren/deaktivieren und mit beliebigen Inhaltsreihen verknüpfen können.

Innerhalb einer Kategorie werden frei konfigurierbare Inhaltsreihen dargestellt. Darin können zum Beispiel Künstler, Alben, Playlists, Hörbücher, Radiosender oder Podcasts mit Cover/Vorschaubild erscheinen. Reihen sollen vorzugsweise horizontal scrollbar sein, während Kategorien vertikal gescrollt werden. Das Layout muss mit Touch auf 800 × 480 funktionieren, ohne auf Hover-Zustände angewiesen zu sein.

Die Navigation soll möglichst flach bleiben. Wichtige Wiedergabefunktionen müssen mit wenigen Berührungen erreichbar sein. Aktive Wiedergabe soll einen kompakten Player-/Now-Playing-Bereich mit Cover, Titel, Fortschritt und großen Bedienelementen erhalten.

## Boxweite TTS-Konfiguration

TTS wird **nicht pro Kategorie** konfiguriert, sondern einmal zentral für die jeweilige Box. Im Admin-Interface soll TTS aktiviert/deaktiviert, die Sprache gewählt und ein TTS-Provider ausgewählt bzw. eingerichtet werden können.

Wenn TTS aktiviert ist, kann das Antippen einer Kategorie ihren lokalisierten Kategorienamen vorlesen. Die gesprochene Sprache richtet sich nach der boxweiten TTS-Sprache. Kategorien speichern daher nur ihre lokalisierten Anzeigenamen; keine eigene Engine- oder Sprachkonfiguration.

TTS muss providerunabhängig abstrahiert werden. Geplant sind:

- mindestens eine lokale/offline nutzbare TTS-Lösung für Standalone-Betrieb,
- optional kostenlose bzw. kostenlos nutzbare externe TTS-Dienste, soweit verfügbar,
- dokumentierte Einrichtung für Provider, die API-Schlüssel oder weitere Zugangsdaten benötigen,
- austauschbare Provider ohne Änderungen an der Player-/Kategorieoberfläche.

Zugangsdaten für externe TTS-Dienste dürfen nicht in Git oder öffentlich lesbaren API-Antworten erscheinen. Browser-TTS darf ausschließlich als Entwicklungsfallback verwendet werden.

## Administration und dynamische Konfiguration

Die Weboberfläche soll weitgehend datengetrieben sein. Das spätere Admin-Interface ist die zentrale Pflegeoberfläche für Kategorien, Reihen, globale Box-Einstellungen und Quellen. Änderungen an Kategorien und Reihen dürfen keine Anpassung von HTML, CSS oder JavaScript erfordern.

Das Admin-Interface soll mindestens verwalten können:

- Kategorien: Anlegen, Löschen, Reihenfolge, Sichtbarkeit, mehrsprachige Namen,
- Inhaltsreihen: Anlegen, Löschen, Reihenfolge, Sichtbarkeit, Provider/Zielquelle,
- manuelle und dynamische Inhalte innerhalb einer Reihe,
- TTS global: Ein/Aus, Sprache, Provider, Provider-Konfiguration,
- Energie/Timer: Idle-Abschaltung und spätere Zeitpläne,
- Lautstärkelimit und weitere Box-Einstellungen,
- später RFID-/Tasten-Zuordnungen, Provider-Konten und weitere Geräteeinstellungen.

Details: [System-, Hardware- und Admin-Einstellungen](system-settings.md)

Dazu gehören Start-/Shutdownsound, Start- und Maximal-Lautstärke, Audioausgabe, Splashscreen, getrenntes Display-Idle, Helligkeit, Adminpasswort, Themes, MuPiHat-/Batterieprofile, Lüfter, Betriebs-LED, native Home-Assistant-Integration, Boxstatus und WLAN-Einrichtung einschließlich eines Best-Effort-Ablaufs für Captive Portals.

## Energie- und Timersteuerung

Die Box soll eine konfigurierbare Idle-Abschaltung erhalten. Beispiel: Wenn für eine eingestellte Zeit keine Wiedergabe läuft und keine relevante Benutzeraktivität stattfindet, darf die Box kontrolliert herunterfahren. `0` bzw. „Aus“ deaktiviert die Idle-Abschaltung.

Später soll die Energieverwaltung zusätzlich Zeitpläne ermöglichen können, etwa erlaubte Betriebszeiten, Nachtruhe oder einen geplanten Shutdown. Ein Shutdown muss immer kontrolliert erfolgen und darf Schreibvorgänge, Datenbankmigrationen oder Playerstatus nicht beschädigen.

## Wiedergabefortschritt

Gesprochene Inhalte verwenden standardmäßig 10-Sekunden-Sprünge. Wiedergabefortschritt wird auch für eine einzelne, viele Stunden lange Audiodatei persistent gespeichert und nach Pause, Neustart oder Shutdown fortgesetzt. Details: [Dynamisches Player- und Wiedergabeprofil](player-model.md).

## Persistenz

Für dynamisch im Admin-Interface gepflegte Daten wird SQLite als bevorzugte Persistenz festgelegt. Dazu gehören insbesondere Kategorien, Reihen, Box-Einstellungen, TTS-Konfiguration, Idle-Timer, Zuordnungen und später Fortschritt/Statusdaten.

JSON bleibt für wenige statische Bootstrap-/Deployment-Einstellungen geeignet, zum Beispiel Listen-Port, Datenbankpfad, Dev-Modus oder initiale Installationswerte. Die laufende Admin-Konfiguration soll nicht durch manuelles Umschreiben einer großen JSON-Datei verwaltet werden.

SQLite muss migrationsfähig verwendet werden. Schemaänderungen benötigen eine nachvollziehbare Versions- und Migrationsstrategie. Einstellungen und Nutzdaten bleiben außerhalb der Programmdatei und werden bei Updates erhalten.

Details: [persistence.md](persistence.md)

## Musikquellen

Verbindlich: lokale Musik nach Verzeichnisstruktur, ganze Ordner, hinterlegte Spotify-Alben, Künstler, Playlists und weitere Inhalte, Musikstreams/Webradio, Podcasts. Quellen austauschbar und später erweiterbar gestalten.

Spotify: Premium Family vorhanden, pro Box eigenes Konto und getrennte Anmeldung/Zugangsdaten. Auswahl, Anmeldung und Wiedergabe als vollständigen Ablauf prüfen. Zugangsdaten niemals ins Repository. Katalog/API, Playback und temporärer Cache werden getrennt; Metadaten/Cover dürfen nur zeitlich begrenzt und regelkonform zwischengespeichert werden, Audiodaten nicht als gewöhnlicher lokaler Cache.

Details: [Spotify-Integration und Cache](spotify.md)

Podcasts: RSS-Abonnements, Episodenliste, gespeicherter Fortschritt.
Amazon Music ist ein Zukunftsziel; technische/zulässige Integration noch nicht geprüft, keine zugesagte Wiedergabeunterstützung.

Einzelne lokale Videoclips und YouTube-Videos sind als spätere Ausbaustufe vorgesehen. Sie sollen dieselbe Navigation, Cover/Vorschaubilder, Player-Policy und Fortschrittsspeicherung verwenden. Die konkrete YouTube-Integration sowie rechtliche/API-seitige Bedingungen werden erst in diesem Folgeschritt festgelegt.

## Eingabemodule

Betriebsarten: nur Touch; Tasten + RFID ohne Display; jede andere Kombination mit Browser. Beispiel: Karte startet Album, Display zeigt Cover, Taste schaltet weiter, Handy pausiert.

Tasten: Play/Pause, Vor/Zurück, Lautstärke, konfigurierbare Belegung. RFID: Karten auf Ordner, Spotify-Inhalt, Radio oder Podcast abbilden; starten/fortsetzen. Beim Entfernen optional pausieren, nur mit Leser, der Kartenanwesenheit zuverlässig meldet. Simulierte Tasten/RFID für Entwicklung erforderlich.

## Software und Daten

Go-Backend als systemd-Dienst. Gemeinsamer Player/Queue, separate Musikquellen, unabhängig aktivierbare Bedienmodule, gekapselte Hardware, responsive Weboberfläche. M1-Entscheidung: eingebettetes HTML/CSS/JS, mpv-Adapter plus Testsimulation; keine Vorentscheidung für Spotify oder spätere Quellen.

Kategorien und Inhaltszuordnungen werden als Datenmodell behandelt, nicht im Frontend verdrahtet. Das Frontend rendert die vom Backend gelieferten Kategorien, Reihen und Medienobjekte. Damit können lokale Musik, Spotify, Radio und Podcasts später in derselben Navigationsstruktur erscheinen.

## Repository und Versionierung

Repository: https://github.com/splitti/MuPiBox-NG
Referenz: https://github.com/splitti/MuPiBox
Installerreferenz: https://raw.githubusercontent.com/splitti/MuPiBox/main/autosetup/autosetup.sh
Alter Stack ist Referenz, keine Pflicht.

Historie erhalten; keine ungeprüfte Löschung alten Codes. Nachvollziehbare Commits, Branches, markierte Releases, sichtbare Version und Änderungsübersicht; später Update mit Rückkehr zur vorherigen Version unter Erhalt von Einstellungen und Musik.

Projekt- und GitHub-Dokumentation wird auf Deutsch und Englisch gepflegt. Die deutsche und englische Fassung müssen dieselben verbindlichen Entscheidungen wiedergeben.

## Meilensteine

1. Go-Dienst, 800×480-Oberfläche, lokale Ordnerbibliothek, lokale Wiedergabe, gemeinsamer Status.
2. Datengetriebene 800×480-Navigation mit Statusleiste, frei konfigurierbaren Kategorien und bildorientierten Inhaltsreihen.
3. SQLite-Persistenz und Admin-Interface für Kategorien, Reihen und globale Box-Einstellungen.
4. Globale TTS-Abstraktion mit lokalem Offline-Provider und optionalen externen Providern.
5. Idle-/Timersteuerung und robuste Energieverwaltung.
6. Persistenter Fortschritt, robuste Fehler-/Wiederanlaufbehandlung und Webradio.
7. RSS-Podcasts und Fortsetzen.
8. Spotify als vollständiger geprüfter Ablauf je Box.
9. Hardwareadapter/Einrichtung, Pi-/MuPiHAT-Tests, Kiosk, Akku-/WLAN-Status und sichere Stromabschaltung.
10. Releases, Update/Rollback; Prüfung weiterer Plattformen wie Amazon.

Die Reihenfolge der Folgemeilensteine kann bei technischen Erkenntnissen angepasst werden.
