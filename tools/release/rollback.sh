#!/usr/bin/env bash
set -euo pipefail
umask 077

if ! declare -F release_node_process >/dev/null; then
  release_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
  source "$release_dir/stage.sh"
  release_load_identity "$release_dir" || {
    printf 'rollback: invalid release identity\n' >&2
    exit 1
  }
fi

env_file= mode= receipt= phase= local_only=false
expected_release= expected_height=
manager=systemd
readiness=false
rollback_executed=false
forward=false

fail() {
  printf 'rollback: %s\n' "$*" >&2
  exit 1
}
need_value() { [ "$#" -ge 2 ] && [ -n "$2" ] || fail "$1 needs a value"; }
finish() {
  local rc=$? result=fail tmp
  trap - EXIT
  if [ "$rc" -eq 0 ]; then result=pass; fi
  if [ -n "$receipt" ]; then
    tmp=$(mktemp "${receipt}.XXXXXX") || exit 1
    printf '{"readiness":%s,"rollback_executed":%s,"result":"%s"}\n' \
      "$readiness" "$rollback_executed" "$result" >"$tmp"
    mv -fT -- "$tmp" "$receipt" || exit 1
  fi
  exit "$rc"
}
trap finish EXIT

while [ "$#" -gt 0 ]; do
  case $1 in
  --env-file)
    need_value "$@"
    env_file=$2
    shift 2
    ;;
  --receipt)
    need_value "$@"
    receipt=$2
    shift 2
    ;;
  --service-manager)
    need_value "$@"
    manager=$2
    shift 2
    ;;
  --forward)
    forward=true
    shift
    ;;
  --local)
    local_only=true
    shift
    ;;
  --phase)
    need_value "$@"
    phase=$2
    shift 2
    ;;
  --expected-release-id)
    need_value "$@"
    expected_release=$2
    shift 2
    ;;
  --expected-halt-height)
    need_value "$@"
    expected_height=$2
    shift 2
    ;;
  --execute | --readiness | --verify)
    [ -z "$mode" ] || fail "choose exactly one mode"
    mode=${1#--}
    shift
    ;;
  *) fail "unknown argument $1" ;;
  esac
done
[ -n "$mode" ] || fail "choose --execute, --readiness or --verify"
case $manager in systemd | foreground) ;; *) fail "invalid service manager" ;; esac
[ -r "$env_file" ] || fail "--env-file is not readable"

