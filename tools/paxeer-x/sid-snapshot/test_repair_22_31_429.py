"""Exercise the production exporter through CSV and a loopback Ethereum JSON-RPC peer."""
import csv
import hashlib
import http.server
import importlib.util
import json
from pathlib import Path
import threading

import pytest

SPEC = importlib.util.spec_from_file_location("sid_snapshot", Path(__file__).with_name("snapshot.py"))
snapshot = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(snapshot)
A = "0x" + "a" * 40
B = "0x" + "b" * 40
C = "0x" + "c" * 40
ZERO = "0x" + "0" * 40
TOKEN = "0x21f7b20a555199fa73a238b1a91fd0f549068fee"
TRANSFER_TOPIC = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
FIELDS = ["transaction_hash", "log_index", "from_address", "to_address", "amount", "block_number"]


def block_hash(height):
    return "0x" + hashlib.sha256(f"committed block {height}".encode()).hexdigest()


class TokenChain:
    """Small event-sourced ERC-20 peer emitting standard RPC blocks, logs and ABI words."""

    def __init__(self):
        self.events = []
        self.finalized = 2005
        self.head = self.finalized
        self.calls = []
        self.advance = False
        self.advanced = False
        self.reorg = False
        self.no_finalized = False
        self.supply_delta = 0
        self.rpc_error = False
        self.short_word = False
        self.removed_log = False

    def send(self, height, frm, to, amount):
        state = self.state(height)
        assert amount >= 0
        if frm != ZERO:
            assert state.get(frm, 0) >= amount
        event = {"address": TOKEN, "topics": [TRANSFER_TOPIC, "0x" + frm[2:].rjust(64, "0"),
                                              "0x" + to[2:].rjust(64, "0")],
                 "data": "0x" + format(amount, "064x"), "blockNumber": hex(height),
                 "blockHash": block_hash(height), "transactionHash": "0x" + format(len(self.events) + 1, "064x"),
                 "transactionIndex": "0x0", "logIndex": "0x0", "removed": False}
        self.events.append(event)
        return event

    def state(self, height):
        balances = {}
        for event in self.events:
            if int(event["blockNumber"], 16) > height:
                continue
            frm, to = ("0x" + topic[-40:] for topic in event["topics"][1:])
            amount = int(event["data"], 16)
            if frm != ZERO:
                balances[frm] = balances.get(frm, 0) - amount
            if to != ZERO:
                balances[to] = balances.get(to, 0) + amount
        return balances

    def respond(self, request):
        assert request["jsonrpc"] == "2.0"
        method, params = request["method"], request["params"]
        self.calls.append(request)
        if method == "eth_getBlockByNumber":
            tag, full = params
            assert full is False
            if tag == "finalized" and self.no_finalized:
                return None
            height = self.finalized if tag == "finalized" else int(tag, 16)
            digest = block_hash(height + 1 if self.reorg and tag != "finalized" else height)
            return {"number": hex(height), "hash": digest, "parentHash": block_hash(height - 1),
                    "transactions": [], "timestamp": "0x1"}
        if method == "eth_chainId":
            assert params == []
            return "0x7d"
        if method == "eth_getLogs":
            query, = params
            assert query["address"] == TOKEN and query["topics"] == [TRANSFER_TOPIC]
            start, end = int(query["fromBlock"], 16), int(query["toBlock"], 16)
            assert 0 <= start <= end <= self.finalized
            events = [dict(e) for e in self.events if start <= int(e["blockNumber"], 16) <= end]
            if self.removed_log and events:
                events[0]["removed"] = True
            return events
        assert method == "eth_call", request
        call, selector = params
        assert call["to"] == TOKEN
        if selector == "latest":
            height = self.head
        else:
            assert selector == {"blockHash": block_hash(self.finalized), "requireCanonical": True}
            height = self.finalized
        state = self.state(height)
        data = call["data"]
        if data == "0x18160ddd":
            value = sum(state.values()) + self.supply_delta
        else:
            assert data.startswith("0x70a08231") and len(data) == 74
            assert data[10:34] == "0" * 24
            value = state.get("0x" + data[-40:], 0)
            if self.advance and not self.advanced:
                self.send(self.finalized + 1, A, B, 10)
                self.head += 1
                self.advanced = True
        return hex(value) if self.short_word else "0x" + format(value, "064x")


