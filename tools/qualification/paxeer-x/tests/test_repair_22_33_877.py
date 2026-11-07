"""Real manifest publication/admission and committed RPC transport, without toolchains.

Opaque artifact transport uses committed source bytes, never a replacement native
executable. The chain peer replays the node producer's signed receipt corpus;
manifest reopening proves the Python evidence component's persistent state.
"""
from contextlib import contextmanager
from copy import deepcopy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import hashlib
import importlib.util
import json
import re
from pathlib import Path
import shutil
import threading

import pytest

QUAL = Path(__file__).resolve().parents[1]
ROOT = QUAL.parents[2]
SOURCE = {"revision": "aa5e0820b3652890f3812a220f4b91f4f8a45387",
          "tree": "b3c095a9646aebc3b7d5fd6bef04b6f14088ea12", "dirty": False}
IDENTITY = {key: SOURCE[key] for key in ("revision", "tree")}
SETTINGS = {"PROGRAMS_TARGET_DIR": "/private/programs", "PROGRAMS_RUNTIME_LIB": "/private/runtime.a"}


def load(name):
    definition = importlib.util.spec_from_file_location(
        "repair_877_" + name.replace("-", "_"), QUAL / (name + ".py"))
    module = importlib.util.module_from_spec(definition)
    definition.loader.exec_module(module)
    return module


def document(directory, name, relative="program-interface-producer.py"):
    path = directory / name
    path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    shutil.copyfile(QUAL / relative, path)
    path.chmod(0o600)
    return path


def rewrite(path, value):
    path.write_text(json.dumps(value))
    path.chmod(0o600)


def native_publication(directory):
    producer = load("native-programs-link")
    source = {"tools/qualification/paxeer-x/native-programs-link.py": producer.digest(QUAL / "native-programs-link.py")}
    production = {name: producer.artifact(document(directory, name)) for name in producer.PRODUCTION}
    runtime = {name: producer.artifact(document(directory, name + ".a")) for name in ("kernel", "programs")}
    producer.publish_manifest(directory, SOURCE["revision"], source, production, runtime)
    path = directory / "native-programs-artifacts.json"
    return producer, path, source


def test_f013_paxd_runtime_publication_and_reopened_admission(tmp_path):
    producer, path, sources = native_publication(tmp_path)
    before = path.read_bytes()
    value = load("native-programs-link").admit_manifest(json.loads(before), SOURCE["revision"], sources)
    assert set(value["production"]) == {"paxd", "layerx-genesis-build", "layerx-module-registry"}
    assert set(value["runtime"]) == {"kernel", "programs"}
    assert "layerxd" not in producer.PRODUCTION
    assert path.read_bytes() == before


@pytest.mark.parametrize("damage", ["revision", "sources", "daemon", "missing-runtime", "binary", "archive"])
def test_f013_keeps_source_executable_and_runtime_digest_binding(tmp_path, damage):
    producer, path, sources = native_publication(tmp_path)
    value = json.loads(path.read_text())
    if damage == "revision":
        value["revision"] = "0" * 40
    elif damage == "sources":
        value["inputs"] = {}
    elif damage == "daemon":
        value["production"]["layerxd"] = value["production"].pop("paxd")
    elif damage == "missing-runtime":
        del value["runtime"]["programs"]
    else:
        saved = value["production"]["paxd"] if damage == "binary" else value["runtime"]["programs"]
        Path(saved["path"]).write_bytes(b"changed after publication")
    rewrite(path, value)
    with pytest.raises(RuntimeError):
        producer.admit_manifest(json.loads(path.read_text()), SOURCE["revision"], sources)


def test_f048_operations_registry_anchors_and_real_producer_contracts():
    text = (ROOT / "contracts/operations.md").read_text()
    headings = {heading.lower().replace(" ", "-") for heading in re.findall(r"^#+ (.+)$", text, re.M)}
    for anchor in ("qual-cli", "qual-env", "docker-cli", "docker-env", "docker-formats"):
        assert f'id="{anchor}"' in text or anchor in headings
    assert "## Bench config" in text and "chain ID" in text and "paxd" in text
    for name in ("native-programs-link", "native-terminal-outcome", "native_send_purpose",
                 "program-interface-producer", "runtime-boundary-witness", "storage-recovery-producer"):
        assert name + ".py" in text
    for field in ("source.json", "native-programs-artifacts.json", "receipt_keys", "build_exit", "required_cases"):
        assert field in text


