# System-, Hardware- und Admin-Einstellungen

> Deutsch. English version: [system-settings.en.md](system-settings.en.md)

## Ziel und Persistenz

Die Admin-Weboberfläche unter `/admin` ist die zentrale Konfiguration einer Box. Dynamische Einstellungen werden in SQLite gespeichert; die Bootstrap-JSON enthält nur Werte, die vor dem Öffnen der Datenbank benötigt werden. Hardwarezugriffe laufen über Adapter und müssen in der Entwicklungs-LXC vollständig simulierbar sein.

Einstellungen werden nach Bereichen gruppiert, validiert und möglichst ohne Neustart übernommen. Änderungen, die einen Neustart erfordern, werden deutlich markiert. Geheimnisse werden weder in Git noch in öffentlichen Status-APIs ausgegeben.

## Audio und Wiedergabe

Konfigurierbar sind:

- Startsound ein/aus sowie eine mitgelieferte oder hochgeladene Audiodatei,
- Shutdownsound ein/aus sowie eine mitgelieferte oder hochgeladene Audiodatei,
- Einschaltlautstärke,
- maximale Lautstärke als harte Obergrenze für Touch, Tasten, Home Assistant und Provider,
- Audioausgabegerät,
- optional Mono/Stereo und weitere vom Adapter angebotene Audiooptionen.

Der Shutdownsound muss vor dem kontrollierten Herunterfahren mit einem festen Timeout abgespielt werden, damit ein defektes oder nicht erreichbares Audiogerät den Shutdown nicht blockiert.

Die Einrichtung soll vorhandene DietPi-/ALSA-Audiokonfiguration erkennen und als Vorauswahl übernehmen. Ein explizit gewähltes Gerät wird gespeichert. Ist es beim Start nicht verfügbar, fällt die Box kontrolliert auf ein gültiges Gerät oder einen klar gekennzeichneten Fehlerzustand zurück; sie darf nicht still auf einem falschen Ausgang spielen.

## Display und Startvorgang

Konfigurierbar sind:

- Splashscreen ein/aus,
- mitgeliefertes Motiv oder eigenes hochgeladenes Bild,
- Displayhelligkeit,
- Display nach einer getrennt konfigurierbaren Idle-Zeit ausschalten,
- Display bei Touch, Taste, RFID oder neu gestarteter Wiedergabe wieder einschalten.

`Display idle off` und `Box idle shutdown` sind zwei unabhängige Timer. Display aus darf laufende Wiedergabe nicht stoppen. Die Helligkeitssteuerung wird als Displayadapter umgesetzt, weil nicht jedes Display dieselbe Backlight-Schnittstelle anbietet.

## Admin-Zugang

Die Adminoberfläche startet zunächst ohne Passwort und weist gut sichtbar auf den ungeschützten Zugang hin. Ein Adminpasswort kann eingerichtet und geändert werden; danach erscheint vor der Verwaltung eine Anmeldeseite.

Passwörter werden ausschließlich als gesalzener PBKDF2-SHA-256-Hash gespeichert. Nach Aktivierung schützen serverseitige Admin-Sitzungen, Logout, Rate-Limit für Anmeldeversuche und die vorhandene Origin-Prüfung die Verwaltung. Die Kinder-/Playeroberfläche bleibt vom Adminzugang getrennt.

## Statusübersicht

Das Admin-Dashboard zeigt, soweit der jeweilige Adapter echte Daten liefert:

- Akku-Ladestand, Ladezustand, Spannung und Warnzustand,
- WLAN-SSID, Signalstärke, IP-Adresse und Internetstatus,
- Audioausgabegerät und Playerstatus,
- Displayzustand und Helligkeit,
- CPU-Temperatur, Lüfterstufe, Laufzeit und Speicherplatz,
- Version, Backend-/Hardwareprofil und letzte Fehler,
- Home-Assistant- und Providerstatus.

Unbekannte Werte werden als unbekannt dargestellt und niemals als `0 %` oder als scheinbar fehlerfreier Zustand ausgegeben.

## MuPiHat und Batterieprofile

Batterieprofile sind Daten und werden in SQLite gespeichert. Spannungen werden intern als Integer in Millivolt gespeichert. Profile müssen im Adminbereich angelegt, kopiert, bearbeitet, validiert und ausgewählt werden können.

Mitgelieferte Ausgangsprofile:

| Profil | 100 % | 75 % | 50 % | 25 % | 0 % | Warnung | Shutdown |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Ansmann 2S1P | 8100 | 7800 | 7400 | 7000 | 6700 | 7000 | 6800 |
| ENERpower 2S2P 10.000mAh | 8000 | 7700 | 7300 | 6900 | 6000 | 6500 | 6150 |
| USB-C mode (no battery) | 1 | 1 | 1 | 1 | 1 | 0 | 0 |
| Custom | 8100 | 7800 | 7400 | 7000 | 6700 | 7000 | 6800 |

Bei Batterieprofilen muss `v_100 > v_75 > v_50 > v_25 > v_0` gelten. Die Prozentanzeige wird zwischen den Stützpunkten interpoliert und geglättet, damit Messrauschen nicht zu springenden Anzeigen führt. Warn- und Shutdownschwellen besitzen Hysterese und eine Mindestdauer, damit ein kurzer Spannungseinbruch keinen sofortigen Shutdown auslöst. Im USB-C-Modus sind Akkuwarnung und akkuspannungsbasierter Shutdown deaktiviert.

Die konkrete MuPiHat-Revision, Messquelle, Ladeerkennung und sichere Ausschaltsequenz müssen auf echter Hardware bestätigt werden.

## CPU-Lüfter

