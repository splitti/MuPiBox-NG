# Backup, Restore und Updates

## Backups

Im Adminbereich unter **Wartung** stehen zwei Downloads bereit:

- **Konfigurationsbackup**: SQLite-Einstellungen, Navigation, Admin-Passwort-Hash und Wiedergabefortschritte.
- **Backup einschließlich Medien**: derselbe Datenbank-Snapshot plus Dateien unter `/srv/mupibox/music`.

Das ZIP-Format enthält `manifest.json`, `database/mupibox.db` und optional `media/...`. SQLite wird mit `VACUUM INTO` konsistent gesichert, während die Box läuft. Große Medienarchive werden direkt zum Browser gestreamt; der Client muss genügend Speicher und eine stabile Verbindung besitzen.

## Restore

Restore akzeptiert nur das MuPiBox-ZIP-Format. Vor dem Import werden Pfade, Dateitypen, Größenbegrenzungen und die SQLite-Integrität geprüft. Vor jeder Änderung entsteht automatisch ein Sicherheitsstand unter `/var/lib/mupibox-ng/backups/`.

Einstellungen, Navigation und Fortschritt werden ersetzt. Wird „Medien ebenfalls wiederherstellen“ gewählt, werden Archivdateien sicher unterhalb des Medienordners zusammengeführt. Bereits vorhandene zusätzliche Medien werden nicht gelöscht. Nach dem Restore wird die Bibliothek neu eingelesen und die Adminsitzung beendet; anschließend gilt das Passwort aus dem Backup.

## GitHub-Releases

Die Adminoberfläche liest veröffentlichte Releases von `splitti/MuPiBox-NG`. Ein Versionswechsel akzeptiert ausschließlich gültige Release-Tags. Vorher werden ein Datenbank-Backup und der aktuelle Commit als Rücksprungziel gespeichert.

Der Updater verweigert lokale Git-Änderungen, stoppt Player/UI, sichert die Datenbank, installiert den Tag und startet die Dienste neu. Bei einem Fehler wird automatisch der vorherige Commit installiert. Währenddessen darf die Box nicht ausgeschaltet werden. **Vorherige Version** nutzt den zuletzt gespeicherten Commit. Solange noch kein GitHub-Release veröffentlicht wurde, bleibt die Liste leer.

```bash
systemctl status 'mupibox-update@*' --no-pager
journalctl -u 'mupibox-update@*' -b --no-pager -n 200
ls -lh /var/lib/mupibox-ng/backups/
```