@pytest.fixture
def peer():
    chain = TokenChain()
    errors = []

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_POST(self):
            try:
                assert self.headers["Content-Type"] == "application/json"
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                if chain.rpc_error and request["method"] == "eth_call":
                    reply = {"jsonrpc": "2.0", "id": request["id"],
                             "error": {"code": -32000, "message": "historical state unavailable"}}
                else:
                    reply = {"jsonrpc": "2.0", "id": request["id"], "result": chain.respond(request)}
                body = json.dumps(reply).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)
            except Exception as exc:
                errors.append(exc)
                self.send_error(500)

        def log_message(self, *args):
            pass

    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield chain, f"http://127.0.0.1:{server.server_port}"
    finally:
        server.shutdown()
        server.server_close()
        thread.join()
        assert not errors, errors


def write_source(path, events):
    with path.open("w", newline="") as stream:
        writer = csv.DictWriter(stream, fieldnames=FIELDS)
        writer.writeheader()
        for event in events:
            frm, to = ("0x" + topic[-40:] for topic in event["topics"][1:])
            writer.writerow(dict(zip(FIELDS, [event["transactionHash"], int(event["logIndex"], 16),
                                              frm, to, int(event["data"], 16), int(event["blockNumber"], 16)])))
    return str(path)


def export(url, sources, out):
    argv = ["--rpc", url, "--out", str(out)]
    for source in sources:
        argv.extend(["--source", source])
    snapshot.main(argv)


def balance_requests(chain):
    return [call for call in chain.calls if call["method"] == "eth_call"
            and call["params"][0]["data"].startswith("0x70a08231")]


def test_f007_pins_all_reads_despite_transfer_during_reconciliation(peer, tmp_path, capsys):
    chain, url = peer
    chain.send(1, ZERO, A, 100)
    chain.send(2, A, B, 40)
    chain.send(2005, B, C, 5)
    # A future explorer row must not contaminate the committed snapshot.
    future = dict(chain.events[1], blockNumber=hex(2006), transactionHash="0x" + "f" * 64)
    old = write_source(tmp_path / "old.csv", chain.events[:2])
    new = write_source(tmp_path / "new.csv", chain.events[1:] + [future])
    chain.advance = True
    out = tmp_path / "holders.json"
    export(url, [old, new], out)
    assert chain.advanced and chain.state(chain.head) == {A: 50, B: 45, C: 5}
    assert json.loads(out.read_text()) == [{"address": A, "balance": "60"},
                                           {"address": B, "balance": "35"},
                                           {"address": C, "balance": "5"}]
    calls = [call for call in chain.calls if call["method"] == "eth_call"]
    assert len(calls) == 4
    assert all(call["params"][1] == {"blockHash": block_hash(2005), "requireCanonical": True} for call in calls)
    assert calls[-1]["params"][0]["data"] == "0x18160ddd"
    ranges = [call["params"][0] for call in chain.calls if call["method"] == "eth_getLogs"]
    assert [(q["fromBlock"], q["toBlock"]) for q in ranges] == [("0x0", "0x7cf"), ("0x7d0", "0x7d5")]
    evidence = json.loads(Path(str(out) + ".evidence.json").read_text())
    assert evidence == {"chain_id": 125, "token": TOKEN,
                        "block": {"number": 2005, "hash": block_hash(2005)}, "commitment": "finalized",
                        "history_from_block": 0, "history_through_block": 2005, "transfer_count": 3,
                        "total_supply": "100", "holder_count": 3,
                        "holders_sha256": hashlib.sha256(out.read_bytes()).hexdigest()}
    assert capsys.readouterr().out == "holders 3 sum 100\n"


def test_f008_zero_history_candidate_is_read_and_stale_history_stops_export(peer, tmp_path):
    chain, url = peer
    chain.send(1, ZERO, A, 100)
    chain.send(2, A, B, 100)
    stale = write_source(tmp_path / "stale.csv", chain.events)
    chain.send(2005, B, A, 25)
    out = tmp_path / "holders.json"
    with pytest.raises(SystemExit, match="history is incomplete"):
        export(url, [stale], out)
    requests = balance_requests(chain)
    assert ["0x" + call["params"][0]["data"][-40:] for call in requests] == [A, B]
    assert chain.state(chain.finalized)[A] == 25
    assert not out.exists() and not Path(str(out) + ".evidence.json").exists()