def corpus():
    value = json.loads((ROOT / "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json").read_text())
    assert value["provenance"]["generator"] == "rpc/sequencer_receipt_vectors_test.go"
    return {item["id"]: item for item in value["cases"]}


class ChainPeer:
    """An in-process JSON-RPC peer replaying original signed producer bytes."""
    def __init__(self):
        cases = corpus()
        self.program = cases["V1-layerxprograms-call"]
        self.refusal = cases["V2-pax-transfer-insufficient-funds"]
        self.records = {item["receiptJson"]["txHash"]: item for item in (self.program, self.refusal)}
        self.requests = []
        self.damage = None
        self.modules = {"eth": "1.0", "pax": "1.0"}
        self.send_error = -32602

    def pins(self):
        pin = self.program["pinnedKey"]
        return [{"key_id": pin["keyId"][2:], "public_key": pin["publicKey"][2:],
                 "first_height": pin["firstHeight"], "last_height": pin["lastHeight"]}]

    def transactions(self):
        # The three execution roles exercise the same original native terminal codec.
        return {name: (self.refusal if name == "terminal-0" else self.program)["receiptJson"]["txHash"]
                for name in load("native-terminal-outcome").REQUIRED}

    def committed(self, reference):
        row = deepcopy(self.records[reference]["receiptJson"])
        row.update(rung="committed", blockNumber=row["height"],
                   blockHash="0x" + hashlib.sha256(reference.encode()).hexdigest())
        return row

    def answer(self, request):
        method, params = request["method"], request["params"]
        self.requests.append((method, params))
        result = None
        if method == "eth_chainId":
            result = "0x7d"
        elif method == "rpc_modules":
            result = self.modules
        elif method == "eth_call":
            assert params == [{"to": "0x" + "00" * 20, "data": "0x"}, "latest"]
            result = "0x"
        elif method == "eth_sendRawTransaction":
            assert params == ["0x"]
            return {"jsonrpc": "2.0", "id": request["id"],
                    "error": {"code": self.send_error, "message": "Invalid params"}}
        elif method == "pax_sequencerReceipt":
            if params[0] in self.records:
                result = self.committed(params[0])
        elif method == "eth_getRawTransactionByHash":
            result = self.records[params[0]]["rawTransaction"]
        elif method == "eth_getBlockByHash":
            assert params[1] is False
            rows = [self.committed(key) for key in self.records]
            row = next(item for item in rows if item["blockHash"] == params[0])
            result = {"hash": row["blockHash"], "number": row["blockNumber"], "transactions": [row["txHash"]]}
        else:
            return {"jsonrpc": "2.0", "id": request["id"], "error": {"code": -32601, "message": "Method not found"}}
        response = {"jsonrpc": "2.0", "id": request["id"], "result": result}
        if self.damage is not None:
            self.damage(method, response)
        return response


@contextmanager
def serving(peer):
    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            assert self.path == "/" and self.headers.get_content_type() == "application/json"
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            assert request["jsonrpc"] == "2.0" and type(request["id"]) is int
            raw = json.dumps(peer.answer(request)).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield "http://127.0.0.1:" + str(server.server_port) + "/"
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()


def test_f016_committed_receipt_transport_terminal_digest_and_all_four_roles(tmp_path):
    terminal = load("native-terminal-outcome")
    peer = ChainPeer()
    with serving(peer) as endpoint:
        produced = terminal.capture_outcomes(endpoint, peer.transactions(), peer.pins())
    path = tmp_path / "committed-terminals.json"
    rewrite(path, produced)
    admitted = load("native-terminal-outcome").admit_captures(json.loads(path.read_text()))
    assert {row["name"] for row in admitted} == terminal.REQUIRED
    assert len(admitted) == 4
    assert sum(row["terminal_sha256"] == hashlib.sha256(b"").hexdigest() for row in admitted) == 1
    for name in ("pax_sequencerReceipt", "eth_getBlockByHash", "eth_getRawTransactionByHash"):
        assert sum(method == name for method, _ in peer.requests) == 4
    assert not any("batch" in method.lower() or "maintenance" in method.lower() for method, _ in peer.requests)


