# Adminbereich und Best Practices

Der Adminbereich trennt Einstellungen nach Verantwortung. Jede sicherheits- oder systemrelevante Option nennt Empfehlung, Auswirkung und mögliche Unterbrechungen direkt an der Einstellung.

## Bereiche

- **Box:** Sprache, Theme, Audio, Display, TTS und Admin-Passwort. Ein Passwort mit mindestens zehn Zeichen wird empfohlen.
- **Inhalte:** Kategorien, lokale Verzeichnisse, Resume-Listen und spätere Online-Provider. Änderungen werden vom Player ohne Neustart neu geladen.
- **Netzwerk:** genau einen WLAN-Adapter anhand seiner stabilen MAC-Adresse auswählen, optional Onboard-WLAN beim Boot deaktivieren, WLAN scannen, Bluetooth koppeln, DietPi-DHCP/feste IPv4 anwenden und Samba für `/srv/mupibox` bedarfsgerecht aktivieren.
- **Provider:** Zugangskonfiguration für Spotify und Amazon Music. Das Speichern der Daten aktiviert noch keinen Wiedergabeadapter. Amazon Music bietet keine allgemeine öffentliche Wiedergabe-API.
- **Hardware:** MuPiHAT, Batterieprofile und Eingangsstrom. Die alten Profile sind als Vorgaben enthalten; `Custom` ist editierbar. Die Hardware-Anbindung folgt als eigener Dienst.
- **Smart Home:** Native Home-Assistant-Integration über die MuPiBox-API; kein zusätzlicher MQTT-Broker.
- **System:** reale Bootzeit, langsamste systemd-Units, Swap, Network-Wait, CPU-Profil, Initial Turbo sowie sicher bestätigte Neustart-/Shutdown-Aktionen.
- **Wartung:** Touch-UI-Neustart, Backup/Restore und Release-Wechsel mit automatischer Sicherung und Rollback.

## Systemprofile

| Option | Empfehlung | Wirkung |
|---|---|---|
| Swap | Für die reine Player-Box meist deaktivieren | Weniger SD-Karten-Schreibzugriffe; bei echtem Speichermangel gibt es weniger Reserve. |
| Network-Wait | Deaktivieren | Kürzerer Offline-Start; Online-Provider laden später nach. |
| CPU-Profil | Ausgewogen | Gutes Verhältnis aus Reaktion, Temperatur und Energieverbrauch. |
| Initial Turbo | 20 Sekunden auf Raspberry Pi/DietPi | Höchster CPU-Takt nur während des frühen Starts; `0` deaktiviert, Änderung erfordert Neustart. |
| Performance | Nur bei nachgewiesenen UI-/Audioengpässen | Schnellere Reaktion, aber mehr Wärme und Verbrauch. |

Die Werte `1–60` Sekunden und die Empfehlung `20` entsprechen der [DietPi-Konfiguration für ARM Initial Turbo](https://github.com/MichaIng/DietPi/blob/master/dietpi/dietpi-config).

Swap wird auf DietPi über das offizielle Werkzeug [`dietpi-set_swapfile`](https://github.com/MichaIng/DietPi/blob/master/dietpi/func/dietpi-set_swapfile) geändert. Statische Adressen werden über [`dietpi-network apply`](https://github.com/MichaIng/DietPi/blob/master/dietpi/dietpi-network) gesetzt; damit verschwindet DHCP für genau dieses Interface aus dessen ifupdown-Konfiguration, ohne einen möglicherweise anderweitig benötigten DHCP-Client global zu entfernen.

Privilegierte Änderungen laufen ausschließlich über den lokalen `mupibox-system-agent`. Er besitzt keinen Netzwerk-Port und akzeptiert nur fest definierte Aktionen über einen Unix-Socket.
