# MuPiBox-NG auf DietPi / Raspberry Pi testen

Diese Anleitung beschreibt den ersten Geräte-Test der MuPiBox-NG auf einem frisch installierten Raspberry Pi. Die lokale Player-Oberfläche läuft **nativ mit Qt Quick über EGLFS/KMS**. Für den Touchscreen wird weder Chromium noch ein Desktop (X11/Wayland) benötigt. Die Browser-/Adminoberfläche bleibt parallel im Netzwerk verfügbar.

## Unterstützungsstrategie für Displays

MuPiBox-NG erzwingt **standardmäßig keine feste Displayauflösung und kein bestimmtes Panel**. Die native UI besitzt ein logisches 800×480-Layout und skaliert dieses seitenverhältnistreu auf die tatsächlich vom Kernel bereitgestellte Bildschirmgröße. Damit kann derselbe Stand zunächst auf unterschiedlichen DSI- und HDMI-Displays getestet werden.

Display-spezifische Device-Tree-Overlays gehören nicht in die generische Installation. Sie werden nur über explizite Hardwareprofile aktiviert, wenn ein Panel sie wirklich benötigt.

### Aktuelle Testhardware

Der aktuelle Referenztest kann mit folgendem Aufbau erfolgen:

- Raspberry Pi 4, 1 GB RAM
- offizielles Raspberry Pi 7-inch Touch Display der ersten Generation
- DSI-Verbindung
- 800×480 Pixel

Auf einem normalen Raspberry Pi 4 wird das offizielle Raspberry-Pi-Touchdisplay vom Raspberry-Pi-Stack erkannt. Für diesen Test **keine Waveshare-Option verwenden und keinen zusätzlichen DSI-Overlay erzwingen**.

## DietPi und Displaykonfiguration

Ab DietPi 10.5 basiert die Raspberry-Pi-Grafikkonfiguration auf dem modernen KMS/DRM-Stack. `dietpi-display` ist das DietPi-Werkzeug für Displaymodus/Auflösung und Rotation.

Auf DietPi daher für das offizielle 7-inch Touch Display zunächst einfach:

```sh
sudo dietpi-display
```

verwenden, wenn Auflösung oder Rotation geändert werden sollen. Für einen normal montierten 800×480-Test ist normalerweise keine Änderung nötig.

Wichtig: `dietpi-display` konfiguriert den KMS/DRM-Ausgabemodus. Es ersetzt nicht den Device-Tree-Treiber eines fremden DSI-Panels. Ein Drittanbieter-DSI-Display kann weiterhin ein herstellerspezifisches Overlay benötigen.

## 1. DietPi installieren

Ein aktuelles 64-Bit-DietPi-Image auf die SD-Karte schreiben und den normalen DietPi-Erststart durchführen. SSH bzw. eine lokale Shell muss funktionieren.

Für den aktuellen Pi-4-Test das offizielle 7-inch DSI Touch Display vor dem Einschalten anschließen.

## 2. MuPiBox-NG aus Git laden

```sh
sudo apt update
sudo apt install -y git
sudo git clone --branch rebuild/go-foundation --single-branch https://github.com/splitti/MuPiBox-NG.git /opt/mupibox-ng
cd /opt/mupibox-ng
```

## 3. Installer ausführen

### Raspberry Pi 4 + offizielles Raspberry Pi 7-inch Touch Display

Keine Displayoption angeben:

```sh
sudo bash scripts/install-dietpi.sh
sudo reboot
```

### Waveshare 5-inch 800×480 DSI LCD / LCD (B)

Nur wenn genau dieses Panel verwendet wird:

```sh
sudo bash scripts/install-dietpi.sh --waveshare-5-dsi
sudo reboot
```

Der Installer:

- installiert mpv, ALSA und die benötigten Qt-6-/EGLFS-Pakete,
- stellt Go >= 1.24 bereit, falls die Distribution eine ältere Go-Version liefert,
- baut und testet das Go-Backend,
- legt den Systembenutzer `mupibox` an,
- installiert Backend, Konfiguration und native Qt-Quick-Oberfläche,
- installiert das neue MuPiBox-Startbild,
- aktiviert `mupibox-ng.service` und `mupibox-ui.service`,
- legt `/srv/mupibox/music` für lokale Medien an.

