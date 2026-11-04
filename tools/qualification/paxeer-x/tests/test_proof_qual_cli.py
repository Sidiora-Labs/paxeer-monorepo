"""Proves: kernel-genesis.render_step -> subprocess/LXGB -> lxp_genesis_build_cli.parse_request.

The eight CLI producers are executed as real processes, including their help and
malformed-input paths. Retired replay modes cross the actual make recipe. Registry
material is produced and reopened by the production bootstrap qualification.
The historical offline crossing uses build/bin/layerx-genesis-build, never a
substitute. Current paxd qualification uses the committed native command/file
transcript with the production bootstrap, generation broker and descriptor client.

Repairs: envelope main used ad-hoc argv parsing (malformed input raised a
traceback), and imported its runtime fixture before parsing CLI options;
argparse producers silently accepted abbreviated option names;
104.30.4.sh and 108.1.10.mk depended on the caller's working directory;
kernel-genesis.render_step created output before rejecting decimals above 38;
agent_operation_envelope.operations required a stale 51-operation catalogue
including the retired faucet instead of the 53 live schema operations.
Release and balance subparsers also need their own allow_abbrev=False setting.
The legacy replay CLI and make entries now refuse the retired batch-WAL modes
instead of referring to producers removed by task 22.16.4. Every current daemon
producer reference is checked by the repository producer-existence consumer.

Historical offline regression: cmd/layerx-genesis/lxp_genesis_build_cli.c::parse_request still
rejects a zero guarantor count. kernel-genesis.render_step correctly emits zero
after retirement. The expected eventual behavior is successful native genesis
creation; this proof preserves the current native refusal without inventing a
guarantor or replacing the binary. The request remains readable across process
termination, and duplicate production is refused without changing it.
"""

import hashlib
import importlib.util
import json
import os
from contextlib import contextmanager
from pathlib import Path
import subprocess
import sys

import pytest
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat


ROOT = Path(__file__).resolve().parents[4]
QUAL = ROOT / "tools/qualification/paxeer-x"
# The module's interpreter-provided name is also used by importlib in the
# process-restart checks below.
__name__: str
PRODUCERS = (
    QUAL / "custody_governance.py",
    QUAL / "replay-authority.py",
    QUAL / "program_spend_composition.py",
    QUAL / "registry-router-bootstrap.py",
    ROOT / "tools/qualification/paxeer_x_release.py",
    QUAL / "agent_http_bounds.py",
    QUAL / "agent_operation_envelope.py",
    QUAL / "program_balance_read.py",
)
# Deliberately constructed locally: no operator URL, path, configuration name,
# credential or service is inherited from the environment.
ENV = {
    "PATH": f"{Path(sys.executable).parent}:/root/.cargo/bin:/usr/local/bin:/usr/bin:/bin",
    "LC_ALL": "C.UTF-8",
    "PYTHONDONTWRITEBYTECODE": "1",
}


def run(argv, cwd, code=0):
    result = subprocess.run(
        [str(arg) for arg in argv],
        cwd=cwd,
        env=ENV,
        stdin=subprocess.DEVNULL,
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )
    assert result.returncode == code, (argv, result.returncode, result.stdout, result.stderr)
    return result


def module(path):
    assert path.is_file(), f"missing repository producer: {path}"
    spec = importlib.util.spec_from_file_location(path.stem.replace("-", "_"), path)
    assert spec is not None and spec.loader is not None, str(path)
    loaded = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(loaded)
    return loaded


@pytest.mark.parametrize("producer", PRODUCERS, ids=lambda path: path.stem)
def test_qual_cli_real_process_help_and_malformed_refusal(producer, tmp_path):
    assert producer.is_file(), f"missing repository producer: {producer}"
    help_result = run([sys.executable, producer, "--help"], tmp_path)
    assert "usage:" in help_result.stdout
    refused = run([sys.executable, producer, "--qual-cli-unknown"], tmp_path, code=2)
    assert "error:" in refused.stderr
    assert "Traceback" not in refused.stderr
    assert list(tmp_path.iterdir()) == []


