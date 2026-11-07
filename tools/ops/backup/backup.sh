#!/usr/bin/env bash
# Capture quiesced state; decrypt and validate a transported recovery image before retention.
# Usage: backup.sh [env-file]
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
. "$here/lib.sh"
load_env "${1:-}"
: "${BACKUP_RECIPIENTS_FILE:?BACKUP_RECIPIENTS_FILE must be set}"
: "${BACKUP_SOURCES:?BACKUP_SOURCES must be set}"
: "${BACKUP_NODE_HOME:?BACKUP_NODE_HOME must be set}"
: "${BACKUP_KERNEL_DIR:?BACKUP_KERNEL_DIR must be set}"
: "${BACKUP_REQUIRED_KEYS:?BACKUP_REQUIRED_KEYS must list all required private key paths}"
: "${BACKUP_IDENTITY:?BACKUP_IDENTITY is required for restore validation}"
: "${BACKUP_QUIESCE_CMD:?BACKUP_QUIESCE_CMD must stop and flush every source writer}"
: "${BACKUP_RESUME_CMD:?BACKUP_RESUME_CMD must release the quiesce hold}"
: "${BACKUP_VALIDATE_CMD:?BACKUP_VALIDATE_CMD must validate isolated committed state and keys}"
[ -s "$BACKUP_RECIPIENTS_FILE" ] || die "recipients file empty or missing: $BACKUP_RECIPIENTS_FILE"
[ -s "$BACKUP_IDENTITY" ] || die "identity file empty or missing: $BACKUP_IDENTITY"
command -v python3 >/dev/null || die "python3 not found on PATH"

umask 077
# Serialize the stop/capture/resume lifecycle, including concurrent timer/manual runs.
exec 9>"${BACKUP_LOCK_FILE:-/run/lock/paxeer-backup.lock}"
flock -n 9 || die "another backup owns the quiesce hold"
stage=$(mktemp -d)
resume_needed=0
pending=0
published=0
cleanup() {
  local result=$?
  trap - EXIT
  if [ "$resume_needed" -eq 1 ]; then
    sh -c "$BACKUP_RESUME_CMD" >&2 || { echo "backup: emergency resume failed" >&2; result=1; }
  fi
  if [ "$pending" -eq 1 ]; then
    store "rm -f -- $dir/$n.part $dir/$n.sha256.part" || result=1
    if [ "$published" -eq 0 ]; then
      store "if [ ! -e $dir/$n ]; then rm -f -- $dir/$n.sha256; fi" || result=1
    fi
  fi
  rm -rf -- "$stage"
  exit "$result"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

capture_args=(capture "$stage/snapshot.tar.gz" "$stage/manifest.json"
  --node-home "$BACKUP_NODE_HOME" --kernel-dir "$BACKUP_KERNEL_DIR")
for src in $BACKUP_SOURCES; do capture_args+=(--source "$src"); done
for key in $BACKUP_REQUIRED_KEYS; do capture_args+=(--required-key "$key"); done

# Mark first: even a partially failed stop must release its hold on the error path.
resume_needed=1
sh -c "$BACKUP_QUIESCE_CMD" >&2 || die "quiesce failed; nothing captured or pruned"
if [ -n "${BACKUP_DB_DUMP_CMD:-}" ]; then
  sh -c "$BACKUP_DB_DUMP_CMD" >"$stage/wallet-db.dump" || die "database dump failed"
  [ -s "$stage/wallet-db.dump" ] || die "database dump is empty"
  capture_args+=(--dump "$stage/wallet-db.dump")
fi
# Any producer failure, including a source changing while read, rejects the capture.
python3 "$here/checkpoint.py" "${capture_args[@]}" || die "incomplete capture; nothing pruned"
sh -c "$BACKUP_RESUME_CMD" >&2 || die "resume failed; nothing pruned"
resume_needed=0

name="$BACKUP_PREFIX-$(date -u +%Y%m%dT%H%M%S.%NZ).tar.gz.age"
dir=$(q "$BACKUP_DIR")
n=$(q "$name")
age -R "$BACKUP_RECIPIENTS_FILE" "$stage/snapshot.tar.gz" >"$stage/archive.age" || die "encryption failed"
expected=$(sha256sum "$stage/archive.age")
expected=${expected%% *}
pending=1
store "mkdir -p $dir && umask 077 && cat > $dir/$n.part" <"$stage/archive.age" || die "upload failed; nothing pruned"
# Read the bytes back through the actual store transport; framing is not recovery proof.
store "cat $dir/$n.part" >"$stage/returned.age" || die "readback failed; nothing pruned"
actual=$(sha256sum "$stage/returned.age")
[ "${actual%% *}" = "$expected" ] || die "transport checksum mismatch; nothing pruned"
age -d -i "$BACKUP_IDENTITY" "$stage/returned.age" >"$stage/returned.tar.gz" || die "decrypt failed; nothing pruned"
python3 "$here/checkpoint.py" restore "$stage/returned.tar.gz" "$stage/recovered" \
  --expected-manifest "$stage/manifest.json" || die "isolated restore failed; nothing pruned"
# The owning validator must reopen both recovered stores, compare committed state,
# and load required keys without network access or signing. Never validate live roots.
BACKUP_RESTORE_ROOT="$stage/recovered" BACKUP_SNAPSHOT_MANIFEST="$stage/manifest.json" \
  BACKUP_NODE_HOME="$stage/recovered$BACKUP_NODE_HOME" \
  BACKUP_KERNEL_DIR="$stage/recovered$BACKUP_KERNEL_DIR" \
  sh -c "$BACKUP_VALIDATE_CMD" >&2 || die "committed state/key validation failed; nothing pruned"

# Publish the sidecar first: a visible archive always has its verified checksum.
store "cd $dir && printf '%s  %s\\n' '$expected' $n > $n.sha256.part && mv -- $n.sha256.part $n.sha256 && mv -- $n.part $n" || die "publish failed; nothing pruned"
published=1
p=$(q "$BACKUP_PREFIX")
store "cd $dir && ls -1 -- $p-*.tar.gz.age | sort | head -n -$BACKUP_RETENTION | while read -r f; do rm -f -- \"\$f\" \"\$f.sha256\"; done"
echo "$name"