@pytest.mark.parametrize("damage", ["rung", "block", "transaction", "signature", "return-data", "logs", "rpc-id"])
def test_f016_refuses_uncommitted_substituted_or_corrupt_evidence(damage):
    terminal = load("native-terminal-outcome")
    peer = ChainPeer()

    def corrupt(method, response):
        row = response["result"]
        if damage == "rpc-id":
            response["id"] = 2
        elif method == "pax_sequencerReceipt":
            if damage == "rung":
                row["rung"] = "receipted"
            elif damage == "signature":
                row["receipt"] = row["receipt"][:-2] + ("01" if row["receipt"][-2:] != "01" else "02")
            elif damage == "return-data":
                row["returnData"] = "0x00"
            elif damage == "logs":
                row["logs"] = []
        elif method == "eth_getBlockByHash":
            if damage == "block":
                row["hash"] = "0x" + "00" * 32
            elif damage == "transaction":
                row["transactions"] = []

    peer.damage = corrupt
    with serving(peer) as endpoint, pytest.raises((RuntimeError, ValueError)):
        terminal.capture_outcomes(endpoint, peer.transactions(), peer.pins())


def test_f016_rejects_missing_role_and_changed_saved_capture(tmp_path):
    terminal = load("native-terminal-outcome")
    peer = ChainPeer()
    with serving(peer) as endpoint:
        with pytest.raises(RuntimeError, match="four"):
            terminal.capture_outcomes(endpoint, {"terminal-0": peer.refusal["receiptJson"]["txHash"]}, peer.pins())
        produced = terminal.capture_outcomes(endpoint, peer.transactions(), peer.pins())
    produced["captures"][0]["terminal_sha256"] = "0" * 64
    with pytest.raises(RuntimeError, match="changed"):
        terminal.admit_captures(produced)


def copy_source(root, relative, original=None):
    source = ROOT / (original or relative)
    target = root / relative
    target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    shutil.copyfile(source, target)
    return target


def custody_tree(root):
    for relative in ("tools/qualification/paxeer-x/native_send_purpose.py",
                     "apps/web/e2e/software-authenticator.ts", "services/agent/schema/agent-api/v1.kvx",
                     "services/human/Cargo.toml", "services/human/crates/layerx-human-service/src/lib.rs",
                     "services/agent/crates/layerx-client/src/lib.rs", "services/wallet/attestor/go.mod",
                     "services/wallet/attestor/cmd/attestor/main.go", "go.mod", "go.sum",
                     "programs/crates/layerx-programs-runtime/src/lib.rs",
                     "programs/crates/layerx-programs-sandbox/src/programs_contract.md"):
        copy_source(root, relative)


def custody_publication(directory):
    producer = load("native_send_purpose")
    workspace = directory / "workspace"
    custody_tree(workspace)
    files = producer.inventory(workspace)
    source = producer.source_digest(files)
    artifact = document(directory, "opaque-executable")
    artifact.chmod(0o700)
    record = {"path": str(artifact), "sha256": producer.B.digest(artifact),
              "source_revision": SOURCE["revision"], "source_digest": source, "build_exit": 0}
    value = {"schema": "layerx-human-native-send-purpose-build.v1", "source_files": files,
             "source_revision": SOURCE["revision"], "source_digest": source,
             "artifacts": {name: dict(record) for name in ("attestor", "go_corpus", "rust_corpus")}}
    path = directory / "build.json"
    rewrite(path, value)
    return producer, workspace, path


def test_f021_current_production_roots_bind_original_manifest(tmp_path):
    producer, root, path = custody_publication(tmp_path)
    material = json.loads(path.read_text())
    assert any(name.startswith("services/human/") for name in material["source_files"])
    assert any(name.startswith("services/agent/") for name in material["source_files"])
    assert any(name.startswith("services/wallet/") for name in material["source_files"])
    artifacts = producer.admit_build(material, SOURCE["revision"], SOURCE["revision"], root)
    assert set(artifacts) == {"attestor", "go_corpus", "rust_corpus"}


