"""Bounded shell/source qualification and production Python recovery paths.

Go/Cargo protocol examples below are parser unit data, never native execution
or artifact evidence. No compiler, native executable or emitted ELF is replaced.
"""

import ast
from contextlib import contextmanager
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import socket
import ssl
import subprocess
import sys
import threading
import tomllib
from types import SimpleNamespace

import pytest
import yaml

ROOT = Path(__file__).resolve().parents[3]
GATES = ROOT / "tools/paxeer-x/gates"
NODE = ROOT / "services/platform/hosted/node"
FIXTURES = NODE / "tests/fixtures/repair_22_31_355"
sys.dont_write_bytecode = True
sys.path.insert(0, str(NODE))
import generation_client as client
import generation_transport as transport
import native_runtime as native
from reset_state import Store, StoreError


def source(selector):
    return (GATES / (selector + ".sh")).read_text()


def blocks(selector):
    return dict(re.findall(r"<<'([A-Z]+)'\n(.*?)\n\1(?:\n|$)", source(selector), re.S))


def declarations(selector, block):
    tree = ast.parse(blocks(selector)[block])
    nodes = [node for node in tree.body if isinstance(node, (ast.Import, ast.ImportFrom,
                                                          ast.FunctionDef, ast.ClassDef))]
    namespace = {}
    exec(compile(ast.Module(body=nodes, type_ignores=[]), selector, "exec"), namespace)
    return namespace


def assignment(code, name):
    for node in ast.walk(ast.parse(code)):
        if isinstance(node, ast.Assign) and any(isinstance(target, ast.Name) and target.id == name
                                               for target in node.targets):
            return ast.literal_eval(node.value)
    raise AssertionError("missing assignment: " + name)


def load_module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def dispatcher_counts(output):
    # Compile the dispatcher's actual declared matcher, not a copied expression.
    code = re.search(r"<<'PY'\n(.*)\nPY", (GATES.parent / "verify-task.sh").read_text(), re.S)[1]
    node = next(node for node in ast.parse(code).body if isinstance(node, ast.Assign)
                and any(isinstance(target, ast.Name) and target.id == "COUNT" for target in node.targets))
    namespace = {"re": re}
    exec(compile(ast.Module(body=[node], type_ignores=[]), "dispatcher", "exec"), namespace)
    matches = namespace["COUNT"].findall(output)
    assert len(matches) == 1
    tests, skipped = map(int, matches[0])
    assert tests > 0 and skipped == 0
    for guard in ("elif not counts or tests == 0:", "elif skipped:", "elif code:"):
        assert guard in code
    return tests, skipped


def marker(selector, block, total):
    tree = ast.parse(blocks(selector)[block])
    call = next(node for node in ast.walk(tree) if isinstance(node, ast.Expr)
                and isinstance(node.value, ast.Call) and isinstance(node.value.func, ast.Name)
                and node.value.func.id == "print" and "PAXEER_X_GATE" in ast.unparse(node))
    from contextlib import redirect_stdout
    output = io.StringIO()
    with redirect_stdout(output):
        exec(compile(ast.Module(body=[call], type_ignores=[]), selector, "exec"), {"total": total})
    return output.getvalue()


@pytest.mark.parametrize("selector", ["24.18", "24.20", "5.1"])
def test_every_shell_and_embedded_python_is_parsed(selector):
    result = subprocess.run(["bash", "-n", str(GATES / (selector + ".sh"))],
                            capture_output=True, text=True, timeout=15)
    assert result.returncode == 0, result.stderr
    assert "set -euo pipefail" in source(selector) and "umask 077" in source(selector)
    assert blocks(selector)
    for code in blocks(selector).values():
        ast.parse(code)


