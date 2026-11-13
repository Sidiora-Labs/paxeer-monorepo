#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
umask 077

python3 - "$root" "$work" <<'PY'
import ctypes
import errno
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tomllib

root, work = map(Path, sys.argv[1:])
script = root / "tools/release/preflight.sh"
labels = ("binary", "digest", "layout", "configuration", "consensus-key", "node-key", "disk", "clock")
secret = "private-key-material"
app_text = '''[state-commit]
sc-enable = true
sc-directory = ""
[state-store]
ss-enable = true
ss-db-directory = ""
evm-ss-db-directory = ""
[state-sync]
snapshot-interval = 100
snapshot-keep-recent = 2
[sequencer]
enable = false
role = "primary"
mode = "sequential"
wal-dir = "wal"
receipt-key-file = "receipt.key"
standby-address = ""
replication-timeout = "1s"
replication-tls-cert = ""
replication-tls-key = ""
replication-tls-ca = ""
replication-peer-name = ""
replication-peer-spki = ""
wal-retain-blocks = 2
nonce-queue-max-per-sender = 10
nonce-queue-max-total = 100
lease-duration = "3s"
'''
config_text = '''db-dir = "data"
node-key-file = "config/node_key.json"
[priv-validator]
key-file = "config/priv_validator_key.json"
'''


def write_binary(path, version="v7.0.0", code=0):
    path.write_text(
        '#!/usr/bin/env bash\n'
        '[[ "$*" == "version --long --output json" ]] || exit 2\n'
        f"printf '%s\\n' '{json.dumps({'version': version})}'\n"
        f"exit {code}\n"
    )
    path.chmod(0o755)


def fixture(name):
    directory = work / name
    directory.mkdir()
    home = directory / "node home"
    (home / "config").mkdir(parents=True)
    (home / "data").mkdir()
    (home / "config/app.toml").write_text(app_text)
    (home / "config/config.toml").write_text(config_text)
    for key in ("priv_validator_key.json", "node_key.json"):
        path = home / "config" / key
        path.write_text(secret)
        path.chmod(0o600)
    binary = directory / "new paxd"
    write_binary(binary)
    staged = home / "cosmovisor/upgrades/v7.0/bin/paxd"
    staged.parent.mkdir(parents=True)
    shutil.copy2(binary, staged)
    info = directory / "upgrade-info.json"
    checksum = hashlib.sha256(binary.read_bytes()).hexdigest()
    document = {"name": "v7.0", "info": json.dumps({"binaries": {
        "linux/amd64": f"https://example.invalid/paxd?checksum=sha256:{checksum}"
    }})}
    info.write_text(json.dumps(document))
    return home, binary, staged, info


def manifest(directory):
    return {
        str(path.relative_to(directory)): (
            path.stat().st_mode,
            path.stat().st_mtime_ns,
            os.readlink(path) if path.is_symlink() else
            hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None,
        ) for path in directory.rglob("*")
    }


def probe(answer):
    path = work / ("clock probe " + answer.replace(" ", "-"))
    if not path.exists():
        path.write_text(f"#!/usr/bin/env bash\nprintf '%s\\n' {json.dumps(answer)}\n")
        path.chmod(0o755)
    return path


def invoke(case, expected=0, lines=None, minimum="0", include_info=True, preexec=None,
           clock_answer="synchronised"):
    home, binary, staged, info = case
    before = manifest(home.parent)
    args = ["bash", str(script), "--home", str(home), "--binary", str(binary),
            "--min-free-gib", minimum]
    if include_info:
        args += ["--upgrade-info", str(info)]
    environment = os.environ.copy()
    if clock_answer is not None:
        environment["PAXEER_CLOCK_PROBE"] = str(probe(clock_answer))
    else:
        environment.pop("PAXEER_CLOCK_PROBE", None)
    result = subprocess.run(args, capture_output=True, text=True, preexec_fn=preexec,
                            env=environment)
    assert result.returncode == expected, (result.returncode, result.stdout, result.stderr)
    assert not result.stderr, result.stderr
    output = result.stdout.splitlines()
    assert len(output) == len(labels), output
    assert [line.split(":", 1)[0] for line in output] == list(labels), output
    for label, reason in (lines or {}).items():
        assert f"{label}: {reason}" in output, output
    if expected == 0:
        assert all(line.endswith("ok") for line in output), output
    assert secret not in result.stdout + result.stderr
    assert str(home) not in result.stdout + result.stderr
    assert manifest(home.parent) == before, "preflight changed node files"
    return output