@pytest.mark.parametrize("relative", ["services/human/crates/layerx-human-service/src/lib.rs",
    "services/agent/crates/layerx-client/src/lib.rs", "services/wallet/attestor/cmd/attestor/main.go",
    "programs/crates/layerx-programs-runtime/src/lib.rs",
    "programs/crates/layerx-programs-sandbox/src/programs_contract.md", "go.mod"])
def test_f021_rejects_stale_artifacts_after_compiled_source_changes(tmp_path, relative):
    producer, root, path = custody_publication(tmp_path)
    with (root / relative).open("a") as output:
        output.write("\n")
    with pytest.raises(RuntimeError, match="source binding"):
        producer.admit_build(json.loads(path.read_text()), SOURCE["revision"], SOURCE["revision"], root)


def test_f021_rejects_source_addition_and_removal(tmp_path):
    producer, root, path = custody_publication(tmp_path)
    added = copy_source(root, "services/agent/crates/layerx-client/src/new.rs",
                        "services/agent/crates/layerx-client/src/lib.rs")
    with pytest.raises(RuntimeError, match="source binding"):
        producer.admit_build(json.loads(path.read_text()), SOURCE["revision"], SOURCE["revision"], root)
    added.unlink()
    (root / "services/wallet/attestor/cmd/attestor/main.go").unlink()
    with pytest.raises(RuntimeError, match="source binding"):
        producer.admit_build(json.loads(path.read_text()), SOURCE["revision"], SOURCE["revision"], root)


def interface_publication(directory):
    producer = load("program-interface-producer")
    record = producer.build_inputs(SOURCE, directory / "native", 4, {})
    artifacts = {name: document(directory, name) for name in producer.ARTIFACTS}
    logs = [document(directory, f"build-{number}.log") for number in range(producer.BUILD_LOG_COUNT)]
    path = producer.publish_manifest(directory, record, artifacts, logs)
    return producer, path


def test_f061_actual_producer_output_admitted_unchanged_by_real_consumer(tmp_path):
    producer, path = interface_publication(tmp_path)
    original = path.read_bytes()
    consumer = load("program-interface")
    value = consumer.admit_manifest(path, SOURCE)
    assert path.read_bytes() == original
    assert len(value["commands"]) == len(value["logs"]) == 3
    assert set(value["artifacts"]) == producer.ARTIFACTS
    assert [row[0] for row in consumer.interface_suites(value["artifacts"])] == ["protocol", "legacy"]
    assert "native_receipt_authorized_interface_reads" in value["required_protocol_cases"]
    assert load("program-interface").admit_manifest(path, SOURCE) == value
    assert producer.NATIVE_FIXTURE_CASES == consumer.producer.NATIVE_FIXTURE_CASES


@pytest.mark.parametrize("damage", ["artifact", "log", "case", "source", "native-evidence"])
def test_f061_retains_consumer_admission_and_native_fixture_requirements(tmp_path, damage):
    producer, path = interface_publication(tmp_path)
    consumer = load("program-interface")
    value = json.loads(path.read_text())
    if damage == "native-evidence":
        with pytest.raises(ValueError, match="required"):
            consumer.native_fixtures(None, SOURCE, tmp_path / "fixtures")
        with pytest.raises(FileNotFoundError):
            producer.publish_native_evidence(tmp_path, SOURCE, tmp_path / "absent-cases")
        return
    if damage == "artifact":
        Path(value["artifacts"]["sandbox-staticlib"]["path"]).write_bytes(b"changed")
    elif damage == "log":
        value["logs"].pop()
    elif damage == "case":
        value["required_protocol_cases"].remove("native_receipt_authorized_interface_reads")
    else:
        value["source"]["revision"] = "0" * 40
    rewrite(path, value)
    with pytest.raises(ValueError):
        consumer.admit_manifest(path, SOURCE)


def test_f061_publisher_refuses_an_incompatible_case_inventory(tmp_path):
    producer = load("program-interface-producer")
    record = producer.build_inputs(SOURCE, tmp_path / "native", 4, {})
    record["required_protocol_cases"] = []
    with pytest.raises(ValueError, match="case inventory"):
        producer.publish_manifest(tmp_path, record, {}, [])
    assert not (tmp_path / "manifest.json").exists()