def test_f028_retained_real_go_corpus_and_dispatcher_transport():
    code = blocks("24.18")["PY"]
    expected = (
        ("./custodyproof", ("TestRealStateCreditAndClockBounds", "TestRealStateCreditRefusesMissingAuthority",
                            "TestNativeBankCommittedStateAndRecovery", "TestNativeBankMalformedProofRefusal")),
        ("./consensus/internal/consensus/types", ("TestLeaderPinRoundRobinFromActivation",)),
        ("./modules/sequencer/keeper", ("TestReceiptKeysGovernanceOnly", "TestReceiptKeysRotationBoundary")),
        ("./node/sequencer", ("TestStandbyAckBeforeRelease", "TestStandbyNoAckNoReceipt")),
    )
    assert assignment(code, "CORPORA") == expected
    for package, names in expected:
        files = list((ROOT / package.removeprefix("./")).glob("*_test.go"))
        for name in names:
            assert any(re.search(r"func\s+" + name + r"\(t \*testing.T\)", path.read_text())
                       for path in files), name
    for fact in ("['go', 'test', '-json', '-count=1', '-run', pattern, package]",
                 "re.escape(name)", "if result.returncode:", "raise SystemExit",
                 "total += executed_counts(result.stdout, names)", "timeout=900"):
        assert fact in code
    assert code.index("total += executed_counts") < code.index("print(f'PAXEER_X_GATE")
    assert dispatcher_counts(marker("24.18", "PY", 9)) == (9, 0)
    assert "foundation-artifacts.py --build" in source("24.18")


@pytest.mark.parametrize("selector,block", [("24.18", "PY"), ("24.20", "VERIFY")])
def test_go_counts_refuse_missing_skipped_failed_duplicate_or_unordered_cases(selector, block):
    count = declarations(selector, block)["executed_counts"]
    name = "TestProtocolUnit"
    rows = [{"Action": "run", "Test": name}, {"Action": "pass", "Test": name}, {"Action": "pass"}]
    encode = lambda items: "\n".join(json.dumps(row) for row in items)
    assert count(encode(rows), (name,)) == 1
    children = [rows[0], {"Action": "run", "Test": name + "/child"},
                {"Action": "pass", "Test": name + "/child"}, *rows[1:]]
    assert count(encode(children), (name,)) == 2
    invalid = [[], [rows[-1]], rows[:-1], [rows[1], rows[0], rows[-1]],
               [rows[0], rows[0], *rows[1:]], [rows[0], rows[1], rows[1], rows[-1]],
               [rows[0], {"Action": "skip", "Test": name}, rows[-1]],
               [rows[0], {"Action": "fail", "Test": name}, rows[-1]],
               [*children[:-1], {"Action": "run", "Test": "TestOther/child"},
                {"Action": "pass", "Test": "TestOther/child"}, rows[-1]]]
    for events in invalid:
        with pytest.raises(ValueError):
            count(encode(events), (name,))
    with pytest.raises((ValueError, TypeError)):
        count("not JSON", (name,))


