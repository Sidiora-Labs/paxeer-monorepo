"""Offline source and production exporter proof for audit 22.30.181/f060,f009.

The real-contract integration is inspected, not executed; no external runtime is
needed. The existing event-store peer supplies standard RPC to the real client.
"""

import argparse
import ast
import hashlib
import importlib.util
import json
from pathlib import Path
import shlex
import sys
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
HERE = ROOT / "tools/paxeer-x/sid-snapshot"
RECORD = Path(__file__).with_name("repair_22_33_846.kvx")
sys.path.insert(0, str(HERE))
try:
    import snapshot
    import contract_test
finally:
    sys.path.pop(0)

SPEC = importlib.util.spec_from_file_location("sid_snapshot_prior_proof", HERE / "test_repair_22_31_429.py")
prior = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(prior)
peer = prior.peer
A, B, C, ZERO = prior.A, prior.B, prior.C, prior.ZERO
D = "0x" + "d" * 40


def function_source(path, name):
    text = path.read_text()
    tree = ast.parse(text)
    node, = [node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == name]
    return ast.get_source_segment(text, node)


def export(url, sources, output, height=2005):
    argv = ["--rpc", url, "--block", str(height), "--out", str(output)]
    for source in sources:
        argv.extend(["--source", source])
    snapshot.main(argv)


def test_f060_documented_release_height_and_reconciliation_match_source():
    readme = (HERE / "README.md").read_text()
    command, = [part.split("```", 1)[0] for part in readme.split("```sh\n")[1:]
                if "snapshot.py" in part.split("```", 1)[0]]
    argv = shlex.split(command.replace("\\\n", " "))
    assert argv[:2] == ["python3", "tools/paxeer-x/sid-snapshot/snapshot.py"]
    assert argv[argv.index("--block") + 1] == "$MIGRATION_HEIGHT"
    assert argv.count("--source") == 2 and "--rpc" in argv and "--out" in argv
    for fact in (
        "one committed migration height", "Complete indexed history", "from genesis",
        "including addresses with zero or", "negative history balances", "all holder candidates",
        "`totalSupply` equality", "mint-minus-burn history", "block's hash",
        "A receipt is final when its block is", "committed", "root path",
        "does not freeze balances", "not consumed by the upgrade handler",
    ):
        assert fact in readme, fact
    assert "There is no height override" not in readme
    main = function_source(HERE / "snapshot.py", "main")
    assert 'ap.add_argument("--block", type=int,' in main
    assert 'block = migration_block(args.rpc, args.block)' in main
    for guard in (
        'held.setdefault(address, 0)', 'for holder in sorted(held):',
        'onchain = balance_of(args.rpc, holder, block)',
        'supply = token_uint(args.rpc, "0x18160ddd", block)',
        'if history != canonical:', 'if sum(held.values()) != supply:',
        'if history_supply != supply:', 'if committed_block(args.rpc, hex(block["number"])) != block:',
    ):
        assert guard in main, guard
        assert main.index(guard) < main.index('Path(args.out).write_bytes(payload)')
    assert '{"blockHash": block["hash"], "requireCanonical": True}' in function_source(
        HERE / "snapshot.py", "token_uint"
    )
    assert 'AND block_number <= %s AND block_consensus = TRUE' in snapshot.QUERY


def test_f060_selects_requested_historical_committed_height(peer):
    chain, url = peer
    block = snapshot.migration_block(url, 3)
    assert block == {"number": 3, "hash": prior.block_hash(3)}
    assert [call["params"] for call in chain.calls] == [["finalized", False], ["0x3", False]]
    assert snapshot.migration_block(url, None) == {
        "number": chain.finalized, "hash": prior.block_hash(chain.finalized)
    }


@pytest.mark.parametrize("height", [-1, 2006])
def test_f060_refuses_negative_or_uncommitted_migration_height(peer, tmp_path, height):
    chain, url = peer
    source = prior.write_source(tmp_path / "empty.csv", [])
    output = tmp_path / "holders.json"
    with pytest.raises(SystemExit, match="nonnegative and already finalized"):
        export(url, [source], output, height)
    assert [call["method"] for call in chain.calls] == ["eth_getBlockByNumber"]
    assert not output.exists() and not Path(str(output) + ".evidence.json").exists()


