# Verbindliche Projektvorgaben

Stand 16.09.2026, übernommen aus Ollis Übergabe. Anforderungen sind keine Behauptung,
dass diese Funktionen bereits implementiert sind. Der Ist-Stand steht in README und CHANGELOG.

## Produkt und Plattform

MuPiBox-NG wird ein Musikplayer für Kinder/Familien mit Touch, Tasten, RFID und Heimnetz-Browser.
Bedienarten müssen einzeln oder frei kombiniert funktionieren; alle steuern denselben Player,
denselben Status und dieselbe Warteschlange. Die Anwendung darf ohne optionale Hardware nicht abstürzen.

Ziel: DietPi, ARM64/64 Bit, Raspberry Pi 3 als Mindestplattform. Pi 2 ist kein Ziel mehr.
Entwicklung und automatisierte Tests in Debian-13-LXC ohne Pi-Hardware.
Testhardware: Pi 3, vermutlich Pi 4, ein MuPiHAT. Revision noch offen.
Audio, GPIO, Stromversorgung und Display müssen später am Pi getestet werden.

## Hardware

MuPiHAT bevorzugen, andere Hardware über gekapselte Adapter ermöglichen.
Benötigt: Akku und Laden, Ein/Aus und geordneter Shutdown, Audio mono/stereo.
Hersteller: https://mupihat.de/ — MuPiHAT+ wird als Verstärker/Stromversorgung/USV/Lader/Schalter beschrieben.
Vor Umsetzung prüfen: genaue Revision, Pinbelegung, Treiber, Shutdown-Sequenz,
auslesbare Akku-/Ladeinformationen. Keine Akku-Prozentanzeige ohne nachgewiesene Telemetrie.

Hauptdisplay: Waveshare Raspberry Pi 5inch Capacitive 5-Points Touch Display,
800 × 480, DSI, Low Power Consumption, Querformat. Andere Auflösungen weiter unterstützen.
Große Cover und Touchziele; responsive Zugriff per Handy/PC im Heimnetz.
Noch kein verbindlich abgestimmtes fertiges Design.

Sinnvolle Ausgangswerte: einfache kindgerechte Bedienung, separater Verwaltungsbereich,
maximale Lautstärke, Hörbuch-/Podcast-Fortschritt. Details der Verwaltung sind noch offen.

## Musikquellen

Verbindlich: lokale Musik nach Verzeichnisstruktur, ganze Ordner, hinterlegte Spotify-Alben,
Künstler, Playlists und weitere Inhalte, Musikstreams/Webradio, Podcasts.
Quellen austauschbar und später erweiterbar gestalten.

Spotify: Premium Family vorhanden, pro Box eigenes Konto und getrennte Anmeldung/Zugangsdaten.
Auswahl, Anmeldung und Wiedergabe als vollständigen Ablauf prüfen. librespot ist Kandidat, keine
bereits endgültig bestätigte Lösung. Zugangsdaten niemals ins Repository.

Podcasts: RSS-Abonnements, Episodenliste, gespeicherter Fortschritt.
Amazon Music ist ein Zukunftsziel; technische/zulässige Integration noch nicht geprüft,
keine zugesagte Wiedergabeunterstützung.

## Eingabemodule

Betriebsarten: nur Touch; Tasten + RFID ohne Display; jede andere Kombination mit Browser.
Beispiel: Karte startet Album, Display zeigt Cover, Taste schaltet weiter, Handy pausiert.
Tasten: Play/Pause, Vor/Zurück, Lautstärke, konfigurierbare Belegung.
RFID: Karten auf Ordner, Spotify-Inhalt, Radio oder Podcast abbilden; starten/fortsetzen.
Beim Entfernen optional pausieren, nur mit Leser, der Kartenanwesenheit zuverlässig meldet.
Leser noch nicht festgelegt. Einrichtung soll vorhandene Module auswählen lassen.
Simulierte Tasten/RFID für Entwicklung erforderlich.

## Software und Daten

Go-Backend als systemd-Dienst. Gemeinsamer Player/Queue, separate Musikquellen,
unabhängig aktivierbare Bedienmodule, gekapselte Hardware, responsive Weboberfläche.
Frontend und Audio-Backend waren in der Übergabe nicht festgelegt.
M1-Entscheidung: eingebettetes HTML/CSS/JS, mpv-Adapter plus Testsimulation;
keine Vorentscheidung für Spotify oder spätere Quellen.
Einstellungen, Nutzdaten, Musik und Zugangsdaten von Programmdateien trennen.

## Repository und Versionierung

Repository: https://github.com/splitti/MuPiBox-NG
Referenz: https://github.com/splitti/MuPiBox
Installerreferenz: https://raw.githubusercontent.com/splitti/MuPiBox/main/autosetup/autosetup.sh
Alter Stack (Node/PM2, Python, MPlayer, librespot) ist Referenz, keine Pflicht.

Erst prüfen, dann auf eigenem Branch neu entwickeln. Historie erhalten;
keine ungeprüfte Löschung alten Codes. Nachvollziehbare Commits, Branches, markierte Releases,
sichtbare Version, Änderungsübersicht; später Update mit Rückkehr zur vorherigen Version,
unter Erhalt von Einstellungen und Musik.

## Meilensteine

1. Go-Dienst, 800×480-Oberfläche, lokale Ordnerbibliothek, lokale Wiedergabe, gemeinsamer Status.
2. Persistenter Fortschritt und robuste Fehler-/Wiederanlaufbehandlung; Webradio.
3. RSS-Podcasts und Fortsetzen.
4. Spotify als vollständiger geprüfter Ablauf je Box.
5. Hardwareadapter/Einrichtung, Pi-/MuPiHAT-Tests, Kiosk und sichere Stromabschaltung.
6. Releases, Update/Rollback; Prüfung weiterer Plattformen wie Amazon.

Reihenfolge der Folgemeilensteine kann bei technischen Erkenntnissen angepasst werden.