def test_f024_current_paxd_profile_keeps_source_and_artifact_admission():
    text = source("24.20")
    assert os.access(GATES / "24.20.sh", os.X_OK)
    for retired in ("layerxd", "layerx-genesis-build", "docker/platform-core", "docker/platform-authority",
                    "build/bin", "LAYERX_TEST_NATIVE_BIN_DIR"):
        assert retired not in text
    assert 'make -j2 LXP_REVISION="$revision" PAXEER_GO_JOBS=2 paxd-build' in text
    assert "paxd-build: paxd-native-libs" in (ROOT / "Makefile").read_text()
    for fact in ("complete clean candidate required", "snapshot symlink refused", "fresh source build required",
                 "stat.S_IMODE(info.st_mode)==0o700", "not p.exists()", "source changed during build",
                 "stat.S_ISREG(info.st_mode)", "os.access(p,os.X_OK)", "b'\\x7fELF'",
                 "'source_revision':git('rev-parse','HEAD')", "'source_tree':git('rev-parse','HEAD^{tree}')",
                 "['git','ls-tree','-r','-z','--full-tree','HEAD']", "os.O_EXCL|os.O_NOFOLLOW,0o600",
                 "os.fsync(f.fileno())", "os.fsync(fd)", "stat.S_IMODE(info.st_mode)==0o600",
                 "info.st_nlink==1", "value['source_revision']==git('rev-parse','HEAD')",
                 "value['source_tree']==git('rev-parse','HEAD^{tree}')", "candidate source mismatch",
                 "assert set(value['executables'])=={'paxd'}", "target=root/'build'/name",
                 "digest(target)==row['sha256']", "target.stat().st_mtime_ns==row['mtime_ns']<=value['recorded_ns']"):
        assert fact in text, fact
    record = ast.parse(blocks("24.20")["RECORD"])
    # Execute only the pure source-list construction, never git/native build IO.
    nodes = [node for node in record.body if isinstance(node, (ast.Assign, ast.AugAssign))
             and isinstance(node.targets[0] if isinstance(node, ast.Assign) else node.target, ast.Name)
             and (node.targets[0] if isinstance(node, ast.Assign) else node.target).id == "paths"]
    namespace = {}
    exec(compile(ast.Module(body=nodes, type_ignores=[]), "source-list", "exec"), namespace)
    paths = namespace["paths"]
    assert len(paths) == len(set(paths)) and all((ROOT / path).is_file() for path in paths)
    verify = ast.parse(blocks("24.20")["VERIFY"])
    check = next(node for node in verify.body if isinstance(node, ast.Assert)
                 and "set(value['candidate_sources'])" in ast.unparse(node.test))
    assert eval(compile(ast.Expression(check.test.comparators[0]), "source-set", "eval")) == set(paths)
    assert {"docker/kernel/kernel_runtime.py", "docker/kernel/Dockerfile", "docker/kernel/init.sh",
            "services/platform/hosted/node/native_runtime.py", "services/platform/hosted/node/reset_state.py",
            "services/platform/hosted/node/data_directory.py", "services/platform/hosted/node/deployment.yaml"} <= set(paths)
    kernel = (ROOT / "docker/kernel/kernel_runtime.py").read_text()
    assert "PAXD = '/usr/local/bin/paxd'" in kernel
    assert "native.committed_health" in kernel and "class CommittedReadiness" in kernel
    deployment = (NODE / "deployment.yaml").read_text()
    assert "- name: paxd" in deployment and "/opt/layerx/kernel_runtime.py" in deployment
    assert "COPY docker/kernel/kernel_runtime.py /opt/layerx/kernel_runtime.py" in (ROOT / "docker/kernel/Dockerfile").read_text()
    # Unit syntax uses systemd's own grammar rather than TOML.
    import configparser
    unit = configparser.ConfigParser(interpolation=None, strict=False)
    unit.read(ROOT / "docker/kernel/layerx-kernel.service")
    assert unit["Service"]["Restart"] == "always"
    assert "-v /var/lib/layerx/kernel:/data" in unit["Service"]["ExecStart"]
    docs = list(yaml.safe_load_all(deployment))
    pod = next(doc["spec"]["template"]["spec"] for doc in docs if doc["kind"] == "StatefulSet")
    containers = {row["name"]: row for row in pod["containers"]}
    assert set(containers) == {"paxd"}
    assert containers["paxd"]["command"] == ["python3", "/opt/layerx/kernel_runtime.py", "--home", "/data/layerx/node"]
    assert containers["paxd"]["readinessProbe"]["httpGet"] == {"path": "/readyz", "port": "health"}
    assert pod["securityContext"]["runAsUser"] == 4020
    assert pod["automountServiceAccountToken"] is False


def test_f024_real_kernel_committed_readiness_stop_reopen_and_recovery(tmp_path):
    proof = load_module("kernel_recovery_851", Path(__file__).with_name("test_repair_22_31_427.py"))
    runtime = proof.runtime.__wrapped__()
    proof.test_kernel_backend_cold_start_and_reopened_committed_readiness(runtime, tmp_path)
    checkpoint = json.loads((tmp_path / "committed.json").read_bytes())
    assert checkpoint["height"] == 23869863
    assert checkpoint["block_hash"] == proof.CAPTURE["status"]["sync_info"]["latest_block_hash"].lower()


