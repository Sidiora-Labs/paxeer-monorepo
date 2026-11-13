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
    inventory = document.split("## Method inventory\n", 1)[1].split(
        "## Client cutover order\n", 1
    )[0]
    entries = re.findall(
        r"^### (\w+)\n(.*?)(?=^### |\Z)", inventory, re.M | re.S
    )
    supplement = INFERENCE_DESIGN.read_text().split("## Method inventory\n", 1)[1].split(
        "## Client cutover order\n", 1
    )[0]
    entries.extend(re.findall(r"^### (\w+)\n(.*?)(?=^### |\Z)", supplement, re.M | re.S))
    names = [name for name, _ in entries]
    assert len(names) == len(set(names)), "duplicate method sections"
    return dict(entries)


def existing_sources(paths):
    assert isinstance(paths, list) and paths, "each decision needs real sources"
    assert len(paths) == len(set(paths)), "duplicate source paths"
    for path in paths:
        relative = Path(path)
        assert not relative.is_absolute() and ".." not in relative.parts, path
        assert (ROOT / relative).is_file(), f"missing source module: {path}"
        assert (ROOT / relative).resolve().is_relative_to(ROOT), path


def test_px_node_root_inventory_matches_all_seventeen_contract_methods():
    names = [method["name"] for method in METHODS]
    expected = set().union(*GROUPS.values(), REFUSED)
    assert len(names) == len(set(names)) == 22
    assert set(names) == expected == set(SOURCE_ANCHORS)
    assert set(method_sections(DESIGN.read_text(encoding="utf-8"))) == expected
    record = tomllib.loads(Path(__file__).with_name("repair_22_39_293.kvx").read_text())
    assert record["repair"]["source_revision"] == PINS["hosted_source_revision"] == "2910889b640274b1b6f2bd14a2fba25019093bb2"
    assert record["repair"]["task"] == "22.39.293"
    assert record["repair"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert len(record["case"]) == 20
    for case in record["case"].values():
        assert (ROOT / case["test_file"]).is_file()
        assert re.search(r"^def " + re.escape(case["test"]) + r"\(", (ROOT / case["test_file"]).read_text(), re.M)
        assert case["assertion"]
        existing_sources(case["producers"])


@pytest.mark.parametrize("method", METHODS, ids=lambda method: method["name"])
def test_px_node_root_method_contract_sources_and_decision(method):
    name = method["name"]
    section = method_sections(DESIGN.read_text(encoding="utf-8"))[name]
    blocks = json_blocks(section)
    assert len(blocks) == 2, f"{name}: one decision and one full contract"
    decision, copied_contract = blocks
    # The full real OpenRPC object is the oracle, including every nested result
    # property, parameter description, error, bound, required flag and name.
    assert copied_contract == PINS["design"][name], f"{name}: archived design contract changed or abbreviated"
    assert method == next(pin for pin in PINS["hosted"] if pin["name"] == name), (
        f"{name}: live hosted contract changed or abbreviated"
    )
    existing_sources(decision["node_sources"])
    existing_sources(decision["gateway_sources"])
    assert SOURCE_ANCHORS[name] <= set(decision["node_sources"])
    assert all(
        path.startswith("services/platform/hosted/gateway/src/")
        for path in decision["gateway_sources"]
    )
    assert re.search(r"[Hh]osted (?:gateway )?.*(?:unchanged|identical)", section)
    if name in REFUSED:
        assert decision["decision"] == "refused"
        assert decision["implementation_task"] == "22.34.2"
        assert decision["refusal"] == {
            "code": -32004,
            "message": "Hosted gateway service operation",
            "data": {"code": "hosted_gateway_service_required", "method": name},
        }
    else:
        assert decision["decision"] == "served"
        assert "refusal" not in decision
        assert decision["implementation_task"] in GROUPS
        assert name in GROUPS[decision["implementation_task"]]


def test_px_node_root_three_groups_partition_served_methods():
    document = DESIGN.read_text(encoding="utf-8")
    group_section = document.split("## Implementation groups\n", 1)[1].split(
        "## Method inventory\n", 1
    )[0]
    blocks = json_blocks(group_section)
    assert len(blocks) == 1
    groups = blocks[0]
    extension = INFERENCE_DESIGN.read_text().split("## Implementation groups\n", 1)[1].split(
        "## Method inventory\n", 1
    )[0]
    extension_blocks = json_blocks(extension)
    assert len(extension_blocks) == 1
    assert not (set(groups) & set(extension_blocks[0]))
    groups.update(extension_blocks[0])
    assert set(groups) == set(GROUPS)
    sections = method_sections(document)
    served = {
        name
        for name, section in sections.items()
        if json_blocks(section)[0]["decision"] == "served"
    }
    assigned = []
    for task, group in groups.items():
        assert set(group["methods"]) == GROUPS[task]
        existing_sources(group["node_sources"])
        assert {"rpc/server.go", "rpc/served_methods.go"} <= set(
            group["node_sources"]
        )
        for name in group["methods"]:
            decision = json_blocks(sections[name])[0]
            assert decision["implementation_task"] == task
            assert set(decision["node_sources"]) <= set(group["node_sources"])
        assigned.extend(group["methods"])
    assert len(assigned) == len(set(assigned)) == 20, "overlapping groups"
    assert set(assigned) == served
    assert not (served & REFUSED)
    assert served | REFUSED == {method["name"] for method in METHODS}


def test_px_node_root_cutover_and_hosted_continuity():
    document = DESIGN.read_text(encoding="utf-8")
    cutover = document.split("## Client cutover order\n", 1)[1].split(
        "## Qualification and limits\n", 1
    )[0]
    clients = [
        "services/agent/crates/layerx-client/src/px.rs",
        "services/agent/crates/layerx-agentd/src/config.rs",
        "services/agent/crates/layerx-agentd/src/read/checkpoint.rs",
        "services/agent/crates/layerx-agentd/src/read/committed.rs",
        "services/agent/crates/layerx-client/src/rpc/commit.rs",
        "services/agent/crates/layerx-client/src/rpc/receipt_commit.rs",
        "apps/web/src/api/gateway.ts",
        "scripts/wallet/check-live.sh",
        "docker/gas-station/init.sh",
    ]
    existing_sources(clients)
    assert all(f"`{path}`" in cutover for path in clients)
    steps = re.findall(r"^(\d+)\. (.*?)(?=^\d+\. |\Z)", cutover, re.M | re.S)
    assert [number for number, _ in steps] == ["1", "2", "3", "4", "5"]
    assert clients[0] in steps[1][1]
    assert clients[1] in steps[2][1] and clients[3] in steps[2][1]
    assert clients[6] in steps[3][1]
    assert clients[7] in steps[4][1] and clients[8] in steps[4][1]
    normalized = " ".join(document.split())
    assert "hosted gateway keeps serving every method until the node serves it" in normalized
    for obligation in (
        "LAYERX_NODE_ENDPOINT",
        "LAYERX_EXPECTED_CHAIN_ID",
        "LAYERX_RECEIPT_KEY_SOURCE",
        "LAYERX_GATEWAY_URL",
        "GAS_STATION_RECEIPT_KEYS",
        "independently trusted",
        "restoring its prior configured gateway route",
    ):
        assert obligation in cutover


def test_px_node_root_authority_and_unavailable_boundaries_are_explicit():
    document = DESIGN.read_text(encoding="utf-8")
    normalized = " ".join(document.split())
    sections = method_sections(document)
    assert "JSON-RPC HTTP POST `/`" in document
    assert "validate positional parameters first (-32602 for invalid params)" in normalized
    assert "node-specific refusal" in document
    assert "indefinitely" in document
    assert "indexer_not_configured" in sections["px_getHistory"]
    assert "indexer_not_configured" in sections["px_getUnifiedHistory"]
    assert "-32001 History unavailable" in sections["px_listHolderRecords"]
    assert "receipt_keys_unavailable" in sections["px_getReceiptKeys"]
    assert "-32000" in sections["pax_sequencerReceipt"]
    assert "contract/implementation discrepancy" in sections["px_getUnifiedHistory"]
    assert "authenticated catalogue" in sections["px_getRouteCatalogue"]
    assert "node readiness is scoped to this node" in sections["px_getRouteCatalogue"]
    assert "no network, credentials, subprocesses or native toolchain" in normalized