def test_f018_separate_real_hosted_schema_and_node_root_dispatch():
    schema = load("router-authority-schema")
    hosted = json.loads((ROOT / "services/platform/hosted/gateway/openrpc.json").read_text())
    assert schema.hosted_schema(hosted) == len(hosted["methods"])
    peer = ChainPeer()
    with serving(peer) as endpoint:
        assert schema.node_schema(endpoint) == 4
    assert {method for method, _ in peer.requests} == {"rpc_modules", *schema.NODE_METHODS}
    assert not {"eth_call", "eth_sendRawTransaction"} & {item["name"] for item in hosted["methods"]}


@pytest.mark.parametrize("damage", ["missing", "duplicate", "foreign-node-method", "shape"])
def test_f018_hosted_schema_refuses_drift(damage):
    schema = load("router-authority-schema")
    value = json.loads((ROOT / "services/platform/hosted/gateway/openrpc.json").read_text())
    if damage == "missing":
        value["methods"] = [item for item in value["methods"] if item["name"] != "px_getBalances"]
    elif damage == "duplicate":
        value["methods"].append(deepcopy(value["methods"][0]))
    elif damage == "foreign-node-method":
        value["methods"].append({"name": "eth_call", "params": [], "result": {}})
    else:
        value["methods"][0]["params"] = {}
    with pytest.raises(RuntimeError):
        schema.hosted_schema(value)


@pytest.mark.parametrize("damage", ["namespace", "chain", "method", "root"])
def test_f018_node_schema_rejects_missing_methods_and_wrong_network(damage):
    schema = load("router-authority-schema")
    peer = ChainPeer()
    if damage == "namespace":
        del peer.modules["pax"]
    elif damage == "method":
        peer.send_error = -32601
    elif damage == "chain":
        def wrong_chain(method, response):
            if method == "eth_chainId":
                response["result"] = "0x1"
        peer.damage = wrong_chain
    with serving(peer) as endpoint, pytest.raises(RuntimeError):
        schema.node_schema(endpoint + "rpc" if damage == "root" else endpoint)


def boundary_tree(root):
    for relative in ("rust-toolchain.toml", "tools/qualification/paxeer-x/runtime-boundary-witness.py",
                     "programs/Cargo.toml", "programs/Cargo.lock",
                     "programs/crates/layerx-programs-runtime/src/lib.rs",
                     "programs/crates/layerx-programs-runtime/tests/boundary_witness.rs",
                     "programs/sdk/rust/src/lib.rs"):
        copy_source(root, relative)


def boundary_publication(directory):
    producer = load("runtime-boundary-witness")
    root = directory / "workspace"
    boundary_tree(root)
    sources = producer.source_inputs(root)
    binary = document(directory, "opaque-executable")
    binary.chmod(0o700)
    log = document(directory, "build.log")
    path = producer.publish_manifest(directory, IDENTITY, sources, binary, log, 0)
    return producer, root, path, sources


def test_f039_exact_source_executable_and_revision_manifest_roundtrip(tmp_path):
    producer, root, path, sources = boundary_publication(tmp_path)
    value = json.loads(path.read_text())
    assert producer.admit_manifest(value, IDENTITY, producer.source_inputs(root)) == Path(value["executable"]["path"])
    assert load("runtime-boundary-witness").admit_manifest(value, IDENTITY, sources).is_file()
    assert value["required_cases"] == sorted(producer.REQUIRED_CASES)
    assert len(producer.REQUIRED_CASES) == 8


@pytest.mark.parametrize("damage", ["empty", "extra", "missing", "source-edit", "source-addition",
                                   "binary", "log", "revision", "cases", "build-exit"])
def test_f039_refuses_incomplete_stale_or_unbound_manifest(tmp_path, damage):
    producer, root, path, sources = boundary_publication(tmp_path)
    value = json.loads(path.read_text())
    if damage == "empty":
        value["sources"] = {}
    elif damage == "extra":
        value["sources"]["foreign.rs"] = "0" * 64
    elif damage == "missing":
        value["sources"].pop(next(iter(sources)))
    elif damage == "source-edit":
        with (root / "programs/crates/layerx-programs-runtime/src/lib.rs").open("a") as output:
            output.write("\n")
    elif damage == "source-addition":
        copy_source(root, "programs/vendor/new/src/lib.rs", "programs/sdk/rust/src/lib.rs")
    elif damage in {"binary", "log"}:
        saved = value["executable" if damage == "binary" else "build_log"]
        Path(saved["path"]).write_bytes(b"changed")
    elif damage == "revision":
        value["source"]["revision"] = "0" * 40
    elif damage == "cases":
        value["required_cases"].pop()
    else:
        value["build_exit"] = 1
    rewrite(path, value)
    with pytest.raises(RuntimeError):
        producer.admit_manifest(json.loads(path.read_text()), IDENTITY, producer.source_inputs(root))


