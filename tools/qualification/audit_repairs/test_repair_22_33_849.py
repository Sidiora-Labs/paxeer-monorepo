"""Source contracts and real filesystem/RPC transport proofs for eight gate repairs.

hf-build owns bash -n. No compiler, shell, native artifact, credential or live
endpoint is executed here. Native qualification invocations are source-proven.
"""

import ast
from contextlib import contextmanager
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import sys
from threading import Thread
import tomllib

import pytest

ROOT = Path(__file__).resolve().parents[3]
GATES = ROOT / "tools/paxeer-x/gates"
RECORD = Path(__file__).with_name("repair_22_33_849.kvx")
SCRIPTS = ("104.33.7", "104.35.4", "104.37.2", "104.38.3", "104.38.6", "108.0.7", "108.1.10")


def source(name):
    return (GATES / (name + ".sh")).read_text()


def module(name):
    spec = importlib.util.spec_from_file_location("repair_849_" + name, GATES / (name + ".py"))
    value = importlib.util.module_from_spec(spec)
    before = list(sys.path)
    try:
        sys.path.insert(0, str(GATES))
        spec.loader.exec_module(value)
    finally:
        sys.path[:] = before
    return value


def function(text, name):
    return next(node for node in ast.parse(text).body if isinstance(node, ast.FunctionDef) and node.name == name)


def body(text, name):
    return ast.unparse(function(text, name))


def embedded(name, delimiter="PY"):
    return source(name).split("<<'" + delimiter + "'\n", 1)[1].rsplit("\n" + delimiter, 1)[0]


def declarations(text, names, namespace=None):
    tree = ast.parse(text)
    selected = [node for node in tree.body
                if isinstance(node, ast.Import)
                or isinstance(node, ast.ImportFrom) and node.module != "candidate"
                or isinstance(node, (ast.FunctionDef, ast.ClassDef)) and node.name in names
                or isinstance(node, ast.Assign) and any(isinstance(t, ast.Name) and t.id in names for t in node.targets)]
    namespace = {} if namespace is None else namespace
    exec(compile(ast.Module(body=selected, type_ignores=[]), "<production declarations>", "exec"), namespace)
    return namespace


def test_f064_retained_source_closure_and_actual_artifact_commands():
    owner = module("program_operations")
    assert all((ROOT / path).is_file() for path in owner.SOURCES)
    assert len(owner.SOURCES) == len(set(owner.SOURCES))
    assert "services/platform/hosted/gateway/src/native_call.rs" not in owner.SOURCES
    assert "services/human/crates/layerx-explorer-index/src/programs.rs" not in owner.SOURCES
    assert {"tools/platform/cli/src/programs.rs", "precompiles/layerxprograms/method_call.go",
            "services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/native_settlement.ex",
            "apps/web/src/explorer/client.ts"} <= set(owner.SOURCES)
    assert shlex.split(source("104.33.7").split("exec ", 1)[1].split("<<", 1)[0]) == ["python3", "-"]
    wrapper = embedded("104.33.7", "GATE")
    ast.parse(wrapper)
    assert "import program_operations as owner" in wrapper
    assert "from hosted_profile import load_native as load_profile" in wrapper
    assert wrapper.index("private(raw)") < wrapper.index("inputs=load_profile(profile_path,revision,root)")
    assert wrapper.index("inputs=load_profile(profile_path,revision,root)") < wrapper.index("owner.qualify(root,inputs)")
    assert "sys.dont_write_bytecode = True" in wrapper
    assert "sys.exit(78)" in wrapper and "'environment files forbidden'" in wrapper
    commands = owner.commands(Path("/private/compiler-evidence"))
    assert set(commands) == {"gateway", "cli", "precompile", "explorer"}
    assert commands["precompile"] == [
        "go", "test", "-c", "-o", "/private/compiler-evidence/layerxprograms.test", "./precompiles/layerxprograms",
    ]
    assert commands["explorer"] == ["mix", "compile", "--warnings-as-errors"]
    for name in ("gateway", "cli"):
        assert commands[name][:3] == ["cargo", "test", "--locked"]
        assert "--no-run" in commands[name] and "--message-format=json" in commands[name]
    assert [commands["cli"][index + 1] for index, item in enumerate(commands["cli"]) if item == "--test"] == [
        "program_evm_calls", "program_evm_registry",
    ]
    assert "--lib" in commands["gateway"] and "layerx-gateway" in commands["gateway"]
    for name, (manifest, target, test) in owner.RUST_TARGETS.items():
        package = tomllib.loads((ROOT / manifest).read_text())["package"]["name"]
        if name.endswith("_lib"):
            assert target == package.replace("-", "_")
        elif name in {"cli_calls", "cli_registry"}:
            assert (ROOT / manifest).parent.joinpath("tests", target + ".rs").is_file()
        assert type(test) is bool
    text = (GATES / "program_operations.py").read_text()
    build = body(text, "build")
    verify = body(text, "verify")
    assert build.index("result.returncode == 0") < build.index("evidence.save(")
    for guard in ("built['identity'] == evidence.source_identity(root)",
                  "selected == built['compiler_artifacts']", "log['exit'] == 0",
                  "evidence.digest(log['path']) == log['sha256']", "evidence.checked_artifact",
                  "beams(directory / 'explorer-build') == built['explorer_beams']"):
        assert guard in verify
    with pytest.raises(RuntimeError, match="exact compiler artifact"):
        owner.rust_artifacts(ROOT, [])


