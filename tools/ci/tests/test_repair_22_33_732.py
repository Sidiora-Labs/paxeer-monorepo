"""In-process wallet codec proofs and checks for required production wiring."""

from contextlib import redirect_stdout
import importlib.util
import io
import json
import re
import sys
import urllib.error
import urllib.request

import pytest

from test_wallet_check_live import (
    ACCOUNT_ADDRESS,
    ROOT,
    endpoint_server,
    network_answer,
    ramp_server,
    test_endpoint_fixture_refuses_retired_paths,
)


CHECKER = ROOT / "scripts/wallet/check-live.sh"
GATE = ROOT / "tools/ci/consensus_commit_gate.py"


def embedded_python(function):
    source = CHECKER.read_text(encoding="utf-8")
    body = source.split(f"\n{function}() {{\n", 1)[1].split("\n}\n", 1)[0]
    program = body.split("python3 -c '\n", 1)[1].split("\n' ", 1)[0]
    return compile(program, str(CHECKER) + ":" + function, "exec")


def judge(function, raw, *args):
    output = io.StringIO()
    saved_argv, saved_stdin = sys.argv, sys.stdin
    try:
        sys.argv, sys.stdin = [str(CHECKER), *args], io.StringIO(raw)
        with redirect_stdout(output):
            exec(embedded_python(function), {"__name__": "__main__"})
    finally:
        sys.argv, sys.stdin = saved_argv, saved_stdin
    return output.getvalue().strip()