declare -A input=()
while IFS= read -r line || [ -n "$line" ]; do
  line=${line#"${line%%[![:space:]]*}"}
  case $line in '' | \#*) continue ;; esac
  line=${line#export }
  [[ $line =~ ^([A-Z_][A-Z0-9_]*)=(.*)$ ]] || fail "env file is not KEY=VALUE"
  key=${BASH_REMATCH[1]}
  value=${BASH_REMATCH[2]}
  value=${value%"${value##*[![:space:]]}"}
  if [[ $value =~ ^\"(.*)\"$ ]] || [[ $value =~ ^\'(.*)\'$ ]]; then value=${BASH_REMATCH[1]}; fi
  [ -z "${input[$key]+present}" ] || fail "duplicate input $key"
  input[$key]=$value
done <"$env_file"
require() { [ -n "${input[$1]:-}" ] || fail "$1 is empty"; }

hosts=()
read -r -a validator_hosts <<<"${input[VALIDATOR_HOSTS]:-}"
read -r -a fullnode_hosts <<<"${input[FULLNODE_HOSTS]:-}"
if [ -n "${input[ARCHIVE_HOST]:-}" ]; then fullnode_hosts+=("${input[ARCHIVE_HOST]}"); fi
declare -A seen_hosts=()
for host in "${validator_hosts[@]}" "${fullnode_hosts[@]}"; do
  [[ $host =~ ^[a-zA-Z0-9_][a-zA-Z0-9_.@:-]*$ ]] || fail "invalid host"
  if [ -z "${seen_hosts[$host]+present}" ]; then
    hosts+=("$host")
    seen_hosts[$host]=1
  fi
done

remote_phase() {
  local host=$1 step=$2 command
  printf -v command 'bash -s -- --env-file %q --local --%s --service-manager systemd' "$env_file" "$mode"
  printf -v command '%s --expected-release-id %q --expected-halt-height %q' \
    "$command" "${input[RELEASE_ID]}" "${input[HALT_HEIGHT]}"
  if "$forward"; then command+=" --forward"; fi
  if [ -n "$step" ]; then printf -v command '%s --phase %q' "$command" "$step"; fi
  {
    declare -f release_operation_lock release_node_process
    declare -p release_version release_upgrade
    cat -- "${BASH_SOURCE[0]}"
  } | ssh -o BatchMode=yes -o ConnectTimeout=10 -- "$host" "$command"
}

if ! "$local_only" && [ "${#hosts[@]}" -gt 0 ]; then
  require RELEASE_ID
  require HALT_HEIGHT
  [ "$manager" = systemd ] || fail "foreground requires --local"
  [ -z "$phase" ] || fail "--phase requires --local"
  if [ "$mode" = execute ]; then
    for step in prepare stop restore start; do
      rc=0
      for host in "${hosts[@]}"; do remote_phase "$host" "$step" || rc=1; done
      [ "$rc" -eq 0 ] || fail "fleet $step failed; no later phase started"
      if [ "$step" = prepare ]; then readiness=true; fi
    done
    rollback_executed=true
  else
    rc=0
    for host in "${hosts[@]}"; do remote_phase "$host" '' || rc=1; done
    [ "$rc" -eq 0 ] || fail "fleet $mode failed"
    readiness=true
    if [ "$mode" = verify ]; then rollback_executed=true; fi
  fi
  exit 0
fi

for key in RELEASE_ID BIN_DIR SNAPSHOT_DIR HALT_HEIGHT; do require "$key"; done
release_id=${input[RELEASE_ID]}
bin_dir=${input[BIN_DIR]}
snapshot_dir=${input[SNAPSHOT_DIR]}
halt_height=${input[HALT_HEIGHT]}
[ -z "$expected_release" ] || [ "$expected_release" = "$release_id" ] || fail "fleet RELEASE_ID differs"
[ -z "$expected_height" ] || [ "$expected_height" = "$halt_height" ] || fail "fleet HALT_HEIGHT differs"
[[ $release_id =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]*$ ]] || fail "invalid RELEASE_ID"
[[ $halt_height =~ ^[1-9][0-9]{0,17}$ ]] || fail "invalid HALT_HEIGHT"
[[ $bin_dir = /* && $snapshot_dir = /* ]] || fail "directories must be absolute"
bin_dir=$(realpath -e -- "$bin_dir")
snapshot_dir=$(realpath -e -- "$snapshot_dir")
release_operation_lock "$bin_dir" || fail "another release operation is running"
previous=$bin_dir/paxd.pre-$release_id
if "$forward"; then
  require RECOVERY_BIN
  require RECOVERY_SHA256
  previous=${input[RECOVERY_BIN]}
  [[ $previous = /* && ${input[RECOVERY_SHA256]} =~ ^[0-9a-f]{64}$ ]] || fail "invalid forward recovery binary"
  [ "$(realpath -e -- "$previous")" = "$previous" ] || fail "recovery binary must be canonical"
  [ "$previous" != "$bin_dir/paxd" ] && [ "$previous" != "$bin_dir/paxd.new" ] || fail "recovery binary must be staged separately"
fi
homes=() units=() validator_homes=() fullnode_homes=() validator_units=() fullnode_units=()
read -r -a validator_homes <<<"${input[VALIDATOR_HOMES]:-}"
read -r -a fullnode_homes <<<"${input[FULLNODE_HOMES]:-}"
read -r -a validator_units <<<"${input[VALIDATOR_UNITS]:-}"
read -r -a fullnode_units <<<"${input[FULLNODE_UNITS]:-}"
[ "${#validator_homes[@]}" -eq "${#validator_units[@]}" ] || fail "validator homes and units differ"
[ "${#fullnode_homes[@]}" -eq "${#fullnode_units[@]}" ] || fail "full-node homes and units differ"
homes=("${validator_homes[@]}" "${fullnode_homes[@]}")
units=("${validator_units[@]}" "${fullnode_units[@]}")
[ "${#homes[@]}" -gt 0 ] || fail "VALIDATOR_HOMES and FULLNODE_HOMES are empty"
declare -A seen_homes=() seen_units=() seen_archives=()
for i in "${!homes[@]}"; do
  home=${homes[$i]}
  [[ $home = /* && $home != / && $home != *'/../'* && $home != */data."$release_version"-* ]] || fail "invalid home"
  [ "$(realpath -e -- "$home")" = "$home" ] || fail "home must be canonical: $home"
  name=${home##*/}
  unit=${units[$i]}
  [[ $unit =~ ^[a-zA-Z0-9][a-zA-Z0-9_.@:-]*$ ]] || fail "invalid unit"
  [ -z "${seen_homes[$home]+present}${seen_units[$unit]+present}${seen_archives[$name]+present}" ] || fail "duplicate home, unit or archive name"
  for other in "${!seen_homes[@]}"; do
    [[ $home != "$other/"* && $other != "$home/"* ]] || fail "nested homes"
  done
  seen_homes[$home]=1
  seen_units[$unit]=1
  seen_archives[$name]=1
done

sha() { sha256sum -- "$1" | cut -d' ' -f1; }
archive() { printf '%s/%s-pre-%s.tar.zst' "$snapshot_dir" "${1##*/}" "$release_version"; }
record_dir() {
  if "$forward"; then
    printf '%s/.forward-recovery-%s' "$1" "$release_id"
  else
    printf '%s/.rollback-%s' "$1" "$release_id"
  fi
}
set_aside() { printf '%s/data.%s-%s' "$1" "$release_version" "$release_id"; }

check_archive() {
  local file=$1 role=$2
  tar --zstd -tf "$file" >/dev/null || fail "invalid snapshot"
  python3 - "$file" "$halt_height" "$role" <<'PY'
import json
import pathlib
import subprocess
import sys
import tarfile

STORES = ("data/committer.db", "data/state_commit/memiavl")


def store_link(member, name):
    target = pathlib.PurePosixPath(member.linkname)
    return (member.issym() and any(name.startswith(f"{root}/") for root in STORES)
            and member.linkname != "" and not target.is_absolute() and ".." not in target.parts)


def halt_proposal(state, halt):
    if (str(state.get("height")) != str(halt + 1) or state.get("round") != 0 or state.get("step") != 1
            or not isinstance(state.get("signature"), str) or not state["signature"]
            or not isinstance(state.get("signbytes"), str)):
        return False
    try:
        signbytes = bytes.fromhex(state["signbytes"])
    except ValueError:
        return False
    length = shift = offset = 0
    while offset < len(signbytes) and offset < 10:
        byte = signbytes[offset]
        offset += 1
        length |= (byte & 0x7F) << shift
        shift += 7
        if not byte & 0x80:
            break
    else:
        return False
    body = signbytes[offset:]
    return length == len(body) and body[:2] == b"\x08\x20"


with subprocess.Popen(["zstd", "-dc", "--", sys.argv[1]], stdout=subprocess.PIPE) as process:
    try:
        seen = set()
        state = None
        store = False
        with tarfile.open(fileobj=process.stdout, mode="r|") as archive:
            for member in archive:
                path = pathlib.PurePosixPath(member.name)
                name = str(path)
                if (path.is_absolute() or ".." in path.parts or not path.parts
                        or path.parts[0] != "data" or name in seen
                        or not (member.isfile() or member.isdir() or store_link(member, name))):
                    raise ValueError("unsafe snapshot member")
                seen.add(name)
                if member.isfile() and any(name.startswith(f"{root}/") for root in STORES):
                    store = True
                if name == "data/priv_validator_state.json" and member.isfile():
                    if member.size > 65536:
                        raise ValueError("invalid signing state size")
                    state = json.load(archive.extractfile(member))
        if not store or state is None:
            raise ValueError("snapshot lacks application store or signing state")
        height = state.get("height")
        if sys.argv[3] == "validator":
            if str(height) != sys.argv[2] and not halt_proposal(state, int(sys.argv[2])):
                raise ValueError("snapshot signing height differs from HALT_HEIGHT")
        elif not str(height).isdigit() or not 0 <= int(height) <= int(sys.argv[2]):
            raise ValueError("snapshot signing state is after HALT_HEIGHT")
        process.stdout.close()
        if process.wait() != 0:
            raise ValueError("snapshot decompression failed")
    except (ValueError, KeyError, tarfile.TarError, OSError) as error:
        process.stdout.close()
        process.wait()
        sys.exit("rollback: " + str(error))
PY
}

matching_pids() {
  release_node_process list "$1"
}

check_stopped() {
  local home i state
  for i in "${!homes[@]}"; do
    home=${homes[$i]}
    release_node_process stopped "$home" || fail "node ownership is not stopped for $home"
    release_node_process stopped "$(set_aside "$home")" || fail "set-aside data is not stopped"
    if [ "$manager" = systemd ]; then
      state=$(systemctl show --property=ActiveState --value "${units[$i]}")
      case $state in inactive | failed) ;; *) fail "unit is not stopped" ;; esac
    fi
  done
}

check_material() {
  local i home file role
  [ -f "$previous" ] && [ -x "$previous" ] && [ ! -L "$previous" ] || fail "no previous binary"
  previous_sha=$(sha "$previous")
  if "$forward"; then
    [ "$previous_sha" = "${input[RECOVERY_SHA256]}" ] || fail "forward recovery checksum mismatch"
    readiness=true
    return
  fi
  for i in "${!homes[@]}"; do
    home=${homes[$i]}
    file=$(archive "$home")
    role=full
    if [ "$i" -lt "${#validator_homes[@]}" ]; then role=validator; fi
    [ -f "$file" ] && [ -r "$file" ] || fail "no snapshot for $home"
    check_archive "$file" "$role"
  done
  readiness=true
}

check_binary() {
  local binary=$1 libraries
  [ "$(sha "$binary")" = "$previous_sha" ] || fail "previous binary checksum mismatch"
  libraries=$(ldd "$binary" 2>&1) || fail "previous binary dependency check failed"
  [[ $libraries != *'not found'* ]] || fail "previous binary has unresolved libraries"
}

prepare() {
  local home path
  check_stopped
  check_material
  check_binary "$previous"
  for home in "${homes[@]}"; do
    [ -d "$home/data" ] && [ ! -L "$home/data" ] || fail "invalid data directory for $home"
    path=$(record_dir "$home")
    if "$forward"; then
      check_signing_state "$home/data/priv_validator_state.json" "$home/data/priv_validator_state.json"
      if [ -e "$path" ] || [ -L "$path" ]; then
        [ -d "$path" ] && [ ! -L "$path" ] || fail "invalid forward recovery journal"
        [ "$(cat "$path/material")" = "$previous_sha" ] || fail "forward recovery material changed"
        check_signing_state "$path/priv_validator_state.json" "$home/data/priv_validator_state.json"
      fi
    else
      [ ! -e "$path" ] && [ ! -L "$path" ] || fail "rollback destination already exists for $home"
    fi
    if ! "$forward"; then
      [ ! -e "$(set_aside "$home")" ] && [ ! -L "$(set_aside "$home")" ] || fail "retained data already exists"
    fi
    [ -f "$home/config/app.toml" ] && [ ! -L "$home/config/app.toml" ] || fail "invalid app config"
    [ "$(grep -cE '^[[:space:]]*halt-height[[:space:]]*=' "$home/config/app.toml")" -eq 1 ] || fail "expected one halt-height"
    python3 - "$home/config/app.toml" <<'PY'
import sys
import tomllib

with open(sys.argv[1], "rb") as stream:
    config = tomllib.load(stream)
height = config.get("halt-height")
if type(height) is not int or height < 0:
    sys.exit("rollback: invalid halt-height")
PY
  done
  if "$forward"; then
    [ ! -L "$bin_dir/paxd.new" ] || fail "invalid staged recovery binary"
    [ ! -e "$bin_dir/paxd.new" ] || [ -f "$bin_dir/paxd.new" ] || fail "invalid staged recovery binary"
  else
    [ ! -e "$bin_dir/paxd.new" ] && [ ! -L "$bin_dir/paxd.new" ] || fail "paxd.new already exists"
  fi
}

stop_nodes() {
  local unit home pid attempt rc=0
  if [ "$manager" = systemd ]; then
    read -r -a sidecars <<<"${input[SIDECAR_UNITS]:-}"
    for unit in "${sidecars[@]}" "${units[@]}"; do systemctl stop -- "$unit" || rc=1; done
    for unit in "${units[@]}"; do
      state=$(systemctl show --property=ActiveState --value "$unit") || rc=1
      case $state in inactive | failed) ;; *) rc=1 ;; esac
    done
  else
    [ -z "${input[SIDECAR_UNITS]:-}" ] || fail "foreground cannot manage sidecar units"
    for home in "${homes[@]}"; do
      release_node_process stop "$home" || rc=1
    done
  fi
  [ "$rc" -eq 0 ] || fail "could not stop every unit"
  for ((attempt = 0; attempt < 120; attempt++)); do
    rc=0
    for home in "${homes[@]}"; do release_node_process stopped "$home" || rc=1; done
    if [ "$rc" -eq 0 ]; then
      check_stopped
      return
    fi
    sleep 1
  done
  fail "nodes did not stop"
}

