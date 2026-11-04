"""Offline source and receipt-consumer qualification; no paxd/toolchain required."""

import ast
import base64
from copy import deepcopy
import gzip
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import re
import shlex
import threading
import time
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
UTILS = ROOT / "tests/integration/utils"
RECORD = Path(__file__).with_name("repair_22_33_762.kvx")
SPEC = importlib.util.spec_from_file_location("tx_receipts_762", UTILS / "_tx_receipts.py")
RECEIPTS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RECEIPTS)


@pytest.fixture
def cases():
    return json.loads((UTILS / "receipt_cases.json").read_text())


@pytest.fixture
def peer(cases):
    """Serve committed wire fixtures over the production JSON-RPC transport."""
    calls = []
    outages = {}

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            method, params = request["method"], request["params"]
            key = (method, params.get("height", "43"))
            calls.append(key)
            assert request["jsonrpc"] == "2.0"
            assert method in {"block", "block_results"}
            assert self.headers["Content-Type"] == "application/json"
            table = cases["blocks" if method == "block" else "results"]
            if outages.get(key, 0):
                outages[key] -= 1
                response = {"error": {"code": -32603, "message": "result not yet available"}}
            elif key[1] not in table:
                response = {"error": {"code": -32603, "message": "height exceeds head"}}
            else:
                response = {"result": table[key[1]]}
            response.update(jsonrpc="2.0", id=request["id"])
            body = json.dumps(response).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01})
    thread.start()
    context = dict(cases["context"], node=f"http://127.0.0.1:{server.server_port}")
    try:
        yield RECEIPTS.RPC(context["node"]), context, calls, outages
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=1)
        assert not thread.is_alive()


def committed(peer, cases, name, timeout=1):
    rpc, context, _, _ = peer
    return RECEIPTS.resolve(rpc, context, cases["submissions"][name],
                            time.monotonic() + timeout, 0.01)


def query_store(cases):
    """Exact-height immutable query transcript; unrecorded queries raise KeyError."""
    entries = {(tuple(entry["command"]), entry["height"]): entry["response"]
               for entry in cases["queries"]}
    calls = []

    def query(command, height):
        key = (tuple(command), height)
        calls.append(key)
        return deepcopy(entries[key])

    return query, calls


def test_tx_helper_inclusion_is_exact_hash_and_position(peer, cases):
    receipt = committed(peer, cases, "bank")
    assert receipt["height"] == 42
    assert receipt["txhash"] == cases["submissions"]["bank"]["txhash"]
    assert receipt["stage"] == "execution"
    assert receipt["code"] == 0
    assert RECEIPTS.finish("height", receipt, None) == 42
    assert peer[2] == [("block", "42"), ("block_results", "42")]
    # The fixture head is 43 and its objects belong to a concurrent writer.
    assert cases["blocks"]["43"]["block"]["header"]["height"] == "43"


def test_tx_helper_retries_same_block_and_execution_position(peer, cases):
    peer[3][("block", "42")] = 1
    peer[3][("block_results", "42")] = 2
    receipt = committed(peer, cases, "bank")
    assert receipt["height"] == 42
    assert peer[2].count(("block", "42")) == 4
    assert peer[2].count(("block_results", "42")) == 3
    assert ("block", "43") not in peer[2]


@pytest.mark.parametrize("unavailable", ["absent_hash", "historical_block", "execution"])
def test_tx_helper_missing_evidence_fails_by_deadline(peer, cases, unavailable):
    if unavailable == "absent_hash":
        cases["submissions"]["bank"]["txhash"] = "A" * 64
    elif unavailable == "historical_block":
        del cases["blocks"]["42"]
    else:
        del cases["results"]["42"]
    started = time.monotonic()
    with pytest.raises(TimeoutError):
        committed(peer, cases, "bank", timeout=0.12)
    assert time.monotonic() - started < 1
    assert peer[2]
    if unavailable == "historical_block":
        assert set(peer[2]) == {("block", "42")}


