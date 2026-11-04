#!/usr/bin/env bash
# Exercise real capture, age encryption, store readback, recovery and retention.
# Requires age and age-keygen already installed; never downloads a tool at runtime.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
tmp=$(mktemp -d)
writer=
cleanup() {
  local result=$?
  if [ -n "$writer" ]; then
    touch "$tmp/stop"
    wait "$writer" || result=1
  fi
  rm -rf "$tmp"
  exit "$result"
}
trap cleanup EXIT
fail() { echo "FAIL: $*" >&2; exit 1; }
for tool in age age-keygen python3; do
  command -v "$tool" >/dev/null || fail "required tool missing: $tool"
done

fx=$tmp/fixture
python3 "$here/recovery_fixture.py" seed "$fx" 1
age-keygen -o "$tmp/id.txt" 2>/dev/null
age-keygen -y "$tmp/id.txt" >"$tmp/recipients.txt"
cat >"$tmp/backup.env" <<ENV
BACKUP_HOST=
BACKUP_DIR=$tmp/store
BACKUP_RETENTION=3
BACKUP_PREFIX=fixture
BACKUP_RECIPIENTS_FILE=$tmp/recipients.txt
BACKUP_SOURCES="$fx/validator $fx/kernel"
BACKUP_NODE_HOME=$fx/validator
BACKUP_KERNEL_DIR=$fx/kernel
BACKUP_REQUIRED_KEYS="$fx/validator/config/priv_validator_key.json $fx/validator/config/node_key.json"
BACKUP_DB_DUMP_CMD="cat $fx/dump.sql"
BACKUP_IDENTITY=$tmp/id.txt
BACKUP_LOCK_FILE=$tmp/backup.lock
BACKUP_QUIESCE_CMD="mkdir $fx/.held"
BACKUP_RESUME_CMD="rmdir $fx/.held"
BACKUP_VALIDATE_CMD='python3 "$here/recovery_fixture.py" validate "\$BACKUP_NODE_HOME" "\$BACKUP_KERNEL_DIR"'
ENV

assert_restore() {
  local archive=$1 destination=$2 expected=$3
  "$here/restore.sh" "$archive" "$destination" "$tmp/backup.env" >/dev/null
  diff -r "$expected/validator" "$destination$fx/validator" || fail "restored node differs"
  diff -r "$expected/kernel" "$destination$fx/kernel" || fail "restored kernel differs"
  cmp "$expected/dump.sql" "$destination/db/wallet-db.dump" || fail "db dump differs"
  [ "$(stat -c %a "$destination$fx/validator/config/priv_validator_key.json")" = 600 ] || fail "mode not preserved"
  python3 "$here/recovery_fixture.py" validate "$destination$fx/validator" "$destination$fx/kernel"
}

name=$("$here/backup.sh" "$tmp/backup.env")
[ -f "$tmp/store/$name" ] && [ -f "$tmp/store/$name.sha256" ] || fail "archive not stored"
if grep -aq 'fixture-private' "$tmp/store/$name"; then fail "archive is not encrypted"; fi
assert_restore latest "$tmp/out" "$fx"
[ ! -e "$fx/.held" ] || fail "writers not resumed"

# Each real snapshot contains a different committed generation, not repeated bytes.
for generation in 2 3 4 5; do
  python3 "$here/recovery_fixture.py" seed "$fx" "$generation"
  good=$("$here/backup.sh" "$tmp/backup.env")
  assert_restore "$good" "$tmp/out-$generation" "$fx"
done
[ "$(find "$tmp/store" -name '*.age' | wc -l)" -eq 3 ] || fail "retention did not keep 3 archives"
[ "$(find "$tmp/store" -name '*.sha256' | wc -l)" -eq 3 ] || fail "retention left stray checksums"
[ ! -e "$tmp/store/$name" ] || fail "oldest archive not pruned"
cp -a "$fx" "$tmp/known-good"
(cd "$tmp/store" && sha256sum -- * | sort) >"$tmp/store-before"