# A forward recovery binary must understand the already committed application state.
# Its explicit checksum is an operator selection, never an automatic downgrade.
check_signing_state() {
  python3 - "$1" "$2" <<'PY_SIGNING'
import json
import sys


def mark(state):
    height = state["height"]
    if not isinstance(height, str) or not height.isascii() or not height.isdecimal():
        raise ValueError("invalid signing height")
    if any(type(state[k]) is not int or state[k] < 0 for k in ("round", "step")):
        raise ValueError("invalid signing round or step")
    return int(height), state["round"], state["step"]


try:
    before, after = [json.load(open(path)) for path in sys.argv[1:]]
    if mark(after) < mark(before) or (mark(after) == mark(before) and after != before):
        raise ValueError("validator signing high-water mark regressed")
except (ValueError, KeyError, OSError) as error:
    sys.exit("rollback: " + str(error))
PY_SIGNING
}

check_rewind() {
  local home=$1
  python3 - "$home" "$(archive "$home")" "${input[ROLLBACK_ISOLATED_FORK]:-false}" \
    "${input[ROLLBACK_SOURCE_GENESIS]:-}" <<'PY_REWIND'
import ipaddress
import json
from pathlib import Path
import subprocess
import sys
import tarfile
import tomllib
from urllib.parse import urlsplit


def loopback(address):
    parts = urlsplit(address)
    return parts.hostname is not None and ipaddress.ip_address(parts.hostname).is_loopback


try:
    home = Path(sys.argv[1])
    live = json.loads((home / "data/priv_validator_state.json").read_text())
    with subprocess.Popen(["zstd", "-dc", "--", sys.argv[2]], stdout=subprocess.PIPE) as process:
        with tarfile.open(fileobj=process.stdout, mode="r|") as archive:
            snapshot = None
            for member in archive:
                if str(Path(member.name)) == "data/priv_validator_state.json":
                    snapshot = json.load(archive.extractfile(member))
        if process.wait() != 0 or snapshot is None:
            raise ValueError("snapshot signing state unavailable")
    def mark(state):
        height = state["height"]
        if not isinstance(height, str) or not height.isascii() or not height.isdecimal():
            raise ValueError("invalid signing height")
        if any(type(state[k]) is not int or state[k] < 0 for k in ("round", "step")):
            raise ValueError("invalid signing round or step")
        return int(height), state["round"], state["step"]
    rewinds = mark(live) > mark(snapshot) or (mark(live) == mark(snapshot) and live != snapshot)
    if sys.argv[3] != "true":
        raise ValueError(("post-halt signing: " if rewinds else "") +
                         "same-chain snapshot restore refused; use --forward with a compatible recovery binary")
    source = json.loads(Path(sys.argv[4]).read_text())
    genesis = json.loads((home / "config/genesis.json").read_text())
    if genesis["chain_id"] == source["chain_id"]:
        raise ValueError("isolated fork must use a different consensus chain identity")
    source_keys = {v["consensus_pubkey"]["key"] for v in source["app_state"]["staking"]["validators"]}
    source_keys.update(v["pub_key"]["value"] for v in source.get("validators", []))
    fork_keys = {v["pub_key"]["value"] for v in genesis["validators"]}
    local = json.loads((home / "config/priv_validator_key.json").read_text())["pub_key"]["value"]
    if not source_keys or not fork_keys or fork_keys & source_keys or local not in fork_keys:
        raise ValueError("isolated fork requires fresh consensus identities for every validator")
    config = tomllib.loads((home / "config/config.toml").read_text())
    if not loopback(config["rpc"]["laddr"]) or not loopback(config["p2p"]["laddr"]):
        raise ValueError("isolated fork listeners must be loopback")
    p2p = config["p2p"]
    if p2p.get("pex", True) or any(p2p.get(k, "") for k in ("external-address", "bootstrap-peers", "seeds")):
        raise ValueError("isolated fork peer discovery must be disabled")
    for peer in p2p.get("persistent-peers", "").split(","):
        if peer and not loopback("tcp://" + peer.split("@", 1)[1]):
            raise ValueError("isolated fork peers must be loopback")
except (ValueError, KeyError, IndexError, TypeError, OSError, tarfile.TarError) as error:
    sys.exit("rollback: " + str(error))
PY_REWIND
}

