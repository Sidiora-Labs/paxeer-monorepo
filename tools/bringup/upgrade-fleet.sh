#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# sync-fleet lends the verified release fetch, its ssh helpers and the result
# printers; check-live, which it loads, lends the host map loader.
# shellcheck disable=SC1091
. "$here/sync-fleet.sh"

upgrade_usage() {
	cat <<'USAGE'
usage: tools/bringup/upgrade-fleet.sh [--stage-only|--watch-only] <height>

Carries every node unit of the fleet across a software upgrade. It fetches
paxd-<version>-linux-amd64 and SHA256SUMS of the tagged release from
PAXD_RELEASE_URL and checks the asset against SHA256SUMS through
tools/bringup/sync-fleet.sh's release path, then:

stage   copies the asset beside the running binary of every unit named in
        UPGRADE_UNITS as <binary>.new-<version> and checks its sha256 there.
watch   reads each unit's journal until it logs the upgrade halt line
        'UPGRADE "<name>" NEEDED at height: <height>', then once per unit
        keeps the running binary as <binary>.pre-<version>, moves the staged
        one into place, checks its sha256 and restarts the unit. A unit whose
        running executable is already the target is checked for readiness and
        committed progress without restarting it. Each unit must be active,
        run the target inode and advance beyond its pre-recovery committed
        height and the upgrade halt. Recovery failure stops all later swaps.
        The private journal retains per-unit phases and failure observations;
        reopen reconciles an interrupted mv or restart, including shared paths.

Without a flag it stages and then watches; --stage-only stops after the
stage, --watch-only skips it. Last it prints one status row per unit:
"<UPGRADE_UNITS[k]> <unit> staged|swapped|current|waiting|fail <step>".
A unit is named by its position in UPGRADE_UNITS and its unit name, never
by its destination.

Environment:
  BRINGUP_HOSTS_FILE     the private host map, required; it must also set
                         UPGRADE_UNITS, a list of
                         <destination>,<unit>[,<binary>,<node-root>,<comet-root>] entries, one per
                         node unit; <binary> defaults to PAXD, roots to
                         http://127.0.0.1:8545 and http://127.0.0.1:26657.
                         Units on one host must use their own RPC ports
  PAXD_RELEASE_URL       the base URL of the tagged release assets, required
  PAXD_RELEASE_DIR       where the fetched assets are kept
  PAXD_VERSION           the release version, default the version of
                         tools/release/version.json
  UPGRADE_NAME           the upgrade plan name, default the upgrade of
                         tools/release/version.json
  PAXD                   the default node binary, default /usr/local/bin/paxd
  UPGRADE_WATCH_TIMEOUT  seconds to watch for the halt, default 21600
  UPGRADE_POLL           seconds between reads, 1..30, default 5
  UPGRADE_RECOVERY_TIMEOUT seconds per-unit recovery, 1..3600, default 300
  UPGRADE_JOURNAL        durable private local journal, default
                         /var/lib/paxeer-bringup/upgrade-<version>.json
  BRINGUP_INSPECT_BIN    bringup-inspect installed locally and on every host
  UPGRADE_JOURNAL_LINES  journal lines read per unit, default 2000

Exits 0 when every unit is staged (--stage-only) or swapped or current, 1
when any is not, 2 on a usage error or a release asset that does not match
its SHA256SUMS.
USAGE
}

# halt_logged <upgrade name> <height>: status 0 when the log on stdin carries
# the upgrade halt line for that plan at exactly that height.
halt_logged() {
	grep -F -- "UPGRADE \"$1\" NEEDED at height" | grep -E -- "NEEDED at height:? ?$2([^0-9]|\$)" >/dev/null
}

# stage_unit <k>: copies the verified asset beside unit k's binary and checks
# it there, once per destination and binary.
stage_unit() {
	local k="$1" label="UPGRADE_UNITS[$1]" reply status=0
	local staged="${ubins[$k]}.new-$version" key="${udests[$k]} ${ubins[$k]}"
	if [ -n "${staged_on[$key]:-}" ]; then
		ustatus[k]=staged
		return 0
	fi
	fleet_scp "$bin" "${udests[$k]}:$staged" || status=$?
	if [ "$status" -ne 0 ]; then
		ustatus[k]="fail stage scp=$status"
 upgrade_record fail "unit-$k" stage-transport || return 1
		return 1
	fi
	reply="$(ssh_read "${udests[$k]}" "chmod 0755 $staged && sha256sum $staged")" || status=$?
	if [ "$status" -ne 0 ] || [ "${reply%% *}" != "$want_sha" ]; then
		ustatus[k]="fail verify sha256=${reply%% *} ssh=$status"
 upgrade_record fail "unit-$k" stage-digest || return 1
		return 1
	fi
	staged_on[$key]=1
	ustatus[k]=staged
	say "staged $label ${uunits[$k]} sha256=$want_sha"
}

