#!/usr/bin/env bash
# Moves the keys of each node home's test keyring into its passphrase-protected file
# keyring, then removes the test keyring once a verified backup archive holds it.
# Usage: keyring-migrate.sh <home>...
# Environment:
#   KEYRING_PASSPHRASE_FILE  file whose first line is the file keyring passphrase (required)
#   PAXD                     node binary, default paxd
#   BACKUP_ENV               env file of the backup kit (tools/ops/backup), default
#                            /etc/paxeer/backup.env; BACKUP_IDENTITY must be set there or here
# For each home, in order:
#   1. every key of <home>/keyring-test is exported armored under the passphrase and
#      imported into <home>/keyring-file, unless that keyring already holds the name at
#      the same address; the name at another address stops the home (reason=conflict).
#   2. the file keyring must reopen, export each private key, and recover its address
#      through import into a fresh private file keyring (reason=file-recovery).
#   3. the newest archive of BACKUP_PREFIX in the store must pass its sha256, be written
#      no earlier than the last change under the test keyring, decrypt with
#      BACKUP_IDENTITY, restore identical key bytes into a private home and recover
#      every key with the actual backend (reason=backup:<why>).
#   4. no running process may name the home together with --keyring-backend test, and
#      <home>/config/client.toml may not select keyring-backend test (reason=in-use).
# Only then is <home>/keyring-test removed. Prints one line per home:
#   "home[k] keys=<n> file=match backup=<archive> test=removed"
#   "home[k] test=absent"
#   "home[k] keys=<n> test=kept reason=<reason>"
# Exits 0 only when no home is left with a test keyring, 1 otherwise, 2 on a usage error.
set -euo pipefail
umask 077
here=$(cd "$(dirname "$0")" && pwd)
paxd=${PAXD:-paxd}

if [ "$#" -eq 0 ] || [ -z "${KEYRING_PASSPHRASE_FILE:-}" ]; then
	sed -n '2,/^set -euo/p' "$0" | sed '$d' | sed 's/^# \{0,1\}//' >&2
	exit 2
fi
[ -r "$KEYRING_PASSPHRASE_FILE" ] || {
	echo "keyring-migrate: KEYRING_PASSPHRASE_FILE is not readable" >&2
	exit 2
}
pass=$(head -n 1 "$KEYRING_PASSPHRASE_FILE")
[ -n "$pass" ] || {
	echo "keyring-migrate: the passphrase file's first line is empty" >&2
	exit 2
}
for tool in "$paxd" python3 sha256sum; do
	command -v "$tool" >/dev/null 2>&1 || {
		echo "keyring-migrate: $tool is required" >&2
		exit 2
	}
done

# pw: the passphrase on enough lines for every prompt one paxd command makes (the
# armor passphrase and a new keyring's confirmation take the same value).
pw() { printf '%s\n' "$pass" "$pass" "$pass" "$pass"; }

# keys <home> <backend>: "name address" per key, sorted.
keys() {
	pw | "$paxd" keys list --home "$1" --keyring-backend "$2" --output json 2>/dev/null |
		python3 "$here/keyring_recovery.py" keys
}

# recover_keys <home> <backend> <expected>: reopen the backend, export private
# material, import into a fresh file backend and reopen each recovered address.
recover_keys() (
	listed=$(keys "$1" "$2") || exit 1
	if [ "$2" = test ] && [ "$listed" != "$3" ]; then exit 1; fi
	proof=$(mktemp -d) || exit 1
	trap 'rm -rf -- "$proof"' EXIT
	while read -r name addr; do
		[ -n "$name" ] || continue
		current=$(awk -v n="$name" '$1 == n {print $2}' <<<"$listed")
		[ "$current" = "$addr" ] || exit 1
		address=$(pw | "$paxd" keys show "$name" --address --home "$1" --keyring-backend "$2" 2>/dev/null) || exit 1
		[ "$address" = "$addr" ] || exit 1
		pw | "$paxd" keys export "$name" --home "$1" --keyring-backend "$2" >"$proof/key.armor" 2>/dev/null || exit 1
		[ -s "$proof/key.armor" ] || exit 1
		pw | "$paxd" keys import "$name" "$proof/key.armor" --home "$proof/home" --keyring-backend file >/dev/null 2>&1 || exit 1
		address=$(pw | "$paxd" keys show "$name" --address --home "$proof/home" --keyring-backend file 2>/dev/null) || exit 1
		[ "$address" = "$addr" ] || exit 1
		rm -f -- "$proof/key.armor" || exit 1
	done <<<"$3"
)