restore_forward() {
  local home record
  check_stopped
  check_material
  check_binary "$previous"
  check_stopped
  # The journal is written before the atomic binary switch; rerunning this phase
  # after interruption reuses it and never replaces the current data directory.
  for home in "${homes[@]}"; do
    record=$(record_dir "$home")
    [ -d "$home/data" ] && [ ! -L "$home/data" ] || fail "invalid live data"
    check_signing_state "$home/data/priv_validator_state.json" "$home/data/priv_validator_state.json"
    if [ -e "$record" ]; then
      [ -d "$record" ] && [ ! -L "$record" ] || fail "invalid forward recovery journal"
      [ "$(cat "$record/material")" = "$previous_sha" ] || fail "forward recovery material changed"
      check_signing_state "$record/priv_validator_state.json" "$home/data/priv_validator_state.json"
    fi
    [ -f "$home/config/app.toml" ] && [ ! -L "$home/config/app.toml" ] || fail "invalid app config"
    [ "$(grep -cE '^[[:space:]]*halt-height[[:space:]]*=' "$home/config/app.toml")" -eq 1 ] || fail "expected one halt-height"
  done
  for home in "${homes[@]}"; do
    record=$(record_dir "$home")
    if [ ! -e "$record" ]; then
      if [ -e "$record.tmp" ]; then
        [ -d "$record.tmp" ] && [ ! -L "$record.tmp" ] || fail "invalid pending recovery journal"
        [ ! -L "$record.tmp/priv_validator_state.json" ] && [ ! -L "$record.tmp/material" ] || fail "invalid pending journal files"
      else
        mkdir -- "$record.tmp"
      fi
      cp -p -- "$home/data/priv_validator_state.json" "$record.tmp/priv_validator_state.json"
      printf '%s\n' "$previous_sha" >"$record.tmp/material"
      sync -f "$record.tmp/priv_validator_state.json"
      sync -f "$record.tmp/material"
      sync -f "$record.tmp"
      mv -T -- "$record.tmp" "$record"
      sync -f "$home"
    fi
    sed -i -E 's/^[[:space:]]*halt-height[[:space:]]*=.*/halt-height = 0/' "$home/config/app.toml"
  done
  [ ! -L "$bin_dir/paxd.new" ] || fail "invalid staged recovery binary"
  cp -p -- "$previous" "$bin_dir/paxd.new"
  check_binary "$bin_dir/paxd.new"
  mv -fT -- "$bin_dir/paxd.new" "$bin_dir/paxd"
  for home in "${homes[@]}"; do : >"$(record_dir "$home")/restored"; done
}