def passing_home_is_unchanged():
    case = fixture("passing")
    assert invoke(case, clock_answer="synchronised") == [f"{label}: ok" for label in labels]
    invoke(case, include_info=False, lines={"digest": "upgrade information not supplied; ok"},
           clock_answer="synchronised")
    case[3].write_text(json.loads(case[3].read_text())["info"])
    invoke(case, clock_answer="synchronised")
    (case[0] / "config/priv_validator_key.json").chmod(0o400)
    invoke(case, clock_answer="synchronised")


def release_plan_matches_sources():
    assert 'const MergeUpgrade = "v7.0"' in (root / "precompiles/common/precompiles.go").read_text()
    assert "const MergeUpgrade = pcommon.MergeUpgrade" in (root / "node/upgrades.go").read_text()
    assert json.loads((root / "tools/release/version.json").read_text()) == {
        "upgrade": "v7.0", "version": "v7.0.0",
    }


def binary_version_and_execution_are_required():
    for index, version in enumerate(("v6.12.0", "v7.0.0-dev", "17.0.0")):
        case = fixture(f"version-{index}")
        write_binary(case[1], version)
        invoke(case, 1, {"binary": "new binary must report version 7.0.0"})
    case = fixture("execution")
    write_binary(case[1], code=1)
    invoke(case, 1, {"binary": "new binary version command failed"})
    case[1].chmod(0o644)
    invoke(case, 1, {"binary": "new binary is missing or not executable"})


def digest_must_match_upgrade_information():
    case = fixture("digest")
    document = json.loads(case[3].read_text())
    info = json.loads(document["info"])
    info["binaries"]["linux/amd64"] = "https://example.invalid/paxd?checksum=sha256:" + "0" * 64
    document["info"] = json.dumps(info)
    case[3].write_text(json.dumps(document))
    invoke(case, 1, {"digest": "new binary SHA-256 differs from upgrade information"})
    for value in ({}, {"binaries": {}}, {"binaries": {"linux/amd64": "https://example.invalid/paxd"}}):
        case[3].write_text(json.dumps(value))
        output = invoke(case, 1)
        assert not output[1].endswith("ok"), output
    case[3].write_text(secret)
    invoke(case, 1, {"digest": "cannot read or validate required input"})
    case[3].unlink()
    invoke(case, 1, {"digest": "cannot read or validate required input"})


def staged_binary_must_match_plan_and_candidate():
    case = fixture("layout")
    case[2].rename(case[2].with_name("other"))
    invoke(case, 1, {"layout": "v7.0 cosmovisor binary is missing or not executable"})
    write_binary(case[2], "v6.12.0")
    invoke(case, 1, {"layout": "v7.0 cosmovisor binary differs from new binary"})
    case[2].chmod(0o644)
    invoke(case, 1, {"layout": "v7.0 cosmovisor binary is missing or not executable"})


def every_runbook_configuration_key_is_required():
    parsed = tomllib.loads(app_text)
    source = (root / "node/sequencer/config.go").read_text()
    for key in parsed["sequencer"]:
        assert f'"sequencer.{key}"' in source, key
    for section, keys in parsed.items():
        for key in keys:
            case = fixture(f"config-{section}-{key}")
            lines = app_text.splitlines()
            current = None
            for index, line in enumerate(lines):
                if line.startswith("["):
                    current = line[1:-1]
                elif current == section and line.startswith(key + " ="):
                    lines[index] = "# " + line
            (case[0] / "config/app.toml").write_text("\n".join(lines) + "\n")
            invoke(case, 1, {"configuration": f"app.toml missing {section}.{key}"})
    case = fixture("configuration-syntax")
    (case[0] / "config/app.toml").write_text(secret)
    invoke(case, 1, {"configuration": "cannot read or validate required input"})
    (case[0] / "config/app.toml").write_text(app_text.replace("sc-enable = true", "sc-enable = false"))
    invoke(case, 1, {"configuration": "state-commit.sc-enable must be true"})
    (case[0] / "config/app.toml").write_text(app_text)
    (case[0] / "config/config.toml").unlink()
    invoke(case, 1, {"configuration": "cannot read or validate required input"})


def keys_require_private_owner_read_permissions():
    for label, name in (("consensus-key", "priv_validator_key.json"), ("node-key", "node_key.json")):
        case = fixture(label)
        key = case[0] / "config" / name
        for mode in (0o644, 0o640, 0o604, 0o620, 0o601, 0o200):
            key.chmod(mode)
            invoke(case, 1, {label: "key file must be owner-readable with no group or other permissions"})
        key.unlink()
        invoke(case, 1, {label: "key file is missing or not a regular file"})


