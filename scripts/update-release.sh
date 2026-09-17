#!/usr/bin/env bash
# Switch an installed MuPiBox appliance to a GitHub release tag or to the last
# installed commit. Invoked only by mupibox-update@.service.
set -euo pipefail

TARGET="${1:-}"
REPO_DIR=/opt/mupibox-ng
STATE_DIR=/var/lib/mupibox-ng/updates
LOCK_FILE=/run/mupibox-update.lock

if [[ ! "$TARGET" =~ ^(rollback|v?[0-9][A-Za-z0-9._-]{0,63})$ ]]; then
    echo "Invalid release target." >&2
    exit 2
fi
if [[ ! -d "$REPO_DIR/.git" ]]; then
    echo "MuPiBox Git checkout not found at $REPO_DIR." >&2
    exit 1
fi

exec 9>"$LOCK_FILE"
flock -n 9 || { echo "Another MuPiBox update is already running." >&2; exit 1; }
mkdir -p "$STATE_DIR" /var/lib/mupibox-ng/backups
git_mupibox() { git -c safe.directory="$REPO_DIR" -C "$REPO_DIR" "$@"; }

if [[ -n "$(git_mupibox status --porcelain)" ]]; then
    echo "The MuPiBox checkout has local changes; update aborted." >&2
    exit 1
fi

CURRENT_COMMIT="$(git_mupibox rev-parse HEAD)"
if [[ "$TARGET" == rollback ]]; then
    [[ -s "$STATE_DIR/previous-ref" ]] || { echo "No previous release is available." >&2; exit 1; }
    TARGET_COMMIT="$(<"$STATE_DIR/previous-ref")"
else
    git_mupibox fetch --tags --prune origin
    TARGET_COMMIT="$(git_mupibox rev-parse --verify "refs/tags/${TARGET}^{commit}")" || { echo "Release tag $TARGET was not found." >&2; exit 1; }
fi
if [[ "$TARGET_COMMIT" == "$CURRENT_COMMIT" ]]; then
    echo "Requested release is already installed."
    exit 0
fi

date -u +%Y-%m-%dT%H:%M:%SZ > "$STATE_DIR/started-at"
printf '%s\n' "$TARGET" > "$STATE_DIR/requested-target"
systemctl stop mupibox-ui.service mupibox-ng.service
if [[ -f /var/lib/mupibox-ng/mupibox.db ]]; then
    cp -a /var/lib/mupibox-ng/mupibox.db "/var/lib/mupibox-ng/backups/pre-update-$(date -u +%Y%m%d-%H%M%S).db"
fi

restore_previous() {
    trap - ERR
    echo "Update failed; restoring $CURRENT_COMMIT." >&2
    git_mupibox checkout --detach "$CURRENT_COMMIT"
    bash "$REPO_DIR/scripts/install-dietpi.sh" --no-start
    systemctl restart mupibox-system-agent.service
    systemctl start mupibox-ng.service mupibox-ui.service
}
trap 'restore_previous' ERR

git_mupibox checkout --detach "$TARGET_COMMIT"
[[ -f "$REPO_DIR/scripts/install-dietpi.sh" ]]
bash "$REPO_DIR/scripts/install-dietpi.sh" --no-start
printf '%s\n' "$CURRENT_COMMIT" > "$STATE_DIR/previous-ref"
printf '%s\n' "$TARGET_COMMIT" > "$STATE_DIR/current-ref"
trap - ERR
systemctl restart mupibox-system-agent.service
systemctl start mupibox-ng.service mupibox-ui.service
echo "MuPiBox update to $TARGET completed."
