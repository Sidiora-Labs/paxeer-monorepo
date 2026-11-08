"""Offline source and embedded-consumer proof for audit 22.30.171/015."""

import ast
from contextlib import redirect_stdout
import io
import json
from pathlib import Path
import re
import shlex
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
SHELL_PATH = ROOT / "tests/wallet/check-live.test.sh"
SHELL = SHELL_PATH.read_text(encoding="utf-8")
CHECKER = (ROOT / "scripts/wallet/check-live.sh").read_text(encoding="utf-8")
RESPONDER = SHELL.split('cat >"$work/responder.mjs" <<\'JS\'\n', 1)[1].split("\nJS\n", 1)[0]


def fixture_object(name):
    """Decode the responder's literal data, rejecting non-JSON expressions."""
    literal = RESPONDER.split(f"const {name} = ", 1)[1].split("\n};", 1)[0] + "\n}"
    literal = re.sub(r'(?m)(^\s*)([A-Za-z_][A-Za-z_0-9]*):', r'\1"\2":', literal)
    literal = re.sub(r'([{,]\s*)([A-Za-z_][A-Za-z_0-9]*):', r'\1"\2":', literal)
    literal = re.sub(r",\s*([}\]])", r"\1", literal)
    return json.loads(literal)


def endpoint_cases():
    cases = {}
    for line in SHELL.replace("\\\n", "").splitlines():
        if line.strip().startswith("expect check_live_"):
            command, name, profile, status, *lines = shlex.split(line)
            assert command == "expect"
            assert name not in cases
            cases[name] = (profile, int(status), lines)
    return cases


def embedded_judge():
    """Run the actual Python consumer with explicit input instead of CLI stdin."""
    program = CHECKER.split("\njudge() {", 1)[1].split("verdict=\"$(printf '%s' \"$body\" | python3 -c '\n", 1)[1]
    program = program.split("\n' \"$check\" \"$account_address\")\"", 1)[0]
    tree = ast.parse(program)
    inputs = {
        "check": "sys.argv[1]",
        "account_address": "sys.argv[2].lower()",
        "raw": "sys.stdin.read()",
    }
    body = []
    found = set()
    for node in tree.body:
        if isinstance(node, ast.Assign) and len(node.targets) == 1:
            target = node.targets[0]
            if isinstance(target, ast.Name) and target.id in inputs:
                assert ast.unparse(node.value) == inputs[target.id]
                found.add(target.id)
                continue
        body.append(node)
    assert found == set(inputs)
    function = ast.FunctionDef(
        name="consume",
        args=ast.arguments(
            posonlyargs=[], args=[ast.arg(arg=name) for name in inputs],
            kwonlyargs=[], kw_defaults=[], defaults=[],
        ),
        body=body, decorator_list=[],
    )
    module = ast.fix_missing_locations(ast.Module(body=[function], type_ignores=[]))
    namespace = {}
    exec(compile(module, "scripts/wallet/check-live.sh:judge", "exec"), namespace)
    return namespace["consume"]


def test_015_shell_syntax_and_guarded_root_transport():
    syntax = subprocess.run(
        ["bash", "-n", str(SHELL_PATH)], capture_output=True, text=True, check=False,
    )
    assert syntax.returncode == 0, syntax.stderr
    endpoint = RESPONDER.split("for (const profile of Object.keys(profiles)) {", 1)[1]
    guard = 'if (request.method !== "POST" || request.url !== "/") {'
    assert guard in endpoint
    refusal = endpoint.split(guard, 1)[1].split('  let body = "";', 1)[0]
    assert 'response.writeHead(404, { "content-type": "application/json" });' in refusal
    assert "return;" in refusal
    assert endpoint.index(guard) < endpoint.index('request.on("data"')
    assert 'request.on("end", () => {' in endpoint
    assert "call = JSON.parse(body);" in endpoint
    assert "profiles[profile][call.method] || base[call.method]" in endpoint
    assert 'JSON.stringify({ jsonrpc: "2.0", id: call.id, ...answer })' in endpoint
    assert 'endpoint.listen(0, "127.0.0.1"' in endpoint
    assert "ports.endpoints[profile] = endpoint.address().port;" in endpoint
    assert 'rest !== "/rpc"' not in RESPONDER
    assert 'request.url.match(/^\\/([a-z_]+)(\\/.*)$/)' not in RESPONDER
    for fact in (
        'const ports = { services: null, endpoints: {} };',
        'ports.services !== null && Object.keys(ports.endpoints).length === Object.keys(profiles).length',
        'fs.writeFileSync(portFile, JSON.stringify(ports));',
        'ports.services = server.address().port;',
        'print(json.load(open(sys.argv[1]))["services"])',
        'print(json.load(open(sys.argv[1]))["endpoints"][sys.argv[2]])',
        "printf 'http://127.0.0.1:%s/' \"$endpoint_port\"",
        'run_checker "$(endpoint_base "$profile")" endpoint',
        'CHECK_LIVE_ENDPOINT_BASE="$1" CHECK_LIVE_TIMEOUT=5 "$checker" "${@:2}" 2>&1',
        '[ "$status" -eq "$want_status" ] || ok=0',
        'grep -qF -- "$line" <<<"$output" || ok=0',
        'failures=$((failures + 1))',
        'if [ "$failures" -ne 0 ]; then',
    ):
        assert fact in SHELL, fact
    root = (ROOT / "rpc/rpcstack.go").read_text(encoding="utf-8")
    assert 'return r.URL.Path == "/"' in root