restore() {
  local home record file
  if "$forward"; then
    restore_forward
    return
  fi
  prepare
  check_stopped
  # Check every home before moving any data or replacing the binary.
  for home in "${homes[@]}"; do check_rewind "$home"; done
  for home in "${homes[@]}"; do
    file=$(archive "$home")
    record=$(record_dir "$home")
    mkdir -- "$record"
    mv -T -- "$home/data" "$(set_aside "$home")"
    tar --zstd -xf "$file" -C "$home" --no-same-owner
    tar --zstd -df "$file" -C "$home"
    cp -p -- "$home/data/priv_validator_state.json" "$record/priv_validator_state.json"
    rm -f -- "$home/data/upgrade-info.json"
    sed -i -E 's/^[[:space:]]*halt-height[[:space:]]*=.*/halt-height = 0/' "$home/config/app.toml"
    printf '%s\n%s\n%s\n' "$previous_sha" "$(sha "$file")" "$halt_height" >"$record/material"
  done
  (
    set -o noclobber
    : >"$bin_dir/paxd.new"
  )
  cp -p -- "$previous" "$bin_dir/paxd.new"
  check_binary "$bin_dir/paxd.new"
  mv -fT -- "$bin_dir/paxd.new" "$bin_dir/paxd"
  check_binary "$bin_dir/paxd"
  for home in "${homes[@]}"; do : >"$(record_dir "$home")/restored"; done
}