def test_f060_fixed_height_preserves_moving_and_zero_history_holders(peer, tmp_path, capsys):
    chain, url = peer
    chain.send(1, ZERO, A, 1000)
    chain.send(2, A, B, 1000)
    chain.send(3, D, D, 0)
    old = prior.write_source(tmp_path / "old.csv", chain.events)
    history, _ = snapshot.balances([old], chain.finalized)
    assert history[A] == 0 and history[D] == 0 and C not in history
    chain.send(2004, B, A, 100)
    chain.send(2005, B, C, 100)
    future = dict(chain.events[-1], blockNumber=hex(2006), transactionHash="0x" + "f" * 64)
    complete = prior.write_source(tmp_path / "explorer.csv", chain.events[1:] + [future])
    chain.advance = True
    output = tmp_path / "holders.json"
    export(url, [old, complete], output)
    assert chain.advanced and chain.state(chain.head) == {A: 90, B: 810, D: 0, C: 100}
    assert json.loads(output.read_text()) == [
        {"address": A, "balance": "100"}, {"address": B, "balance": "800"},
        {"address": C, "balance": "100"},
    ]
    requests = prior.balance_requests(chain)
    assert ["0x" + call["params"][0]["data"][-40:] for call in requests] == [A, B, C, D]
    calls = [call for call in chain.calls if call["method"] == "eth_call"]
    assert len(calls) == 5
    assert all(call["params"][1] == {
        "blockHash": prior.block_hash(2005), "requireCanonical": True
    } for call in calls)
    evidence_path = Path(str(output) + ".evidence.json")
    evidence = json.loads(evidence_path.read_text())
    assert evidence == {
        "chain_id": 125, "token": snapshot.SID,
        "block": {"number": 2005, "hash": prior.block_hash(2005)}, "commitment": "finalized",
        "history_from_block": 0, "history_through_block": 2005, "transfer_count": 5,
        "total_supply": "1000", "holder_count": 3,
        "holders_sha256": hashlib.sha256(output.read_bytes()).hexdigest(),
    }
    original = output.read_bytes(), evidence_path.read_bytes()
    export(url, [old, complete], output)
    assert (output.read_bytes(), evidence_path.read_bytes()) == original
    assert capsys.readouterr().out == "holders 3 sum 1000\nholders 3 sum 1000\n"


@pytest.mark.parametrize("failure", ["stale", "missing_holder", "supply", "conflict"])
def test_f060_incomplete_or_inexact_snapshot_preserves_release_artifact(peer, tmp_path, failure):
    chain, url = peer
    chain.send(1, ZERO, A, 1000)
    chain.send(2, A, B, 1000)
    source = prior.write_source(tmp_path / "old.csv", chain.events)
    sources = [source]
    if failure == "stale":
        chain.send(2005, B, A, 100)
        message = "history is incomplete"
    elif failure == "missing_holder":
        chain.send(2005, B, C, 100)
        message = "history is incomplete"
    elif failure == "supply":
        chain.supply_delta = 1
        message = "holder sum 1000 does not match totalSupply 1001"
    else:
        sources.append(prior.write_source(tmp_path / "conflict.csv", [dict(
            chain.events[0], data="0x" + format(1001, "064x")
        )]))
        message = "sources disagree"
    output = tmp_path / "holders.json"
    evidence = Path(str(output) + ".evidence.json")
    output.write_text("retained release artifact\n")
    evidence.write_text("retained release evidence\n")
    with pytest.raises(SystemExit, match=message):
        export(url, sources, output)
    assert output.read_text() == "retained release artifact\n"
    assert evidence.read_text() == "retained release evidence\n"
    if failure == "missing_holder":
        assert ["0x" + call["params"][0]["data"][-40:] for call in prior.balance_requests(chain)] == [A, B, C]