@pytest.mark.parametrize(
    ("producer", "abbreviation"),
    (
        ("custody_governance.py", "--prod"),
        ("replay-authority.py", "--prepare-f"),
        ("program_spend_composition.py", "--build-d"),
        ("registry-router-bootstrap.py", "--evidence-d"),
        ("agent_http_bounds.py", "--work"),
        ("agent_operation_envelope.py", "--manif"),
    ),
)
def test_qual_cli_refuses_abbreviated_authority_options(producer, abbreviation, tmp_path):
    result = run([sys.executable, QUAL / producer, abbreviation, "unused"], tmp_path, code=2)
    assert "unrecognized arguments:" in result.stderr
    assert "Traceback" not in result.stderr
    assert not list(tmp_path.iterdir())


@pytest.mark.parametrize("arguments", (["--build"], ["--manifest", "unused"], ["--worker", "unused", "--build"]))
def test_envelope_incomplete_or_conflicting_mode_is_parser_refusal(arguments, tmp_path):
    result = run([sys.executable, QUAL / "agent_operation_envelope.py", *arguments], tmp_path, code=2)
    assert "error:" in result.stderr and "Traceback" not in result.stderr
    assert not list(tmp_path.iterdir())


@pytest.mark.parametrize(
    ("producer", "arguments"),
    (
        (ROOT / "tools/qualification/paxeer_x_release.py",
         ["status", "--logical-id", "unused", "--controller-socket", "unused", "--logical-i", "unused"]),
        (QUAL / "program_balance_read.py",
         ["snapshot", "--compiler", "cc", "--cargo", "cargo", "--cppflags=", "--cflags=", "--ldflags=",
          "--comp", "cc"]),
    ),
)
def test_qual_cli_subcommands_refuse_abbreviated_authority_options(producer, arguments, tmp_path):
    result = run([sys.executable, producer, *arguments], tmp_path, code=2)
    assert "unrecognized arguments:" in result.stderr
    assert "Traceback" not in result.stderr
    assert not list(tmp_path.iterdir())


def test_retired_custody_producer_make_and_consumer_gate(tmp_path):
    producer = module(QUAL / "custody_governance.py")
    assert producer.parser().parse_args(["--produce", str(tmp_path)]).produce == tmp_path
    make = ROOT / "tools/paxeer-x/build/108.1.10.mk"
    built = run(
        ["make", "-f", make, "paxeer-x-build-108.1.10", f"PAXEER_X_CUSTODY_GOVERNANCE_ARTIFACTS={tmp_path}"],
        tmp_path,
        code=2,
    )
    retired = "custody governance artifact production and execution have been retired"
    assert retired in built.stderr
    consumed = run(["bash", ROOT / "tools/paxeer-x/gates/108.1.10.sh", tmp_path], tmp_path, code=2)
    assert retired in consumed.stderr
    assert list(tmp_path.iterdir()) == []


def test_spend_and_reference_consumers_use_real_cli_from_other_directory(tmp_path):
    result = run(["bash", ROOT / "tools/paxeer-x/gates/104.30.4.sh", "--help"], tmp_path)
    assert "--build-dir" in result.stdout and "--cc" in result.stdout
    refused = run(["bash", ROOT / "tools/paxeer-x/gates/104.30.4.sh", "--qual-cli-unknown"], tmp_path, code=2)
    assert "unrecognized arguments" in refused.stderr
    reference = ROOT / "scripts/qualification/paxeer-x/reference-custody.py"
    assert "--build" in run([sys.executable, reference, "--help"], tmp_path).stdout
    assert "unrecognized arguments" in run([sys.executable, reference, "--bu"], tmp_path, code=2).stderr


