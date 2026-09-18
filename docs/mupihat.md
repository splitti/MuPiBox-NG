# MuPiHat-Integration

> **Deutsch** · [English](mupihat.en.md)

## Entscheidung

MuPiBox-NG ist **Go-first**: Der BQ25792 wird direkt aus Go per I²C angesprochen, ausschließlich
der für MuPiBox tatsächlich benötigte Funktionsumfang (siehe Registerliste unten) – keine
vollständige 1:1-Portierung der bestehenden ca. 250 KB großen Python-Bibliothek.

Der vorhandene Python-Treiber (GPLv3) bleibt dabei erhalten als:

- Registerreferenz und Vorlage für die Initialisierungssequenz,
- dokumentiertes Vergleichsverhalten für Ladezustände und Fehlerfälle,
- möglicher Fallback, falls der native Go-Zugriff auf echter Hardware einen konkreten,
  nachweisbaren Blocker liefert.

Er ist damit **nicht mehr automatisch die Zielarchitektur**. Tritt ein solcher Blocker auf, wird
er dokumentiert und ein Python-Fallback vorgeschlagen – es wird nicht stillschweigend auf Python
zurückgewechselt. Bei Wiederverwendung von Teilen des bestehenden Treibers (Registerkonstanten,
Initialisierungslogik als Vorlage) bleiben Lizenz, Urheberhinweise und Quellcodezugang gemäß
GPLv3 erhalten.

Der bisherige eigenständige Python-Wrapper (Flask-Dienst auf `0.0.0.0:5000`, Status direkt nach
`/tmp/mupihat.json`) wird nicht übernommen. Privilegierter Hardwarezugriff (I²C/GPIO) erfolgt
über den bestehenden `mupibox-system-agent` bzw. eine sauber begründete Erweiterung davon – kein
zusätzlicher, dauerhaft laufender Hardware-Dienst nur für das MuPiHat. Die genaue Anbindung wird
mit der eigentlichen BQ25792-Implementierung festgelegt.

## Zuständigkeiten

| Bestandteil | Verantwortung |
| --- | --- |
| `mupibox-system-agent` (bzw. Erweiterung) | I²C-Zugriff, BQ25792 initialisieren, Watchdog zurücksetzen, Register lesen und freigegebene Werte schreiben |
| Go-Dienst (`mupibox-ng`) | SQLite-Einstellungen, Plausibilisierung, Admin-API, Statusanzeige, Warnungen und kontrollierter Shutdown |
| Qt/Web-Player | Kinderfreundliche Batterieanzeige ohne technische Werte im normalen Player |
| Adminoberfläche | Profilwahl, Custom-Profil, Stromlimit, Rohwerte, Diagnose und explizite Hardwarefreigabe |

Der privilegierte Zugriff läuft unter einem eingeschränkten Benutzer mit den nötigen Gruppen für
I²C/GPIO, kommuniziert lokal über einen Unix-Socket unter `/run/mupibox-ng/` bzw. den bestehenden
Socket des `mupibox-system-agent`; es wird kein Netzwerkport geöffnet. Fällt der Zugriff aus,
bleibt der Player benutzbar und zeigt „Akku unbekannt“ statt veraltete Werte weiterzuverwenden.

## Statusvertrag

Der Hardware-Agent liefert mindestens:

- Zeitstempel und Verbindungszustand,
- `vbat_mv`, `vbus_mv`, `ibat_ma` und `ibus_ma`,
- IC-Temperatur,
- Ladezustand und Ladephase,
- Batterie erkannt,
- berechneten Akkustand von 0 bis 100 Prozent,
- gewähltes Batterieprofil und Eingangsstromlimit,
- Warn-, Shutdown- und Fehlerzustände.

Der Akkustand wird zwischen `v_0`, `v_25`, `v_50`, `v_75` und `v_100` stückweise linear berechnet. Damit springt die Anzeige nicht mehr nur zwischen fünf groben Werten. Mittelwertbildung und Hysterese verhindern Flackern bei Lastwechseln. Warnung und Shutdown werden erst nach mehreren aufeinanderfolgenden Messungen ausgelöst.

## Batterieprofile

Alle Spannungen werden in SQLite als Ganzzahlen in Millivolt gespeichert.

| Profil | 100 % | 75 % | 50 % | 25 % | 0 % | Warnung | Shutdown |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Ansmann 2S1P | 8100 | 7800 | 7400 | 7000 | 6700 | 7000 | 6800 |
| ENERpower 2S2P 10.000 mAh | 8000 | 7700 | 7300 | 6900 | 6000 | 6500 | 6150 |
| USB-C mode (no battery) | 1 | 1 | 1 | 1 | 1 | 0 | 0 |
| Custom | 8100 | 7800 | 7400 | 7000 | 6700 | 7000 | 6800 |

`Custom` bleibt vollständig editierbar. Das USB-C-Profil deaktiviert Akkuwarnung und Akku-Shutdown. Ein Profilwechsel oder eine Änderung der Grenzwerte wird validiert; `v_100 >= v_75 >= v_50 >= v_25 >= v_0` muss gelten.

## Eingangsstrom

| Profil | IINDMP |
| --- | ---: |
| safe | 1790 mA |
| medium | 2200 mA |
| high | 2700 mA |

Schreibzugriffe auf das Lade-IC sind zunächst gesperrt. Sie werden erst nach dem read-only Hardwaretest auf dem echten Pi/MuPiHat freigeschaltet. Das Admininterface zeigt dabei deutlich an, ob ein Wert nur gespeichert oder bereits an die Hardware übertragen wurde.

## Schutzlogik

- Unterhalb der Warnschwelle erscheint eine deutliche Akkuwarnung.
- Unterhalb der Shutdownschwelle wird nach einer konfigurierten Bestätigungszeit ein kontrollierter Shutdown gestartet.
- Netzspannung, Ladestatus und Messfehler werden berücksichtigt, damit eine kurzfristige Spannungsspitze keinen Shutdown auslöst.
- Der Hardware-Agent darf den Rechner nicht selbst ausschalten. Die Entscheidung liegt im Go-Dienst, damit Wiedergabefortschritt, Shutdownsound und SQLite sauber abgeschlossen werden.
- Bei unplausiblen oder fehlenden Messwerten erfolgt kein automatischer Shutdown.

## Einführung

1. Simulierter Agent und Statusvertrag in LXC-Tests.
2. Read-only Agent auf dem MuPiHat: Watchdog, Rohwerte und Diagnose.
3. SQLite-Profile und Adminoberfläche.
4. Validierter Schreibzugriff für Eingangsstromlimit.
5. Warnung und kontrollierter Shutdown.
6. MuPiHat-GPIO für Betriebs-LED, Power-Taster und spätere Lüftersteuerung.

Vor Schritt 4 und 5 ist ein Testprotokoll mit echter Hardware Pflicht.