# Every decision and mutation acknowledgement crosses the installed production
# state machine. Intent is durable before mv/restart; observations are per unit.
upgrade_record() { "$upgrade_inspector" upgrade-state "$upgrade_journal" "$@"; }

upgrade_observe() {
 local k="$1" command
 printf -v command 'set -e; { systemctl show %q -p ActiveState -p MainPID; echo @@journal; journalctl -u %q -n %q --no-pager -o cat; } | %q upgrade-observe %q %q %q %q %q %q' \
  "${uunits[$k]}" "${uunits[$k]}" "$journal_lines" "$upgrade_inspector" "${uunits[$k]}" "${ubins[$k]}" "${unodes[$k]}" "${ucomets[$k]}" "$upgrade_name" "$height"
 timeout 20s ssh -n -o BatchMode=yes -o ConnectTimeout=8 -- "${udests[$k]}" "$command"
}

swap_unit() {
 local k="$1" b="${ubins[$1]}" staged="${ubins[$1]}.new-$version" command
 # A second unit sharing this path never backs up the already installed target.
 printf -v command 'set -e; if [ "$(sha256sum %q | cut -d" " -f1)" != %q ]; then [ "$(sha256sum %q | cut -d" " -f1)" = %q ]; cp -p %q %q; chmod 0755 %q; mv -f %q %q; fi; [ "$(sha256sum %q | cut -d" " -f1)" = %q ]' \
  "$b" "$want_sha" "$staged" "$want_sha" "$b" "$b.pre-$version" "$staged" "$staged" "$b" "$b" "$want_sha"
 timeout 30s ssh -n -o BatchMode=yes -o ConnectTimeout=8 -- "${udests[$k]}" "$command"
}

upgrade_failed() {
 local k="$1" reason="$2"
 upgrade_record fail "unit-$k" "$reason" || return 1
 ustatus[k]="fail $reason evidence=$upgrade_journal"
 return 1
}

recover_unit() {
 local k="$1" observation action changed=0 command
 while :; do
  observation="$(upgrade_observe "$k")" || { upgrade_failed "$k" observation-transport; return 1; }
  action="$(printf '%s' "$observation" | upgrade_record next "unit-$k")" || { upgrade_failed "$k" observation-reconcile; return 1; }
  case "$action" in
  wait | blocked) ustatus[k]=waiting; return 0 ;;
  current)
   if [ "$changed" -eq 1 ]; then
    ustatus[k]="swapped unit=active"
    say "swapped UPGRADE_UNITS[$k] ${uunits[$k]} unit=active"
   else ustatus[k]=current; fi
   return 0 ;;
  swap)
   swap_unit "$k" || { upgrade_failed "$k" swap-transport; return 1; }
   upgrade_record applied "unit-$k" swap || return 1
   changed=1 ;;
  restart)
   printf -v command 'systemctl restart --no-block %q' "${uunits[$k]}"
   timeout 20s ssh -n -o BatchMode=yes -o ConnectTimeout=8 -- "${udests[$k]}" "$command" || { upgrade_failed "$k" restart-transport; return 1; }
   upgrade_record applied "unit-$k" restart || return 1
   changed=1 ;;
  recover) sleep "$poll" ;;
  failed) ustatus[k]="fail recovery evidence=$upgrade_journal"; return 1 ;;
  *) upgrade_failed "$k" invalid-action; return 1 ;;
  esac
 done
}

watch_units() {
 local k pending start=$SECONDS
 for k in "${!udests[@]}"; do
  case "${ustatus[$k]}" in fail*) return 1 ;; esac
  ustatus[k]=waiting
 done
 while :; do
  pending=0
  for k in "${!udests[@]}"; do
   [ "${ustatus[$k]}" = waiting ] || continue
   # Recovery refusal stops every later unit, including ones sharing a binary.
   recover_unit "$k" || return 1
   [ "${ustatus[$k]}" != waiting ] || pending=$((pending+1))
  done
  [ "$pending" -gt 0 ] && [ $((SECONDS-start)) -lt "$watch_timeout" ] || break
  sleep "$poll"
 done
}

# The dispatch below runs only when this file is executed.
[ "${BASH_SOURCE[0]}" = "$0" ] || return 0

stage=1 watch=1
case "${1:-}" in
--stage-only)
	watch=0
	shift
	;;
--watch-only)
	stage=0
	shift
	;;
-h | --help)
	upgrade_usage
	exit 0
	;;
esac
if [ "$#" -ne 1 ] || ! [[ "$1" =~ ^[1-9][0-9]*$ ]]; then
	upgrade_usage >&2
	exit 2