def test_replay_make_transport_retirement_persists_and_repeated_producer_is_refused(tmp_path):
    # Task 22.16.4 retired the stateful producer. Check all current references,
    # without path exemptions, before exercising its compatibility refusals.
    inventory = module(ROOT / "tests/ci/test_qualification_producers_exist.py")
    inventory.test_qualification_daemon_producers_exist()
    inventory.test_qualification_names_no_retired_daemon_producers()
    producer = module(QUAL / "replay-authority.py")
    assert producer.parser().parse_args(["--prepare-fixture", "unused"]).prepare_fixture == "unused"
    native = tmp_path / "native"
    output = native / "generated/replay-authority-fixture-base.inc"
    command = [
        "make", "-f", ROOT / "tools/paxeer-x/build/104.35.29.mk",
        f"BUILD_DIR={native}", output,
    ]
    # Each process exits before the next invocation: retirement must survive a
    # restart, never manufacture a fixture, and refuse repeated production.
    for _ in range(2):
        result = run(command, tmp_path, code=2)
        assert producer.RETIRED in result.stderr
        assert not native.exists()
        result = run(
            [sys.executable, QUAL / "replay-authority.py", "--prepare-fixture", output],
            tmp_path, code=2,
        )
        assert producer.RETIRED in result.stderr
        assert not native.exists()
    built = run(
        [*command[:-1], "paxeer-x-replay-authority-build"], tmp_path, code=2,
    )
    assert producer.RETIRED in built.stderr
    refused = run(
        [sys.executable, QUAL / "replay-authority.py", "--build", "--prepare-fixture", output],
        tmp_path, code=2,
    )
    assert "not allowed with argument" in refused.stderr
    assert "Traceback" not in refused.stderr
    assert list(tmp_path.iterdir()) == []


def test_registry_real_material_producer_restart_and_consumer_refusals(tmp_path):
    bootstrap = module(QUAL / "registry-router-bootstrap.py")
    tmp_path.chmod(0o700)
    consumer = bootstrap.Run(tmp_path)
    try:
        observed = bootstrap.case_material_tokens(consumer, tmp_path)
    finally:
        consumer.log.close()
    assert "actual producer created distinct credentials once" in observed
    material = tmp_path / "owned-token-material/tokens"
    request, publication = material / "request", material / "publication"
    bootstrap.private_file(request)
    bootstrap.private_file(publication)
    assert request.read_bytes() != publication.read_bytes()
    assert (tmp_path / "commands.log").read_text().count("exit=0") >= 3


def genesis_input(directory, decimals=6):
    seed = Ed25519PrivateKey.generate()
    from cryptography.hazmat.primitives.serialization import NoEncryption, PrivateFormat
    key = directory / "sequencer.key"
    key.write_bytes(seed.private_bytes(Encoding.Raw, PrivateFormat.Raw, NoEncryption()))
    key.chmod(0o600)
    public = seed.public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex()
    treasury = Ed25519PrivateKey.generate().public_key().public_bytes(Encoding.Raw, PublicFormat.Raw).hex()
    asset = hashlib.sha256(b"layerx-asset:125:PAX").hexdigest()
    path = directory / "genesis.env"
    path.write_text(
        f"LAYERX_GENESIS_NETWORK_ID=77\n"
        f"LAYERX_GENESIS_SEQUENCER_PUBLIC_KEY={public}\n"
        f"LAYERX_GENESIS_SEQUENCER_KEY_FILE={key}\n"
        f"LAYERX_GENESIS_TREASURY_PUBLIC_KEY={treasury}\n"
        f"LAYERX_GENESIS_ASSETS=PAX:{asset}:{decimals}\n"
        "LAYERX_GENESIS_MODULES=escrow budget\n"
        "LAYERX_GENESIS_TIMESTAMP_MS=1790000000000\n"
    )
    return path, key


