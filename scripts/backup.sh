#!/bin/sh
# Backs up the database and the uploaded photos of a running stack.
# Run from the repo root on the server. Nightly via cron, for example:
#   15 3 * * * cd /opt/blister64 && ./scripts/backup.sh >> backups/backup.log 2>&1
#
# BACKUP_DIR        where files go (default ./backups)
# BACKUP_KEEP_DAYS  local files older than this are deleted (default 7)
# BACKUP_S3_URI     optional, e.g. s3://my-bucket/blister64: each run is also
#                   copied there with the AWS CLI (the server's instance role).
#
# Restore: see docs/deploy.md.
set -eu

dir=${BACKUP_DIR:-./backups}
keep=${BACKUP_KEEP_DAYS:-7}
stamp=$(date -u +%Y%m%dT%H%M%SZ)
db="$dir/db-$stamp.dump"
photos="$dir/uploads-$stamp.tar.gz"
mkdir -p "$dir"

# A failed run must not leave a truncated file that looks like a backup.
trap 'rm -f "$db" "$photos"' INT TERM
fail() {
	rm -f "$db" "$photos"
	echo "backup failed: $1" >&2
	exit 1
}

docker compose exec -T db sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom' >"$db" || fail "pg_dump"
docker compose exec -T api tar -C /data/uploads -czf - . >"$photos" || fail "uploads archive"
[ -s "$db" ] || fail "empty database dump"
trap - INT TERM

if [ -n "${BACKUP_S3_URI:-}" ]; then
	aws s3 cp --only-show-errors "$db" "$BACKUP_S3_URI/" || fail "copy to S3"
	aws s3 cp --only-show-errors "$photos" "$BACKUP_S3_URI/" || fail "copy to S3"
fi

find "$dir" -name 'db-*.dump' -mtime +"$keep" -delete
find "$dir" -name 'uploads-*.tar.gz' -mtime +"$keep" -delete
echo "backup ok: $db ($(wc -c <"$db") bytes), $photos"