@pytest.mark.parametrize("damage", ["chain", "block_height", "result_height", "positions", "null_result", "code"])
def test_tx_helper_rejects_inconsistent_committed_evidence(peer, cases, damage):
    if damage == "chain":
        cases["blocks"]["42"]["block"]["header"]["chain_id"] = "other-chain"
    elif damage == "block_height":
        cases["blocks"]["42"]["block"]["header"]["height"] = "43"
    elif damage == "result_height":
        cases["results"]["42"]["height"] = "43"
    elif damage == "positions":
        cases["results"]["42"]["txs_results"].pop()
    elif damage == "null_result":
        cases["results"]["42"]["txs_results"][1] = None
    else:
        cases["results"]["42"]["txs_results"][1]["code"] = -1
    with pytest.raises(ValueError):
        committed(peer, cases, "bank")


def test_tx_helper_execution_code_is_not_checktx_acceptance(peer, cases, capsys):
    receipt = committed(peer, cases, "failed")
    assert cases["submissions"]["failed"]["code"] == 0
    assert receipt["code"] == 9
    assert RECEIPTS.finish("code", receipt, None) == 9
    assert "CheckTx accepted" in capsys.readouterr().err
    for mode in ("height", "store", "instantiate"):
        with pytest.raises(ValueError, match="execution failed with code 9"):
            RECEIPTS.finish(mode, receipt, None)


def test_tx_helper_checktx_rejection_is_separately_named(peer, capsys):
    rpc, context, calls, _ = peer
    receipt = RECEIPTS.resolve(rpc, context, {"code": 5, "raw_log": "bad fee"},
                               time.monotonic() + 1, 0.01)
    assert receipt == {"stage": "check_tx", "code": 5}
    assert RECEIPTS.finish("code", receipt, None) == 5
    assert "CheckTx rejected: bad fee" in capsys.readouterr().err
    assert calls == []
    with pytest.raises(ValueError, match="check_tx failed"):
        RECEIPTS.finish("height", receipt, None)


@pytest.mark.parametrize("field,value", [("txhash", "not-a-hash"), ("code", None)])
def test_tx_helper_malformed_submission_is_never_acceptance(peer, cases, field, value):
    cases["submissions"]["bank"][field] = value
    with pytest.raises(ValueError):
        committed(peer, cases, "bank")
    assert peer[2] == []


def test_wasm_store_returns_own_event_and_checks_metadata(peer, cases):
    receipt = committed(peer, cases, "store")
    receipt["events"][0]["attributes"].extend([
        {"key": base64.b64encode(b"feature").decode(),
         "value": base64.b64encode(feature).decode()}
        for feature in (b"staking", b"iterator")
    ])
    query, calls = query_store(cases)
    wasm = bytes.fromhex(cases["wasm_hex"])
    assert RECEIPTS.finish("store", receipt, query, creator=cases["creator"], wasm_bytes=wasm) == 7
    assert calls == [(("wasm", "code-info", "7"), 42)]
    # The node hashes uncompressed code even when the input file is gzip encoded.
    info = cases["queries"][0]["response"]
    RECEIPTS.verify_store(7, info, cases["creator"], gzip.compress(wasm))


@pytest.mark.parametrize("field,value", [("creator", "pax1other"), ("data_hash", "00" * 32), ("code_id", "999")])
def test_wasm_store_rejects_foreign_or_wrong_code(peer, cases, field, value):
    cases["queries"][0]["response"][field] = value
    query, _ = query_store(cases)
    with pytest.raises(ValueError, match="checksum, creator or code ID"):
        RECEIPTS.finish("store", committed(peer, cases, "store"), query,
                        creator=cases["creator"], wasm_bytes=bytes.fromhex(cases["wasm_hex"]))


def test_wasm_instantiate_returns_own_event_and_verifies_initialization(peer, cases):
    receipt = committed(peer, cases, "instantiate")
    query, calls = query_store(cases)
    result = RECEIPTS.finish("instantiate", receipt, query, creator=cases["creator"],
                             code_id=7, label=cases["label"], init_msg=cases["init_msg"])
    assert result == "pax1owncontract"
    assert calls == [(("wasm", "contract", result), 42),
                     (("wasm", "contract-history", result), 42)]


