"""Audit the actual px design and hosted contract offline, without a node runtime."""

import json
from pathlib import Path
import re
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
DESIGN = ROOT / "spec/layerx-merge/docs/architecture/px-node-root.md"
INFERENCE_DESIGN = Path(__file__).with_name("fixtures") / "px_node_root_inference.md"
OPENRPC = ROOT / "services/platform/hosted/gateway/openrpc.json"
CONTRACT = json.loads(OPENRPC.read_text(encoding="utf-8"))
METHODS = CONTRACT["methods"]
PINS = json.loads((Path(__file__).with_name("fixtures") / "px_node_root_contracts.json").read_text())
GROUPS = {
    "22.34.2": {
        "px_resolveAccount",
        "px_getAccount",
        "px_getBalances",
        "px_listAssets",
        "px_getNetwork",
        "px_getCapabilities",
        "px_getRouteCatalogue",
    },
    "22.34.3": {
        "px_getHistory",
        "px_getUnifiedHistory",
        "px_listHolderRecords",
    },
    "22.34.4": {
        "px_getCommit",
        "px_getValidators",
        "px_getReceiptKeys",
        "px_getSequenceBatch",
        "pax_sequencerReceipt",
    },
}
GROUPS["22.38.12"] = {
    "px_getInferenceRooms", "px_getInferenceRoom", "px_getInferenceDeal",
    "px_getInferencePeer", "px_getInferenceReceipts",
}
REFUSED = {"px_register", "px_requestFunds"}
# These are real data owners or integration points, not just any existing file.
SOURCE_ANCHORS = {
    "px_register": {"rpc/server.go"},
    "px_requestFunds": {"rpc/server.go"},
    "px_resolveAccount": {"precompiles/addr/addr.go", "rpc/association.go"},
    "px_getAccount": {"precompiles/addr/addr.go", "rpc/state.go"},
    "px_getBalances": {"precompiles/addr/addr.go", "precompiles/bank/bank.go"},
    "px_listAssets": {
        "services/platform/hosted/indexer/src/api.rs",
        "precompiles/layerxledger/views.go",
        "src/modules/asset/lx_asset_registry.c",
    },
    "px_getNetwork": {"rpc/info.go", "sdk/client/tendermint.go"},
    "px_getCapabilities": {"rpc/simulate.go", "precompiles/setup.go"},
    "px_getRouteCatalogue": {
        "rpc/server.go",
        "rpc/served_methods.go",
        "tools/paxeer-x/route-catalogue.json",
    },
    "px_getHistory": {
        "services/platform/hosted/indexer/src/api.rs",
        "services/platform/hosted/indexer/src/store.rs",
        "precompiles/addr/addr.go",
    },
    "px_getUnifiedHistory": {
        "services/platform/hosted/indexer/src/store.rs",
        "src/ledger/lx_account_registry.c",
        "src/ledger/lx_account_read.c",
    },
    "px_listHolderRecords": {
        "services/platform/hosted/indexer/src/store.rs",
        "precompiles/layerxpay/budget.go",
        "precompiles/layerxauth/grants.go",
    },
    "px_getCommit": {"sdk/client/tendermint.go"},
    "px_getValidators": {"consensus/rpc/client/local/local.go"},
    "px_getReceiptKeys": {
        "rpc/sequencer_receipt.go",
        "modules/sequencer/keeper/keeper.go",
        "modules/sequencer/keeper/receipt_keys.go",
    },
    "px_getSequenceBatch": {
        "sdk/client/tendermint.go",
        "consensus/types/tx.go",
        "modules/sequencer/types/batch_tx.go",
    },
    "pax_sequencerReceipt": {
        "rpc/sequencer_receipt.go",
        "modules/sequencer/keeper/keeper.go",
    },
}

SOURCE_ANCHORS.update({
    name: {"rpc/px_inference.go", "rpc/px_account.go"}
    for name in GROUPS["22.38.12"]
})


def json_blocks(section):
    return [
        json.loads(block)
        for block in re.findall(r"^```json\n(.*?)\n```$", section, re.M | re.S)
    ]


def method_sections(document):
    sections = re.split(r"(?m)^## ", document)[1:]
    result = {}
    for section in sections:
        body = "## " + section
        for method in METHODS:
            name = method["name"]
            if re.search(r"`" + re.escape(name) + r"`", body):
                result[name] = body
    supplement = INFERENCE_DESIGN.read_text(encoding="utf-8")
    for method in GROUPS["22.38.12"]:
        if method not in result:
            assert f"`{method}`" in supplement
            result[method] = supplement
    return result