Der Lüfter ist optional und wird über einen Hardwareadapter gesteuert. Ausgangskonfiguration:

- aktiv/inaktiv,
- GPIO, bisheriger Standard `12`,
- 25 % ab `45 °C`,
- 50 % ab `55 °C`,
- 75 % ab `65 °C`,
- 100 % ab `75 °C`.

Grenzen und GPIO sind im Adminbereich editierbar. Temperaturen müssen streng aufsteigend validiert werden. Eine konfigurierbare Hysterese verhindert schnelles Hoch-/Herunterschalten. Fällt die Temperaturmessung aus, wird ein sicherer Zustand verwendet.

## Ein/Aus und Betriebs-LED

Hardwarefunktionen werden als auswählbares Profil modelliert. Das historische OnOff-SHIM-Profil enthält zunächst:

- `poweroffPin = 4`,
- `triggerPin = 17`,
- `cutPin = 27`,
- `ledPin = 13`,
- `ledBrightnessMax = 100`,
- `ledBrightnessMin = 10`.

Diese Werte sind Legacy-Ausgangswerte und dürfen nicht ungeprüft als MuPiHat-Pinbelegung verwendet werden. Ein MuPiHat-Profil wird nach Prüfung der vorhandenen Revision ergänzt.

Die Betriebs-LED kann im aktiven Zustand mit `ledBrightnessMax` und bei ausgeschaltetem Idle-Display mit `ledBrightnessMin` betrieben werden. Beim Shutdown wird sie über die sichere Hardwaresequenz ausgeschaltet. Min/Max sind im Bereich 0–100 zu validieren. In der LXC liefert der Adapter nur simulierten Status.

## Native Home-Assistant-Integration

Für Home Assistant ist eine native MuPiBox-Integration das bevorzugte Ziel. Sie fragt die versionierte MuPiBox-API über einen eigenen, eingeschränkten API-Token ab und bildet Player, Lautstärke, Akku, WLAN und Boxstatus als Home-Assistant-Entitäten ab. Ein `DataUpdateCoordinator` kann bei aktiver Wiedergabe häufiger und im Idle-Zustand seltener aktualisieren. Neustart und Shutdown werden nicht ohne zusätzliche Freigabe als Automationsaktion veröffentlicht.

MQTT ist nicht mehr Teil des Zielmodells. Die Integration benötigt stattdessen ein langlebiges, widerrufbares und auf die API beschränktes Token; das Admin-Cookie wird dafür niemals wiederverwendet. Schreibende Dienste respektieren insbesondere maximale Lautstärke und sichere Shutdownregeln.

## WLAN-Einrichtung und Captive Portals

Ein langer Druck auf das Netzwerksymbol kann die WLAN-Einrichtung öffnen. Sie bietet:

1. WLAN-Suche mit Signalstärke und Sicherheitsart,
2. Auswahl eines Netzes,
3. Passworteingabe, falls erforderlich,
4. Verbindungsstatus und verständliche Fehlermeldung,
5. Vergessen bzw. Wechseln gespeicherter Netze.

Die Implementierung nutzt einen Netzwerkadapter über die auf DietPi tatsächlich vorhandene Verwaltungsschicht; UI und Backend rufen nicht direkt beliebige Shellbefehle auf. Berechtigungen werden auf die benötigten WLAN-Aktionen begrenzt.

Für Hotel-/Gast-WLANs wird ein Captive-Portal-Ablauf vorgesehen: Die Box prüft nach der WLAN-Verbindung die Internetkonnektivität, erkennt eine mögliche Umleitung und kann eine zeitlich begrenzte Browseransicht zum Bestätigen der Nutzungsbedingungen öffnen. Captive Portals sind sehr unterschiedlich; deshalb wird dies als Best-Effort-Funktion mit klarer Abbruch-/Zurück-Navigation umgesetzt, nicht als garantierte vollautomatische Anmeldung.

## Themes und eigenes Look & Feel

Mitgelieferte Themes sollen mindestens umfassen:

- moderner dunkler Streaming-Look,
- Arcade-/8-Bit-Look,
- klassisch reduziertes Theme.

Ein Theme besteht aus kontrollierten Design-Tokens für Farben, Schriften, Abstände, Radien, Hintergrund, Akzent, Coverdarstellung und optional eigene Logos/Hintergründe. Der Adminbereich bietet Vorschau auf 800×480, Aktivierung sowie Import/Export eines benutzerdefinierten Themes. Freies CSS wird zunächst nicht ungeprüft injiziert; erweiterte Anpassung kann später über eine bewusst abgesicherte Expertenfunktion erfolgen.

## Lokale Medien

Ein oder mehrere lokale Medienpfade können im Adminbereich eingerichtet werden. Ein Pfad kann als Quelle an beliebige Navigationsknoten gebunden werden, beispielsweise als Kategorie, Künstler-/Serienordner, Album oder Playlist.

Der Scanner arbeitet außerhalb der SQLite-Datei, speichert nur Index/Metadaten und verändert die Mediendateien nicht. Er unterstützt erneutes Einlesen, stabile Medien-IDs, Cover/Metadaten und verständliche Meldungen bei nicht erreichbaren Pfaden. Pfadzugriffe werden auf explizit erlaubte Medienwurzeln begrenzt.

## Admin-Struktur

Vorgesehene Bereiche:

- Übersicht,
- Inhalte und Navigation,
- Wiedergabeprofile,
- lokale Medien,
- Spotify und weitere Provider,
- Audio,
- Display und Themes,
- TTS,
- Netzwerk,
- MuPiHat, Batterie, Lüfter und LED,
- Energie und Timer,
- Home Assistant,
- Sicherheit,
- Backup, Update und Diagnose.