def test_f064_main_service_structure_survives_with_native_profile_and_retained_closure():
    text = source("104.33.7")
    roster, ns = catalogue_and_binding()
    services = ["platform-gateway", "human-explorer-index", "platform-program-registry",
                "sequencer", "platform-receipt-authority"]
    assert re.search(r"^# paxeer-x-services: (.+)$", text, re.M)[1].split() == services
    assert set(services) <= set(roster)
    data = {"services": [{"id": name, "bindings": {field: ns["UNKNOWN"] for field in ns["BINDING_FIELDS"]}}
                          for name in roster]}
    bound, unknown = ns["bind"](data, GATES / "104.33.7.sh")
    assert set(bound) == set(services)
    assert set(unknown) == {name + "." + field for name in services for field in ns["BINDING_FIELDS"]}
    assert "<<<<<<<" not in text and "=======" not in text and ">>>>>>>" not in text
    assert "PAXEER_X_HOSTED_PROGRAM_ARTIFACTS" in text
    assert "PAXEER_X_HOSTED_PROGRAM_PROFILE" in text
    assert "set -euo pipefail" in text and "<<'GATE'" in text
    owner = module("program_operations")
    assert {"tools/paxeer-x/gates/hosted_profile.py", "tools/paxeer-x/gates/compiler_manifest.py"} <= set(owner.SOURCES)
    connected = body((GATES / "program_operations.py").read_text(), "connected")
    assert "os.environ.update(inputs)" in connected
    assert "loopback_origin(inputs['LAYERX_PROGRAM_RPC_URL'])" in connected
    assert "evidence.load(Path(inputs['LAYERX_PROGRAM_CALL_ARGUMENTS']))" in connected


def test_f065_native_profile_producer_transport_consumer_preserve_private_current_inputs(tmp_path):
    owner = module("hosted_profile")
    revision = "24ad444517739fd50d4cf816446559d52d7bdd77"
    ca = tmp_path / "ca.pem"
    shutil.copyfile(ROOT / "tools/bringup/tests/fixtures/repair_m794/paxeer-boundary-loopback/ca.pem", ca)
    ca.chmod(0o600)
    recorded = tmp_path / "recorded.json"
    shutil.copyfile(ROOT / "daemon/committed-fixture/vectors/vector-v4-committed.json", recorded)
    recorded.chmod(0o600)
    inputs = {"LAYERX_PROGRAM_RPC_URL": "https://127.0.0.1:8545",
              "LAYERX_PROGRAM_CALL_ARGUMENTS": str(recorded), "LAYERX_GATEWAY_CA_FILE": str(ca)}
    path = tmp_path / "profile.json"
    owner.publish_native(path, revision, inputs, ROOT)
    assert path.stat().st_mode & 0o777 == 0o600
    assert module("hosted_profile").load_native(path, revision, ROOT) == inputs
    assert ca.read_bytes() == (ROOT / "tools/bringup/tests/fixtures/repair_m794/paxeer-boundary-loopback/ca.pem").read_bytes()
    assert recorded.read_bytes() == (ROOT / "daemon/committed-fixture/vectors/vector-v4-committed.json").read_bytes()
    with pytest.raises(FileExistsError):
        owner.publish_native(path, revision, inputs, ROOT)
    with pytest.raises(ValueError, match="current private native"):
        owner.load_native(path, "0" * 40, ROOT)
    ca.chmod(0o644)
    with pytest.raises(ValueError, match="bounded private owner"):
        owner.load_native(path, revision, ROOT)
    ca.chmod(0o600)
    path.chmod(0o644)
    with pytest.raises(ValueError, match="bounded private owner"):
        owner.load_native(path, revision, ROOT)


