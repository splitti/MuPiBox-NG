# MuPiBox Qt Quick UI

Native Touch-Oberfläche für DietPi/Raspberry Pi ohne Browser.

Die Oberfläche wird in einem logischen 800×480-Koordinatensystem entworfen und von Qt seitenverhältnistreu auf die tatsächlich erkannte Bildschirmgröße skaliert. Damit ist 800×480 der Referenzmodus, aber kein fest verdrahtetes Display-Limit.

Aktueller Hardware-Test:

- Raspberry Pi 4, 1 GB RAM
- offizielles Raspberry Pi 7-inch Touch Display (DSI, 800×480)

Weitere DSI- und HDMI-Displays sollen über denselben DRM/KMS-/Qt-Quick-Pfad funktionieren. Panel-spezifische Device-Tree-Overlays werden bewusst getrennt vom UI-Code behandelt.