def configured_key_paths_are_checked():
    case = fixture("configured-keys")
    for name in ("node_key.json", "priv_validator_key.json"):
        original = case[0] / "config" / name
        original.rename(original.with_name("selected-" + name))
    (case[0] / "config/config.toml").write_text(config_text.replace("config/", "config/selected-"))
    invoke(case, clock_answer="synchronised")
    (case[0] / "config/selected-node_key.json").chmod(0o644)
    invoke(case, 1, {"node-key": "key file must be owner-readable with no group or other permissions"})


def disk_space_must_exceed_operator_threshold():
    case = fixture("disk")
    invoke(case, 1, {"disk": "available data-volume space must exceed minimum GiB"}, minimum=str(2**64))
    (case[0] / "data").rmdir()
    invoke(case, 1, {"disk": "data directory is missing or not a directory"})
    (case[0] / "data").mkdir()
    for key in ("sc-directory", "ss-db-directory", "evm-ss-db-directory"):
        external = case[0].parent / ("external-" + key)
        text = app_text.replace(f'{key} = ""', f'{key} = {json.dumps(str(external))}')
        (case[0] / "config/app.toml").write_text(text)
        invoke(case, 1, {"disk": "data directory is missing or not a directory"})
        external.mkdir()
        invoke(case, clock_answer="synchronised")


def clock_must_be_synchronised():
    case = fixture("clock-probe")
    invoke(case, clock_answer="synchronised")
    invoke(case, 1, {"clock": "host clock is not synchronised"},
           clock_answer="not synchronised")
    invoke(case, 1, {"clock": "cannot read kernel clock synchronisation status"},
           clock_answer=secret)


def clock_status_must_be_readable():
    library = ctypes.CDLL("libseccomp.so.2", use_errno=True)
    library.seccomp_init.argtypes = [ctypes.c_uint32]
    library.seccomp_init.restype = ctypes.c_void_p
    library.seccomp_syscall_resolve_name.argtypes = [ctypes.c_char_p]
    library.seccomp_rule_add.argtypes = [ctypes.c_void_p, ctypes.c_uint32, ctypes.c_int, ctypes.c_uint]
    library.seccomp_load.argtypes = [ctypes.c_void_p]
    library.seccomp_release.argtypes = [ctypes.c_void_p]

    def restrict_clock_query():
        context = library.seccomp_init(0x7FFF0000)
        if not context:
            os._exit(120)
        for syscall in (b"adjtimex", b"clock_adjtime"):
            number = library.seccomp_syscall_resolve_name(syscall)
            if number < 0 or library.seccomp_rule_add(context, 0x00050000 | errno.EPERM, number, 0):
                os._exit(121)
        if library.seccomp_load(context):
            os._exit(122)
        library.seccomp_release(context)

    invoke(fixture("clock"), 1, {"clock": "cannot read kernel clock synchronisation status"},
           preexec=restrict_clock_query, clock_answer=None)


def help_and_invalid_options():
    for flag in ("--help", "-h"):
        result = subprocess.run(["bash", str(script), flag], capture_output=True, text=True)
        assert result.returncode == 0 and result.stdout.startswith("Usage: preflight.sh"), result
        assert "PAXEER_CLOCK_PROBE" in result.stdout, result.stdout
        assert not result.stderr
    for args in ([], ["--home"], ["--unknown", secret], ["--home", secret, "--binary", secret, "--min-free-gib", "-1"]):
        result = subprocess.run(["bash", str(script), *args], capture_output=True, text=True)
        assert result.returncode == 1, result
        assert secret not in result.stdout + result.stderr


passed = 0
for test in (
    passing_home_is_unchanged, release_plan_matches_sources,
    binary_version_and_execution_are_required, digest_must_match_upgrade_information,
    staged_binary_must_match_plan_and_candidate, every_runbook_configuration_key_is_required,
    keys_require_private_owner_read_permissions, configured_key_paths_are_checked,
    disk_space_must_exceed_operator_threshold, clock_must_be_synchronised,
    clock_status_must_be_readable,
    help_and_invalid_options,
):
    try:
        test()
    except Exception as error:
        print(f"FAIL {test.__name__}: {error}", file=sys.stderr)
        print(f"preflight: {passed} passed, 1 failed")
        sys.exit(1)
    print(f"PASS {test.__name__}", flush=True)
    passed += 1
print(f"preflight: {passed} passed, 0 failed")
PY