@pytest.mark.parametrize("origin", ["http://127.0.0.1:8545", "https://example.com:8545",
                                    "https://localhost", "https://localhost:8545/rpc",
                                    "https://user@localhost:8545", "https://localhost:8545?route=rpc"])
def test_f065_native_profile_preserves_main_https_disposable_root_refusals(origin):
    owner = module("hosted_profile")
    inputs = {"LAYERX_PROGRAM_RPC_URL": origin, "LAYERX_PROGRAM_CALL_ARGUMENTS": "/private/call.json",
              "LAYERX_GATEWAY_CA_FILE": "/private/ca.pem"}
    document = {"schema": owner.NATIVE_SCHEMA, "source_revision": "revision", "inputs": inputs}
    with pytest.raises(ValueError, match="disposable loopback HTTPS root"):
        owner.validate_native(document, "revision")
    document["schema"] = owner.SCHEMA
    with pytest.raises(ValueError, match="current private native"):
        owner.validate_native(document, "revision")


def test_f064_retains_nonempty_native_and_explorer_acceptance_corpora():
    text = (GATES / "program_operations.py").read_text()
    qualify = body(text, "qualify")
    suite = body(text, "rust_suite")
    for item in ("tests::production_route_refuses_every_program_and_activity_route", "programs_wire_tests::",
                 "TestProgramsReplayedActivityReturnsStoredReceipt", "TestProgramsSubmitKernelRefusalReverts",
                 "TestProgramsCallReturnsTerminalOutcome", "TestProofLayerxprograms0x101d"):
        assert item in qualify
    assert "tuple(map(int, summary[0])) == (len(names), 0, 0)" in suite
    assert "require(names," in suite
    assert "passed == names and '--- SKIP' not in output" in qualify
    assert "['mix', 'test', '--no-compile', EXPLORER_TEST]" in qualify
    assert "summary[0] == ('1', '0')" in qualify
    assert "count == ['4']" in qualify and "'# skipped 0' in output" in qualify
    assert "{'rust', 'typescript', 'python', 'go', 'java', 'swift', 'csharp'}" in qualify
    assert "supplied['skipped'] == 0" in qualify
    assert qualify.index("built = verify(root, manifest)") < qualify.index("rust_suite(")
    assert qualify.rindex("verify(root, manifest)") > qualify.index("connected(root,")
    gateway = (ROOT / "services/platform/hosted/gateway/src/lib.rs").read_text()
    assert "fn production_route_refuses_every_program_and_activity_route()" in gateway
    explorer = ROOT / "services/explorer/backend/apps/explorer/test/explorer/chain/paxeer_x/committed_vectors_test.exs"
    assert 'test "committed_vectors_drive_verify"' in explorer.read_text()
    assert "SequencerReceipt.verify(tx_hash" in explorer.read_text()


def test_f065_connected_path_uses_real_evm_producer_reads_commitment_and_refusals():
    text = (GATES / "program_operations.py").read_text()
    connected = body(text, "connected")
    commit = body(text, "committed")
    assert "'/v1/programs/call'" not in connected and "Idempotency-Key" not in connected
    assert "['registry', 'get', program]" in connected
    assert "['interface', 'get', program]" in connected
    assert "['simulate', program, *options]" in connected
    assert "['call', program, '--evm-key-file', key_file, *options]" in connected
    assert "simulated['outcome']['terminal_result'] == 0" in connected
    assert "'tx_hash' not in simulated" in connected
    assert "called['rung'] == 'submitted'" in connected
    assert "'eth_getTransactionByHash'" in connected
    assert "'eth_getRawTransactionByHash'" in connected
    assert "'eth_sendRawTransaction', [raw]" in connected
    assert "'eth_sendRawTransaction', [raw[:-2]]" in connected
    assert "'error' in replay" in connected and "'error' in tampered" in connected
    assert "submission_refusal(replay, 'duplicate')" in connected
    assert "submission_refusal(tampered, 'tampered')" in connected
    assert "error['code'] in {-32000, -32602}" in body(text, "submission_refusal")
    assert "committed(origin, tx_hash, transaction) == receipt" in connected
    assert "independent genuine signed receipt verification required" in connected
    for guard in ("transaction.get('to', '').lower() == ADDRESS",
                  "receipt.get('transactionHash', '').lower() == tx_hash.lower()",
                  "receipt.get('status') == '0x1'", "int(height, 16) > 0",
                  "block.get('hash') == receipt.get('blockHash')",
                  "tx_hash in block.get('transactions', [])", "'eth_blockNumber'"):
        assert guard in commit
    producer = (ROOT / "tools/platform/cli/src/programs.rs").read_text()
    assert 'pub fn call(' in producer and 'prepare_program_transaction(client, data, key_file)?' in producer
    assert '"eth_sendRawTransaction"' in producer
    assert 'const PROGRAMS_ADDRESS: &str = "0x000000000000000000000000000000000000101d"' in producer
    assert "pub fn registry_get" in producer and "decode_program_view(program_id, &result)" in producer
    assert "pub fn interface_get" in producer and "decode_interface_view(program_id, &result)" in producer
    verifier = (ROOT / "services/platform/examples/agent-program-call/verify-receipt.mjs").read_text()
    assert "return client.resolve(raw);" in verifier
    assert "keccak256(raw)" in verifier
    runtime = (ROOT / "services/platform/examples/support/runtime.mjs").read_text()
    assert "await verifySequencerReceipt(signedReceipt, raw, [key])" in runtime


