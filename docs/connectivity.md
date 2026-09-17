# WLAN und Bluetooth

> **Deutsch** · [English](connectivity.en.md)

## WLAN

Die native Touchoberfläche öffnet nach ungefähr einer Sekunde Halten des WLAN-Symbols eine WLAN-Verwaltung. Sie zeigt gefundene SSIDs mit kindgerechten Empfangsbalken, Sicherheitsart und bestehender Verbindung. Nach Auswahl kann das Passwort über die native Qt-Bildschirmtastatur eingegeben werden.

Der Go-Dienst verwendet die auf dem System vorhandene Verwaltung:

1. NetworkManager über `nmcli`, falls vorhanden,
2. sonst DietPi/`wpa_supplicant` über `wpa_cli`.

Der Installer ersetzt oder konvertiert die aktive DietPi-Netzwerkkonfiguration ausdrücklich nicht. Er installiert lediglich `wpasupplicant` und nimmt den Dienstbenutzer in die vorhandene Gruppe `netdev` auf. WLAN-Passwörter werden nicht in SQLite, der MuPiBox-Konfiguration oder MuPiBox-Logs gespeichert. Die dauerhafte Speicherung übernimmt ausschließlich der aktive Netzwerkmanager.

API:

- `GET /api/connectivity/wifi` – Netze suchen,
- `POST /api/connectivity/wifi/connect` – ausgewähltes Netz verbinden.

Die Admin-Weboberfläche bietet dieselbe Scan-/Verbindungsfunktion als Rückfallweg.

Der Backend-Dienst verwendet bewusst kein isoliertes `PrivateTmp`: `wpa_cli` erzeugt seinen lokalen Antwort-Socket unter `/tmp`, und der außerhalb der Unit laufende `wpa_supplicant` muss diesen Pfad erreichen können. Die übrigen systemd-Schutzoptionen bleiben aktiv.

Sind mehrere WLAN-Adapter vorhanden, kann jeder Adapter für MuPiBox freigegeben oder ausgeschlossen und ein bevorzugter Adapter bestimmt werden. Die automatische Suche nutzt den bevorzugten einsatzbereiten Adapter; ist dieser nicht verfügbar, wird auf einen anderen freigegebenen Adapter mit aktivem Link bzw. `wpa_supplicant`-Steuerkanal zurückgefallen. Vorhandene, aber nicht verwaltete Adapter werden angezeigt und nicht ungefragt gescannt.

### Hotel-WLAN / Captive Portal

Diese erste Stufe verbindet offene und WPA/WPA2-Personal-Netze. Ein Hotel-WLAN kann anschließend zwar verbunden sein, seine Anmeldeseite wird aber noch nicht in der Box geöffnet.

Die nächste Stufe erkennt eine Umleitung auf ein Captive Portal und öffnet eine zeitlich begrenzte, stark eingeschränkte Browseransicht. Cookies werden nur für die jeweilige Sitzung behalten. Wegen des Raspberry Pi mit 1 GB RAM wird vorab auf dem Testgerät entschieden, ob Qt WebEngine tragbar ist oder ein schlanker separater Browserprozess verwendet wird. Captive Portals bleiben Best-Effort, weil Formulare, Voucher, SMS-Anmeldung und Zertifikatsfehler je Anbieter unterschiedlich sind.

## Bluetooth

Bluetooth ist standardmäßig ausgeschaltet und wird als globale SQLite-Einstellung verwaltet. Im Adminbereich kann es aktiviert oder deaktiviert werden. Bei aktiviertem Bluetooth sind verfügbar:

- Geräte suchen,
- koppeln und als vertrauenswürdig markieren,
- verbinden und trennen,
- gespeicherte Geräte entfernen.

Der Adapter verwendet BlueZ über `bluetoothctl`. Der Installer ergänzt `bluez`, `rfkill` und – sofern vorhanden – die Gruppe `bluetooth`.

Kopplungen mit „Just Works“ funktionieren in dieser ersten Stufe. Geräte, die einen PIN-/Passkey-Dialog oder eine Bestätigung auf beiden Seiten verlangen, erhalten später einen eigenen Agent-Dialog auf dem Touchdisplay.

## Medientasten

Play/Pause, Weiter und Zurück von Bluetooth-Geräten sollen dieselben Befehle wie Touch, Tasten und RFID auslösen. Dafür ist ein BlueZ-MediaPlayer-Ereignisadapter vorgesehen. Die Scan-/Pairing-Stufe legt die Schnittstelle dafür an, wertet AVRCP-Ereignisse aber noch nicht aus. Die Zuordnung wird erst mit realen Kopfhörern/Lautsprechern getestet, damit keine gerätespezifischen Annahmen fest eingebaut werden.

## Sicherheit

Ein Admin-Passwort kann im Bereich „Sicherheit“ gesetzt und geändert werden. Es wird nur als gesalzener PBKDF2-SHA-256-Hash gespeichert. Danach schützen eine serverseitige Sitzung, Abmeldung und ein Rate-Limit für Fehlversuche die Admin- und extern aufgerufenen Konnektivitäts-Endpunkte. Die lokale Touchoberfläche darf weiterhin über Loopback WLAN einrichten, damit sich die Box nicht selbst aussperrt.
