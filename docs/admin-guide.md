# Adminbereich und Best Practices

Der Adminbereich trennt Einstellungen nach Verantwortung. Jede sicherheits- oder systemrelevante Option nennt Empfehlung, Auswirkung und mögliche Unterbrechungen direkt an der Einstellung.

## Bereiche

- **Box:** Sprache, Theme, Audio, Display, TTS und Admin-Passwort. Ein Passwort mit mindestens zehn Zeichen wird empfohlen.
- **Inhalte:** Kategorien, lokale Verzeichnisse, Resume-Listen und spätere Online-Provider. Änderungen werden vom Player ohne Neustart neu geladen.
- **Netzwerk:** WLAN-Adapter aktivieren, priorisieren, scannen und verbinden; Bluetooth-Geräte koppeln; DHCP oder feste IPv4-Werte hinterlegen. DHCP ist die sichere Vorgabe. Die statische Konfiguration wird auf DietPi erst automatisch angewendet, wenn das erkannte Netzwerk-Backend ohne Gefahr für SSH geändert werden kann.
- **Provider:** Zugangskonfiguration für Spotify und Amazon Music. Das Speichern der Daten aktiviert noch keinen Wiedergabeadapter. Amazon Music bietet keine allgemeine öffentliche Wiedergabe-API.
- **Hardware:** MuPiHAT, Batterieprofile und Eingangsstrom. Die alten Profile sind als Vorgaben enthalten; `Custom` ist editierbar. Die Hardware-Anbindung folgt als eigener Dienst.
- **Smart Home:** MQTT und Home-Assistant-Discovery. Empfohlen wird ein eigener Broker-Benutzer mit Zugriff nur auf das Box-Topic. Der Publisher folgt nach Festlegung der Status- und Steuertopics.
- **System:** reale Bootzeit, langsamste systemd-Units, Swap, Network-Wait und CPU-Profil.
- **Wartung:** Touch-UI-Neustart, Backup/Restore und Release-Wechsel mit automatischer Sicherung und Rollback.

## Systemprofile

| Option | Empfehlung | Wirkung |
|---|---|---|
| Swap | Für die reine Player-Box meist deaktivieren | Weniger SD-Karten-Schreibzugriffe; bei echtem Speichermangel gibt es weniger Reserve. |
| Network-Wait | Deaktivieren | Kürzerer Offline-Start; Online-Provider laden später nach. |
| CPU-Profil | Ausgewogen | Gutes Verhältnis aus Reaktion, Temperatur und Energieverbrauch. |
| Performance | Nur bei nachgewiesenen UI-/Audioengpässen | Schnellere Reaktion, aber mehr Wärme und Verbrauch. |

Privilegierte Änderungen laufen ausschließlich über den lokalen `mupibox-system-agent`. Er besitzt keinen Netzwerk-Port und akzeptiert nur fest definierte Aktionen über einen Unix-Socket.
