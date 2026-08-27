#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

FAKA_ROOT="${FAKA_ROOT:-/srv/faka}"
APP_DIR="$FAKA_ROOT/app"
DATA_DIR="$FAKA_ROOT/data"
BACKUP_DIR="${FAKA_BACKUP_DIR:-$FAKA_ROOT/backup/data}"
RETENTION_DAYS="${FAKA_BACKUP_RETENTION_DAYS:-14}"
LOCK_FILE="${FAKA_BACKUP_LOCK_FILE:-/run/lock/faka-backup.lock}"

DB_SOURCE="$DATA_DIR/db/dujiao.db"
CONFIG_SOURCE="$APP_DIR/config.yml"
BINARY_SOURCE="$APP_DIR/dujiao-server"

log() {
  printf '[faka-backup] %s\n' "$*"
}

fail() {
  printf '[faka-backup] ERROR: %s\n' "$*" >&2
  exit 1
}

for command in python3 tar sha256sum flock install mktemp; do
  command -v "$command" >/dev/null 2>&1 || fail "required command not found: $command"
done

[[ -f "$DB_SOURCE" ]] || fail "database not found: $DB_SOURCE"
[[ -f "$CONFIG_SOURCE" ]] || fail "config not found: $CONFIG_SOURCE"
[[ -x "$BINARY_SOURCE" ]] || fail "server binary not found or not executable: $BINARY_SOURCE"
[[ "$RETENTION_DAYS" =~ ^[0-9]+$ ]] || fail "FAKA_BACKUP_RETENTION_DAYS must be a non-negative integer"

install -d -m 700 "$BACKUP_DIR"
install -d -m 755 "$(dirname "$LOCK_FILE")"
exec 9>"$LOCK_FILE"
flock -n 9 || fail "another backup is already running"

STAMP="$(TZ=Asia/Shanghai date +%Y%m%d-%H%M%S)"
DEST_DIR="$BACKUP_DIR/faka-$STAMP"
TMP_DIR="$(mktemp -d "$BACKUP_DIR/.tmp-$STAMP-XXXXXX")"

cleanup() {
  status=$?
  if [[ -d "$TMP_DIR" ]]; then
    rm -rf -- "$TMP_DIR"
  fi
  exit "$status"
}
trap cleanup EXIT INT TERM

log "creating consistent SQLite backup"
python3 - "$DB_SOURCE" "$TMP_DIR/dujiao.db" <<'PY'
import sqlite3
import sys
from pathlib import Path
from urllib.parse import quote

source = str(Path(sys.argv[1]).resolve())
destination = str(Path(sys.argv[2]).resolve())
source_uri = f"file:{quote(source)}?mode=ro"

src = sqlite3.connect(source_uri, uri=True, timeout=60)
dst = sqlite3.connect(destination, timeout=60)
try:
    src.backup(dst, pages=1024, sleep=0.05)
    result = dst.execute("PRAGMA quick_check").fetchone()
    if not result or result[0] != "ok":
        raise RuntimeError(f"backup quick_check failed: {result!r}")
finally:
    dst.close()
    src.close()
PY
chmod 600 "$TMP_DIR/dujiao.db"

log "copying production config"
install -m 600 "$CONFIG_SOURCE" "$TMP_DIR/config.yml"

log "archiving uploads from app/uploads and data/uploads"
mkdir -p "$TMP_DIR/upload-stage/app" "$TMP_DIR/upload-stage/data"
if [[ -d "$APP_DIR/uploads" ]]; then
  cp -a "$APP_DIR/uploads" "$TMP_DIR/upload-stage/app/"
fi
if [[ -d "$DATA_DIR/uploads" ]]; then
  cp -a "$DATA_DIR/uploads" "$TMP_DIR/upload-stage/data/"
fi
tar -C "$TMP_DIR/upload-stage" -czf "$TMP_DIR/uploads.tar.gz" .
rm -rf -- "$TMP_DIR/upload-stage"
chmod 600 "$TMP_DIR/uploads.tar.gz"

log "archiving application logs"
mkdir -p "$TMP_DIR/log-stage/app" "$TMP_DIR/log-stage/data"
if [[ -d "$APP_DIR/logs" ]]; then
  cp -a "$APP_DIR/logs" "$TMP_DIR/log-stage/app/"
fi
if [[ -d "$DATA_DIR/logs" ]]; then
  cp -a "$DATA_DIR/logs" "$TMP_DIR/log-stage/data/"
fi
tar -C "$TMP_DIR/log-stage" -czf "$TMP_DIR/logs.tar.gz" .
rm -rf -- "$TMP_DIR/log-stage"
chmod 600 "$TMP_DIR/logs.tar.gz"

BINARY_SHA256="$(sha256sum "$BINARY_SOURCE" | cut -d' ' -f1)"
EMBEDDED_VERSION="$(grep -aoE 'viva-[0-9a-f]{40}' "$BINARY_SOURCE" 2>/dev/null | sort -u | tail -n 1 || true)"
SERVICE_STATE="$(systemctl is-active dujiao-next 2>/dev/null || true)"

cat > "$TMP_DIR/manifest.txt" <<MANIFEST
created_at=$(TZ=Asia/Shanghai date -Is)
host=$(hostname)
service_state=$SERVICE_STATE
database_source=$DB_SOURCE
config_source=$CONFIG_SOURCE
uploads_sources=$APP_DIR/uploads,$DATA_DIR/uploads
logs_sources=$APP_DIR/logs,$DATA_DIR/logs
binary_source=$BINARY_SOURCE
binary_sha256=$BINARY_SHA256
embedded_version=$EMBEDDED_VERSION
retention_days=$RETENTION_DAYS
MANIFEST
chmod 600 "$TMP_DIR/manifest.txt"

(
  cd "$TMP_DIR"
  sha256sum dujiao.db config.yml uploads.tar.gz logs.tar.gz manifest.txt > SHA256SUMS
)
chmod 600 "$TMP_DIR/SHA256SUMS"

log "verifying backup checksums"
(
  cd "$TMP_DIR"
  sha256sum -c SHA256SUMS >/dev/null
)

[[ ! -e "$DEST_DIR" ]] || fail "backup destination already exists: $DEST_DIR"
mv "$TMP_DIR" "$DEST_DIR"
TMP_DIR=""
ln -sfn "$(basename "$DEST_DIR")" "$BACKUP_DIR/latest"

if (( RETENTION_DAYS > 0 )); then
  find "$BACKUP_DIR" -mindepth 1 -maxdepth 1 -type d -name 'faka-*' -mtime "+$RETENTION_DAYS" -print -exec rm -rf -- {} +
fi

log "backup complete: $DEST_DIR"
du -sh "$DEST_DIR"