def test_015_network_fixture_matches_retained_producer_and_consumer():
    base = fixture_object("base")
    profiles = fixture_object("profiles")
    network = base["px_getNetwork"]["result"]
    assert network == {
        "network_id": "local",
        "paxeer": {"chain_id": "0x7d", "latest_block": "0x10"},
        "consensus": None, "reason": "consensus_unavailable",
    }
    assert profiles["network_consensus"]["px_getNetwork"]["result"]["consensus"] == {
        "latest_height": "9", "catching_up": False,
    }
    assert profiles["wrong_chain"]["eth_chainId"] == {"result": "0x1"}
    assert profiles["network_wrong_chain"]["px_getNetwork"]["result"]["paxeer"]["chain_id"] == "0x1"
    assert "paxeer" not in profiles["network_malformed"]["px_getNetwork"]["result"]
    assert profiles["network_wrong_reason"]["px_getNetwork"]["result"]["reason"] == "unknown"
    assert profiles["account_read_refused"]["px_getAccount"]["error"] == {
        "code": -32001, "message": "Native account reverse resolution unsupported",
        "data": {"code": "native_account_reverse_resolution_unsupported"},
    }
    producer = (ROOT / "rpc/px_account.go").read_text(encoding="utf-8")
    for fact in (
        '"network_id": sdkCtx.ChainID()',
        '"chain_id": hexutil.EncodeBig(a.state.keeper.ChainID(sdkCtx))',
        '"latest_block": hexutil.EncodeUint64(uint64(height))',
        '"consensus": nil', 'result["reason"] = "consensus_unavailable"',
        '"latest_height": new(big.Int).SetInt64(status.SyncInfo.LatestBlockHeight).String()',
        '"catching_up": status.SyncInfo.CatchingUp',
    ):
        assert fact in producer, fact


@pytest.mark.parametrize("name", [
    "check_live_passing_endpoint", "check_live_consensus_endpoint",
    "check_live_network_malformed", "check_live_wrong_chain_id",
    "check_live_network_wrong_chain", "check_live_network_wrong_reason",
    "check_live_account_read_refused", "check_live_resolve_read_error",
])
def test_015_existing_endpoint_cases_reach_real_embedded_consumer(name):
    profile, status, expected = endpoint_cases()[name]
    profiles, base = fixture_object("profiles"), fixture_object("base")
    consume = embedded_judge()
    lines = []
    for method in ("eth_chainId", "px_resolveAccount", "px_getNetwork", "px_getAccount"):
        assert re.search(r'(?m)^\s*run node "\$node_rpc_url" ' + re.escape(method) + r"[ \t]", CHECKER)
        answer = profiles[profile].get(method, base[method])
        wire = json.dumps({"jsonrpc": "2.0", "id": 1, **answer})
        output = io.StringIO()
        with redirect_stdout(output):
            consume(method, "0x000000000000000000000000000000000000dead", wire)
        verdict, detail = output.getvalue().strip().split(" ", 1)
        assert verdict in ("pass", "fail")
        lines.append(f"{verdict} {method} {detail}")
    failures = sum(line.startswith("fail ") for line in lines)
    assert status == (1 if failures else 0)
    lines.append(f"check-live: {failures} check(s) failed" if failures else "check-live: all checks passed")
    actual = "\n".join(lines)
    for line in expected:
        assert line in actual, (name, line, actual)
    assert expected[-1] == lines[-1]


def test_015_record_maps_proof_and_out_of_scope_wiring():
    record = tomllib.loads(Path(__file__).with_name("repair_22_33_733.kvx").read_text())
    finding = record["finding"]["015"]
    assert finding["audit"] == "22.30.171"
    assert finding["file"] == "tests/wallet/check-live.test.sh"
    assert finding["class"] == "retired-reference"
    assert set(finding["tests"]) == {
        "test_015_shell_syntax_and_guarded_root_transport",
        "test_015_network_fixture_matches_retained_producer_and_consumer",
        "test_015_existing_endpoint_cases_reach_real_embedded_consumer",
    }
    for name in finding["tests"]:
        assert callable(globals()[name])
    task = record["task"]["22"]["33"]["733"]
    assert task["base_revision"] == "76c050242a1edcad6212aa52fac15670ecb5612d"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_733.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "scripts/wallet/check-live.sh" in task["wiring_request"]
    assert "domain 16" in task["wiring_request"]
    assert "HTTP" in task["review_only"]
    assert "No baseline failures are claimed" in task["baseline"]