fi
height="$1"
manifest="$here/../../tools/release/version.json"
version="${PAXD_VERSION:-$(sed -n 's/^ *"version": *"\([^"]*\)".*/\1/p' "$manifest")}"
upgrade_name="${UPGRADE_NAME:-$(sed -n 's/^ *"upgrade": *"\([^"]*\)".*/\1/p' "$manifest")}"
watch_timeout="${UPGRADE_WATCH_TIMEOUT:-21600}"
poll="${UPGRADE_POLL:-5}"
journal_lines="${UPGRADE_JOURNAL_LINES:-2000}"
recovery_timeout="${UPGRADE_RECOVERY_TIMEOUT:-300}"
upgrade_inspector="${BRINGUP_INSPECT_BIN:-bringup-inspect}"
upgrade_journal="${UPGRADE_JOURNAL:-/var/lib/paxeer-bringup/upgrade-$version.json}"
if ! [[ "$version" =~ ^[A-Za-z0-9_.-]+$ && "$upgrade_name" =~ ^[A-Za-z0-9_.-]+$ && "$watch_timeout" =~ ^[0-9]+$ && "$poll" =~ ^[1-9][0-9]?$ && "$journal_lines" =~ ^[1-9][0-9]{0,4}$ && "$recovery_timeout" =~ ^[1-9][0-9]{0,3}$ && "$upgrade_inspector" =~ ^[A-Za-z0-9_./-]+$ ]] || [ "$poll" -gt 30 ] || [ "$recovery_timeout" -gt 3600 ]; then
 echo "upgrade-fleet: invalid bounded upgrade input" >&2
 exit 2
fi
if [ -z "$version" ] || [ -z "$upgrade_name" ]; then
	echo "upgrade-fleet: no release version or upgrade name" >&2
	exit 2
fi
for tool in ssh scp timeout curl sha256sum flock "$upgrade_inspector"; do
	if ! command -v "$tool" >/dev/null 2>&1; then
		echo "upgrade-fleet: $tool is required" >&2
		exit 2
	fi
done
load_hosts
if [ -z "${UPGRADE_UNITS:-}" ]; then
	echo "upgrade-fleet: BRINGUP_HOSTS_FILE lacks UPGRADE_UNITS" >&2
	exit 2
fi
release_asset "$version" || exit 2
echo "verified ${bin##*/} sha256=$want_sha upgrade=$upgrade_name height=$height"

declare -a udests uunits ubins unodes ucomets ustatus
declare -A staged_on=()
read -r -a entries <<<"$UPGRADE_UNITS"
for k in "${!entries[@]}"; do
	IFS=, read -r udests[k] uunits[k] ubins[k] unodes[k] ucomets[k] <<<"${entries[$k]}"
	ubins[k]="${ubins[$k]:-$paxd}"
 unodes[k]="${unodes[$k]:-http://127.0.0.1:8545}"
 ucomets[k]="${ucomets[$k]:-http://127.0.0.1:26657}"
	if [ -z "${udests[$k]}" ] || ! [[ "${uunits[$k]}" =~ ^[A-Za-z0-9@_.-]+$ ]]; then
		echo "upgrade-fleet: UPGRADE_UNITS[$k] is not <destination>,<unit>[,<binary>,<node-root>,<comet-root>]" >&2
		exit 2
	fi
	ustatus[k]=pending
done

# The process lock spans observation, durable intent, transport and publication.
umask 077
mkdir -p "$(dirname "$upgrade_journal")"
exec {upgrade_lock}>"$upgrade_journal.lock"
flock -n "$upgrade_lock" || { echo "upgrade-fleet: another owner holds the journal" >&2; exit 1; }
{
 for k in "${!udests[@]}"; do
  printf 'unit-%s %s %s %s %s %s\n' "$k" "${udests[$k]}" "${uunits[$k]}" "${ubins[$k]}" "${unodes[$k]}" "${ucomets[$k]}"
 done
} | upgrade_record begin "$version" "$upgrade_name" "$want_sha" "$height" "$recovery_timeout" || exit 2

if [ "$stage" -eq 1 ]; then
	for k in "${!udests[@]}"; do
		if ! stage_unit "$k"; then break; fi
	done
fi
if [ "$watch" -eq 1 ] && ! watch_units; then
 echo "upgrade-fleet: recovery stopped evidence=$upgrade_journal" >&2
fi

bad=0
printf '%-18s %-20s %s\n' unit name status
for k in "${!udests[@]}"; do
	printf '%-18s %-20s %s\n' "UPGRADE_UNITS[$k]" "${uunits[$k]}" "${ustatus[$k]}"
	case "${ustatus[$k]}" in
	swapped* | current) ;;
	staged) [ "$watch" -eq 0 ] || bad=$((bad + 1)) ;;
	*) bad=$((bad + 1)) ;;
	esac
done
if [ "$bad" -ne 0 ]; then
	echo "upgrade-fleet: $bad unit(s) not upgraded"
	exit 1
fi
echo "upgrade-fleet: all units passed"