@pytest.mark.parametrize("damage", ["creator", "code_id", "label", "address", "message", "operation", "history_code", "empty_history"])
def test_wasm_instantiate_rejects_wrong_identity_or_initialization(peer, cases, damage):
    info = cases["queries"][1]["response"]
    history = cases["queries"][2]["response"]
    if damage in {"creator", "code_id", "label"}:
        info["contract_info"][damage] = "999" if damage == "code_id" else "other"
    elif damage == "address":
        info["address"] = "pax1othercontract"
    elif damage == "message":
        history["entries"][0]["msg"]["count"] = 8
    elif damage == "operation":
        history["entries"][0]["operation"] = "CONTRACT_CODE_HISTORY_OPERATION_TYPE_MIGRATE"
    elif damage == "history_code":
        history["entries"][0]["code_id"] = "999"
    else:
        history["entries"] = []
    query, _ = query_store(cases)
    with pytest.raises(ValueError):
        RECEIPTS.finish("instantiate", committed(peer, cases, "instantiate"), query,
                        creator=cases["creator"], code_id=7,
                        label=cases["label"], init_msg=cases["init_msg"])


@pytest.mark.parametrize("mode", ["store", "instantiate"])
@pytest.mark.parametrize("damage", ["missing", "multiple", "duplicate_attribute", "wrong_code"])
def test_wasm_requires_unambiguous_transaction_events(peer, cases, mode, damage):
    receipt = committed(peer, cases, mode)
    if damage == "missing":
        receipt["events"] = []
    elif damage == "multiple":
        receipt["events"] *= 2
    elif damage == "duplicate_attribute":
        receipt["events"][0]["attributes"] *= 2
    else:
        attributes = receipt["events"][0]["attributes"]
        for attribute in attributes:
            if base64.b64decode(attribute["key"]) == b"code_id":
                attribute["value"] = base64.b64encode(b"999").decode()
    query, _ = query_store(cases)
    with pytest.raises((ValueError, KeyError)):
        RECEIPTS.finish(mode, receipt, query, creator=cases["creator"],
                        wasm_bytes=bytes.fromhex(cases["wasm_hex"]),
                        code_id=7, label=cases["label"], init_msg=cases["init_msg"])


def shell_function(name):
    source = (UTILS / "_tx_helpers.sh").read_text()
    match = re.search(rf"^{name}\(\) \{{\n(.*?)^\}}$", source, re.M | re.S)
    assert match, name
    # Lex quoted arguments/continuations without executing an external shell.
    return match.group(1), shlex.split(match.group(1).replace("\\\n", " "))


def test_shell_submission_transport_and_failure_guards():
    source = (UTILS / "_tx_helpers.sh").read_text()
    assert '${BASH_SOURCE[0]}' in source
    body, tokens = shell_function("_tx_receipt")
    assert tokens[:3] == ["python3", "$_TX_RECEIPTS", "$@"]
    assert ["--timeout", "$TX_WAIT_TIMEOUT"] == tokens[-4:-2]
    assert tokens[-2:] == ["--interval", "$TX_WAIT_INTERVAL"]
    for function, mode in (("bank_send_and_get_height", "height"),
                           ("submit_tx_and_wait", "code"),
                           ("store_wasm", "store"), ("instantiate_wasm", "instantiate")):
        body, tokens = shell_function(function)
        assert 'context=$(_tx_receipt prepare) || return 1' in body
        assert body.index("_tx_receipt prepare") < body.index("$paxdbin tx")
        assert '--broadcast-mode=sync' in tokens
        assert '--output=json)' in tokens
        assert ') || return 1' in body
        assert f"| _tx_receipt {mode} --context \"$context\"" in body
        assert 'printf \'%s\\n\' "$resp"' in body
        for forbidden in ("_get_account_sequence", "h_obs", "list-code", "list-contract", "comm -13", "|| true"):
            assert forbidden not in body
        if mode in {"store", "instantiate"}:
            assert 'keys show "$keyname" -a) || return 1' in body
            assert '--creator "$creator"' in body
    assert '_get_max_wasm_code_id' not in source
    assert '_list_contracts_by_code' not in source