check_restored() {
  local home record want
  check_material
  check_binary "$bin_dir/paxd"
  if "$forward"; then
    for home in "${homes[@]}"; do
      record=$(record_dir "$home")
      [ -f "$record/restored" ] && [ ! -L "$record" ] || fail "no completed forward recovery"
      [ "$(cat "$record/material")" = "$previous_sha" ] || fail "forward recovery material changed"
      check_signing_state "$record/priv_validator_state.json" "$home/data/priv_validator_state.json"
      grep -qx 'halt-height = 0' "$home/config/app.toml" || fail "halt-height is not zero"
    done
    return
  fi
  for home in "${homes[@]}"; do
    record=$(record_dir "$home")
    [ -d "$(set_aside "$home")" ] && [ ! -L "$(set_aside "$home")" ] || fail "no retained data for $home"
    [ -d "$home/data" ] && [ ! -L "$home/data" ] || fail "invalid restored data"
    [ -f "$record/restored" ] && [ ! -L "$record" ] || fail "no completed restore for $home"
    want=$(printf '%s\n%s\n%s' "$previous_sha" "$(sha "$(archive "$home")")" "$halt_height")
    [ "$(cat "$record/material")" = "$want" ] || fail "restored material changed"
    [ ! -e "$home/data/upgrade-info.json" ] && [ ! -L "$home/data/upgrade-info.json" ] || fail "upgrade file remains"
    grep -qx 'halt-height = 0' "$home/config/app.toml" || fail "halt-height is not zero"
    release_node_process stopped "$(set_aside "$home")" || fail "set-aside data is not stopped"
  done
}