@contextmanager
def broker_connection(broker, socket_path):
    # Real one-request Unix listener, production SO_PEERCRED/SCM_RIGHTS handler.
    with socket.socket(socket.AF_UNIX) as listener:
        listener.bind(str(socket_path))
        socket_path.chmod(0o660)
        listener.listen(1)
        listener.settimeout(5)
        errors = []
        def handle():
            try:
                connection, _ = listener.accept()
                broker.handle(connection)
            except BaseException as error:
                errors.append(error)
        worker = threading.Thread(target=handle)
        worker.start()
        try:
            yield
        finally:
            worker.join(timeout=6)
            assert not worker.is_alive()
            assert not errors
            socket_path.unlink()


def test_f024_generation_producer_descriptor_transport_consumer_reopens(tmp_path):
    capture = json.loads((FIXTURES / "native_configuration.json").read_bytes())
    data, state, run = (tmp_path / name for name in ("data", "state", "run"))
    for path in (data, state, run):
        path.mkdir(mode=0o700)
    (data / "config").mkdir(mode=0o700)
    for name in ("genesis.json", "config.toml", "app.toml"):
        native.write_file(data / "config" / name, (FIXTURES / name).read_bytes())
    sequencer_id = hashlib.sha256(bytes.fromhex(capture["receipt_public_key"])).hexdigest()
    node = {"LAYERX_NODE_NETWORK_ID": str(capture["network_id"]), "LAYERX_NODE_CHAIN_ID": capture["chain_id"],
            "LAYERX_NODE_EVM_CHAIN_ID": str(capture["evm_chain_id"]), "LAYERX_NODE_SEQUENCER_ID": sequencer_id,
            "LAYERX_NODE_SEQUENCER_PUBLIC_KEY": capture["receipt_public_key"],
            "LAYERX_NODE_RPC_URL": "http://127.0.0.1:8545", "LAYERX_NODE_CONSENSUS_RPC_URL": "http://127.0.0.1:26657"}
    core = {"LAYERX_CORE_SEQUENCER_ID": sequencer_id, "LAYERX_CORE_TREASURY_ASSET": capture["asset"],
            "LAYERX_CORE_TREASURY_SIGNER_SOCKET": str(run / "treasury.sock"),
            "LAYERX_CORE_SEQUENCER_ENDPOINT": "https://localhost:9443",
            "LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER": "/generation/rpc-ca.der",
            "LAYERX_CORE_CHAIN_RPC_URL": node["LAYERX_NODE_RPC_URL"],
            "LAYERX_CORE_FUNDING_SENDER_KEY_FILE": str(run / "funding.key")}
    ca = ssl.PEM_cert_to_DER_cert((FIXTURES / "tls-certificate.pem.txt").read_text())
    binding = native.publish_generation(data, node, core, ca)
    store = Store(str(state), str(data), str(run), capture["network_id"], {})
    store.initialize()
    retained = (state / "state.json").read_bytes()
    for reopening in range(2):
        if reopening:
            del broker, store
            store = Store(str(state), str(data), str(run), capture["network_id"], {})
            store.initialize()
        broker = transport.Broker(str(data), str(state), str(run), os.geteuid(), os.getegid())
        socket_path = run / "generation.sock"
        with broker_connection(broker, socket_path):
            result, descriptors = client.receive(str(socket_path), "core", 1, os.geteuid(), os.getegid())
        try:
            assert result["generation"] == store.generation() == 1
            assert result["public"]["LAYERX_NODE_SEQUENCER_ID"] == sequencer_id
            for name, descriptor in zip(result["artifacts"], descriptors, strict=True):
                assert hashlib.sha256(os.pread(descriptor, 64 * 1024 * 1024, 0)).hexdigest() == binding[name]
            adopted = client.child_environment(result, descriptors, "core")
            view, adopted = client.materialize(result, descriptors, "core", adopted)
            try:
                assert native.read_regular(adopted["LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER"]) == ca
                assert transport.environment(native.read_regular(adopted["LAYERX_CORE_ENV_FILE"])) == core
                assert adopted["LAYERX_GENERATION_NUMBER"] == "1"
            finally:
                view.cleanup()
        finally:
            client.close_descriptors(descriptors)
        assert native.generation_binding(data) == binding
        assert (state / "state.json").read_bytes() == retained
    with broker_connection(broker, socket_path):
        with pytest.raises(StoreError):
            client.receive(str(socket_path), "core", 2, os.geteuid(), os.getegid())