def test_f009_shell_executes_guarded_real_sid_contract_scenario():
    shell = (HERE / "test.sh").read_text()
    lines = [line for line in shell.splitlines() if line and not line.startswith("#")]
    assert shell.startswith("#!/usr/bin/env bash\n")
    assert lines[0] == "set -euo pipefail"
    assert lines[1] == 'here="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"'
    assert shlex.split(lines[2]) == ["exec", "python3", "$here/contract_test.py", "$@"]
    assert len(lines) == 3
    helper = HERE / "contract_test.py"
    send = function_source(helper, "send_sid")
    assert '"0xa9059cbb" + recipient[2:].rjust(64, "0") + format(amount, "064x")' in send
    assert 'snapshot.rpc_call(rpc, "eth_sendTransaction", [' in send
    assert '"from": sender, "to": snapshot.SID, "data": data' in send
    assert 'return committed_receipt(rpc, tx_hash)' in send
    receipt = function_source(helper, "committed_receipt")
    for guard in (
        '"eth_getTransactionReceipt", [tx_hash]', 'snapshot.quantity(receipt["status"]) == 1',
        'finalized["number"] >= height', 'block = snapshot.migration_block(rpc, height)',
        'block["hash"] == snapshot.hex_bytes(receipt["blockHash"], 32)',
    ):
        assert guard in receipt, guard
    scenario = function_source(helper, "run_contract_snapshot")
    for fact in (
        '"eth_chainId", [])) == 125', '"eth_getCode", [snapshot.SID, "latest"]) != "0x"',
        'new_holder not in prior_candidates', 'indexed[holder] == 0',
        'send_sid(rpc, holder, recipient, amount)', 'send_sid(rpc, recipient, holder, returned)',
        'block = send_sid(rpc, recipient, new_holder, returned)',
        'later = send_sid(rpc, holder, recipient, 1)', 'later["number"] > block["number"]',
        '"--block", str(block["number"])', 'snapshot.main(argv)',
        'snapshot.chain_transfers(rpc, later)', 'sum(expected.values()) == supply',
        'sum(int(row["balance"]) for row in holders) == supply',
        'evidence["block"] == block', 'evidence["total_supply"] == str(supply)',
        '"history is incomplete"', '"sources disagree"',
    ):
        assert fact in scenario, fact
    assert scenario.index('block = send_sid') < scenario.index('later = send_sid') < scenario.index('snapshot.main(argv)')
    assert 'http.server' not in helper.read_text()
    readme = (HERE / "README.md").read_text()
    assert "does not execute an EVM" in readme and "disposable loopback paxd chain" in readme


@pytest.mark.parametrize("rpc", ["http://127.0.0.1:8545", "http://[::1]:8545/"])
def test_f009_real_transfer_driver_accepts_disposable_loopback_rpc(rpc):
    assert contract_test.loopback_rpc(rpc) == rpc


@pytest.mark.parametrize("rpc", [
    "https://api1.mainnet-beta.paxeer.network", "http://192.0.2.1:8545",
    "http://localhost:8545", "http://127.0.0.1:8545/evm", "http://user:pass@127.0.0.1:8545",
    "http://127.0.0.1:8545/?secret=value",
])
def test_f009_real_transfer_driver_refuses_non_disposable_rpc(rpc):
    with pytest.raises(argparse.ArgumentTypeError):
        contract_test.loopback_rpc(rpc)


def test_f009_contract_log_csv_producer_feeds_real_exporter(peer, tmp_path):
    chain, url = peer
    chain.send(1, ZERO, A, 1000)
    chain.send(2005, A, B, 400)
    block = snapshot.migration_block(url, 2005)
    history = snapshot.chain_transfers(url, block)
    source = tmp_path / "producer.csv"
    contract_test.write_history(source, history)
    balances, indexed = snapshot.balances([str(source)], 2005)
    assert indexed == history and balances == {A: 600, B: 400}
    output = tmp_path / "holders.json"
    export(url, [str(source)], output)
    assert json.loads(output.read_text()) == [
        {"address": A, "balance": "600"}, {"address": B, "balance": "400"}
    ]
    assert json.loads(Path(str(output) + ".evidence.json").read_text())["total_supply"] == "1000"


def test_repair_record_maps_findings_and_declares_evidence_limits():
    record = tomllib.loads(RECORD.read_text())
    assert record["repair"]["task"] == "22.33.846"
    assert record["repair"]["audit"] == "22.30.181"
    assert record["repair"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert set(record["finding"]) == {"f060", "f009"}
    names = {node.name for node in ast.parse(Path(__file__).read_text()).body
             if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    for finding in record["finding"].values():
        assert finding["tests"] and set(finding["tests"]) <= names
    assert "not executed" in record["limits"]["real_contract"]
    assert "No baseline gate ran" in record["repair"]["baseline"]
    assert "node/testdata/sid-holders.json" in record["repair"]["wiring_request"]
