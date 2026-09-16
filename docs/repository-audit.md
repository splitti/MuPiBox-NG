# Bestandsprüfung vom 16.09.2026

> Deutsch. English version: [repository-audit.en.md](repository-audit.en.md)

Geprüfter Ausgangspunkt: `main` bei `598c92c59ca18e19fef80b4f8c7587ecf6f0839d`
(10.02.2026, „UI update“). GitHub-Lesen und Schreibberechtigung bestätigt; Branch
`rebuild/go-foundation` von genau diesem Commit angelegt. Kein Überschreiben von main.

Keine README, keine AGENTS.md, keine Tests/CI im Ausgangsbaum. Go 1.19, Modul `mupibox`,
keine externen Go-Abhängigkeiten. Enthalten: cmd, catalog/player/state/status,
einfache Weboberfläche mit Fonts/Icons, zwei Installationsskripte und ein Schema.

| Bestandteil | Befund | Entscheidung |
| --- | --- | --- |
| Go-Struktur / Modul | Kleine, dependency-freie Basis | Strukturprinzip und Modulnamen übernehmen; Ziel Go 1.24 |
| `webui/webui.go` | embed-Handler vorhanden, main nutzte stattdessen Dateisystem | Einbettungsansatz aktiv einsetzen |
| Player | MemoryPlayer mit Demo-Zeiten und Titeln, keine Audioausgabe | Durch Controller + austauschbaren Adapter ersetzen |
| HTTP-API | Viele Methoden ohne Fehlerpfad; Sammlungen fest codiert | API mit validierten JSON-Befehlen und echtem Bibliotheksbezug |
| Oberfläche | Fest 800×480; Kachel klickt nur console.log; Lautstärke lokal gehalten | Responsive, echte Befehle und gemeinsam abgefragter Status |
| Katalog | Spotify/Amazon/RSS nur Beispiel-IDs und URLs | Als Entwurfsreferenz archivieren, nicht als unterstützte Quellen anzeigen |
| Source-Resolver | Meldet jede bekannte Quelle ungeprüft als verfügbar | Nicht übernehmen; keine falschen Verfügbarkeitszusagen |
| Fortschrittsspeicher | Modell nützlich; ignorierte Lese-/JSON-/Schreibfehler, nicht atomar | Konzept übernehmen, robuste Persistenz gesondert implementieren |
| Hardwarestatus | Feste int/bool-Felder ohne „unbekannt“ | Keine Hardwarewerte anzeigen; später Fähigkeiten/nullable Telemetrie |
| Installer | Globales apt upgrade, Root-Dienst, Code/Daten vermischt | Archivieren; begrenzte systemd-Vorlage mit eigenem Benutzer |
| Fonts/Cover/Icons | Nur Icon-Hinweis unter LICENSES; Herkunft weiterer Assets ungeklärt | Unverändert archivieren; aktive UI nutzt Systemschrift und CSS |

Der gesamte Prototyp einschließlich Assets bleibt bytegleich in `legacy/prototype/`.
Seine eigene go.mod grenzt ihn vom neuen Build ab. Die vorhandene LICENSES-Datei bleibt erhalten.
Es wurde keine neue Projektlizenz erfunden; Projektlizenz und Asset-Rechte sind vor Distribution zu klären.
Die alten Installer sind historische Referenz und dürfen nicht für die neue Anwendung ausgeführt werden.

## Alte MuPiBox

Das aktuelle autosetup-Skript der Referenz wurde erneut gelesen: Node.js/PM2, Python-Hardwarepakete,
MPlayer und librespot sowie Releaseauswahl über version.json sind erkennbar.
Es greift weit in das System ein und kopiert/ersetzt Konfigurationen und Programmteile.
Wir übernehmen weder diese Änderungen noch den Installer ungeprüft. Keine Hardware-Pinbelegung
wurde aus bloßen Paketnamen abgeleitet.

## Entwicklungsumgebung

MuPiBox Dev stellt weiterhin ausschließlich `list_files`, `read_file`, `write_file`, `go_check`,
`git_status` und `git_diff` bereit. Go 1.24.4 linux/amd64 bestätigt. Anfangszustand nur
`dev-access-check.txt`; Git-Status „not a git repository“. Ein Testfehler zeigte später den echten
Projektpfad `/opt/mupibox-ng`. Dort werden nun die neuen Quellen gebaut/getestet.

Der separate ChatGPT-Checkout wurde über Git geklont. Er ist nicht die LXC.
GitHub-Zugriff ersetzt keine Git-Initialisierung in der LXC. `scripts/link-lxc.sh` bereitet den
Abgleich ohne Überschreiben abweichender lokaler Dateien vor; Ausführung muss mangels Shell-Aktion
einmal durch splitti erfolgen. Kein Test missbraucht die Go-Aktion zum Klonen oder Ausführen von Git.