@pytest.mark.parametrize("value", ["https://example.com", "http://localhost/rpc", "http://user@localhost",
                                   "http://127.0.0.1?route=rpc", "http://localhost/#rpc", "ftp://localhost"])
def test_f065_refuses_non_disposable_or_non_root_rpc(value):
    with pytest.raises(RuntimeError, match="disposable root"):
        module("program_operations").loopback_origin(value)


@contextmanager
def transcript_server(receipt, envelope=None):
    """Serve a committed recorder transcript, without simulating node execution."""
    observed = []

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            length = int(self.headers["Content-Length"])
            request = json.loads(self.rfile.read(length))
            observed.append((self.path, self.headers["Content-Type"], request))
            response = {"jsonrpc": "2.0", "id": request["id"], "result": receipt} if envelope is None else envelope
            encoded = json.dumps(response).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(encoded)))
            self.end_headers()
            self.wfile.write(encoded)

        def log_message(self, *_args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield "http://127.0.0.1:" + str(server.server_port), observed
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()


def recorded_receipt():
    return json.loads((ROOT / "daemon/committed-fixture/vectors/vector-v4-committed.json").read_text())["settlement"]["receipt"]


def test_f065_real_rpc_transport_preserves_committed_recorder_evidence():
    owner = module("program_operations")
    receipt = recorded_receipt()
    assert receipt["rung"] == "committed" and receipt["receipt"].startswith("0x")
    with transcript_server(receipt) as (origin, observed):
        origin = owner.loopback_origin(origin)
        assert owner.result(origin, "pax_sequencerReceipt", [receipt["txHash"]]) == receipt
    assert observed == [("/", "application/json", {
        "jsonrpc": "2.0", "id": 1, "method": "pax_sequencerReceipt", "params": [receipt["txHash"]],
    })]


@pytest.mark.parametrize("envelope", [
    {"jsonrpc": "1.0", "id": 1, "result": None},
    {"jsonrpc": "2.0", "id": True, "result": None},
    {"jsonrpc": "2.0", "id": 2, "result": None},
    {"jsonrpc": "2.0", "id": 1, "result": None, "error": {"code": -32601}},
    {"jsonrpc": "2.0", "id": 1},
])
def test_f065_rpc_transport_refuses_malformed_envelopes(envelope):
    owner = module("program_operations")
    with transcript_server(recorded_receipt(), envelope) as (origin, _):
        with pytest.raises(RuntimeError, match="matching JSON-RPC response"):
            owner.rpc(origin, "pax_sequencerReceipt", [])


def test_f065_missing_program_transaction_cannot_claim_commitment():
    owner = module("program_operations")
    with pytest.raises(RuntimeError, match="actual Programs EVM transaction"):
        owner.committed("http://127.0.0.1", recorded_receipt()["txHash"], None)


@pytest.mark.parametrize("value", [
    {"error": {"code": -32601, "message": "method unavailable"}},
    {"result": recorded_receipt()["txHash"]},
    {"error": {"code": -32000, "message": "node unavailable"}},
])
def test_f065_unavailable_rpc_or_success_cannot_satisfy_transaction_refusal(value):
    owner = module("program_operations")
    for kind in ("duplicate", "tampered"):
        with pytest.raises(RuntimeError, match="refusal"):
            owner.submission_refusal(value, kind)


def test_f034_manifest_is_checked_before_both_original_lifecycle_corpora():
    text = embedded("104.35.4", "GATE")
    ast.parse(text)
    assert text.index("built = owner.verify(root, manifest_path)") < text.index("market = binary(")
    assert text.rindex("owner.verify(root, manifest_path)") > text.index("runtime = binary(")
    assert "private compiler-produced current-revision market manifest required" in text
    assert "for variable in (*owner.TARGETS, 'LAYERX_MARKET_WASM')" in text
    assert "os.environ[variable] != actual" in text
    assert "os.environ[variable] = actual" in text
    corpus = text.split("market = binary('LAYERX_MARKET_TEST_BINARY')", 1)[1].split("owner.verify(root, manifest_path)", 1)[0]
    assert hashlib.sha256(corpus.encode()).hexdigest() == "80d82f91310a64c11b36da43a923b0725995fab22aad2cd9364e128ba8745f0f"
    assert "len(declared)" in corpus and "counts[0][1:] != ('0', '0')" in corpus
    assert "test result: ok\\. 1 passed; 0 failed; 0 ignored;" in corpus


def test_f034_compiler_produces_exact_binaries_and_runtime_guest():
    owner = module("market_build")
    assert set(owner.TARGETS) == {"LAYERX_MARKET_TEST_BINARY", "LAYERX_MARKET_RUNTIME_TEST_BINARY"}
    assert owner.TEST_COMMAND == [
        "cargo", "test", "--locked", "--manifest-path", "programs/Cargo.toml", "-p", "layerx-programs-market",
        "-p", "layerx-programs-runtime", "--lib", "--no-run", "--message-format=json",
    ]
    assert owner.GUEST_COMMAND == [
        "cargo", "build", "--locked", "--manifest-path", "programs/Cargo.toml", "-p", "layerx-programs-market",
        "--release", "--target", "wasm32-unknown-unknown", "--message-format=json",
    ]
    for manifest, target in owner.TARGETS.values():
        parsed = tomllib.loads((ROOT / manifest).read_text())
        assert parsed["package"]["name"].replace("-", "_") == target
    runtime = (ROOT / "programs/crates/layerx-programs-runtime/src/market_tests.rs").read_text()
    assert '"LAYERX_MARKET_WASM",\n            "layerx-programs-market"' in runtime
    loader = (ROOT / "programs/crates/layerx-programs-runtime/tests/support/compiled_guest.rs").read_text()
    assert "std::env::var_os(variable)" in loader and "std::fs::read(&path)" in loader
    text = (GATES / "market_build.py").read_text()
    build = body(text, "build")
    verify = body(text, "verify")
    assert build.index("result.returncode == 0") < build.index("evidence.save(")
    for guard in ("built['identity'] == evidence.source_identity(root)", "log['exit'] == 0",
                  "log['command'] == command", "evidence.digest(log['path']) == log['sha256']",
                  "selected == built['compiler_artifacts']", "evidence.checked_artifact(",
                  "guest_artifact(selected['LAYERX_MARKET_WASM']) == built['artifacts']['LAYERX_MARKET_WASM']"):
        assert guard in verify
    with pytest.raises(RuntimeError, match="exact compiler market test target"):
        owner.compiler_records(ROOT, [], [])


def test_f034_private_manifest_transport_reopens_and_rejects_unsafe_files(tmp_path):
    evidence = module("market_build").evidence
    directory = tmp_path / "private"
    directory.mkdir(mode=0o700)
    path = directory / "recorded-receipt.json"
    recorded = recorded_receipt()
    evidence.save(path, recorded)
    assert path.stat().st_mode & 0o777 == 0o600
    assert module("market_build").evidence.load(path) == recorded
    with pytest.raises(FileExistsError):
        evidence.save(path, recorded)
    path.chmod(0o644)
    with pytest.raises(RuntimeError, match="private regular evidence"):
        evidence.load(path)
    path.chmod(0o600)
    link = directory / "linked.json"
    link.symlink_to(path)
    with pytest.raises(RuntimeError, match="canonical evidence"):
        evidence.load(link)
    hard = directory / "hard.json"
    os.link(path, hard)
    with pytest.raises(RuntimeError, match="canonical evidence"):
        evidence.load(path)
    hard.unlink()
    path.write_text('{"record":1,"record":2}')
    with pytest.raises(RuntimeError, match="duplicate manifest field"):
        evidence.load(path)


def test_f034_source_closure_hashes_real_build_inputs_and_detects_changes(tmp_path):
    evidence = module("market_build").evidence
    names = ["programs/Cargo.toml", "programs/Cargo.lock", "programs/crates/layerx-programs-market/src/lib.rs",
             "programs/crates/layerx-programs-runtime/tests/support/compiled_guest.rs", "Makefile"]
    for name in names:
        target = tmp_path / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / name, target)
    original = evidence.source_hashes(tmp_path, names)
    assert set(original) == set(names)
    target = tmp_path / "Makefile"
    target.write_bytes(target.read_bytes() + b"\n# changed source input\n")
    changed = evidence.source_hashes(tmp_path, names)
    assert original["Makefile"] != changed["Makefile"]
    with pytest.raises(RuntimeError, match="unavailable input"):
        evidence.source_hashes(tmp_path, names + ["missing-input"])
    with pytest.raises(RuntimeError, match="repository-relative"):
        evidence.source_hashes(tmp_path, ["../outside"])
    with pytest.raises(RuntimeError, match="compiled executable digest changed"):
        # Read an actual executable gate source; no native artifact is fabricated or executed.
        evidence.checked_artifact({"path": str(GATES / "104.35.4.sh"), "sha256": "0" * 64}, GATES / "104.35.4.sh")


