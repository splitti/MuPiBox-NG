# Backup, restore and updates

## Backups

The **Maintenance** section provides a configuration backup containing SQLite settings, navigation, the password hash and playback progress, plus an optional backup including files below `/srv/mupibox/music`.

The ZIP contains `manifest.json`, `database/mupibox.db` and optionally `media/...`. SQLite is captured consistently with `VACUUM INTO`. Large media archives are streamed to the browser, so the client needs sufficient storage and a stable connection.

## Restore

Paths, file types, size limits and SQLite integrity are checked before import. Every restore first creates a safety snapshot below `/var/lib/mupibox-ng/backups/`. Settings, navigation and progress are replaced. Selected media are merged safely below the media root; extra existing files are not deleted. The admin session ends and the password stored in the backup applies.

## GitHub releases

Administration reads published releases from `splitti/MuPiBox-NG`. The updater rejects local Git changes, stops player/UI, backs up the database, installs the selected tag and restarts services. On failure it automatically reinstalls the previous commit. Do not power off during this process. The list remains empty until the first GitHub release is published.

```bash
systemctl status 'mupibox-update@*' --no-pager
journalctl -u 'mupibox-update@*' -b --no-pager -n 200
ls -lh /var/lib/mupibox-ng/backups/
```