def test_worker_cli_pins_queries_and_enforces_shared_deadline():
    source = (UTILS / "_tx_receipts.py").read_text()
    tree = ast.parse(source)
    functions = {node.name: node for node in tree.body if isinstance(node, ast.FunctionDef)}
    run = [node for node in ast.walk(functions["cli"])
           if isinstance(node, ast.Call) and ast.unparse(node.func) == "subprocess.run"]
    assert len(run) == 1
    keywords = {item.arg: ast.unparse(item.value) for item in run[0].keywords}
    assert keywords["check"] == "True"
    assert keywords["timeout"] == "remaining(deadline)"
    assert 'shlex.split(paxdbin) + args' in source
    assert '["q"] + command + ["--height", str(height), "-o", "json"]' in source
    assert 'receipt = resolve(RPC(context["node"]), context, json.load(sys.stdin), deadline, args.interval)' in source
    assert 'timeout=remaining(deadline)' in source
    assert 'remaining(deadline)\n    print(result)' in source
    assert 'context["chain_id"] != args.chain_id' in source
    assert 'sys.exit(1)' in source


def test_supported_producer_shapes_match_receipt_and_wasm_consumers():
    blocks = (ROOT / "consensus/internal/rpc/core/blocks.go").read_text()
    assert 'func (env *Environment) Block(' in blocks
    assert 'giga.BlockByNumber(ctx, gbn)' in blocks
    assert 'env.StateStore.LoadFinalizeBlockResponses(height)' in blocks
    assert 'TxsResults:            results.TxResults' in blocks
    assert 'Height:                height' in blocks
    assert 'crypto.Checksum(tx)' in (ROOT / "consensus/types/tx.go").read_text()
    assert 'sha256.Sum256(bz)' in (ROOT / "consensus/crypto/crypto.go").read_text()
    abci = (ROOT / "consensus/abci/types/types.pb.go").read_text()
    assert 'json:"code,omitempty"' in abci
    assert 'Key   []byte' in abci and 'Value []byte' in abci
    serialization = (ROOT / "consensus/abci/types/types.go").read_text()
    assert 'func (r *EventAttribute) MarshalJSON()' in serialization
    assert 'jsonpbMarshaller.MarshalToString(r)' in serialization
    queries = (ROOT / "wasm/x/wasm/client/cli/query.go").read_text()
    for command in ('code-info [code_id]', 'contract [bech32_address]', 'contract-history [bech32_address]'):
        assert command in queries
    assert 'clientCtx.PrintProto(res.CodeInfoResponse)' in queries
    types = (ROOT / "wasm/x/wasm/types/query.pb.go").read_text()
    assert 'name=code_id' in types and 'name=data_hash' in types
    keeper = (ROOT / "wasm/x/wasm/keeper/keeper.go").read_text()
    assert 'types.EventTypeStoreCode' in keeper
    assert 'types.EventTypeInstantiate' in keeper
    assert 'contractInfo.InitialHistory(initMsg)' in keeper
    events = (ROOT / "wasm/x/wasm/types/events.go").read_text()
    for value in ('"store_code"', '"instantiate"', '"code_id"', '"_contract_address"'):
        assert value in events
    raw = (ROOT / "wasm/x/wasm/types/tx.go").read_text()
    assert 'return json.RawMessage(r).MarshalJSON()' in raw


def test_repair_record_names_every_finding_and_qualification_limits():
    record = tomllib.loads(RECORD.read_text())
    tests = {node.name for node in ast.parse(Path(__file__).read_text()).body
             if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    assert set(record["finding"]) == {"tx-helper-inclusion-fallback", "tx-helper-execution-code", "wasm-helper-global-identity"}
    for finding in record["finding"].values():
        assert finding["tests"] and set(finding["tests"]) <= tests
    assert record["baseline"]["verification_run"] is False
    assert record["repair"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert record["repair"]["review_only"]
    assert record["wiring"]["requests"] == []