# verify_backup <home> <manifest> <expected keys>: prints the archive name only
# after byte comparison and backend recovery. Runs in a private subshell.
verify_backup() (
	. "$here/backup/lib.sh"
	load_env "" || die "environment"
	: "${BACKUP_IDENTITY:?BACKUP_IDENTITY is unset}"
	[ -r "$BACKUP_IDENTITY" ] || die "BACKUP_IDENTITY is not readable"
	dir=$(q "$BACKUP_DIR")
	name=$(store "cd $dir 2>/dev/null && ls -1 -- $(q "$BACKUP_PREFIX")-*.tar.gz.age 2>/dev/null | sort | tail -n 1") || die "listing"
	[ -n "$name" ] || die "absent"
	case $name in */* | .* | *$'\n'*) die "archive-name" ;; esac
	work=$(mktemp -d) || die "temporary-home"
	trap 'rm -rf "$work"' EXIT
	store "cat $dir/$(q "$name")" >"$work/$name" || die "download"
	store "cat $dir/$(q "$name").sha256" >"$work/$name.sha256" || die "checksum-download"
	(cd "$work" && sha256sum --quiet -c "$name.sha256" >/dev/null 2>&1) || die "checksum"
	ts=${name#"$BACKUP_PREFIX"-}
	written=$(date -u -d "${ts:0:4}-${ts:4:2}-${ts:6:2} ${ts:9:2}:${ts:11:2}:${ts:13:2}" +%s 2>/dev/null) || die "unnamed-time"
	changed=$(find "$1/keyring-test" -printf '%T@\n' | sort -n | tail -n 1) || die "source-time"
	[ "$written" -ge "${changed%.*}" ] || die "stale"
	age -d -i "$BACKUP_IDENTITY" "$work/$name" >"$work/snapshot.tar.gz" 2>/dev/null || die "decrypt"
	python3 "$here/keyring_recovery.py" restore "$work/snapshot.tar.gz" "$1" "$work/recovered" "$2" || die "content"
	recover_keys "$work/recovered" test "$3" || die "key-recovery"
	echo "$name"
)

# in_use <home>: true when a running process names the home with the test backend or
# the home's client.toml selects it.
in_use() {
	local p
	grep -Eqs '^[[:space:]]*keyring-backend[[:space:]]*=[[:space:]]*"test"' "$1/config/client.toml" && return 0
	for p in /proc/[0-9]*/cmdline; do
		tr '\0' ' ' 2>/dev/null <"$p" | grep -F -- "$1" | grep -Eq -- '--keyring-backend[= ]test( |$)' && return 0
	done
	return 1
}

failures=0
k=-1
scratch=""
trap '[ -z "$scratch" ] || rm -rf -- "$scratch"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
for home in "$@"; do
	[ -z "$scratch" ] || rm -rf -- "$scratch"
	scratch=""
	k=$((k + 1))
	home=$(realpath -m -- "$home")
	test_dir="$home/keyring-test"
	if [ ! -d "$test_dir" ]; then
		echo "home[$k] test=absent"
		continue
	fi
	kept() {
		echo "home[$k] keys=$n test=kept reason=$1"
		failures=$((failures + 1))
	}
	n=unknown
	scratch=$(mktemp -d)
	python3 "$here/keyring_recovery.py" snapshot "$test_dir" >"$scratch/source.json" || {
		kept source-content
		continue
	}
	want=$(keys "$home" test) || {
		kept source-enumeration
		continue
	}
	n=$(grep -c . <<<"$want" || true)
	have=$(keys "$home" file) || {
		kept passphrase
		continue
	}
	stop=""
	while read -r name addr; do
		[ -n "$name" ] || continue
		cur=$(awk -v n="$name" '$1 == n {print $2}' <<<"$have")
		[ "$cur" = "$addr" ] && continue
		[ -z "$cur" ] || {
			stop=conflict
			break
		}
		pw | "$paxd" keys export "$name" --home "$home" --keyring-backend test >"$scratch/key.armor" 2>/dev/null || {
			stop=export
			break
		}
		pw | "$paxd" keys import "$name" "$scratch/key.armor" --home "$home" --keyring-backend file >/dev/null 2>&1 || {
			stop=import
			break
		}
		rm -f -- "$scratch/key.armor"
	done <<<"$want"
	[ -z "$stop" ] || {
		kept "$stop"
		continue
	}
	recover_keys "$home" file "$want" || {
		kept file-recovery
		continue
	}
	err="$scratch/backup.err"
	archive=$(verify_backup "$home" "$scratch/source.json" "$want" 2>"$err") || {
		kept "backup:$(tail -n 1 "$err" | sed 's/^backup: //; s/.*: //' | tr ' ' -)"
		rm -f "$err"
		continue
	}
	rm -f "$err"
	if in_use "$home"; then
		kept in-use
		continue
	fi
	current=$(keys "$home" test) || {
		kept source-enumeration
		continue
	}
	[ "$current" = "$want" ] && python3 "$here/keyring_recovery.py" check "$test_dir" "$scratch/source.json" || {
		kept source-changed
		continue
	}
	rm -rf -- "$test_dir"
	echo "home[$k] keys=$n file=match backup=$archive test=removed"
done
[ "$failures" -eq 0 ]
