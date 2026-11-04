"""Offline proof of the real accessibility generator and JSON route inventory."""

import ast
from copy import deepcopy
from hashlib import sha256
import importlib.util
import json
from pathlib import Path
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
DIRECTORY = ROOT / "apps/launchpad/qualification/accessibility"
CORPUS = DIRECTORY / "corpus.json"
RECORD = Path(__file__).with_name("repair_22_33_1023.kvx")
SPEC = importlib.util.spec_from_file_location(
    "accessibility_inventory", DIRECTORY / "inventory.py"
)
inventory = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(inventory)


def load_corpus(path=CORPUS):
    return json.loads(path.read_text(encoding="utf-8"))


def digest(value):
    return sha256(json.dumps(value, sort_keys=True).encode()).hexdigest()


def test_accessibility_source_path_and_exact_current_page_coverage():
    corpus = load_corpus()
    assert corpus["source"] == "apps/launchpad/web/src/app/**/page.tsx"
    pages = sorted(ROOT.glob(corpus["source"]))
    app = ROOT / "apps/launchpad/web/src/app"
    assert pages
    expected = sorted(
        "/" + "/".join(page.relative_to(app).parts[:-1]) for page in pages
    )
    actual = [row["route"] for row in corpus["routes"]]
    assert actual == expected
    assert len(actual) == len(set(actual))
    assert {
        "/", "/watchlist", "/bounties/[bountyId]", "/terminal/[slug]"
    } <= set(actual)
    baseline = tomllib.loads(RECORD.read_text(encoding="utf-8"))["baseline"]
    assert set(actual) - set(baseline["routes"]) == set(baseline["added_routes"])
    assert set(baseline["routes"]) - set(actual) == set(baseline["removed_routes"])


def test_generator_json_transport_and_inventory_consumer(tmp_path):
    target = tmp_path / "corpus.json"
    target.write_bytes(CORPUS.read_bytes())
    original = load_corpus(target)
    inventory.refresh(ROOT, target)
    consumed = load_corpus(target)
    before_generate = deepcopy(original)
    assert consumed == original == inventory.generate(ROOT, original)
    assert original == before_generate
    assert len(consumed["journeys"]) == 10
    pages = {row["route"] for row in consumed["routes"]}
    for routes in consumed["journeys"].values():
        assert routes and set(routes) <= pages
    assert consumed["observed"] is False
    for row in consumed["routes"]:
        assert row["checks"] == list(inventory.CHECKS)
        assert len(row["checks"]) == 6
        assert row["account"] == "synthetic-only"
        assert row["status"] == "pending-browser-and-manual-AT"
    assert inventory.main(["--check"]) == 0
    before = target.read_bytes()
    inventory.refresh(ROOT, target, check=True)
    assert target.read_bytes() == before


def test_generator_detects_and_repairs_missing_route_without_qualifying(tmp_path):
    target = tmp_path / "corpus.json"
    stale = load_corpus()
    stale["source"] = "client/apps/web/src/app/**/page.tsx"
    stale["routes"] = [
        row for row in stale["routes"] if row["route"] != "/watchlist"
    ]
    stale["missingDedicatedRoutes"] = ["portfolio", "watchlists", "notifications"]
    target.write_text(json.dumps(stale), encoding="utf-8")
    before = target.read_bytes()
    with pytest.raises(ValueError, match="inventory is stale"):
        inventory.refresh(ROOT, target, check=True)
    assert target.read_bytes() == before
    inventory.refresh(ROOT, target)
    assert load_corpus(target) == load_corpus()
    assert load_corpus(target)["observed"] is False


def test_retained_evidence_contract_and_dedicated_routes():
    corpus = load_corpus()
    baseline = tomllib.loads(RECORD.read_text(encoding="utf-8"))["baseline"]
    retained = [
        row for row in corpus["routes"] if row["route"] in baseline["routes"]
    ]
    assert len(retained) == len(baseline["routes"]) == 23
    assert digest(retained) == baseline["retained_rows_sha256"]
    metadata = {
        k: v for k, v in corpus.items()
        if k not in ("source", "routes", "missingDedicatedRoutes")
    }
    assert digest(metadata) == baseline["preserved_metadata_sha256"]
    assert corpus["missingDedicatedRoutes"] == ["portfolio", "notifications"]
    assert corpus["assistiveTechnology"] == ["NVDA+Chrome", "VoiceOver+Safari"]
    journeys = (DIRECTORY / "journeys.ts").read_text(encoding="utf-8")
    for required in (
        "pending-observed-browser-and-manual-AT", "actual-axe-report",
        "manual-AT-recording", "focus-trace", "network-trace", "source-revision",
    ):
        assert required in journeys


@pytest.mark.parametrize("fault, message", [
    ("observed", "cannot replace observed evidence"),
    ("route_evidence", "cannot replace route evidence"),
    ("duplicate", "Duplicate inventory route"),
    ("journey", "Journey references missing page routes"),
])
def test_generator_refuses_to_overwrite_evidence_or_invalid_inventory(
    tmp_path, fault, message
):
    corpus = deepcopy(load_corpus())
    if fault == "observed":
        corpus["observed"] = True
    elif fault == "route_evidence":
        corpus["routes"][0]["status"] = "observed-browser-and-manual-AT"
    elif fault == "duplicate":
        corpus["routes"].append(deepcopy(corpus["routes"][0]))
    elif fault == "journey":
        corpus["journeys"]["discovery"].append("/absent-page")
    target = tmp_path / "corpus.json"
    target.write_text(json.dumps(corpus), encoding="utf-8")
    before = target.read_bytes()
    with pytest.raises(ValueError, match=message):
        inventory.refresh(ROOT, target)
    assert target.read_bytes() == before


def test_generator_rejects_missing_or_empty_route_tree(tmp_path):
    with pytest.raises(ValueError, match="Missing route tree"):
        inventory.generate(tmp_path, load_corpus())
    (tmp_path / "apps/launchpad/web/src/app").mkdir(parents=True)
    with pytest.raises(ValueError, match="Route tree has no pages"):
        inventory.generate(tmp_path, load_corpus())


def test_record_maps_finding_to_executing_tests_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    finding = record["finding"]["accessibility-source-path"]
    audit = tomllib.loads(
        (ROOT / "spec/layerx-merge/programme/audit/records/22.30.38.kvx").read_text()
    )
    for key in ("file", "class", "severity", "owner_domain"):
        assert finding[key] == audit["finding"]["accessibility-source-path"][key]
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    task = record["task"]["22"]["33"]["1023"]
    assert task["audit"] == "22.30.38"
    assert task["wiring_requests"] == []
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in task["evidence"]
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