def test_native_genesis_crossing_reopens_real_request_and_preserves_known_refusal(tmp_path):
    binary = ROOT / "build/bin/layerx-genesis-build"
    assert binary.is_file(), f"missing repository producer: {binary}; build_cmd must build it"
    assert binary.read_bytes()[:4] == b"\x7fELF"
    source, key = genesis_input(tmp_path)
    output = tmp_path / "rendered"
    renderer = ROOT / "tools/bringup/kernel-genesis.sh"
    result = run(["bash", renderer, "offline-render", source, output], tmp_path, code=1)
    assert "layerx-genesis-build refused the rendered genesis (1)" in result.stderr
    request = output / "genesis-request.lxgb"
    produced = request.read_bytes()
    assert produced[:5] == b"LXGB\x02"
    assert int.from_bytes(produced[7:11], "big") == 77
    assert b"module-enable:budget" in produced and b"module-enable:escrow" in produced
    assert not (output / "signer.key").exists()
    assert not (output / "genesis/genesis.manifest").exists()
    # A new native process reopens the real producer's retained request.
    run([binary, request, key, tmp_path / "reopened"], tmp_path, code=1)
    assert request.read_bytes() == produced
    duplicate = run(["bash", renderer, "offline-render", source, output], tmp_path, code=1)
    assert "exists; the genesis is rendered into a new directory" in duplicate.stderr
    assert request.read_bytes() == produced
    # Real artifact producer/consumer functions agree after the native process
    # has stopped. No hand-written manifest or executable stands in for it.
    spend = module(QUAL / "program_spend_composition.py")
    balance = module(QUAL / "program_balance_read.py")
    envelope = module(QUAL / "agent_operation_envelope.py")
    reference = module(ROOT / "scripts/qualification/paxeer-x/reference-custody.py")
    expected = spend.artifact(request)
    assert balance.artifact(request) == envelope.record(request) == reference.evidence.artifact(request) == expected
    assert spend.checked(expected) == request
    native_record = spend.artifact(binary)
    assert spend.checked(native_record, elf=True) == binary
    request.write_bytes(produced + b"\x00")
    with pytest.raises(ValueError, match="prebuilt artifact changed"):
        spend.checked(expected)
    assert balance.artifact(request) != expected


def test_genesis_bad_decimals_and_wrong_signer_refuse_before_output(tmp_path):
    source, key = genesis_input(tmp_path, decimals=39)
    renderer = ROOT / "tools/bringup/kernel-genesis.sh"
    invalid = tmp_path / "invalid"
    result = run(["bash", renderer, "offline-render", source, invalid], tmp_path, code=1)
    assert "decimals must be 0..38" in result.stderr
    assert not invalid.exists()
    key.write_bytes(bytes(32))
    result = run(["bash", renderer, "offline-render", source, invalid], tmp_path, code=1)
    assert "does not hold the key" in result.stderr
    assert not invalid.exists()


@contextmanager
def paxd_genesis_harness(tmp_path):
    tests = ROOT / "services/platform/hosted/node/tests"
    sys.path.insert(0, str(tests))
    try:
        from native_key_harness import NativeKeyHarness

        harness = NativeKeyHarness(tmp_path, recorded=True)
        try:
            yield harness
        finally:
            harness.close()
    finally:
        sys.path.remove(str(tests))


def test_paxd_genesis_producer_consumer_success_and_reopen(tmp_path):
    import base64

    with paxd_genesis_harness(tmp_path) as harness:
        binding = harness.bootstrap()
        native = sys.modules["native_runtime"]
        reset = sys.modules["reset_state"]
        fixtures = ROOT / "services/platform/hosted/node/tests/fixtures/repair_22_31_355"
        genesis = (fixtures / "genesis.json").read_bytes()
        expected = json.loads(genesis)
        assert expected["chain_id"] == harness.config["chain_id"]
        assert expected["validators"]
        assert expected["app_state"]["bank"]["balances"]
        assert expected["app_state"]["lxphost"]["last_sequence"] == 0
        public = bytes.fromhex(harness.config["receipt_public_key"])
        pins = expected["app_state"]["sequencer"]["receipt_keys"]
        assert any(
            base64.b64decode(pin["public_key"], validate=True) == public
            and base64.b64decode(pin["key_id"], validate=True) == hashlib.sha256(public).digest()
            and pin["first_height"] == 1 and pin["last_height"] >= 1
            for pin in pins
        )
        assert binding["config/genesis.json"] == hashlib.sha256(genesis).hexdigest()
        assert native.generation_binding(harness.data) == binding
        commands = [json.loads(line) for line in harness.peer.journal.read_text().splitlines()]
        assert [row["command"][1] for row in commands] == ["init", "validate-genesis"]
        assert all(row["exit"] == 0 for row in commands)

        retained = {name: (harness.data / name).read_bytes() for name in binding}
        harness.close()
        first = None
        state = None
        for _ in range(2):
            # Each handoff closes its socket, descriptors and materialized view;
            # the next one opens a new production broker on the retained store.
            with harness.consume() as (result, adopted, view):
                assert result["generation"] == 1 and result["reset_id"] is None
                assert set(result["artifacts"]) == set(binding)
                assert (view / "genesis.json").read_bytes() == genesis
                assert json.loads((view / "genesis.json").read_bytes()) == expected
                assert result["public"]["LAYERX_NODE_SEQUENCER_PUBLIC_KEY"] == public.hex()
                assert adopted["LAYERX_CORE_SEQUENCER_ID"] == hashlib.sha256(public).hexdigest()
                for name, digest in binding.items():
                    artifact = view / name.rsplit("/", 1)[-1]
                    assert hashlib.sha256(artifact.read_bytes()).hexdigest() == digest
                    assert artifact.stat().st_mode & 0o777 == 0o600
                if first is None:
                    first = result
                else:
                    assert result == first
            assert not view.exists()
            reset.Store(harness.state, harness.data, harness.run, harness.config["network_id"],
                        {"owner": "native-key-harness"}, os.getegid()).initialize()
            recovered = (harness.state / "state.json").read_bytes()
            if state is None:
                state = recovered
            else:
                assert recovered == state
            assert {name: (harness.data / name).read_bytes() for name in binding} == retained
        assert native.generation_binding(harness.data) == binding


