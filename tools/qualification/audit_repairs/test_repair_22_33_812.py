"""Wallet audit source proofs and production shell result-consumer regressions.

No Go, Rust or Node toolchain, credentials or live service is required. Formatter
results are transcript inputs to the production result consumer; actual gofmt
parsing is a review obligation, as recorded in the accompanying KVX.
"""

import ast
from contextlib import redirect_stdout
import io
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys
import tempfile
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
WALLET = ROOT / "scripts/wallet"
RECORD = Path(__file__).with_name("repair_22_33_812.kvx")
SCRIPTS = ("check-deploy.sh", "check-live.sh", "gate-lint.sh", "scan-secrets.sh")
ACCOUNT = "0x000000000000000000000000000000000000dEaD"
MALFORMED_GO = "package fixture\nfunc broken( {\n"
FORMATTER_FAILURE = {
    "source": MALFORMED_GO,
    "stdout": "",
    "stderr": "broken.go:2:14: expected ')', found '{'\n",
    "exit": 2,
}


def source(name):
    return (WALLET / name).read_text(encoding="utf-8")


def bash(program, *args, cwd=ROOT, env=None, **kwargs):
    return subprocess.run(
        ["bash", "-c", program, "wallet-proof", *map(str, args)],
        cwd=cwd,
        env=env,
        capture_output=True,
        text=True,
        timeout=15,
        check=False,
        **kwargs,
    )


def gofmt_parts():
    command = re.search(r"^GOFMT_CMD='(.*?)'\nPNPM_ESLINT_CMD=", source("gate-lint.sh"), re.M | re.S)
    assert command is not None
    consumer, invocation = command[1].split('; out="$(gofmt -l .)"; ')
    return command[1], consumer, invocation


@pytest.mark.parametrize("name", SCRIPTS)
def test_wallet_shell_syntax(name):
    checked = subprocess.run(
        ["bash", "-n", str(WALLET / name)], capture_output=True, text=True, check=False
    )
    assert checked.returncode == 0, checked.stderr


@pytest.mark.parametrize(
    "regions,passes",
    [
        (("iad", "ord", "sjc", "sea", "gru"), True),
        (("sin", "nrt", "hkg", "syd", "jnb"), True),
        (("iad", "ord", "gru", "jnb", "fra"), True),
        (("iad", "ord", "sjc", "sea", "yyz"), False),
        (("sin", "nrt", "hkg", "syd", "bom"), False),
        (("iad", "ord", "gru", "jnb", "unknown"), False),
    ],
)
def test_58_deployment_diversity_real_validator(tmp_path, monkeypatch, regions, passes):
    """Run the complete embedded production validator against parsed TOML inputs."""
    (tmp_path / "env").write_text("LOG_LEVEL - logging verbosity\n")
    for index, region in enumerate(regions, 1):
        (tmp_path / f"attestor-{index}.toml").write_text(
            f'primary_region = "{region}"\n'
            '[build]\nimage = "registry.example/attestor:fixture"\n'
            '[env]\nLOG_LEVEL = "info"\n'
            '[[mounts]]\nsource = "state"\ndestination = "/state"\n'
            '[[services]]\ninternal_port = 9000\nmin_machines_running = 1\n'
            'auto_start_machines = false\nauto_stop_machines = false\n'
            '[[services.http_checks]]\npath = "/health"\n'
        )
    for name in ("gateway", "endpoint"):
        (tmp_path / f"{name}.toml").write_text(
            '[build]\nimage = "registry.example/public:fixture"\n'
            '[http_service]\ninternal_port = 8080\nforce_https = true\n'
            '[[http_service.checks]]\npath = "/health"\n'
        )
    code = source("check-deploy.sh").split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]
    monkeypatch.setattr(sys, "argv", ["check-deploy", str(tmp_path), str(ROOT)])
    output = io.StringIO()
    with redirect_stdout(output), pytest.raises(SystemExit) as result:
        exec(compile(code, str(WALLET / "check-deploy.sh"), "exec"), {})
    assert result.value.code == (0 if passes else 1), output.getvalue()
    failed = [line for line in output.getvalue().splitlines() if line.startswith("fail ")]
    assert len(failed) == (0 if passes else 1), output.getvalue()
    if not passes:
        assert failed[0].startswith("fail continents ")