assert_unchanged() {
  (cd "$tmp/store" && sha256sum -- * | sort) >"$tmp/store-after"
  cmp "$tmp/store-before" "$tmp/store-after" || fail "failed capture changed recovery points"
  [ -f "$tmp/store/$good" ] && [ -f "$tmp/store/$good.sha256" ] || fail "known-good point pruned"
  [ ! -e "$fx/.held" ] || fail "failed capture did not resume writers"
  [ -z "$(find "$tmp/store" -name '*.part')" ] || fail "failed capture left a partial upload"
}

# Real incomplete input must fail before publication or pruning.
mv "$fx/validator/config/node_key.json" "$tmp/node-key.json"
if "$here/backup.sh" "$tmp/backup.env" >"$tmp/failed-name" 2>"$tmp/err"; then
  fail "incomplete capture succeeded"
fi
grep -q 'incomplete capture; nothing pruned' "$tmp/err" || fail "capture failure not reported"
[ ! -s "$tmp/failed-name" ] || fail "failed capture returned an archive name"
assert_unchanged
mv "$tmp/node-key.json" "$fx/validator/config/node_key.json"

# A real age recipient parsing failure must also leave all recovery points intact.
cp "$tmp/recipients.txt" "$tmp/valid-recipients.txt"
printf '%s\n' 'invalid-age-recipient' >"$tmp/recipients.txt"
if "$here/backup.sh" "$tmp/backup.env" >"$tmp/failed-name" 2>"$tmp/err"; then
  fail "encryption with invalid recipients succeeded"
fi
grep -q 'encryption failed' "$tmp/err" || fail "encryption failure not reported"
[ ! -s "$tmp/failed-name" ] || fail "failed encryption returned an archive name"
assert_unchanged
mv "$tmp/valid-recipients.txt" "$tmp/recipients.txt"

# An uncooperative writer changes real source bytes while checkpoint.py reads them.
# No capture/encryption executable is replaced; partial plaintext is always rejected.
python3 - "$fx/kernel/volume/changing.bin" <<'PY'
from pathlib import Path
import sys
Path(sys.argv[1]).write_bytes(bytes(32 * 1024 * 1024))
PY
python3 "$here/recovery_fixture.py" churn "$fx/kernel/volume/changing.bin" "$tmp/ready" "$tmp/stop" &
writer=$!
for _ in {1..200}; do
  [ ! -e "$tmp/ready" ] || break
  kill -0 "$writer" 2>/dev/null || fail "source writer stopped early"
  sleep 0.01
done
[ -e "$tmp/ready" ] || fail "source writer did not start"
if "$here/backup.sh" "$tmp/backup.env" >"$tmp/failed-name" 2>"$tmp/err"; then
  fail "changing source capture succeeded"
fi
touch "$tmp/stop"
wait "$writer"
writer=
grep -q 'incomplete capture; nothing pruned' "$tmp/err" || fail "changing capture failure not reported"
[ ! -s "$tmp/failed-name" ] || fail "changing capture returned an archive name"
assert_unchanged
# Reopen the surviving encrypted image after both failed capture processes exited.
assert_restore "$good" "$tmp/after-failure" "$tmp/known-good"

# Exercise actual age authentication as well as the outer transport checksum.
cp "$tmp/store/$good" "$tmp/pristine.age"
python3 - "$tmp/store/$good" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
data = bytearray(path.read_bytes())
data[-1] ^= 1
path.write_bytes(data)
PY
if "$here/restore.sh" latest "$tmp/bad-checksum" "$tmp/backup.env" 2>"$tmp/err"; then
  fail "tampered checksum restored"
fi
grep -q 'checksum mismatch' "$tmp/err" || fail "checksum failure not reported"
[ ! -e "$tmp/bad-checksum" ] || fail "tampered archive wrote output"
(cd "$tmp/store" && sha256sum "$good" >"$good.sha256")
if "$here/restore.sh" latest "$tmp/bad-age" "$tmp/backup.env" 2>"$tmp/err"; then
  fail "tampered ciphertext restored with a matching transport checksum"
fi
grep -q 'archive does not decrypt' "$tmp/err" || fail "age authentication failure not reported"
[ ! -e "$tmp/bad-age" ] || fail "unauthenticated plaintext wrote output"
cp "$tmp/pristine.age" "$tmp/store/$good"
(cd "$tmp/store" && sha256sum "$good" >"$good.sha256")
assert_restore latest "$tmp/final" "$tmp/known-good"
echo "backup round-trip ok"
