"""Offline doc-drift proof; execute the production ABI generator in-process.

Rust service behavior is cross-checked as source, with runnable integration cases
registered separately. This Python gate does not claim native runtime execution.
"""

import ast
import importlib.util
import json
from pathlib import Path
import re
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
SERVICE = ROOT / "services/explorer/services/sig-provider"
SOURCES = SERVICE / "sig-provider/src/sources"
RECORD = Path(__file__).with_name("repair_22_33_312.kvx")


def source(relative):
    return (SERVICE / relative).read_text(encoding="utf-8")


def prose():
    return " ".join(source("README.md").split())


@pytest.fixture(scope="module")
def generated_inventory():
    path = SOURCES / "generate_precompiles.py"
    spec = importlib.util.spec_from_file_location("sig_provider_inventory", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    # This is the real producer-to-embedded-ABI generator, with its normal paths.
    abi, inventory = module.generate()
    return {"abi": abi, "inventory": inventory}


def test_precompile_documented_default_and_deployment_override():
    readme = source("README.md")
    documented = re.findall(r"```toml\n(.*?)\n```", readme, re.S)
    assert len(documented) == 1
    example = tomllib.loads(documented[0])
    base = tomllib.loads(source("sig-provider-server/config/base.toml"))
    assert example["sources"]["precompiles"]["enabled"] is True
    assert base["sources"]["precompiles"] == example["sources"]["precompiles"]
    settings = source("sig-provider-server/src/settings.rs")
    assert re.search(
        r"impl Default for PrecompilesSettings\s*\{\s*fn default\(\) -> Self\s*"
        r"\{\s*Self \{ enabled: true \}", settings
    )
    assert 'const SERVICE_NAME: &\'static str = "SIG_PROVIDER";' in settings
    assert "pub precompiles: PrecompilesSettings" in settings
    for fact in (
        "local `PrecompileSource`, enabled by default",
        "SIG_PROVIDER__SOURCES__PRECOMPILES__ENABLED=false",
        "`SIG_PROVIDER__CONFIG`", "`__` separators override file values",
        "backend's `MICROSERVICE_*` variables enable the client integration",
    ):
        assert fact in prose(), fact
    run = source("sig-provider-server/src/run.rs")
    enable_block = (
        "if settings.precompiles.enabled {\n"
        "        sources.push(Arc::new(PrecompileSource::new()));\n    }"
    )
    # Both signature and complete-ABI paths are controlled by the same switch.
    assert enable_block in run
    assert re.search(
        r"if settings.precompiles.enabled\s*\{\s*"
        r"sources.push\(Arc::new\(PrecompileSource::new\(\)\)\);\s*\}"
        r"\s*if settings.eth_bytecode_db.enabled", run
    )
    assert run.index(enable_block) < run.index("Arc::new(sigeth::Source::new")
    assert run.index("Arc::new(sigeth::Source::new") < run.index("Arc::new(fourbyte::Source::new")


def test_precompile_documented_provenance_matches_real_generator(generated_inventory):
    embedded = json.loads((SOURCES / "precompiles.json").read_text())
    assert generated_inventory == embedded
    assert embedded["abi"] and embedded["inventory"]
    assert {item["status"] for item in embedded["inventory"]} == {
        "versioned", "merge", "historical"
    }
    doc = prose()
    for fact in (
        "`inventory` manifest alongside the `abi` array",
        "`name`, repository `origin`", "`status`",
        "`versioned`: ABI files declared by `precompileInfoSources`",
        "`GetCustomPrecompilesE`", "`v6120Contracts`",
        "`merge`: ABIs selected by `MergeAddressTable`",
        "served ABI constants", "wrapper-added events",
        "`historical`: the previous exchange ABI", "launchpad predecessor",
        "overloads and distinct indexed event layouts",
        "Git revision of `precompiles.json`, shipped with the service binary",
        "A node ABI change requires regeneration and a new service binary",
        "Inventory status describes provenance, not current executability",
        "without an address parameter",
    ):
        assert fact in doc, fact
    required_links = {
        "sig-provider/src/sources/precompiles.json",
        "sig-provider/src/sources/precompiles_historical.json",
        "sig-provider/src/sources/generate_precompiles.py",
        "sig-provider/src/sources/precompiles_abi_test.rs",
        "../../../../precompiles/setup.go", "../../../../node/app.go",
        "../../../../precompiles/layerxagent",
        "../../../../precompiles/launchpad/legacy/v610/abi.json",
    }
    links = set(re.findall(r"\]\(([^)]+)\)", source("README.md")))
    assert required_links <= links
    for link in required_links:
        assert (SERVICE / link).exists(), link
    # Independently read the declared producer ABI files/Go constants, preserving
    # distinct indexed layouts. No generated fixture or hand-built ABI substitute.
    expected = set()
    for item in embedded["inventory"]:
        origin, _, symbol = item["origin"].partition("#")
        path = SOURCES / origin if origin == "precompiles_historical.json" else ROOT / origin
        raw = path.read_text()
        if symbol:
            matches = re.findall(r"\b" + re.escape(symbol) + r"\s*=\s*`([^`]+)`", raw)
            assert len(matches) == 1, item
            raw = matches[0]
        for entry in json.loads(raw):
            if entry["type"] in ("function", "event"):
                expected.add(json.dumps(entry, sort_keys=True))
    assert {json.dumps(entry, sort_keys=True) for entry in embedded["abi"]} == expected
    generator = (SOURCES / "generate_precompiles.py").read_text()
    assert "python3 sig-provider/src/sources/generate_precompiles.py" in doc
    assert "`--stdout` emits the same inventory without writing" in doc
    assert 'parser.add_argument("--stdout", action="store_true")' in generator
    local = (SOURCES / "precompiles.rs").read_text()
    assert 'include_str!("precompiles.json")' in local
    assert "static SIGNATURES: OnceLock<Signatures>" in local
    assert 'anyhow::bail!("embedded precompile inventory is read-only")' in local


def test_precompile_complete_event_documentation_matches_consumer():
    doc = prose()
    assert "local complete event ABIs before the bytecode database" in doc
    assert "complete event ABIs, including argument names, order and indexed flags" in doc
    assert "only events without complete ABIs use inferred signature-only layouts" in doc
    assert "Signature uploads do not change the read-only local inventory" in doc
    local = (SOURCES / "precompiles.rs").read_text()
    assert "impl super::CompleteSignatureSource for PrecompileSource" in local
    assert "serde_json::to_value(event)" in local
    aggregator = source("sig-provider/src/aggregator.rs")
    assert "crate::sources::decode_complete_event(&event, raw)" in aggregator
    assert "input.indexed = Some(param.indexed)" in aggregator
    assert "layout: EventLayout::Exact" in aggregator
    assert "if !has_complete_signatures {" in aggregator
    assert "layout: EventLayout::Candidate" in aggregator
    assert "let names: Vec<_> = decoded" in aggregator


def test_precompile_unknown_and_unavailable_documented_outcomes():
    doc = prose()
    for fact in (
        "**unknown selector or topic** returns no local candidates",
        "`OutsideCoverage`, allowing external directories to contribute",
        "successful source also allows partial results when another source fails",
        "does not prove every source was available",
        "`*_with_coverage` methods expose `Succeeded`, `Failed` and `OutsideCoverage`",
        "capable source fails and no capable source succeeds",
        "typed `LookupUnavailable`", "`signature lookup unavailable`",
        "rather than converting the failure to an empty candidate list",
        "miss in the finite local inventory does not count as a successful directory lookup",
        "gRPC `INTERNAL` (HTTP 500)",
        "candidate-array API does not expose the internal coverage report",
        "known local signature can still decode while external sources are unavailable",
    ):
        assert fact in doc, fact
    assert "error is logged and omitted" not in doc
    assert "not returned as a distinct unavailable-source result" not in doc
    local = (SOURCES / "precompiles.rs").read_text()
    assert "!Self::lookup(&Self::signatures().functions, hex).is_empty()" in local
    assert "!Self::lookup(&Self::signatures().events, hex).is_empty()" in local
    aggregator = source("sig-provider/src/aggregator.rs")
    availability = aggregator.split("fn ensure_available(", 1)[1].split("\nimpl SourceAggregator", 1)[0]
    assert "outcome.status == SourceStatus::Succeeded" in availability
    assert "matches!(outcome.status, SourceStatus::Failed(_))" in availability
    assert "if failed && !succeeded {" in availability
    assert "return Err(LookupUnavailable {" in availability
    assert "None => (SourceStatus::OutsideCoverage, None)" in aggregator
    lookup = aggregator.split("pub async fn get_function_signatures_with_coverage(", 1)[1].split("pub async fn get_function_abi(", 1)[0]
    assert "if source.covers_function(hex)" in lookup
    assert "ensure_available(&coverage)?;" in lookup
    service = source("sig-provider-server/src/service.rs")
    function_handler = service.split("async fn get_function_abi(", 1)[1].split("async fn get_event_abi(", 1)[0]
    assert ".get_function_abi(&bytes)" in function_handler
    assert ".map_err(|e| tonic::Status::internal(e.to_string()))" in function_handler


def test_precompile_real_rust_integration_is_registered():
    manifest = tomllib.loads(source("sig-provider-server/Cargo.toml"))
    targets = [item for item in manifest["test"] if item["name"] == "repair_22_33_312"]
    assert targets == [{"name": "repair_22_33_312", "path": "../tests/repair_22_33_312.rs"}]
    rust = source("tests/repair_22_33_312.rs")
    assert "use sig_provider_server::{new_service, Service, SourcesSettings};" in rust
    assert "route_abi_service(config, service.clone());" in rust
    assert "HttpServer::new" in rust and 'TcpListener::bind("127.0.0.1:0")' in rust
    assert 'producer("precompiles/spot/abi/stoporders.json")' in rust
    assert 'producer("precompiles/launchpad/abi.json")' in rust
    record = tomllib.loads(RECORD.read_text())
    finding = record["finding"]["sig-provider-precompile-doc"]
    cases = re.findall(r"#\[actix_web::test\]\s*async fn (\w+)\(", rust)
    assert set(cases) == set(finding["rust_tests"])
    assert len(cases) == 3


def test_precompile_repair_record_maps_finding_and_evidence():
    record = tomllib.loads(RECORD.read_text())
    task = record["task"]["22"]["33"]["312"]
    finding = record["finding"]["sig-provider-precompile-doc"]
    assert finding["audit"] == "22.30.135"
    assert finding["class"] == "doc-drift" and finding["severity"] == "medium"
    assert finding["file"] == "services/explorer/services/sig-provider/README.md"
    tests = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == tests
    assert task["wiring_requests"] == []
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["runtime_limits"]
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_312.py"