def test_58_provider_location_inventory():
    code = source("check-deploy.sh").split("<<'PY'\n", 1)[1].rsplit("\nPY", 1)[0]
    assignment = next(
        node for node in ast.parse(code).body
        if isinstance(node, ast.Assign) and any(
            isinstance(target, ast.Name) and target.id == "CONTINENTS" for target in node.targets
        )
    )
    groups = ast.literal_eval(assignment.value)
    assert groups["south-america"] == {"gru"}
    assert groups["africa"] == {"jnb"}
    assert "gru" not in groups["north-america"]
    assert "jnb" not in groups["asia-pacific"]
    assert len(set().union(*groups.values())) == sum(map(len, groups.values()))
    assert "https://fly.io/docs/reference/regions/" in source("check-deploy.sh")


def test_57_rpc_owner_routing_source():
    script = source("check-live.sh")
    endpoint = script[script.index('node_rpc_url="${CHECK_LIVE_NODE_RPC_URL-https://api1.mainnet-beta.paxeer.network/}"'):]
    assert "endpoint mode does not call it" in script
    assert "CHECK_LIVE_HOSTED_RPC_URL" not in endpoint
    assert "CHECK_LIVE_ENDPOINT_BASE" not in script
    assert 'url.path in ("", "/")' in endpoint
    assert "raise SystemExit" in endpoint.split("failures=0", 1)[0]
    assert '"$base/rpc"' not in script
    call = endpoint.split("call() {\n", 1)[1].split("\n}\n", 1)[0]
    assert 'local url="$1" method="$2" params="$3"' in call
    assert call.endswith('"$url" 2>&1')
    assert '"$url/' not in call
    routes = [shlex.split(line) for line in endpoint.splitlines() if line.startswith("run ")]
    assert [(row[1], row[2], row[3]) for row in routes] == [
        ("node", "$node_rpc_url", "eth_chainId"),
        ("node", "$node_rpc_url", "px_resolveAccount"),
        ("node", "$node_rpc_url", "px_getNetwork"),
        ("node", "$node_rpc_url", "px_getAccount"),
    ]
    assert routes[0][4] == routes[2][4] == "[]"
    assert routes[1][4] == routes[3][4] == '["$account_address"]'
    assert 'body="$(call "$url" "$check" "$params")" || status=$?' in endpoint
    assert 'judge "$check" "$status" "$body" "$owner" "$url"' in endpoint


@pytest.mark.parametrize("method,result", [
    ("eth_chainId", "0x7d"),
    ("px_resolveAccount", {"bound": False}),
    ("px_getNetwork", {"paxeer": {"chain_id": "0x7d"}, "consensus": {"latest_height": "42"}}),
    ("px_getNetwork", {"paxeer": {"chain_id": "0x7d"}, "consensus": None, "reason": "consensus_unavailable"}),
    ("px_getAccount", {
        "account": {"bound": False, "evm_address": ACCOUNT.lower()},
        "paxeer": {"address": ACCOUNT, "balance": "0x0", "nonce": "0x0", "height": 0},
    }),
])
def test_57_real_response_consumer_reports_owner(method, result):
    """Drive the unchanged response decoder with fixture JSON-RPC documents."""
    script = source("check-live.sh")
    judge = "judge() {\n" + script.split("judge() {\n", 1)[1].split("\n}\n", 1)[0] + "\n}\n"
    owner = "node" if method == "eth_chainId" else "hosted"
    url = f"https://{owner}.example/"
    for status, body, expected in (
        (0, {"jsonrpc": "2.0", "id": 1, "result": result}, "pass"),
        (0, {"jsonrpc": "2.0", "id": 1, "error": {"code": -32601, "message": "Method not found"}}, "fail"),
        (7, "connection refused", "fail"),
        (0, "not-json", "fail"),
    ):
        raw = json.dumps(body) if isinstance(body, dict) else body
        checked = bash(
            'account_address="$1"; failures=0\n' + judge
            + '\njudge "$2" "$3" "$4" "$5" "$6"\nprintf "failures=%s\\n" "$failures"',
            ACCOUNT, method, status, raw, owner, url,
        )
        assert checked.returncode == 0, checked.stderr
        assert checked.stdout.startswith(f"{expected} {method} owner={owner} url={url} ")
        assert checked.stdout.splitlines()[-1] == f"failures={int(expected == 'fail')}"