def boundary_output(producer):
    return "\n".join([*(f"test {name} ... ok" for name in sorted(producer.REQUIRED_CASES)),
        f"test result: ok. {len(producer.REQUIRED_CASES)} passed; 0 failed; 0 ignored; 0 measured; 0 filtered out;"])


def test_f039_requires_every_executed_boundary_result():
    producer = load("runtime-boundary-witness")
    output = boundary_output(producer)
    assert producer.case_results(output, 0) == 8
    source = (ROOT / "programs/crates/layerx-programs-runtime/tests/boundary_witness.rs").read_text()
    for name in producer.REQUIRED_CASES:
        assert f"fn {name}()" in source
    for changed, exit_code in (("", 0), (output, 1),
        (output.replace("test integer_boundaries_restore_and_execute_the_real_engine_step ... ok", ""), 0),
        (output.replace("0 ignored", "1 ignored"), 0),
        (output.replace("0 filtered out", "1 filtered out"), 0),
        (output + "\ntest unreachable_preserves_the_original_trap_refusal ... ok", 0)):
        with pytest.raises(RuntimeError):
            producer.case_results(changed, exit_code)


def recovery_publication(directory):
    producer = load("storage-recovery-producer")
    scoped = producer.sources()
    record = producer.build_inputs(SOURCE, scoped, SETTINGS)
    record["exit"] = 0
    for name in producer.TESTS:
        document(directory, "bin/" + name, "storage-recovery-producer.py")
    document(directory, producer.LOG)
    path = producer.publish_manifest(directory, record)
    return producer, path, scoped


def test_f012_retained_recovery_producer_and_consumer_stop_reopen(tmp_path):
    producer, path, scoped = recovery_publication(tmp_path)
    consumer = load("storage-recovery")
    original = path.read_bytes()
    first = consumer.admit_manifest(path, SOURCE, scoped)
    assert set(first["artifacts"]) == {"lxp_test_recovery", "lxp_test_log_durability", "lxp_test_rebuild"}
    assert not any("feed_store" in name or "batch_wal" in name for name in producer.SOURCES)
    assert all((ROOT / name).is_file() for name in producer.SOURCES)
    assert [name for name, _ in consumer.recovery_suites(tmp_path)] == list(producer.TESTS)
    del consumer, producer
    reopened = load("storage-recovery")
    recovered = reopened.admit_manifest(path, SOURCE, reopened.producer.sources())
    assert recovered == first and path.read_bytes() == original
    recovery = (ROOT / "tests/storage/lxp_test_recovery.c").read_text()
    assert "recover_from_checkpoint(false)" in recovery
    assert "recover_from_checkpoint(true)" in recovery
    assert "test_snapshot_crash_same_sequence_retry()" in recovery


@pytest.mark.parametrize("damage", ["retired-suite", "retired-source", "binary", "log", "revision"])
def test_f012_reopened_consumer_refuses_retired_or_changed_recovery_inputs(tmp_path, damage):
    producer, path, scoped = recovery_publication(tmp_path)
    value = json.loads(path.read_text())
    if damage == "retired-suite":
        value["artifacts"]["lxp_test_batch_wal_recovery"] = value["artifacts"].pop("lxp_test_rebuild")
    elif damage == "retired-source":
        value["sources"]["src/modules/programs/feed_store.c"] = "0" * 64
    elif damage == "binary":
        (tmp_path / "bin/lxp_test_rebuild").write_bytes(b"changed after publication")
    elif damage == "log":
        (tmp_path / producer.LOG).write_bytes(b"changed after publication")
    else:
        value["revision"] = "0" * 40
    rewrite(path, value)
    with pytest.raises(ValueError):
        load("storage-recovery").admit_manifest(path, SOURCE, scoped)