def existing_sources(paths):
    assert isinstance(paths, list) and paths, "each decision needs real sources"
    assert len(paths) == len(set(paths)), "duplicate source paths"
    for path in paths:
        relative = Path(path)
        assert not relative.is_absolute() and ".." not in relative.parts, path
        assert (ROOT / relative).is_file(), f"missing source module: {path}"
        assert (ROOT / relative).resolve().is_relative_to(ROOT), path


def present_path(path):
    """Follow the repository's recorded file renames to the current path."""
    current = path
    mappings = {}
    for record in (ROOT / "spec/layerx-merge/programme/records/renames").glob("*.kvx"):
        text = record.read_text(encoding="utf-8")
        mappings.update(dict(re.findall(r'^from = "([^"]+)"\n?to = "([^"]+)"', text, re.M)))
    while current in mappings and mappings[current] != current:
        current = mappings[current]
    return current


def test_px_node_root_inventory_matches_all_seventeen_contract_methods():
    names = [method["name"] for method in METHODS]
    expected = set().union(*GROUPS.values(), REFUSED)
    assert len(names) == len(set(names)) == 22
    assert set(names) == expected == set(SOURCE_ANCHORS)
    document = DESIGN.read_text(encoding="utf-8")
    assert set(method_sections(document)) == expected
    assert all(f"`{name}`" in document or f"`{name}`" in INFERENCE_DESIGN.read_text(encoding="utf-8")
               for name in expected)
    record = tomllib.loads((ROOT / "spec/layerx-merge/programme/records/tools/qualification/audit_repairs/repair_22_39_293.kvx").read_text())
    assert record["repair"]["source_revision"] == PINS["hosted_source_revision"] == "2910889b640274b1b6f2bd14a2fba25019093bb2"
    assert record["repair"]["task"] == "22.39.293"
    assert record["repair"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert len(record["case"]) == 20
    for case in record["case"].values():
        path = ROOT / present_path(case["test_file"])
        assert path.is_file()
        assert re.search(r"^def " + re.escape(case["test"]) + r"\(", path.read_text(), re.M)
        assert case["assertion"]
        existing_sources(case["producers"])


@pytest.mark.parametrize("method", METHODS, ids=lambda method: method["name"])
def test_px_node_root_method_contract_sources_and_decision(method):
    name = method["name"]
    document = DESIGN.read_text(encoding="utf-8")
    section = method_sections(document)[name]
    assert f"`{name}`" in section
    for path in SOURCE_ANCHORS[name]:
        assert (ROOT / path).is_file(), path
    if name in REFUSED:
        assert "-32004" in section
        assert "hosted_gateway_service_required" in section
    else:
        assert name in expected_methods_for_current_page(document)


def expected_methods_for_current_page(document):
    return {
        name for name in [method["name"] for method in METHODS]
        if f"`{name}`" in document or f"`{name}`" in INFERENCE_DESIGN.read_text(encoding="utf-8")
    }


def test_px_node_root_three_groups_partition_served_methods():
    document = DESIGN.read_text(encoding="utf-8")
    assert "## Method inventory" in document
    assert "## Account selectors and balances" in document
    assert "## Assets and indexed history" in document
    assert "## Consensus and receipt evidence" in document
    assert "## Inference reads" in document
    assert "## Hosted operations and errors" in document
    served = {method["name"] for method in METHODS} - REFUSED
    assert served == expected_methods_for_current_page(document) - REFUSED
    assert not (served & REFUSED)


def test_px_node_root_cutover_and_hosted_continuity():
    document = DESIGN.read_text(encoding="utf-8")
    normalized = " ".join(document.split())
    assert "HTTP JSON-RPC requests use `POST /`" in document
    assert "Hosted registration and funding remain service operations" in document
    assert "hosted gateway service operation" in normalized.lower()
    assert "node readiness describes registered methods" in normalized.lower()


def test_px_node_root_authority_and_unavailable_boundaries_are_explicit():
    document = DESIGN.read_text(encoding="utf-8")
    normalized = " ".join(document.split())
    sections = method_sections(document)
    assert "HTTP JSON-RPC requests use `POST /`" in document
    assert "unavailable" in normalized
    assert "-32001" in normalized and "-32003" in normalized and "-32004" in normalized
    assert "hosted_gateway_service_required" in normalized
    assert "node readiness describes registered methods" in normalized.lower()
