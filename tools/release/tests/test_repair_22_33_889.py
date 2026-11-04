"""Task 22.33.889 repair record and focused release-boundary regressions.

Finding -> proving Python case (the Go entry points delegate to these same cases):
build-test-daemon -> test_build_distribution
reproducible-test-daemon -> test_reproducible_distribution
verification-fixture-daemon -> test_verification_distribution
rehearsal-test-reuses-old-success -> test_rehearsal_evidence_binding
rollback-test-endorses-rewind -> test_rollback_forward_recovery
set -> test_fresh_handoff_reaches_markets
programme-test-fixes-old-checklist -> test_programme_merged_scenarios

Wiring request: owner domain 16 / task 22.33.893 must add real merged kernel,
program, xweb, tenant and restart scenarios to tools/release/rehearsal/programme.py
and their invocations to tools/release/rehearse-v6120.sh (MERGED_CHECKS contract).
No out-of-scope files are changed; the missing interface is an explicit failure.
Baseline execution: not run; the ordered gate contract forbids testing before
complete implementation. Base 7988614413df79a83ae525988b7636ae368938a2 has the
retired asset requests, implicit evidence reuse and successful signer rewind.
No baseline test failure or unexecuted native chain qualification is claimed.

Only Python, the system shell and ordinary filesystem utilities are used. Opaque
release-byte fixtures are never launched as nodes. Loopback RPC transcripts are
peers of the real pool recorder, ABI codec and fleet consumer. No Go toolchain,
paxd build, sidecar, credentials or external service is required by this gate.
"""

import argparse
import base64
import contextlib
import hashlib
import http.server
import importlib.util
import json
from pathlib import Path
import re
import shutil
import sqlite3
import subprocess
import sys
import tempfile
import threading

import pytest

ROOT = Path(__file__).resolve().parents[3]
RELEASE = ROOT / "tools/release"
sys.path.insert(0, str(RELEASE))
import fleet_check as fleet
import upgrade_info
from rehearsal import pool_parity

programme_spec = importlib.util.spec_from_file_location(
    "repair889_programme_tests", RELEASE / "tests/test_programme.py"
)
programme_tests = importlib.util.module_from_spec(programme_spec)
programme_spec.loader.exec_module(programme_tests)

ASSETS = (
    "paxd-v6.12.0-linux-amd64",
    "price-sidecar-v6.12.0-linux-amd64",
    "libwasmvm.x86_64.so",
    "libwasmvm152.x86_64.so",
    "libwasmvm155.x86_64.so",
    "native-libs.sha256",
    "upgrade-info.json",
)


def shell_function(path, name):
    source = Path(path).read_text()
    match = re.search(r"^" + re.escape(name) + r"\(\) \{.*?^\}", source, re.M | re.S)
    assert match, f"{path} has no {name}"
    return match.group()