def test_f029_real_pytest_reports_and_standard_dispatcher_count(tmp_path):
    code = blocks("24.20")["VERIFY"]
    corpus = assignment(code, "PYTHON_CORPORA")
    assert corpus[0] == "services/platform/hosted/node/tests/test_generation_transport.py"
    for target in corpus[1:]:
        path, name = target.split("::")
        assert (ROOT / path).is_file()
        assert any(isinstance(node, ast.FunctionDef) and node.name == name
                   for node in ast.parse((ROOT / path).read_text()).body)
    assert any("cold_start_and_reopened_committed_readiness" in target for target in corpus)
    assert "loadTestsFromModule" not in code and "PAXEER_X_GENERATION_BROKER_GATE" not in code
    for fact in ("pytest.main(['-q', '--tb=short', *PYTHON_CORPORA], plugins=[cases])",
                 "total = cases.count(code)", "total += executed_counts(result.stdout,names)",
                 "['go','test','-json','-count=1','-run',pattern,'./custodyproof']",
                 "if result.returncode:", "timeout=750"):
        assert fact in code
    cases = declarations("24.20", "VERIFY")["ExecutedCases"]()
    # Drive the real pytest producer into the production gate plugin. The four
    # collected cases exercise protected recovery-record refusals, with no skip.
    target = Path(__file__).with_name("test_repair_22_31_427.py")
    code = pytest.main(["-q", "--tb=short", str(target) + "::test_kernel_backend_refuses_untrusted_recovery_record"], plugins=[cases])
    assert cases.count(code) == 4
    assert dispatcher_counts(marker("24.20", "VERIFY", cases.count(code))) == (4, 0)
    assert len(cases.reports) == 12
    cases.deselected = True
    with pytest.raises(ValueError):
        cases.count(code)


def test_f029_python_counts_refuse_every_incomplete_or_unsuccessful_outcome():
    factory = declarations("24.20", "VERIFY")["ExecutedCases"]
    for stage in ("setup", "call", "teardown"):
        for outcome in ("failed", "skipped"):
            cases = factory()
            cases.pytest_collection_finish(SimpleNamespace(items=[SimpleNamespace(nodeid="case")]))
            for when in ("setup", "call", "teardown"):
                report = pytest.TestReport("case", ("path", 1, "case"), {},
                                           outcome if when == stage else "passed", None, when)
                cases.pytest_runtest_logreport(report)
            with pytest.raises(ValueError):
                cases.count(0)
    for fault in ("empty", "collection", "exit", "missing", "duplicate", "xfail"):
        cases = factory()
        if fault != "empty":
            cases.pytest_collection_finish(SimpleNamespace(items=[SimpleNamespace(nodeid="case")]))
        if fault == "collection":
            cases.pytest_collectreport(SimpleNamespace(passed=False))
        for stage in ("setup", "call", "teardown"):
            report = pytest.TestReport("case", ("path", 1, "case"), {}, "passed", None, stage)
            if fault == "xfail":
                report.wasxfail = "protocol input"
            if not (fault == "missing" and stage == "call"):
                cases.pytest_runtest_logreport(report)
            if fault == "duplicate":
                cases.pytest_runtest_logreport(report)
        with pytest.raises(ValueError):
            cases.count(1 if fault == "exit" else 0)


