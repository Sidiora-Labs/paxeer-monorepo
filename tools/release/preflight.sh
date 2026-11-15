#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
version_file=$script_dir/version.json

usage() {
  cat <<'USAGE'
Usage: preflight.sh --home DIRECTORY --binary FILE [--upgrade-info FILE] [--min-free-gib INTEGER]
Read-only host checks for Paxeer before the v7.0 upgrade.
Requires Linux and Python 3.11 or newer; the release binary targets linux/amd64.
  --home           Node home containing config/ and data/.
  --binary         New executable; also stage it in cosmovisor/upgrades/v7.0/bin/paxd.
  --upgrade-info   Optional release JSON containing the linux/amd64 SHA-256 checksum URL.
  --min-free-gib   Free space must exceed this many GiB on every configured data volume (default 10).
  PAXEER_CLOCK_PROBE names an optional executable clock probe. It must print
                   synchronised or not synchronised; the kernel adjtimex probe is the default.
  -h, --help       Print this usage.
USAGE
}

home= binary= upgrade_info= min_free_gib=10
while (($#)); do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --home|--binary|--upgrade-info|--min-free-gib)
      if (($# < 2)) || [[ -z $2 || $2 == --* ]]; then
        printf 'options: missing option value\n' >&2
        exit 1
      fi
      case "$1" in
        --home) home=$2 ;;
        --binary) binary=$2 ;;
        --upgrade-info) upgrade_info=$2 ;;
        --min-free-gib) min_free_gib=$2 ;;
      esac
      shift 2
      ;;
    *) printf 'options: unknown option\n' >&2; exit 1 ;;
  esac
done
if [[ -z $home || -z $binary || ! $min_free_gib =~ ^[0-9]+$ ]]; then
  usage >&2
  exit 1
fi
if ! command -v python3 >/dev/null; then
  printf 'preflight: Python 3.11 or newer is required\n' >&2
  exit 1
fi

exec python3 - "$home" "$binary" "$upgrade_info" "$min_free_gib" "$version_file" <<'PY'
import ctypes
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import tomllib
from urllib.parse import parse_qs, urlsplit