start_nodes() {
  local i home
  check_stopped
  check_restored
  for i in "${!homes[@]}"; do
    home=${homes[$i]}
    if [ "$manager" = systemd ]; then
      systemctl start -- "${units[$i]}"
      systemctl is-active --quiet "${units[$i]}"
    else
      nohup "$bin_dir/paxd" start --home "$home" >"$(record_dir "$home")/node.log" 2>&1 </dev/null 9>&- &
    fi
    : >"$(record_dir "$home")/started"
  done
  rollback_executed=true
}

verify_nodes() {
  local home pids pid before after attempt endpoint
  check_restored
  for home in "${homes[@]}"; do
    [ -f "$(record_dir "$home")/started" ] || fail "rollback did not restart $home"
    endpoint=$(
      python3 - "$home/config/config.toml" <<'PY'
import sys
import tomllib
from urllib.parse import urlsplit

with open(sys.argv[1], "rb") as stream:
    endpoint = tomllib.load(stream)["rpc"]["laddr"]
parts = urlsplit(endpoint)
if parts.scheme not in ("tcp", "http", "https") or not parts.hostname or not parts.port:
    sys.exit("rollback: invalid consensus RPC address")
host = parts.hostname
if host in ("0.0.0.0", "::"):
    host = "localhost"
if ":" in host:
    host = "[" + host + "]"
print(("https" if parts.scheme == "https" else "http") + "://" + host + ":" + str(parts.port))
PY
    )
    before=
    for ((attempt = 0; attempt < 120; attempt++)); do
      pids=$(matching_pids "$home") || fail "process inspection failed"
      if [[ $pids =~ ^[0-9]+$ ]]; then
        pid=$pids
        [ "$(sha "/proc/$pid/exe")" = "$previous_sha" ] || fail "running binary checksum mismatch"
        if after=$(curl --fail --silent --show-error --max-time 3 -H 'Content-Type: application/json' \
          --data '{"jsonrpc":"2.0","id":1,"method":"status","params":{}}' "$endpoint" 2>/dev/null |
          jq -er '.result.sync_info | select(.catching_up == false) | .latest_block_height | select(test("^[0-9]{1,18}$"))'); then
          if [ -n "$before" ] && [ "$after" -gt "$before" ] && [ "$after" -gt "$halt_height" ]; then break; fi
          if [ -z "$before" ]; then before=$after; fi
        fi
      elif [ -n "$pids" ]; then
        fail "multiple processes for $home"
      fi
      sleep 1
    done
    [ "$attempt" -lt 120 ] || fail "chain height did not rise past HALT_HEIGHT for $home"
    if ! "$forward"; then
      [ ! -e "$home/data/upgrade-info.json" ] || fail "upgrade file reappeared"
    fi
  done
  rollback_executed=true
}

case $mode in
readiness)
  [ -z "$phase" ] || fail "phase requires --execute"
  check_material
  if "$forward"; then
    for home in "${homes[@]}"; do
      check_signing_state "$home/data/priv_validator_state.json" "$home/data/priv_validator_state.json"
    done
  fi
  ;;
verify)
  [ -z "$phase" ] || fail "phase requires --execute"
  verify_nodes
  ;;
execute)
  if [ -n "$phase" ]; then
    "$local_only" || fail "phase requires --local"
    case $phase in
    prepare) prepare ;;
    stop) stop_nodes ;;
    restore) restore ;;
    start) start_nodes ;;
    *) fail "invalid phase" ;;
    esac
  else
    prepare
    stop_nodes
    restore
    start_nodes
  fi
  ;;
esac