def test_f025_singleton_compiler_events_flow_into_source_bound_manifest():
    code = blocks("5.1")["RECORD"]
    tree = ast.parse(code)
    loop = next(node for node in tree.body if isinstance(node, ast.For))
    assert ast.literal_eval(loop.iter) == (("probe", "LAYERX_MULTI_ASSET_PROBE_EVENTS", "layerx-node-probe", "bin"),)
    assert [node.id for node in loop.target.elts] == ["key", "event", "name", "kind"]
    assert "rows=[json.loads(line) for line in Path(os.environ[event]).read_text().splitlines() if line.strip()]" in code
    assert "path=compiler_artifact(rows,name,kind)" in code
    for fact in ("git('rev-parse','HEAD')==revision", "git('status','--porcelain','--untracked-files=all')",
                 "path.is_absolute()", "not any(p.is_symlink()", "os.access(path,os.X_OK)",
                 "stream.read(4)==b'\\x7fELF'", "hashlib.file_digest(stream,'sha256').hexdigest()",
                 "artifacts[key]={'path':str(path),'sha256':digest}", "'source_tree':git('rev-parse','HEAD^{tree}')",
                 "'source_revision':revision", "'build_exit':0", "'artifacts':artifacts",
                 "os.O_EXCL|os.O_NOFOLLOW,0o600", "os.fsync(stream.fileno())", "os.fsync(fd)"):
        assert fact in code
    assert code.index("path=compiler_artifact") < code.index("artifacts[key]=") < code.index("value=")
    shell = source("5.1")
    assert 'cargo build --locked --release --manifest-path services/platform/hosted/node/tests/probe/Cargo.toml --message-format=json > "$LAYERX_MULTI_ASSET_PROBE_EVENTS"' in shell
    manifest = tomllib.loads((NODE / "tests/probe/Cargo.toml").read_text())
    assert manifest["package"]["name"] == "layerx-node-probe"
    assert manifest["bin"] == [{"name": "layerx-node-probe", "path": "src/main.rs"}]


def test_f025_production_compiler_selector_refuses_failed_empty_missing_and_ambiguous_outputs():
    select = declarations("5.1", "RECORD")["compiler_artifact"]
    # Cargo protocol unit data only: no executable or evidence file is created.
    emitted = {"reason": "compiler-artifact", "executable": "/compiler-output/probe",
               "target": {"name": "layerx-node-probe", "kind": ["bin"]}}
    finished = {"reason": "build-finished", "success": True}
    rows = [emitted, finished]
    assert select(rows, "layerx-node-probe", "bin") == Path(emitted["executable"])
    assert select([emitted, emitted, finished], "layerx-node-probe", "bin") == Path(emitted["executable"])
    for invalid in ([], [finished], [emitted], [emitted, {"reason": "build-finished", "success": False}],
                    [{**emitted, "executable": None}, finished],
                    [{**emitted, "target": {"name": "other", "kind": ["bin"]}}, finished],
                    [{**emitted, "target": {"name": "layerx-node-probe", "kind": ["lib"]}}, finished],
                    [emitted, {**emitted, "executable": "/another-output/probe"}, finished]):
        with pytest.raises(AssertionError):
            select(invalid, "layerx-node-probe", "bin")


def test_record_maps_each_finding_to_its_executable_proof_and_discloses_limits():
    record = tomllib.loads(Path(__file__).with_name("repair_22_33_851.kvx").read_text())
    assert set(record["finding"]) == {"f028", "f024", "f029", "f025"}
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef)}
    for finding in record["finding"].values():
        assert finding["tests"] and set(finding["tests"]) <= declared
        assert finding["source_proof"] and finding["review_only"]
    assert record["task"]["base_revision"] == "7f73e1e62e9ec5f5baf39701e42280257640cf0a"
    assert record["task"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "not run" in record["task"]["baseline"].lower()
    assert all(row["path"] and row["owner_domain"] and row["request"] for row in record["wiring"].values())