def test_paxd_genesis_duplicate_and_tampered_reopen_refuse(tmp_path):
    with paxd_genesis_harness(tmp_path) as harness:
        binding = harness.bootstrap()
        native = sys.modules["native_runtime"]
        reset = sys.modules["reset_state"]
        retained = {name: (harness.data / name).read_bytes() for name in binding}
        with pytest.raises(ValueError, match="native bootstrap requires an empty generation"):
            harness.bootstrap()
        assert {name: (harness.data / name).read_bytes() for name in binding} == retained
        with harness.consume() as (_, _, view):
            assert (view / "genesis.json").read_bytes() == retained["config/genesis.json"]
        proof = harness.data / ".generation-proof/config/genesis.json"
        original = proof.read_bytes()
        native.write_file(proof, original + b"\n")
        with pytest.raises(reset.StoreError, match="generation_unavailable"):
            with harness.consume():
                pytest.fail("tampered genesis reached the production consumer")
        assert native.generation_binding(harness.data) == binding
        assert {name: (harness.data / name).read_bytes() for name in binding} == retained
        native.write_file(proof, original)
        with harness.consume() as (result, _, view):
            assert result["generation"] == 1
            assert (view / "genesis.json").read_bytes() == original


def test_release_private_producer_reopened_by_real_artifact_readers(tmp_path):
    release = module(ROOT / "tools/qualification/paxeer_x_release.py")
    spend = module(QUAL / "program_spend_composition.py")
    balance = module(QUAL / "program_balance_read.py")
    envelope = module(QUAL / "agent_operation_envelope.py")
    # Use the real generated operation inventory as input, not a constructed
    # workflow completion or replacement controller response.
    catalogue = {"operations": envelope.operations()}
    assert len(catalogue["operations"]) == len(set(catalogue["operations"])) == 53
    assert {"budget.state", "mcp.invoke", "tenant.readiness"} <= set(catalogue["operations"])
    assert "faucet.claim" not in catalogue["operations"]
    path = tmp_path / "operations.json"
    # The actual producer writes and fsyncs the state in a process that exits
    # before the production readers reopen it.
    produce = [
        sys.executable, "-c",
        "import sys; sys.path[:0] = sys.argv[1:3]; "
        "import agent_operation_envelope as envelope; import paxeer_x_release as release; "
        "release.write_private(sys.argv[3], {'operations': envelope.operations()})",
        QUAL, ROOT / "tools/qualification", path,
    ]
    run(produce, tmp_path)
    observed = balance.load_private(path)
    assert observed == envelope.load_private(path, "operation catalogue") == catalogue
    assert release.request_id(observed) == release.request_id(catalogue)
    retained = path.read_bytes()
    duplicate = run(produce, tmp_path, code=1)
    assert "FileExistsError" in duplicate.stderr
    assert path.read_bytes() == retained
    with pytest.raises(FileExistsError):
        spend.write_private(path, catalogue)
    path.chmod(0o644)
    with pytest.raises(ValueError, match="private artifact record required"):
        balance.load_private(path)