def test_59_gofmt_producer_checks_status_and_logs_stderr():
    command, consumer, invocation = gofmt_parts()
    assert invocation == 'status=$?; gofmt_result "$status" "$out"'
    assert consumer.index('if [ "$status" -ne 0 ]; then') < consumer.index('if [ -n "$out" ]; then')
    assert 'return "$status"' in consumer
    assert 'return 1' in consumer
    assert "2>/dev/null" not in command and "2>&1" not in command
    script = source("gate-lint.sh")
    assert script.count('"$GOFMT_CMD" go.mod') == 2
    assert '(cd "$dir" && timeout --kill-after=10 "$remaining" bash -c "${CMDS[$i]}") 2>&1 | tee "$log"' in script
    assert 'rc=${PIPESTATUS[0]}' in script


@pytest.mark.parametrize("status,out,expected", [(0, "", 0), (0, "bad.go", 1), (2, "", 2), (2, "bad.go", 2)])
def test_59_real_gofmt_result_consumer(status, out, expected):
    _, consumer, _ = gofmt_parts()
    checked = bash(consumer + '\ngofmt_result "$1" "$2"', status, out)
    assert checked.returncode == expected, checked.stdout + checked.stderr
    assert checked.stdout == ("unformatted:\n" + out + "\n" if status == 0 and out else "")


def test_59_malformed_go_transcript_fails_real_target_gate(tmp_path):
    """Replay a parse-error result through the production consumer and gate logger."""
    assert FORMATTER_FAILURE["source"] == "package fixture\nfunc broken( {\n"
    _, consumer, _ = gofmt_parts()
    # Target files are tab-delimited; retain the production function as shell
    # tokens on one line, without introducing a replacement formatter.
    assert "\n" not in consumer
    one_line = consumer
    command = (
        one_line + "; printf '%s' " + shlex.quote(FORMATTER_FAILURE["stderr"]) + " >&2; "
        + "gofmt_result " + str(FORMATTER_FAILURE["exit"]) + " " + shlex.quote(FORMATTER_FAILURE["stdout"])
    )
    # Encode the diagnostic newline in printf, so the target remains one line.
    command = command.replace(shlex.quote(FORMATTER_FAILURE["stderr"]), shlex.quote(FORMATTER_FAILURE["stderr"].rstrip("\n")))
    targets = tmp_path / "targets"
    targets.write_text(f"malformed-go\t{tmp_path}\t{command}\n")
    (tmp_path / "broken.go").write_text(FORMATTER_FAILURE["source"])
    env = {**os.environ, "WALLET_GATE_TARGETS_FILE": str(targets), "WALLET_GATE_LOG_DIR": str(tmp_path / "logs")}
    checked = bash('exec bash "$1"', WALLET / "gate-lint.sh", env=env)
    assert checked.returncode == 1, checked.stdout + checked.stderr
    assert "failed: malformed-go:" in checked.stdout
    assert "exit code 2" in checked.stdout
    assert "summary: ran=1 passed=0 failed=1 absent=0 result=FAIL" in checked.stdout
    assert (tmp_path / "logs/malformed-go.log").read_text() == FORMATTER_FAILURE["stderr"].rstrip("\n")