def catalogue_and_binding():
    candidate = (ROOT / "tools/paxeer-x/candidate.py").read_text()
    ns = declarations(candidate, {"catalogue", "keys", "require", "Invalid", "BINDING_FIELDS", "UNKNOWN"})
    roster = ns["catalogue"](ROOT / "spec/layerx-merge/spec.kvx")
    dispatcher = (ROOT / "tools/paxeer-x/verify-task.sh").read_text().split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]
    declarations(dispatcher, {"bind", "Refuse", "EXIT"}, ns)
    return roster, ns


@pytest.mark.parametrize("gate,services", [("104.37.2", ["merge-tools", "sequencer"]),
                                            ("104.38.3", ["human-service", "human-kms"])])
def test_f045_f046_real_catalogue_to_dispatcher_binding_and_unknown_refusal(gate, services, tmp_path):
    roster, ns = catalogue_and_binding()
    header = re.search(r"^# paxeer-x-services: (.+)$", source(gate), re.M)[1].split()
    assert header == services and set(header) <= set(roster)
    data = {"services": [{"id": name, "bindings": {field: ns["UNKNOWN"] for field in ns["BINDING_FIELDS"]}}
                          for name in roster]}
    bound, unknown = ns["bind"](data, GATES / (gate + ".sh"))
    assert set(bound) == set(services)
    assert set(unknown) == {name + "." + field for name in services for field in ns["BINDING_FIELDS"]}
    old = tmp_path / "unknown.sh"
    old.write_text("# paxeer-x-services: " + ("hpx" if gate == "104.37.2" else "human") + "\n")
    with pytest.raises(ns["Refuse"], match="absent from the candidate") as refusal:
        ns["bind"](data, old)
    assert refusal.value.code == 6