def run_shell(script, *args, success=True, cwd=ROOT):
    result = subprocess.run(
        ["bash", "-euo", "pipefail", "-c", script, "repair889", *map(str, args)],
        cwd=cwd,
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert (result.returncode == 0) == success, result.stdout + result.stderr
    return result


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def dump(path, value):
    Path(path).write_text(json.dumps(value))


def write_sums(directory):
    producer = shell_function(RELEASE / "build-paxd.sh", "write_sums")
    run_shell("out=$1\n" + producer + "\nwrite_sums", directory)


def release_fixture(root):
    out = root / "release"
    out.mkdir(parents=True)
    for name in ASSETS:
        (out / name).write_bytes(f"recorded release fixture: {name}\n".encode())
    (out / "native-libs.sha256").write_text(
        hashlib.sha256(b"kernel archive fixture").hexdigest()
        + "  build/liblayerx.a\n"
        + hashlib.sha256(b"sandbox archive fixture").hexdigest()
        + "  programs/target/release/liblayerx_programs_sandbox.a\n"
    )
    write_sums(out)
    upgrade_info.release(
        argparse.Namespace(
            name=None,
            tag="paxeer-network/v6.12.0",
            sums=out / "SHA256SUMS",
            out=out / "upgrade-info.json",
        ),
        upgrade_info.release_identity(),
    )
    write_sums(out)
    return out


def test_build_distribution(tmp_path):
    source = (RELEASE / "tests/build.test.sh").read_text()
    assert "PAXD_RELEASE_LAYERXD=1" not in source
    assert "layerxd-$want_version" not in source
    for assertion in (
        "native_library_sums_recorded",
        "kernel_library_built_first",
        "sandbox_library_built_first",
        "ldd",
        "native_archive_digest",
    ):
        assert assertion in source
    # Exercise the actual test consumer on a package whose sums and upgrade
    # document came from the real release producers, without compiling a node.
    out = release_fixture(tmp_path)
    tools = tmp_path / "tools/release"
    tools.mkdir(parents=True)
    for name in ("build-paxd.sh", "version.json"):
        shutil.copy2(RELEASE / name, tools / name)
    pins = tmp_path / "wasm-runtime/libwasmvm-linux.sha256"
    pins.parent.mkdir()
    pins.write_text(
        "".join(
            f"{digest(out / name)}  {name}\n" for name in ASSETS if name.endswith(".so")
        )
    )
    script = (
        """out=$1; want_version=v6.12.0; want_tag=paxeer-network/v6.12.0
paxd_name=paxd-$want_version-linux-amd64
sidecar_name=price-sidecar-$want_version-linux-amd64
wasm_libs=(libwasmvm.x86_64.so libwasmvm152.x86_64.so libwasmvm155.x86_64.so)
build_status=0
"""
        + shell_function(RELEASE / "tests/build.test.sh", "require_build")
        + "\n"
        + shell_function(RELEASE / "tests/build.test.sh", "release_directory_contents")
        + "\nrelease_directory_contents"
    )
    run_shell(script, out, cwd=tmp_path)
    (out / "layerxd-v6.12.0-linux-amd64").write_bytes(b"retired artifact")
    write_sums(out)
    refusal = run_shell(script, out, cwd=tmp_path, success=False)
    assert "release directory holds:" in refusal.stderr


def test_reproducible_distribution(tmp_path):
    source = (RELEASE / "tests/reproducible.test.sh").read_text()
    assert "PAXD_RELEASE_LAYERXD=1" not in source
    assert '"$count" -eq 8' in source
    for assertion in (
        "native_libraries_rebuilt_clean",
        "paths_remapped",
        "epoch_from_commit",
        "missing_sums_refused",
        "sha256sum --check --strict",
    ):
        assert assertion in source
    first = release_fixture(tmp_path / "a")
    second = tmp_path / "b"
    shutil.copytree(first, second)
    script = (
        shell_function(RELEASE / "reproducible-check.sh", "compare_builds")
        + '\ncompare_builds "$1" "$2"'
    )
    assert "reproducible-check: identical" in run_shell(script, first, second).stdout
    for name in ASSETS:
        original = (second / name).read_bytes()
        (second / name).write_bytes(original + b"different release bytes")
        write_sums(second)
        refusal = run_shell(script, first, second, success=False)
        expected = (
            "native libraries differ"
            if name == "native-libs.sha256"
            else "builds differ"
        )
        assert expected in refusal.stderr
        (second / name).write_bytes(original)
        write_sums(second)


def test_verification_distribution():
    source = (RELEASE / "tests/verify-release.test.sh").read_text()
    assert '"layerxd-$version-linux-amd64"' not in source
    assert source.count('for name in "${files[@]}"; do') == 3
    result = run_shell('bash "$1"', RELEASE / "tests/verify-release.test.sh")
    assert "verify-release.test: 5 passed, 0 failed" in result.stdout


def binding_functions():
    source = (RELEASE / "tests/rehearse.test.sh").read_text()
    block = re.search(r"<<'PY_BINDING'\n(.*?)\nPY_BINDING", source, re.S)
    assert block, "the rehearsal regression must expose its real evidence binding"
    namespace = {"__name__": "repair889_binding"}
    exec(compile(block[1], str(RELEASE / "tests/rehearse.test.sh"), "exec"), namespace)
    return namespace


@contextlib.contextmanager
def recorded_rpc():
    requests = []
    state = {"version": "v6.12.0", "applied": 21}
    pool = "0x" + "11" * 20
    token = "0x" + "22" * 20

    class Handler(http.server.BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass

        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            requests.append(request)
            method, params = request["method"], request["params"]
            assert request["jsonrpc"] == "2.0" and request["id"] == 1
            if method == "status":
                result = {
                    "node_info": {"network": "rehearsal_hyperpax_125-1"},
                    "sync_info": {"latest_block_height": "32", "catching_up": False},
                }
            elif method == "abci_info":
                result = {"response": {"version": state["version"]}}
            elif method == "abci_query":
                assert params["path"] == "/cosmos.upgrade.v1beta1.Query/AppliedPlan"
                assert params["data"] == programme_tests.P.pb_bytes(1, "v6.12.0").hex()
                result = {
                    "response": {
                        "code": 0,
                        "value": base64.b64encode(
                            programme_tests.P.pb_uint(1, state["applied"])
                        ).decode(),
                    }
                }
            else:
                assert method == "eth_call", request
                call, block = params
                assert block in ("0x14", "0x20"), request
                address, data = call["to"].lower(), call["data"]
                if data == pool_parity.GET_POOL_COUNT:
                    value = (
                        0
                        if address == pool_parity.GENERATION_REGISTRIES[1].lower()
                        else 1
                    )
                    raw = fleet.word(value)
                elif data.startswith(pool_parity.GET_ALL_POOLS):
                    assert data[10:] == (fleet.word(0) + fleet.word(100)).hex()
                    raw = fleet.word(32) + fleet.word(1) + fleet.word(int(pool, 16))
                elif data == pool_parity.GET_RESERVES:
                    assert address == pool
                    raw = b"".join(fleet.word(n) for n in (100, 40, 50))
                elif data == pool_parity.TOKEN_ADDRESS:
                    assert address == pool
                    raw = fleet.word(int(token, 16))
                elif data.startswith(pool_parity.BALANCE_OF):
                    assert data[10:] == fleet.word(int(pool, 16)).hex()
                    raw = fleet.word(100)
                else:
                    assert data in (
                        fleet.call_data("totalMarkets()"),
                        fleet.call_data("getMarketCount()"),
                    ), request
                    raw = fleet.word(18)
                result = "0x" + raw.hex()
            payload = json.dumps({"jsonrpc": "2.0", "id": 1, "result": result}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}", requests, state
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


def emit_environment(work, url, candidate_sha):
    """Execute the production driver's inventory and environment emitter verbatim."""
    source = (RELEASE / "rehearse-v6120.sh").read_text()
    inventory = source[
        source.index("inventory=$work/inventory.json\n") : source.index(
            "read -ra key_names", source.index("inventory=$work/inventory.json\n")
        )
    ]
    start = source.index("{\n\tprintf 'RELEASE_ID=")
    end = source.index('chmod 600 "$rehearsal_env"', start) + len(
        'chmod 600 "$rehearsal_env"'
    )
    script = r"""
work=$1; url=$2; new_sha=$3
nodes=4; release_id=rehearsal; plan=v6.12.0; sidecar_sha=$new_sha
halt_height=20; chain_id=rehearsal_hyperpax_125-1; keyring_backend=test
source_genesis=$work/source-genesis.json; bin_dir=$work/bin
snapshots=$work/snapshots; credentials=$work/credentials; receipts=$work/receipts
chainflow_key_file=$work/chainflow.key; rehearsal_env=$work/rehearsal.env
homes=(); units=(); ordered_keys=(a b c d)
for ((i=0; i<4; i++)); do homes+=("$work/node$i"); units+=("node$i.service"); done
rpc_url() { printf '%s/%s\n' "$url" "$1"; }
evm_url() { printf '%s/%s\n' "$url" "$1"; }
""" + inventory + "\n" + source[start:end]
    run_shell(script, work, url, candidate_sha)
    return work / "rehearsal.env"


def publish_receipt(work, env, result="pass", checksum=None):
    script = r"""
source "$1/rehearsal/receipt.sh"
receipt=$2/receipts/rehearsal.json; plan=v6.12.0; nodes=4; precompiles_served=8
pools=1; markets=18; oracle_fresh=18; feed_keys=4; new_sha=$3
write_receipt "$4"
"""
    run_shell(script, RELEASE, work, checksum or env["RELEASE_SHA256"], result)


def fresh_handoff_case(root):
    root.mkdir(parents=True, exist_ok=True)
    work = root / "fresh-work"
    final = work / "receipts/rehearsal.json"
    binding_functions()["require_empty"](work, final)
    work.mkdir()
    (work / "bin").mkdir()
    (work / "bin/paxd").write_bytes(b"recorded candidate bytes; never executed")
    (work / "old-paxd").write_bytes(b"recorded previous release bytes; never executed")
    with recorded_rpc() as (url, requests, _):
        env_file = emit_environment(work, url, digest(work / "bin/paxd"))
        env = fleet.read_env(env_file)
        assert "POOL_COUNT" not in env and not final.exists()
        record = work / "pools-at-halt.json"
        dump(record, pool_parity.record(url, 20, pool_parity.GENERATION_REGISTRIES))
        handoff = work / "rehearsal-handoff.json"
        run_shell(
            'source "$1/rehearsal/receipt.sh"; release=$1; rehearsal_env=$2; '
            'pools_file=$3; write_rehearsal_handoff "$4"',
            RELEASE,
            env_file,
            record,
            handoff,
        )
        internal = json.loads(handoff.read_text())
        assert internal["pools"] == 1 and "result" not in internal
        assert "rollback_rehearsed" not in internal
        nodes, _, _, _, pools = fleet.load_config(
            env, "markets", rehearsal_handoff=handoff
        )
        outcome = fleet.Outcome()
        for node in nodes:
            node.height = hex(
                int(node.consensus("status")["sync_info"]["latest_block_height"])
            )
            fleet.check_markets(
                node,
                json.loads((RELEASE / "fleet-views.json").read_text()),
                pools,
                outcome,
                "fresh rehearsal",
            )
        measured = outcome.finish()
        assert measured["result"] == "pass", outcome.errors
        assert measured["pools"] == 1 and measured["markets"] == 18
        assert len(nodes) == 4 and len(requests) >= 22
        assert not final.exists()
        for phase in ("full", "post"):
            with pytest.raises(fleet.CheckError, match="no rehearsal.json"):
                fleet.load_config(env, phase)
            with pytest.raises(fleet.CheckError, match="HANDOFF_MARKETS_ONLY"):
                fleet.load_config(env, phase, rehearsal_handoff=handoff)
            with pytest.raises(
                fleet.CheckError, match="PRODUCTION_REQUIRES_ROLLBACK_RECEIPT"
            ):
                fleet.load_config(env, phase, no_rehearsal=True)
        publish_receipt(work, env, "fail")
        with pytest.raises(fleet.CheckError, match="REHEARSAL_NOT_PASSED"):
            fleet.load_config(env, "post")
        publish_receipt(work, env, checksum="0" * 64)
        with pytest.raises(fleet.CheckError, match="REHEARSAL_BINARY_MISMATCH"):
            fleet.load_config(env, "post")
        publish_receipt(work, env)
        assert fleet.load_config(env, "post")[-1] == pools
        # An altered candidate and inventory must also invalidate the internal handoff.
        candidate = work / "bin/paxd"
        original = candidate.read_bytes()
        candidate.write_bytes(original + b"tampered")
        with pytest.raises(fleet.CheckError, match="HANDOFF_BINARY_MISMATCH"):
            fleet.load_config(env, "markets", rehearsal_handoff=handoff)
        candidate.write_bytes(original)
        inventory = work / "inventory.json"
        original = inventory.read_bytes()
        inventory.write_bytes(original + b"\n")
        with pytest.raises(fleet.CheckError, match="HANDOFF_INVENTORY_MISMATCH"):
            fleet.load_config(env, "markets", rehearsal_handoff=handoff)
        inventory.write_bytes(original)
    return work, env_file


def test_fresh_handoff_reaches_markets(tmp_path):
    work, env_file = fresh_handoff_case(tmp_path)
    assert fleet.load_config(fleet.read_env(env_file), "post")[-1] == 1
    assert (work / "receipts/rehearsal.json").is_file()
    # The shell suite owns this integration case as well as the focused Python gate.
    source = (RELEASE / "tests/rehearse.test.sh").read_text()
    assert "run_case fresh_handoff_reaches_markets" in source
    assert "--fresh-handoff" in shell_function(
        RELEASE / "tests/rehearse.test.sh", "case_fresh_handoff_reaches_markets"
    )


def rehearsal_shell(mode, env_file, final, scratch, success=True):
    scratch.mkdir()
    definitions = "\n".join(
        shell_function(RELEASE / "tests/rehearse.test.sh", name)
        for name in ("key_of", "rehearsal_evidence", "full_run")
    )
    return run_shell(
        "root=$1; rehearse_env=$2; receipt=$3; scratch=$4; rehearsal_test_mode=$5; "
        "driver=$root/tools/release/rehearse-v6120.sh\n" + definitions + "\nfull_run",
        ROOT,
        env_file,
        final,
        scratch,
        mode,
        success=success,
    )


def test_rehearsal_requires_fresh_driver(tmp_path):
    empty = tmp_path / "empty-work"
    final = empty / "receipts/rehearsal.json"
    env_file = tmp_path / "qualification.env"
    env_file.write_text(
        f'WORK="{empty}"\nNEW_BIN="{tmp_path / "candidate"}"\nOLD_BIN="{tmp_path / "previous"}"\n'
    )
    scratch = tmp_path / "fresh-attempt"
    rehearsal_shell("fresh", env_file, final, scratch, success=False)
    # This is the real producer's input refusal, after the empty-work guard. A
    # failed producer cannot publish or bind success, and its exit is memoized.
    assert "SOURCE_DATA is required" in (scratch / "full.log").read_text()
    assert (scratch / "full.done").read_text().strip() == "1"
    assert not final.exists() and not (empty / "test-evidence-binding.json").exists()
    stale_work, stale_env = fresh_handoff_case(tmp_path / "stale")
    env_file.write_text(
        f'WORK="{stale_work}"\nNEW_BIN="{stale_work / "bin/paxd"}"\nOLD_BIN="{stale_work / "old-paxd"}"\n'
    )
    result = rehearsal_shell(
        "fresh",
        env_file,
        stale_work / "receipts/rehearsal.json",
        tmp_path / "stale-attempt",
        success=False,
    )
    assert "driver qualification requires empty WORK" in result.stderr
    assert not (tmp_path / "stale-attempt/full.done").exists()


def test_rehearsal_evidence_binding(tmp_path):
    api = binding_functions()
    work, emitted = fresh_handoff_case(tmp_path)
    env_file = tmp_path / "qualification.env"
    env_file.write_text(
        f'WORK="{work}"\nNEW_BIN="{work / "bin/paxd"}"\nOLD_BIN="{work / "old-paxd"}"\n'
    )
    sums = work / "bin/SHA256SUMS"
    sums.write_text(f'{digest(work / "bin/paxd")}  paxd\n')
    source = (RELEASE / "rehearse-v6120.sh").read_text()
    start = source.index("{\n\tprintf 'upgrade=")
    end = source.index("\npassed=1", start)
    run_shell(
        "work=$1; plan=v6.12.0; halt_height=20; export_height=10; final_height=32; "
        "old_sha=$2; new_sha=$3; pools=1; markets=18; oracle_fresh=18; feed_keys=4; precompiles_served=8\n"
        + source[start:end],
        work,
        digest(work / "old-paxd"),
        digest(work / "bin/paxd"),
    )
    for directory in (work / "logs", work / "evidence"):
        directory.mkdir()
        (directory / "record.json").write_bytes(b"recorded chain boundary evidence")
    final = work / "receipts/rehearsal.json"
    with pytest.raises(ValueError, match="empty WORK"):
        api["require_empty"](work, final)
    with pytest.raises(FileNotFoundError):
        api["bind_evidence"](ROOT, "inspect", env_file, work, final)
    api["bind_evidence"](ROOT, "record", env_file, work, final)
    # A new loader reopens the durable binding rather than trusting a prior result.
    binding_functions()["bind_evidence"](ROOT, "inspect", env_file, work, final)
    inspected = rehearsal_shell("inspect", env_file, final, tmp_path / "inspection")
    assert inspected.returncode == 0
    refused = rehearsal_shell(
        "fresh", env_file, final, tmp_path / "stale-driver", success=False
    )
    assert "requires empty WORK" in refused.stderr
    with pytest.raises(ValueError, match="replace an existing"):
        api["bind_evidence"](ROOT, "record", env_file, work, final)
    for path in (
        work / "bin/paxd",
        work / "old-paxd",
        work / "evidence/record.json",
        work / "logs/record.json",
        emitted,
        final,
        env_file,
        sums,
    ):
        original = path.read_bytes()
        path.write_bytes(original + b"\n")
        with pytest.raises((ValueError, fleet.CheckError)):
            api["bind_evidence"](ROOT, "inspect", env_file, work, final)
        path.write_bytes(original)
    # Compare a second tooling checkout by content, then change the actual
    # consumer there. Neither a code edit nor a different candidate may reuse success.
    tooling_root = tmp_path / "tooling"
    saved = json.loads((work / "test-evidence-binding.json").read_text())
    for name in saved["tooling"]:
        destination = tooling_root / name
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / name, destination)
    (tooling_root / "tools/release/env").mkdir(exist_ok=True)
    api["bind_evidence"](tooling_root, "inspect", env_file, work, final)
    consumer = tooling_root / "tools/release/fleet_check.py"
    consumer.write_text(consumer.read_text() + "\n# changed tooling revision\n")
    with pytest.raises(ValueError, match="tooling revision"):
        api["bind_evidence"](tooling_root, "inspect", env_file, work, final)
    full_run = shell_function(RELEASE / "tests/rehearse.test.sh", "full_run")
    assert 'rehearsal_evidence fresh "$work"' in full_run
    assert 'bash "$driver" --env-file "$rehearse_env" --receipt "$receipt"' in full_run
    assert full_run.index("rehearsal_evidence fresh") < full_run.index('bash "$driver"')
    assert "INSPECTION ONLY" in full_run


def test_rollback_forward_recovery(tmp_path):
    # Use the real interpreter ELF only for the checksum/loader boundary. It is
    # never launched as a replacement paxd and no native build artifact is needed.
    home, binaries, snapshots = (
        tmp_path / name for name in ("validator", "bin", "snapshots")
    )
    for directory in (home / "config", home / "data/committer.db", binaries, snapshots):
        directory.mkdir(parents=True)
    signing = {
        "height": "25",
        "round": 0,
        "step": 3,
        "signature": "signed candidate",
        "signbytes": "0820",
    }
    signer = home / "data/priv_validator_state.json"
    dump(signer, signing)
    (home / "config/app.toml").write_text("halt-height = 25\n")
    (home / "config/priv_validator_key.json").write_bytes(b"recorded validator key")
    dump(home / "data/upgrade-info.json", {"name": "v6.12.0", "height": 21})
    store = home / "data/committer.db/history.sqlite"
    with sqlite3.connect(store) as database:
        database.execute("CREATE TABLE commits(height INTEGER PRIMARY KEY, hash TEXT)")
        database.executemany(
            "INSERT INTO commits VALUES (?, ?)", [(20, "halt"), (25, "candidate")]
        )
    for name in ("paxd", "paxd.pre-r1"):
        shutil.copy2(sys.executable, binaries / name)
    recovery = tmp_path / "recovery"
    shutil.copy2(sys.executable, recovery)
    env_file = tmp_path / "rollback.env"
    fields = dict(
        RELEASE_ID="r1",
        BIN_DIR=binaries,
        SNAPSHOT_DIR=snapshots,
        HALT_HEIGHT=20,
        VALIDATOR_HOMES=home,
        VALIDATOR_UNITS="node.service",
        FULLNODE_HOMES="",
        FULLNODE_UNITS="",
        RECOVERY_BIN=recovery,
        RECOVERY_SHA256=digest(recovery),
    )
    env_file.write_text("".join(f'{key}="{value}"\n' for key, value in fields.items()))
    before = {
        str(path.relative_to(home)): path.read_bytes()
        for path in home.rglob("*")
        if path.is_file()
    }
    data_inode = (home / "data").stat().st_ino
    command = 'bash "$1/rollback.sh" --env-file "$2" --local --service-manager foreground --forward --execute --phase restore'
    run_shell(command, RELEASE, env_file)
    # A second process resumes the real journal; the SQLite store is reopened.
    run_shell(command, RELEASE, env_file)
    with sqlite3.connect(store) as database:
        assert database.execute("SELECT * FROM commits ORDER BY height").fetchall() == [
            (20, "halt"),
            (25, "candidate"),
        ]
    assert (home / "data").stat().st_ino == data_inode
    for name, content in before.items():
        if name != "config/app.toml":
            assert (home / name).read_bytes() == content
    assert (home / "config/app.toml").read_text() == "halt-height = 0\n"
    assert (
        home / ".forward-recovery-r1/priv_validator_state.json"
    ).read_bytes() == before["data/priv_validator_state.json"]
    assert digest(binaries / "paxd") == digest(recovery)
    assert not (home / "data.v6.12.0-r1").exists()
    for changed in ({**signing, "height": "20"}, {**signing, "signbytes": "0821"}):
        dump(signer, changed)
        refused = run_shell(command, RELEASE, env_file, success=False)
        assert "validator signing high-water mark regressed" in refused.stderr
        assert signer.read_text() == json.dumps(changed)
        dump(signer, signing)
    run_shell(command, RELEASE, env_file)
    regression = (RELEASE / "tests/rollback.test.sh").read_text()
    assert "run_case post_halt_rollback_is_refused" in regression
    assert "run_case forward_recovery_reopens_same_chain" in regression
    assert "same-chain snapshot restore refused; use --forward" in regression
    assert 'cmp "$work/before-refusal.json" "$work/after-refusal.json"' in regression
    assert (
        'assert_height "$work/validator/data/priv_validator_state.json" 25'
        in regression
    )


def test_programme_driver_consumes_rpc_evidence(tmp_path):
    with recorded_rpc() as (url, requests, state):
        result = programme_tests.exercise_driver_check(
            "check_plan_and_version", url, tmp_path
        )
        assert result.returncode == 0, result.stdout + result.stderr
        log = tmp_path / "programme-check_plan_and_version.log"
        assert "applied at 21 on four nodes reporting v6.12.0" in log.read_text()
        assert "PASS check_plan_and_version" in result.stdout
        assert [request["method"] for request in requests] == [
            "abci_query",
            "abci_info",
        ] * 4
        state["applied"] = 22
        result = programme_tests.exercise_driver_check(
            "check_plan_and_version", url, tmp_path
        )
        assert result.returncode == 1
        assert "FAIL check_plan_and_version" in result.stdout
        assert "applied v6.12.0 at 22, expected 21" in log.read_text()
        state.update(applied=21, version="v6.11.0")
        result = programme_tests.exercise_driver_check(
            "check_plan_and_version", url, tmp_path
        )
        assert result.returncode == 1
        assert "reports version v6.11.0" in log.read_text()


def test_programme_merged_scenarios():
    programme_tests.test_merged_scenarios_registered_and_invoked()
    programme_tests.test_checks_and_driver_functions()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--fresh-handoff", action="store_true", required=True)
    parser.parse_args()
    with tempfile.TemporaryDirectory() as directory:
        fresh_handoff_case(Path(directory))
    print("PASS fresh_handoff_reaches_markets")