def test_f008_complete_history_queries_zero_candidates_and_emits_each_holder_once(peer, tmp_path):
    chain, url = peer
    chain.send(1, ZERO, A, 100)
    chain.send(2, A, B, 100)
    chain.send(3, C, C, 0)
    chain.send(2005, B, A, 25)
    source = write_source(tmp_path / "complete.csv", chain.events)
    out = tmp_path / "holders.json"
    export(url, [source, source], out)
    assert json.loads(out.read_text()) == [{"address": A, "balance": "25"}, {"address": B, "balance": "75"}]
    assert ["0x" + call["params"][0]["data"][-40:] for call in balance_requests(chain)] == [A, B, C]


def test_f008_missing_holder_stops_even_when_rpc_reconciliation_matches_supply(peer, tmp_path):
    chain, url = peer
    chain.send(1, ZERO, A, 100)
    incomplete = write_source(tmp_path / "incomplete.csv", chain.events)
    chain.send(2005, ZERO, C, 10)
    out = tmp_path / "holders.json"
    with pytest.raises(SystemExit, match="history is incomplete"):
        export(url, [incomplete], out)
    assert ["0x" + call["params"][0]["data"][-40:] for call in balance_requests(chain)] == [A, C]
    assert sum(chain.state(chain.finalized).values()) == 110
    assert not out.exists()


def test_f007_supply_mismatch_preserves_existing_output(peer, tmp_path):
    chain, url = peer
    chain.send(1, ZERO, A, 100)
    source = write_source(tmp_path / "history.csv", chain.events)
    chain.supply_delta = 1
    out = tmp_path / "holders.json"
    out.write_text("previous qualified output\n")
    with pytest.raises(SystemExit, match="holder sum 100 does not match totalSupply 101"):
        export(url, [source], out)
    assert out.read_text() == "previous qualified output\n"
    assert not Path(str(out) + ".evidence.json").exists()


@pytest.mark.parametrize("failure, message", [("reorg", "pinned block changed"),
                                              ("no_finalized", "committed block unavailable"),
                                              ("rpc_error", "eth_call failed"),
                                              ("short_word", "invalid 32-byte hex"),
                                              ("removed_log", "invalid or removed")])
def test_f007_unavailable_or_uncommitted_evidence_fails_closed(peer, tmp_path, failure, message):
    chain, url = peer
    chain.send(1, ZERO, A, 100)
    source = write_source(tmp_path / "history.csv", chain.events)
    setattr(chain, failure, True)
    out = tmp_path / "holders.json"
    with pytest.raises(SystemExit, match=message):
        export(url, [source], out)
    assert not out.exists() and not Path(str(out) + ".evidence.json").exists()


def test_f008_conflicting_sources_are_rejected(peer, tmp_path):
    chain, url = peer
    event = chain.send(1, ZERO, A, 100)
    source = write_source(tmp_path / "history.csv", [event])
    conflict = write_source(tmp_path / "conflict.csv", [dict(event, data="0x" + format(101, "064x"))])
    out = tmp_path / "holders.json"
    with pytest.raises(SystemExit, match="sources disagree"):
        export(url, [source, conflict], out)
    assert not out.exists()


def test_f008_unbounded_legacy_csv_is_rejected(peer, tmp_path):
    _, url = peer
    source = tmp_path / "legacy.csv"
    source.write_text("transaction_hash,log_index,from_address,to_address,amount\n")
    out = tmp_path / "holders.json"
    with pytest.raises(SystemExit, match="source CSV requires.*block_number"):
        export(url, [str(source)], out)
    assert not out.exists()


def test_f007_burn_and_empty_supply_are_reconciled(peer, tmp_path):
    chain, url = peer
    chain.send(1, ZERO, A, 100)
    chain.send(2, A, ZERO, 100)
    source = write_source(tmp_path / "burned.csv", chain.events)
    out = tmp_path / "holders.json"
    export(url, [source], out)
    assert json.loads(out.read_text()) == []
    assert len(balance_requests(chain)) == 1
    assert json.loads(Path(str(out) + ".evidence.json").read_text())["total_supply"] == "0"