def test_f045_hpx_remains_build_dependency_and_full_runtime_checks_survive():
    text = source("104.37.2")
    assert "# Build dependencies: tools/hpx; wasm-runtime/internal/api; wasm/x/wasm/artifacts" in text
    assert hashlib.sha256(text.split("\n", 3)[3].encode()).hexdigest() == "2bec265ccacf84f8447990a7b6fd47983c95ab7ab8a6ed87b5a8cbfe5cd9d75c"
    assert 'kill "$registry_pid"' in text and 'restart lost persisted peers' in text
    assert 'libwasmvm155.aarch64.so' in text and 'owner-approved' in text


def test_f046_full_service_browser_and_component_checks_survive():
    assert hashlib.sha256(source("104.38.3").split("\n", 2)[2].encode()).hexdigest() == "6ff79e1b9f7d15eadb101c1b78b07a6d9e86ebe14a3defe965e175172442043e"
    text = embedded("104.38.3")
    ast.parse(text)
    assert "'identity-boundary'" in text and "'browser-boundary'" in text
    assert "'components'" in text and "owner.executable(" in text
    assert "results[0][1:] == ('0', '0')" in text


def test_f066_retained_refusal_producer_drives_drift_check_and_all_seven_suites():
    owner = module("receipt_refusals")
    assert owner.check(ROOT) == 6
    original = json.loads((ROOT / owner.SOURCE).read_text())
    assert owner.expected_cases(ROOT) == [case for case in original["cases"] if case["vector"] == "V3"]
    producer = (ROOT / owner.PRODUCER).read_text()
    assert "func TestVectorFilesV1ToV3MatchSequencerReceiptFixture" in producer
    assert '{"vector-v3-receipt-refusals", "V3", 6}' in producer
    text = embedded("104.38.6")
    ast.parse(text)
    assert "['python3', 'tools/paxeer-x/gates/receipt_refusals.py', '--check']" in text
    assert "generate_program_receipt_refusals.py" not in text
    corpus = source("104.38.6").split("    credit('fixture derivation', 1)", 1)[1]
    assert hashlib.sha256(corpus.encode()).hexdigest() == "855ea3fa9c519f8d2621a2d4697193d8bd96b8e59167331ebe8da1256d5485fc"
    for language in ("Rust", "Go", "TypeScript", "Python", "JVM", "Swift", ".NET"):
        assert language + " SDK receipts" in text
    assert "require(count > 0" in text