version_file = Path(sys.argv[5]).absolute()
try:
    identity = json.loads(version_file.read_text(encoding="utf-8"))
    if (not isinstance(identity, dict) or set(identity) != {"upgrade", "version"}
            or identity.get("upgrade") != "v7.0"
            or not isinstance(identity.get("version"), str)
            or not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", identity["version"])):
        raise ValueError
except (OSError, ValueError, TypeError, json.JSONDecodeError):
    print("preflight: invalid release identity in tools/release/version.json", file=sys.stderr)
    raise SystemExit(1)
RELEASE_VERSION = identity["version"]
RELEASE_VERSION_UNPREFIXED = RELEASE_VERSION.removeprefix("v")
UPGRADE = "v7.0"
VERSIONS = (RELEASE_VERSION_UNPREFIXED, RELEASE_VERSION)
APP_KEYS = {
    "state-commit": {"sc-enable": bool, "sc-directory": str},
    "state-store": {
        "ss-enable": bool, "ss-db-directory": str, "evm-ss-db-directory": str,
    },
    "state-sync": {"snapshot-interval": int, "snapshot-keep-recent": int},
    "sequencer": {
        "enable": bool, "role": str, "mode": str, "wal-dir": str,
        "receipt-key-file": str, "standby-address": str,
        "replication-timeout": str, "replication-tls-cert": str,
        "replication-tls-key": str, "replication-tls-ca": str,
        "replication-peer-name": str, "replication-peer-spki": str,
        "wal-retain-blocks": int, "nonce-queue-max-per-sender": int,
        "nonce-queue-max-total": int, "lease-duration": str,
    },
}
home = Path(sys.argv[1]).absolute()
binary = Path(sys.argv[2]).absolute()
upgrade_info = sys.argv[3]
minimum = int(sys.argv[4]) * 1024**3
app = consensus = None
binary_digest = None
failed = False


class Refusal(Exception):
    pass


def require(condition, reason):
    if not condition:
        raise Refusal(reason)


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def check(label, operation):
    global failed
    try:
        detail = operation()
        print(f"{label}: {detail + '; ' if detail else ''}ok")
    except Refusal as error:
        failed = True
        print(f"{label}: {error}")
    except (OSError, ValueError, TypeError, KeyError, subprocess.SubprocessError):
        failed = True
        print(f"{label}: cannot read or validate required input")


def check_binary():
    global binary_digest
    require(binary.is_file() and os.access(binary, os.X_OK), "new binary is missing or not executable")
    result = subprocess.run(
        [str(binary), "version", "--long", "--output", "json"],
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=15,
    )
    require(result.returncode == 0, "new binary version command failed")
    version = json.loads(result.stdout)
    require(isinstance(version, dict) and version.get("version") in VERSIONS,
            f"new binary must report version {RELEASE_VERSION_UNPREFIXED}")
    binary_digest = digest(binary)


def check_digest():
    if not upgrade_info:
        return "upgrade information not supplied"
    require(binary_digest is not None, "new binary could not be validated")
    document = json.loads(Path(upgrade_info).read_bytes())
    require(isinstance(document, dict), "upgrade information must be an object")
    if "name" in document:
        require(document["name"] == UPGRADE, "upgrade information plan must be v7.0")
    info = document.get("info", document)
    if isinstance(info, str):
        info = json.loads(info)
    require(isinstance(info, dict) and isinstance(info.get("binaries"), dict),
            "upgrade information has no binary checksums")
    url = info["binaries"].get("linux/amd64")
    require(isinstance(url, str), "upgrade information has no linux/amd64 binary")
    checksums = parse_qs(urlsplit(url).query).get("checksum", [])
    require(len(checksums) == 1 and re.fullmatch(r"sha256:[0-9a-fA-F]{64}", checksums[0]),
            "upgrade information requires one SHA-256 checksum")
    require(binary_digest == checksums[0][7:].lower(), "new binary SHA-256 differs from upgrade information")


def check_layout():
    staged = home / "cosmovisor" / "upgrades" / UPGRADE / "bin" / "paxd"
    require(staged.is_file() and os.access(staged, os.X_OK),
            "v7.0 cosmovisor binary is missing or not executable")
    require(binary_digest is not None, "new binary could not be validated")
    require(digest(staged) == binary_digest, "v7.0 cosmovisor binary differs from new binary")


def check_configuration():
    global app, consensus
    with (home / "config" / "app.toml").open("rb") as stream:
        loaded_app = tomllib.load(stream)
    with (home / "config" / "config.toml").open("rb") as stream:
        loaded_consensus = tomllib.load(stream)
    for section, keys in APP_KEYS.items():
        table = loaded_app.get(section)
        require(isinstance(table, dict), f"app.toml missing [{section}]")
        for key, kind in keys.items():
            require(key in table, f"app.toml missing {section}.{key}")
            require(type(table[key]) is kind, f"app.toml invalid type for {section}.{key}")
    require(loaded_app["state-commit"]["sc-enable"] is True,
            "state-commit.sc-enable must be true")
    for key in ("db-dir", "node-key-file"):
        require(isinstance(loaded_consensus.get(key), str) and loaded_consensus[key],
                f"config.toml missing or invalid {key}")
    validator = loaded_consensus.get("priv-validator")
    require(isinstance(validator, dict) and isinstance(validator.get("key-file"), str)
            and validator["key-file"], "config.toml missing or invalid priv-validator.key-file")
    app, consensus = loaded_app, loaded_consensus


def check_key(kind):
    require(consensus is not None, "node configuration could not be validated")
    value = consensus["priv-validator"]["key-file"] if kind == "consensus" else consensus["node-key-file"]
    path = home / value
    require(path.is_file(), "key file is missing or not a regular file")
    mode = path.stat().st_mode
    require(mode & stat.S_IRUSR and not mode & (stat.S_IRWXG | stat.S_IRWXO),
            "key file must be owner-readable with no group or other permissions")
    require(os.access(path, os.R_OK), "key file is not readable")


def check_disk():
    require(app is not None and consensus is not None, "node configuration could not be validated")
    paths = [home / "data", home / consensus["db-dir"]]
    for section, key in (
        ("state-commit", "sc-directory"), ("state-store", "ss-db-directory"),
        ("state-store", "evm-ss-db-directory"),
    ):
        value = app[section][key]
        if value:
            path = Path(value)
            require(path.is_absolute(), "configured data directories must be absolute")
            paths.append(path)
    for path in paths:
        require(path.is_dir(), "data directory is missing or not a directory")
        volume = os.statvfs(path)
        require(volume.f_bavail * volume.f_frsize > minimum,
                "available data-volume space must exceed minimum GiB")


def check_clock():
    probe = os.environ.get("PAXEER_CLOCK_PROBE")
    if probe is not None:
        require(probe, "cannot read kernel clock synchronisation status")
        result = subprocess.run(
            [probe], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=15,
        )
        require(result.returncode == 0, "cannot read kernel clock synchronisation status")
        status = result.stdout.strip().lower()
        require(status in ("synchronised", "not synchronised"),
                "cannot read kernel clock synchronisation status")
        require(status == "synchronised", "host clock is not synchronised")
        return
    require(sys.platform == "linux", "Linux clock synchronisation status is required")
    libc = ctypes.CDLL(None, use_errno=True)
    query = libc.adjtimex
    query.argtypes = [ctypes.c_void_p]
    query.restype = ctypes.c_int
    # modes=0 reads kernel NTP status; the zeroed buffer accommodates struct timex.
    timex = ctypes.create_string_buffer(4096)
    state = query(timex)
    require(state >= 0, "cannot read kernel clock synchronisation status")
    require(state in (0, 1, 2, 3, 4), "host clock is not synchronised")


check("binary", check_binary)
check("digest", check_digest)
check("layout", check_layout)
check("configuration", check_configuration)
check("consensus-key", lambda: check_key("consensus"))
check("node-key", lambda: check_key("node"))
check("disk", check_disk)
check("clock", check_clock)
sys.exit(1 if failed else 0)
PY
