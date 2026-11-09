"""Offline source proofs for the two workflow retired-reference findings.

Read the production YAML and committed producer/consumer sources. No workflow,
compiler, shell, network endpoint or external service is executed by this gate.
"""

import ast
from copy import deepcopy
from fnmatch import fnmatchcase
import hashlib
import json
from pathlib import Path
import re
import shlex
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[3]
GENERATOR = "services/wallet/attestor/internal/policy/lx/testdata/kernelvectors"
RETIRED_XWEB = {
    "include/layerx/lx_web.h", "include/layerx/lx_batch.h",
    "src/modules/web/**", "src/modules/programs/call.c", "tests/modules/**",
    "tests/sequencer/**", "tests/network/**", "tests/fixtures/web/**",
    "tests/test_programs_web_read.c", "tests/test_web_program_path.c",
}


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def workflow(name):
    # BaseLoader preserves GitHub's YAML 1.2 `on` key (SafeLoader uses YAML 1.1).
    return yaml.load(source(f".github/workflows/{name}.yml"), Loader=yaml.BaseLoader)


def filters(document):
    triggers = document["on"]
    assert set(triggers) == {"pull_request", "push"}
    assert triggers["push"]["branches"] == ["main"]
    result = [triggers[event]["paths"] for event in ("pull_request", "push")]
    assert result[0] == result[1]
    for paths in result:
        assert len(paths) == len(set(paths))
        assert all(isinstance(path, str) and not path.startswith("!") for path in paths)
    return result


def assert_watched(paths, filenames):
    for filename in filenames:
        assert (ROOT / filename).is_file(), filename
        assert any(fnmatchcase(filename, pattern) for pattern in paths), filename


def test_wallet_retired_vectors_filters_follow_live_producer():
    document = workflow("wallet-go-test")
    manifest = tomllib.loads(source(f"{GENERATOR}/Cargo.toml"))
    assert manifest["package"]["name"] == "attestor-kernel-vectors"
    assert manifest["bin"] == [{"name": "attestor-kernel-vectors", "path": "src/main.rs"}]
    generator = source(f"{GENERATOR}/src/main.rs")
    assert "use layerx_crypto::evm_transaction::Eip1559Call;" in generator
    assert "fn generated_vectors() -> String" in generator
    assert 'include_bytes!("../../kernel_activities.json")' in generator
    pinned = json.loads(source(f"{GENERATOR}/../kernel_activities.json"))
    assert pinned["generator"] == GENERATOR
    assert pinned["activities"]
    consumer = source("services/wallet/attestor/internal/policy/lx/lx_test.go")
    assert 'kernelVectorPath = "testdata/kernel_activities.json"' in consumer
    inputs = [
        f"{GENERATOR}/Cargo.toml", f"{GENERATOR}/Cargo.lock",
        f"{GENERATOR}/src/main.rs", f"{GENERATOR}/../kernel_activities.json",
        "services/wallet/attestor/internal/policy/lx/lx_test.go",
        "services/wallet/ceremony/go.mod", "Makefile",
    ]
    for name in ("layerx-crypto", "layerx-types"):
        dependency = manifest["dependencies"][name]
        producer = (ROOT / GENERATOR / dependency["path"]).resolve()
        assert producer == ROOT / f"services/agent/crates/{name}"
        inputs.extend(str((producer / path).relative_to(ROOT)) for path in ("Cargo.toml", "src/lib.rs"))
    for paths in filters(document):
        assert "Makefile" in paths
        vector_input = "services/agent/crates/layerx-wire/tests/support/wallet_vectors/activities.json"
        wire_consumer = source("services/wallet/attestor/internal/lxwire/lxwire_test.go")
        assert vector_input in wire_consumer
        assert_watched(paths, [vector_input])
        assert_watched(paths, inputs)


def test_wallet_retains_every_attestor_and_ceremony_test():
    document = workflow("wallet-go-test")
    assert set(document["jobs"]) == {"attestor", "ceremony"}
    for name, job in document["jobs"].items():
        directory = f"services/wallet/{name}"
        assert job["defaults"]["run"]["working-directory"] == directory
        setup = [step for step in job["steps"] if step.get("uses", "").startswith("actions/setup-go@")]
        assert len(setup) == 1
        assert setup[0]["with"] == {
            "go-version-file": f"{directory}/go.mod",
            "cache-dependency-path": f"{directory}/go.sum",
        }
        pins = json.loads(Path(__file__).with_name("fixtures").joinpath("wallet_workflow_commands.json").read_text())
        assert [shlex.split(step["run"]) for step in job["steps"] if "run" in step] == pins[name]
        assert pins[name][-1] == (["sh", "../../../tools/runtime/run-with-clock.sh", "go", "test", "-count=1", "./..."]
                                  if name == "attestor" else ["go", "test", "./..."])



def test_xweb_retired_inputs_filters_follow_chain_and_attestor():
    document = workflow("xweb-test")
    files = [
        "modules/xweb/keeper/program.go", "modules/xweb/keeper/program_fulfil.go",
        "modules/xweb/types/testdata/preimage-vectors.json",
        "precompiles/xweb/xweb.go", "precompiles/xweb/abi.json",
        "precompiles/common/precompiles.go", "precompiles/utils/types.go",
        "services/interop/crates/x-websearch/src/attest.rs",
        "services/interop/crates/x-websearch/src/submit.rs",
        "services/interop/crates/x-websearch/tests/attest.rs",
        "services/interop/crates/layerx-interop-gateway/Cargo.toml",
        "services/wallet/attestor/internal/policy/policy.go",
        "contracts/src/precompiles/IXWeb.sol", "contracts/test/XWebConsumer.t.sol",
        "programs/crates/layerx-programs-runtime/src/abi/manifest.rs",
        "programs/sdk/rust/src/abi.rs", "include/layerx/programs.h",
        "go.mod", "go.sum", "Makefile", "chain.mk", "foundry.paxeer.toml",
    ]
    for paths in filters(document):
        assert not RETIRED_XWEB.intersection(paths)
        assert_watched(paths, files)
        assert "modules/xweb/**" in paths and "precompiles/xweb/**" in paths
        assert "services/wallet/attestor/**" in paths
    proof = source("precompiles/xweb/proof_xweb_0x1019_test.go")
    assert 'policy.Dir = filepath.Join(root, "services/wallet/attestor")' in proof
    assert '"./internal/policy"' in proof
    assert '"services/interop/Cargo.toml", "-p", "x-websearch"' in proof