@pytest.mark.parametrize("change", ["receipt", "missing", "producer"])
def test_f066_real_fixture_drift_is_refused_without_rewriting(change, tmp_path):
    owner = module("receipt_refusals")
    for name in (owner.SOURCE, owner.OUTPUT):
        target = tmp_path / name
        target.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / name, target)
    path = tmp_path / (owner.SOURCE if change == "producer" else owner.OUTPUT)
    value = json.loads(path.read_text())
    if change == "receipt":
        value["cases"][0]["signedReceipt"] = value["cases"][0]["signedReceipt"][:-2]
    elif change == "missing":
        value["cases"].pop()
    else:
        value["provenance"]["generator"] = "retired"
    path.write_text(json.dumps(value))
    before = path.read_bytes()
    with pytest.raises(ValueError, match="drift|retained sequencer producer"):
        owner.check(tmp_path)
    assert path.read_bytes() == before


def test_f021_native_funding_uses_retained_bank_send_and_public_root_roster():
    owner = module("onboarding_funding")
    inventory = (ROOT / owner.INVENTORY).read_text()
    roots = owner.public_roots(inventory)
    assert len(roots) == 30
    assert "https://api14.mainnet-beta.paxeer.network" in roots
    assert "wss://api-mainnet-beta.paxeer.network" in roots
    assert owner.bank_send_contract(ROOT) == {
        ("address", "address", "string", "uint256"),
        ("address", "address", "string", "uint256", "bytes32"),
    }
    bank = (ROOT / "precompiles/bank/Bank.sol").read_text()
    assert "BANK_PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000001001" in bank
    assert "function send(" in bank
    funding = (ROOT / owner.BANK_PRODUCER).read_text()
    assert "pub(super) fn sign_native(" in funding
    assert '"eth_sendRawTransaction"' in funding and "DEPLOYMENT_CHAIN_ID: u64 = 125" in funding
    assert owner.native_funding((ROOT / "docs/site/docs/overview/getting-started.md").read_text(), roots)
    text = source("108.0.7")
    assert "funded_by_custody_credit" not in text and "router=" not in text
    assert 'python3 tools/paxeer-x/gates/onboarding_funding.py "$1"' in text
    assert 'check "native-bank-send:$page" funded_by_native_bank_send "$page"' in text
    assert text.count('docs/wiki/Getting-Started-Beta.md') == 1
    assert "docs/site/docs/platform/index.md" in text


@pytest.mark.parametrize("suffix", ["/rpc", "/evm", "/rpc/ws", "?route=rpc", ":8545"])
def test_f021_public_roster_refuses_prefixed_or_non_public_roots(suffix):
    owner = module("onboarding_funding")
    text = (ROOT / owner.INVENTORY).read_text().replace(
        "`https://api1.mainnet-beta.paxeer.network`", "`https://api1.mainnet-beta.paxeer.network" + suffix + "`"
    )
    with pytest.raises(ValueError, match="served root endpoint"):
        owner.public_roots(text)


@pytest.mark.parametrize("addition", ["\nFAUCET_URL=https://faucet.paxeer.network\n",
                                      "\n```sh\npython tests/bridge/custody_credit.py attest\n```\n",
                                      "\n```sh\nbuild/tests/bridge/sign-credit profile credit\n```\n"])
def test_f021_funding_refuses_unserved_faucet_and_retired_producers(addition):
    owner = module("onboarding_funding")
    roots = owner.public_roots((ROOT / owner.INVENTORY).read_text())
    current = (ROOT / "docs/site/docs/overview/getting-started.md").read_text()
    with pytest.raises(ValueError, match="forbidden|retired funding"):
        owner.native_funding(current + addition, roots)