def scan(paths, allow, **kwargs):
    return bash(
        'exec bash "$1" "${@:2}"', WALLET / "scan-secrets.sh", *paths,
        env={**os.environ, "SCAN_SECRETS_ALLOW": str(allow)}, **kwargs,
    )


def test_60_scanner_distinguishes_clean_findings_and_allowlist_errors(tmp_path):
    selected = tmp_path / "selected.txt"
    allow = tmp_path / "allow"
    selected.write_text("ordinary non-secret content\n")
    clean = scan([selected], allow)
    assert clean.returncode == 0, clean.stdout + clean.stderr
    assert "scanned 1 files, 0 findings" in clean.stdout
    selected.write_text("-----BEGIN PRIVATE KEY-----\n")
    found = scan([selected], allow)
    assert found.returncode == 1, found.stdout + found.stderr
    assert f"{selected}:1: pem-private-key" in found.stdout
    allow.write_text("selected.txt:PRIVATE KEY\n")
    allowed = scan([selected], allow)
    assert allowed.returncode == 0, allowed.stdout + allowed.stderr
    allow.write_text("selected.txt:[\n")
    error = scan([selected], allow)
    assert error.returncode == 2, error.stdout + error.stderr
    assert "allowlist error" in error.stderr
    assert str(selected) in error.stderr
    assert "incomplete scan" in error.stderr
    assert "grep:" in error.stderr


def test_60_unreadable_file_fails_even_with_other_matches():
    # Root can read mode-000 files; drop only the child's DAC-bypass privilege.
    with tempfile.TemporaryDirectory(prefix="wallet-scan-") as work:
        directory = Path(work)
        directory.chmod(0o755)
        unreadable = directory / "unreadable input.txt"
        unreadable.write_text("ordinary content\n")
        unreadable.chmod(0)
        readable = directory / "readable.txt"
        readable.write_text("-----BEGIN PRIVATE KEY-----\n")
        readable.chmod(0o644)

        def unprivileged():
            if os.geteuid() == 0:
                os.setgroups([])
                os.setgid(65534)
                os.setuid(65534)

        for paths, findings in (([unreadable], "0 findings"), ([unreadable, readable], "1 findings")):
            checked = scan(paths, directory / "no-allowlist", preexec_fn=unprivileged)
            assert checked.returncode == 2, checked.stdout + checked.stderr
            assert "Permission denied" in checked.stderr
            assert f"could not inspect {unreadable}" in checked.stderr
            assert "incomplete scan; affected paths:" in checked.stderr
            assert checked.stderr.splitlines()[-1] == str(unreadable)
            assert findings in checked.stdout
            if len(paths) == 2:
                assert f"{readable}:1: pem-private-key" in checked.stdout


def test_60_error_guards_source():
    script = source("scan-secrets.sh")
    assert 'grep "${opts[@]}" -e "$re" -- "$file" > "$work/matches" || status=$?' in script
    assert 'case "$status" in\n      0|1) ;;' in script
    assert 'printf \'%s\\n\' "$file" >> "$work/errors"' in script
    assert "xargs -0 grep" not in script
    assert "2>/dev/null" not in script and "|| true" not in script
    assert script.index('if [ -s "$work/errors" ]; then') < script.index('[ "$found" -eq 0 ]')


def test_repair_record_maps_every_finding():
    record = tomllib.loads(RECORD.read_text())
    assert set(record["finding"]) == {"57", "58", "59", "60"}
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef)}
    for finding in record["finding"].values():
        assert finding["tests"]
        assert set(finding["tests"]) <= declared
    assert record["task"]["baseline_tests_run"] is False
    assert record["task"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "tools/ci/tests/test_wallet_check_live.py" in record["task"]["wiring_requests"][0]