Vorhandene `/etc/mupibox-ng/config.json` wird bei erneutem Ausführen nicht überschrieben.

## Wie das Bild auf das Display kommt

Es läuft bewusst **kein Browser-Kiosk**. Die Anzeige besteht aus dieser Kette:

1. Linux stellt das erkannte DSI-/HDMI-Display über DRM/KMS bereit.
2. `mupibox-ui.service` startet Qt Quick mit `QT_QPA_PLATFORM=eglfs`.
3. Qt rendert direkt über DRM/KMS/EGL auf den Bildschirm.
4. Die QML-Oberfläche liest die tatsächliche Bildschirmgröße und skaliert ihr logisches 800×480-Layout seitenverhältnistreu.
5. Zuerst wird `assets/mupibox-startscreen.jpg` als MuPiBox-Startscreen gezeigt.
6. Danach blendet die native Player-Oberfläche ein.
7. Touch-Ereignisse gelangen über die Linux-Input-Geräte direkt zu Qt.

Dadurch entfallen Chromium und ein kompletter Desktop-Stack.

Das installierte Startbild liegt unter:

```text
/usr/local/share/mupibox-ng/ui/assets/mupibox-startscreen.jpg
```

Die native Oberfläche liegt unter:

```text
/usr/local/share/mupibox-ng/ui/Main.qml
```

## Verschiedene Displaytypen

Die MuPiBox soll nicht an ein bestimmtes Display gekoppelt werden. Geplant bzw. bereits berücksichtigt sind:

- offizielles Raspberry Pi Touch Display über DSI,
- weitere DSI-Panels mit Kernel-/Device-Tree-Treiber,
- HDMI-Displays mit USB-Touch,
- unterschiedliche Auflösungen und Seitenverhältnisse.

Für 800×480 ist das Layout nativ passend. Bei anderen Auflösungen wird es zunächst proportional skaliert und zentriert. Später können zusätzliche responsive Layoutstufen ergänzt werden, ohne die Playerlogik oder das Backend zu ändern.

## Was nach dem Neustart passieren soll

Nach dem Kernel-/DietPi-Start startet automatisch die native MuPiBox-UI. Für rund zwei Sekunden erscheint das neue MuPiBox-Logo; anschließend erscheint die native Oberfläche.

Die UI probiert das lokale Go-Backend zunächst auf `127.0.0.1:8090` und anschließend auf `127.0.0.1:8080`. Sie verwendet `/api/home`, sofern die parallele NextGen-Arbeit bereits integriert ist, und fällt für den älteren Foundation-Stand auf `/api/library` zurück.

Diese erste Geräteversion ist bewusst ein Abnahmestand für Boot, Display, Touch-/Qt-Basis, Logo und Backend-Anbindung.

## Prüfung und Fehlersuche

Backend und UI:

```sh
systemctl status mupibox-ng --no-pager
systemctl status mupibox-ui --no-pager
curl http://127.0.0.1:8090/api/health || curl http://127.0.0.1:8080/api/health
```

UI-Log:

```sh
journalctl -u mupibox-ui -b --no-pager -n 100
```

Display-/Input-Geräte:

```sh
ls -l /dev/dri /dev/input
cat /sys/class/drm/*/status 2>/dev/null
```

Auf DietPi:

```sh
sudo dietpi-display
```

Verfügbare Audioausgänge:

```sh
aplay -l
mpv --audio-device=help
```

Bei schwarzem Display zuerst Kabel, Stromversorgung und Kernel-Erkennung prüfen, bevor Display-Overlays verändert werden.

## Browserzugriff für Entwicklung/Admin

Die konfigurierte Adresse lässt sich so prüfen:

```sh
grep '"listen"' /etc/mupibox-ng/config.json
```

Die native Player-Oberfläche und die Browser-/Adminoberfläche verwenden dasselbe Backend und dieselben persistenten Daten.