def test_f021_keeps_unserved_host_refusal_and_private_page_corpus():
    text = source("108.0.7")
    corpus = text.split("private_network_only() {", 1)[1].split("check docs-names-clean", 1)[0]
    assert hashlib.sha256(corpus.encode()).hexdigest() == "795b4896435ae93c81738d19542161b89b386f179c1009f6db716cfa8da35656"
    assert "docs-names: docs/offending.md: faucet.paxeer.network is not a public name" in text
    assert '[[ $(wc -l <<<"$out") -eq 2 ]]' in text
    assert "services/platform/docs/beta-resets.ics" in text
    assert "check package-info-docs-location no_docs_host" in text
    scanner = (ROOT / "tools/bringup/docs-names.sh").read_text()
    assert "inventory=docs/site/docs/reference/public-rpc.md" in scanner
    assert 'docs-names "$inventory" "$spec" "${files[@]}"' in scanner


@contextmanager
def arguments(values):
    before = sys.argv
    try:
        sys.argv = ["custody_governance.py", *values]
        yield
    finally:
        sys.argv = before


@pytest.mark.parametrize("flags", [[], ["--produce", "retired"], ["--manifest", "retired/manifest.json"]])
def test_f035_actual_custody_producer_and_consumer_always_refuse(flags, capsys):
    path = ROOT / "tools/qualification/paxeer-x/custody_governance.py"
    spec = importlib.util.spec_from_file_location("retired_custody_849", path)
    owner = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(owner)
    with arguments(flags), pytest.raises(SystemExit) as refused:
        owner.main()
    assert refused.value.code == 2
    assert "custody governance artifact production and execution have been retired" in capsys.readouterr().err


def test_f035_registry_and_selector_are_explicitly_retired_with_current_qualification_pointer():
    registry = tomllib.loads((GATES / "registry.kvx").read_text())
    entry = registry["task"]["108"]["1"]["10"]
    assert entry["gate"] == "tools/paxeer-x/gates/108.1.10.sh"
    assert entry["status"] == "retired"
    assert entry["replacement"] == "tools/paxeer-x/gov/test.sh"
    text = source("108.1.10")
    assert "PAXEER_X_CUSTODY_GOVERNANCE_ARTIFACTS" not in text
    assert "--manifest" not in text
    assert "tools/paxeer-x/gov/test.sh" in text
    assert shlex.split(text.split("exec ", 1)[1]) == ["python3", "tools/qualification/paxeer-x/custody_governance.py"]
    assert "PAXEER_X_GATE" not in text
    qualification = (ROOT / entry["replacement"]).read_text()
    assert '"$ROOT/tools/paxeer-x/gov/render.sh"' in qualification
    assert 'paxd offline dry run' in qualification
    retirement = (ROOT / "precompiles/layerxcustody/retire_test.go").read_text()
    assert "TestEveryCustodyMethodRevertsModuleDisabledFromTheMergePlanHeight" in retirement
    assert "LXP_MODULE_DISABLED: module disabled; layerxCustody retired" in retirement


def test_record_maps_all_findings_to_real_tests_and_scoped_wiring_requests():
    record = tomllib.loads(RECORD.read_text())
    audit = tomllib.loads((ROOT / "spec/layerx-merge/programme/audit/records/22.30.181.kvx").read_text())
    assert set(record["finding"]) == {"f064", "f065", "f034", "f045", "f046", "f066", "f021", "f035"}
    tests = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef)}
    for name, finding in record["finding"].items():
        assert finding["file"] == audit["finding"][name]["file"]
        assert finding["tests"] and set(finding["tests"]) <= tests
        assert finding["facts"] and finding["review_only"]
    task = record["repair"]
    assert task["base_revision"] == "57000844c90cca6b33229318d195f087a30c1998"
    assert task["continuation_base_revision"] == "24ad444517739fd50d4cf816446559d52d7bdd77"
    assert "main's exact service roster" in task["continuation"]
    assert "46 passed" in task["prior_evidence"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "Not run" in task["baseline"]
    assert "content-derived" in task["verified_revision"]
    assert "no native" in task["proof_boundary"]
    assert set(record["wiring"]) >= {"program_artifacts", "market_artifacts", "onboarding", "custody_build", "duplicate_policy"}
    for request in record["wiring"].values():
        assert request["path"] and request["owner_domain"] and request["request"]
    assert "hf-build" in task["shell_parsing"] and "bash -n" in task["shell_parsing"]
    for script in SCRIPTS:
        assert source(script).startswith("#!/usr/bin/env bash\n")
        assert source(script).endswith("\n")
