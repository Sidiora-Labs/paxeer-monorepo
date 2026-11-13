"""Offline documentation contracts for audit 22.30.147's environment finding.

These checks read production sources; they do not execute Go or Elixir or
claim runtime synchronization, deployment, or recovery qualification.
"""

import ast
import hashlib
from pathlib import Path
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
ENV_PATH = "services/explorer/deploy/env/backend.example.env"
BASE_ASSIGNMENTS_SHA256 = (
    "c5bac886c69ce0e51c0d01e8ae2aef7f65a337cd6bc881bf6ad116b5b4a957bc"
)


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def require_facts(text, facts):
    normalized = " ".join(text.replace("\n# ", " ").split())
    for fact in facts:
        assert fact in normalized, fact


def test_env_template_describes_merged_chain_and_retired_surfaces():
    identity = source("docs/readme/NETWORK.md")
    require_facts(identity, (
        "one BFT network with EVM chain ID **125** (`0x7d`)",
        "The LayerX kernel is embedded in the node",
        "proposes blocks inside `paxd`",
        "validators commit those blocks through BFT consensus",
        "A receipt is final once its block is committed",
        "There is no separate kernel daemon, sequencer service or checkpoint deployment",
        "retired refusal surfaces retained for compatibility",
    ))
    env = source(ENV_PATH)
    assert "CHAIN_TYPE=paxeer_x" in env
    assert "ETHEREUM_JSONRPC_VARIANT=paxeer_x" in env


def test_env_sync_comments_match_node_transport_and_adapter_source():
    transport_comments = source(ENV_PATH)
    require_facts(transport_comments, (
        "HTTP request endpoints and WebSocket subscriptions are separate settings",
        "CHAIN_TYPE and ETHEREUM_JSONRPC_VARIANT select paxeer_x",
    ))
    assert "reads eth_syncing's refusal as" not in source(ENV_PATH)

    producer = source("rpc/info.go")
    syncing = producer.split("func (i *InfoAPI) Syncing(", 1)[1].split(
        "\n// PendingBaseFee", 1
    )[0]
    assert re.search(r"status, err := i\.tmClient\.Status\(ctx\)\s+"
                     r"if err != nil \{\s+return nil, err\s+\}", syncing)
    assert re.search(r"if !info\.CatchingUp \{\s+return false, nil\s+\}", syncing)
    assert "return &SyncingResult{" in syncing
    for field in ("startingBlock", "currentBlock", "highestBlock"):
        assert re.search(r'hexutil\.Uint64\s+`json:"' + field + r'"`', producer)

    http = source(
        "services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/http.ex"
    )
    assert '%{"error" => error} -> {:error, standardize_error(error)}' in http
    assert '%{"result" => result} -> {:ok, result}' in http
    adapter = source(
        "services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/paxeer_x.ex"
    )
    body = adapter.split("  def fetch_syncing_status(", 1)[1].split("\n  @doc", 1)[0]
    assert '%{id: 0, method: "eth_syncing", params: []}' in body
    assert "with {:ok, result} <- json_rpc(syncing_request, json_rpc_named_arguments) do" in body
    assert re.search(r"false ->\s+\{:ok, :synced\}", body)
    assert '%{"currentBlock" => current_block, "highestBlock" => highest_block}' in body
    assert "{:ok, {:syncing, quantity_to_integer(current_block), quantity_to_integer(highest_block)}}" in body
    assert "with {:ok, result} <- json_rpc(syncing_request, json_rpc_named_arguments)" in body
    assert "else" not in body  # with propagates the original error unchanged
    assert "{:error, {:unexpected_syncing_result, other}}" in body
    assert body.count("{:ok, :synced}") == 1


def test_all_effective_env_assignments_are_preserved():
    assignments = [
        line for line in source(ENV_PATH).splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]
    values = {}
    for line in assignments:
        match = re.fullmatch(r"([A-Z][A-Z0-9_<>]*)=(.*)", line)
        assert match, line
        key, value = match.groups()
        assert key not in values, key
        values[key] = value
    assert len(assignments) == 575
    payload = ("\n".join(assignments) + "\n").encode("utf-8")
    assert hashlib.sha256(payload).hexdigest() == BASE_ASSIGNMENTS_SHA256
    assert values["ETHEREUM_JSONRPC_VARIANT"] == "paxeer_x"
    assert values["ETHEREUM_JSONRPC_TRANSPORT"] == "http"
    assert values["ETHEREUM_JSONRPC_HTTP_TIMEOUT"] == "30"
    for name in (
        "ETHEREUM_JSONRPC_HTTP_URL", "ETHEREUM_JSONRPC_TRACE_URL",
        "ETHEREUM_JSONRPC_ETH_CALL_URL", "ETHEREUM_JSONRPC_WS_URL",
    ):
        assert name not in values
    for name, value in {
        "MAX_INFLIGHT": "32", "RPS": "200", "TRACE_MAX_INFLIGHT": "4",
        "ARCHIVE_MAX_INFLIGHT": "8", "COALESCE_ENABLED": "true",
    }.items():
        assert values["ETHEREUM_JSONRPC_THROTTLE_" + name] == value


def test_http_websocket_and_throttle_names_reach_existing_consumers():
    comments = source(ENV_PATH)
    require_facts(comments, (
        "HTTP request endpoints", "WebSocket subscriptions are separate settings",
    ))
    compose = source("services/explorer/deploy/docker-compose.local.yml")
    assert "      - ./env/backend.example.env" in compose
    for name, alias in {
        "HTTP_URL": "rpc-http", "TRACE_URL": "rpc-http",
        "ETH_CALL_URL": "rpc-http", "WS_URL": "rpc-ws",
    }.items():
        assert f"      ETHEREUM_JSONRPC_{name}: *{alias}" in compose
    runtime = source("services/explorer/backend/config/runtime.exs")
    catalogue = source(
        "services/explorer/backend/docker-compose/envs/common-blockscout.env"
    )
    for name in (
        "MAX_INFLIGHT", "RPS", "TRACE_MAX_INFLIGHT", "ARCHIVE_MAX_INFLIGHT",
        "COALESCE_ENABLED",
    ):
        variable = "ETHEREUM_JSONRPC_THROTTLE_" + name
        assert f'"{variable}"' in runtime
        assert "# " + variable + "=" in catalogue


def test_repair_record_maps_finding_to_declared_source_proofs():
    record = tomllib.loads(
        (ROOT / "spec/layerx-merge/programme/records/tools/qualification/audit_repairs/repair_22_33_447.kvx").read_text(encoding="utf-8")
    )
    repair = record["repair"]
    assert repair["task"] == "22.33.447"
    assert repair["audit"] == "22.30.147"
    assert repair["acceptance"] == "22998.1"
    assert repair["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_explorer_env_sync_contracts.py"
    )
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["verified_revision"]
    assert "not run" in repair["baseline"]
    finding = record["finding"]["env-template-sync-and-retired"]
    assert finding["file"] == ENV_PATH
    assert finding["class"] == "doc-drift"
    assert finding["severity"] == "medium"
    tests = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == tests - {
        "test_repair_record_maps_finding_to_declared_source_proofs"
    }
    assert len(finding["tests"]) == len(set(finding["tests"]))
    assert "Domain 14" in repair["wiring_requests"][0]
    assert "paxeer_x_test.exs" in repair["wiring_requests"][0]