def rpc(base, method, params, path="/"):
    document = {"jsonrpc": "2.0", "id": 1, "method": method, "params": params}
    request = urllib.request.Request(
        base + path,
        data=json.dumps(document).encode(),
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(request, timeout=2) as response:
        assert response.status == 200
        assert response.headers.get_content_type() == "application/json"
        answer = response.read().decode()
    decoded = json.loads(answer)
    assert decoded["jsonrpc"] == "2.0"
    assert decoded["id"] == document["id"]
    return answer


def account_answer(bound=True, height=42):
    return {
        "result": {
            "account": {"bound": bound, "evm_address": ACCOUNT_ADDRESS.lower()},
            "paxeer": {
                "address": ACCOUNT_ADDRESS.upper(),
                "balance": "0x1aF",
                "nonce": "0x0",
                "height": height,
            },
        }
    }


def recorded_consensus_gate():
    spec = importlib.util.spec_from_file_location("repair_consensus_gate", GATE)
    gate = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(gate)
    recording = json.loads(
        (ROOT / "tools/ci/tests/fixtures/consensus-history-22-33-731.json").read_text(
            encoding="utf-8"
        )
    )
    assert recording["producer"].startswith("git version ")
    assert tuple(recording["history_args"]) == gate.HISTORY_ARGS
    return gate, recording["cases"]


@pytest.mark.parametrize(
    "name", ["merge_only", "resolved_merge", "merge_deletion", "octopus_merge"]
)
def test_finding_011_merge_inventory_includes_merge_revisions(name):
    gate, cases = recorded_consensus_gate()
    case = cases[name]
    assert "--no-merges" not in gate.HISTORY_ARGS
    assert "-c" in gate.HISTORY_ARGS
    assert gate.consensus_changes_from_history(case["history"])[case["ids"]["merge"]] == [
        "node/sequencer/rule.go"
    ]
    assert gate.unlisted_from_history(case["ledger"], case["history"]) == {
        case["ids"]["merge"]: ["node/sequencer/rule.go"]
    }


@pytest.mark.parametrize(
    "before_name,after_name",
    [("merge_only", "merge_listed"), ("resolved_merge", "resolved_merge_listed")],
)
def test_finding_011_merge_approval_is_pinned_to_recorded_head(
    tmp_path, before_name, after_name
):
    gate, cases = recorded_consensus_gate()
    before, after = cases[before_name], cases[after_name]
    merge = before["ids"]["merge"]
    assert merge == after["ids"]["merge"]
    expected = {merge: ["node/sequencer/rule.go"]}
    working_ledger = tmp_path / "consensus-commits.md"
    working_ledger.write_text(after["ledger"], encoding="utf-8")
    assert merge in gate.listed_commits(working_ledger)
    assert gate.unlisted_from_history(before["ledger"], before["history"]) == expected
    assert gate.unlisted_from_history(after["ledger"], after["history"]) == {}
    working_ledger.write_text(before["ledger"], encoding="utf-8")
    assert merge not in gate.listed_commits(working_ledger)
    assert gate.unlisted_from_history(after["ledger"], after["history"]) == {}
    assert gate.unlisted_from_history(before["ledger"], before["history"]) == expected


def test_finding_011_merge_approval_keeps_unlisted_parent_refused():
    gate, cases = recorded_consensus_gate()
    case = cases["unlisted_parent"]
    assert case["ids"]["merge"] in gate._table_commits(case["ledger"])
    assert gate.unlisted_from_history(case["ledger"], case["history"]) == {
        case["ids"]["left"]: ["node/sequencer/rule.go"]
    }


@pytest.mark.parametrize("name", ["clean_merge", "inherited_resolution"])
def test_finding_011_parent_covered_merge_keeps_existing_approvals(name):
    gate, cases = recorded_consensus_gate()
    case = cases[name]
    listed = gate._table_commits(case["ledger"])
    assert {case["ids"]["left"], case["ids"]["right"]} <= listed
    assert case["ids"]["merge"] not in listed
    assert gate.unlisted_from_history(case["ledger"], case["history"]) == {}


def test_finding_011_real_release_ledger_parser_preserves_merge_hash(tmp_path):
    spec = importlib.util.spec_from_file_location("repair_consensus_gate", GATE)
    gate = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(gate)
    merge = "1234567890abcdef1234567890abcdef12345678"
    path = tmp_path / "consensus-commits.md"
    path.write_text(
        "| Full commit hash | State-machine change | Non-test consensus paths | Upgrade |\n"
        "| --- | --- | --- | --- |\n"
        f"| `{merge}` | Merge-only rule | `node/app.go` | V6120Upgrade |\n",
        encoding="utf-8",
    )
    assert gate.listed_commits(path) == {merge}
    path.write_text("| --- | --- | --- | --- |\n", encoding="utf-8")
    assert gate.listed_commits(path) == set()


@pytest.mark.parametrize("consensus_available", [True, False])
@pytest.mark.parametrize("bound,height", [(True, 42), (False, 0)])
def test_finding_012_network_and_account_real_codecs(consensus_available, bound, height):
    with endpoint_server(account_answer(bound, height), consensus_available) as (base, requests):
        for method, params, expected in (
            ("eth_chainId", [], "pass 0x7d"),
            ("px_resolveAccount", [ACCOUNT_ADDRESS], "pass bound=true"),
            ("px_getNetwork", [], "pass consensus=" + ("42" if consensus_available else "unavailable")),
            ("px_getAccount", [ACCOUNT_ADDRESS], f"pass bound={str(bound).lower()} height={height}"),
        ):
            raw = rpc(base, method, params)
            assert judge("judge", raw, method, ACCOUNT_ADDRESS) == expected
    assert [path for path, _, _ in requests] == ["/"] * 4
    assert [request["params"] for _, request, _ in requests] == [
        [], [ACCOUNT_ADDRESS], [], [ACCOUNT_ADDRESS]
    ]
    assert all(headers["content-type"] == "application/json" for _, _, headers in requests)
    result = network_answer(consensus_available)["result"]
    assert set(result["paxeer"]) == {"chain_id", "latest_block"}
    assert result["network_id"]
    if consensus_available:
        assert set(result["consensus"]) == {
            "latest_height", "latest_block_hash", "latest_app_hash", "catching_up"
        }
        assert "reason" not in result
    else:
        assert result["consensus"] is None
        assert result["reason"] == "consensus_unavailable"


@pytest.mark.parametrize("result", [
    {"kernel": {"available": False, "reason": "not_configured"}},
    {"paxeer": {"chain_id": "0x7d"}, "consensus": None},
    {"paxeer": {"chain_id": "0x7e"}, "consensus": {}},
])
def test_finding_012_network_refuses_retired_or_incomplete_answers(result):
    raw = json.dumps({"jsonrpc": "2.0", "id": 1, "result": result})
    assert judge("judge", raw, "px_getNetwork", ACCOUNT_ADDRESS).startswith("fail ")


@pytest.mark.parametrize("code,data", [
    (-32001, {"code": "native_account_reverse_resolution_unsupported"}),
    (-32001, {"code": "paxeer_rpc_not_configured"}),
    (-32010, {"code": "kernel_unavailable", "backend": "kernel", "reason": "unconfigured"}),
])
def test_finding_012_account_read_refusals_remain_failures(code, data):
    answer = {"error": {"code": code, "message": "Paxeer read unavailable", "data": data}}
    with endpoint_server(answer) as (base, requests):
        verdict = judge("judge", rpc(base, "px_getAccount", [ACCOUNT_ADDRESS]), "px_getAccount", ACCOUNT_ADDRESS)
    assert verdict.startswith("fail ")
    assert f'"code":{code}' in verdict
    assert data["code"] in verdict
    assert requests[0][1]["params"] == [ACCOUNT_ADDRESS]


@pytest.mark.parametrize("field,value", [
    ("balance", "0x00"), ("nonce", "0x"), ("height", -1),
    ("height", True), ("address", "0x" + "0" * 40),
])
def test_finding_012_malformed_accounts_are_refused(field, value):
    answer = account_answer()
    answer["result"]["paxeer"][field] = value
    with endpoint_server(answer) as (base, _):
        verdict = judge("judge", rpc(base, "px_getAccount", [ACCOUNT_ADDRESS]), "px_getAccount", ACCOUNT_ADDRESS)
    assert verdict.startswith("fail ")


def test_finding_014_usage_and_ramp_dispatch():
    source = CHECKER.read_text(encoding="utf-8")
    usage = source.split("cat <<'EOF'\n", 1)[1].split("\nEOF", 1)[0]
    assert usage.splitlines()[0] == (
        "usage: scripts/wallet/check-live.sh endpoint | attestors | gateway | "
        "cutover | human | human-session | ramp"
    )
    assert 'if [ "$mode" = ramp ]; then\n\tramp\n\texit $?\nfi' in source
    assert '*)\n\tusage >&2\n\texit 2\n\t;;' in source
    assert not re.search(r'\blx_[A-Za-z0-9_]*', source)
    with ramp_server() as (base, requests):
        for path in ("/readyz", "/internal/v1/work", "/internal/v1/rebalances"):
            request = urllib.request.Request(
                base + path,
                data=None if path == "/readyz" else b"{}",
                headers={"Cache-Control": "no-cache"} if path == "/readyz" else {"Content-Type": "application/json"},
            )
            try:
                response = urllib.request.urlopen(request, timeout=2)
            except urllib.error.HTTPError as refusal:
                response = refusal
            with response:
                raw = response.read().decode()
                if path != "/readyz":
                    raw += "\n" + str(response.status)
            assert judge("ramp", raw, path) == "pass ramp " + path
    assert [(method, path, body) for method, path, body, _ in requests] == [
        ("GET", "/readyz", None),
        ("POST", "/internal/v1/work", {}),
        ("POST", "/internal/v1/rebalances", {}),
    ]
    assert all("authorization" not in headers for _, _, _, headers in requests)


@pytest.mark.parametrize("scope,status", [("external-settlement", 401), ("local-journal", 200)])
def test_finding_014_ramp_rejects_wrong_scope_or_successful_unauthenticated_work(scope, status):
    with ramp_server(scope, status) as (base, _):
        path = "/readyz" if scope != "local-journal" else "/internal/v1/work"
        request = urllib.request.Request(base + path, data=None if path == "/readyz" else b"{}")
        with urllib.request.urlopen(request, timeout=2) as response:
            raw = response.read().decode()
            if path != "/readyz":
                raw += "\n" + str(response.status)
        with pytest.raises(SystemExit, match="fail ramp " + path):
            judge("ramp", raw, path)