def test_xweb_shared_abi_producer_and_consumers_remain_connected():
    abi = json.loads(source("precompiles/xweb/abi.json"))
    functions = {entry["name"]: entry for entry in abi if entry["type"] == "function"}
    assert {"request", "fulfil", "fulfilProgram", "getRequest", "getResult", "refund"} <= functions.keys()
    assert functions["getRequest"]["outputs"][0]["type"] == "tuple"
    assert "//go:embed abi.json" in source("precompiles/xweb/xweb.go")
    submit = source("services/interop/crates/x-websearch/src/submit.rs")
    assert '.join("../../../../precompiles/xweb/abi.json")' in submit
    assert "fn published_fields()" in submit
    attestation = source("services/interop/crates/x-websearch/tests/attest.rs")
    assert '.join("../../../../modules/xweb/types/testdata/preimage-vectors.json")' in attestation
    vectors = json.loads(source("modules/xweb/types/testdata/preimage-vectors.json"))
    assert any(vector["name"] == "program-search" for vector in vectors["vectors"])
    manifest = tomllib.loads(source("services/interop/crates/x-websearch/Cargo.toml"))
    assert manifest["package"]["name"] == "x-websearch"
    assert manifest["lib"]["path"] == "src/lib.rs"
    assert manifest["dependencies"]["layerx-x402"]["path"] == "../layerx-x402"
    foundry = tomllib.loads(source("foundry.paxeer.toml"))["profile"]["default"]
    assert foundry["src"] == "contracts/src" and foundry["test"] == "contracts/test"
    makefile = source("Makefile")
    target = re.search(r"^paxd-native-libs: (.+)$", makefile, re.M)
    assert target and target[1].split() == ["$(LIBRARY)", "programs-release-build", "evmone-lib"]


def test_xweb_retains_request_escrow_receipt_checks():
    document = workflow("xweb-test")
    assert set(document["jobs"]) == {"sidecar-crate", "module-precompile", "consumer-contract", "agent-clients"}
    commands = [step["run"] for job in document["jobs"].values() for step in job["steps"] if "run" in step]
    for command in (
        "cargo test --locked --manifest-path services/interop/Cargo.toml -p x-websearch",
        "make paxd-native-libs",
        "go test -count=1 ./modules/xweb/... ./precompiles/xweb/...",
        "FOUNDRY_CONFIG=foundry.paxeer.toml forge test --evm-version=prague --match-path contracts/test/XWebConsumer.t.sol",
        "cargo test --locked --manifest-path services/agent/Cargo.toml -p layerx-mcp --test web",
        "(cd services/agent/sdk/typescript && npm ci && npm run build && node dist/test/web-search.test.js)",
        "PYTHONPATH=services/agent/sdk/python python3 -m pytest -q tests/agent/sdk/python/test_web_search.py",
    ):
        assert commands.count(command) == 1
    precompile_tests = source("precompiles/xweb/xweb_test.go")
    for name in ("TestRequestTakesTheFeeAndEmits", "TestFulfilDeliversTheCallback", "TestRefundAfterTheTimeout"):
        assert f"func {name}(t *testing.T)" in precompile_tests
    assert "func TestFulfilAtThresholdStoresTheResultAndSplitsTheFee(t *testing.T)" in source(
        "modules/xweb/keeper/fulfil_test.go"
    )


def test_only_event_path_filters_change_in_workflows():
    # Fingerprints of the live producer YAML with only event paths removed.
    # Pin jobs, all commands, conditions, permissions and tool versions together.
    expected = {
        "wallet-go-test": "e8a2fab1d5cfe130875aaedd9d32c199c3c59c8a5d70235b1102a26725096144",
        "xweb-test": "009e85355b053b6b0bddb201e0550abd57a61ef7d41f036adf7dd7c5e34b81e2",
    }
    for name, digest in expected.items():
        document = deepcopy(workflow(name))
        for event in ("pull_request", "push"):
            del document["on"][event]["paths"]
        assert hashlib.sha256(json.dumps(document, sort_keys=True).encode()).hexdigest() == digest


def test_repair_record_maps_both_findings():
    record = tomllib.loads(source("tools/qualification/audit_repairs/repair_22_33_700.kvx"))
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(record["finding"]) == {"wallet-retired-vectors", "xweb-retired-inputs"}
    mapped = set()
    for finding in record["finding"].values():
        assert finding["audit"] == "22.30.169" and finding["owner_domain"] == 16
        assert finding["class"] == "retired-reference"
        assert finding["tests"] and set(finding["tests"]) <= declared
        mapped.update(finding["tests"])
    assert mapped == declared - {"test_repair_record_maps_both_findings"}
    repair = record["repair"]
    assert repair["task"] == "22.33.700"
    assert repair["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_700.py"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "No baseline failures are claimed" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    assert repair["wiring_requests"]
