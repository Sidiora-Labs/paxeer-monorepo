"""Offline source-contract proof for the three EVM script audit findings.

Only bash -n parses shell; no cluster, CLI, npm, Go or RPC is executed.
Runtime consensus and signed setup remain integration-review obligations.
"""

import ast
import json
from pathlib import Path
import re
import shlex
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPTS = ROOT / "tests/integration/evm_module/scripts"
SCRIPT_NAMES = ("evm_giga_mixed_tests.sh", "evm_giga_tests.sh", "evm_rpc_tests.sh")
RECORD = Path(__file__).with_name("repair_22_33_744.kvx")


def source(name):
    return (SCRIPTS / name).read_text(encoding="utf-8")


def function_body(text, name):
    match = re.search(r"^" + re.escape(name) + r"\(\) \{\n(.*?)^\}\n", text, re.M | re.S)
    assert match, f"missing shell function {name}"
    return match.group(1)


def ordered(text, *facts):
    offset = 0
    for fact in facts:
        found = text.find(fact, offset)
        assert found >= 0, f"missing or out-of-order source fact: {fact}"
        offset = found + len(fact)


def commands(text):
    """Tokenize logical shell lines without evaluating substitutions or commands."""
    return [
        shlex.split(line, comments=True)
        for line in text.replace("\\\n", " ").splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    ]


@pytest.mark.parametrize("name", SCRIPT_NAMES)
def test_repaired_shell_syntax(name):
    result = subprocess.run(
        ["bash", "-n", str(SCRIPTS / name)],
        capture_output=True,
        text=True,
        check=False,
        timeout=10,
    )
    assert result.returncode == 0, result.stderr


def test_giga_mixed_final_agreement_all_validators_progress_at_one_height():
    text = source(SCRIPT_NAMES[0])
    assert "set -euo pipefail" in text
    body = function_body(text, "check_mixed_final_agreement")
    assert body.count("for node in 0 1 2 3; do") == 2
    ordered(
        body,
        'height=$(validator_height "$node")',
        "baseline[node]=$height",
        'if [ "$height" -gt "$target" ]; then target=$height; fi',
        "target=$((target + 2))",
        "deadline=$((SECONDS + 120))",
        'height=$(validator_height "$node")',
        '[ "$height" -ge "$target" ] && [ "$height" -gt "${baseline[node]}" ]',
        '[ "$SECONDS" -ge "$deadline" ]',
        "return 1",
        "sleep 2",
        'validator_rpc "$node" "block?height=$target"',
    )
    assert ".result.block.header.height == $height" in body
    assert ".error == null" in body
    assert '.app_hash | type == "string" and test("^[A-Fa-f0-9]{64}$")' in body
    assert '.last_results_hash | type == "string" and test("^[A-Fa-f0-9]{64}$")' in body
    ordered(
        body,
        "[.app_hash, .last_results_hash] | @tsv",
        'if [ "$node" -eq 0 ]; then',
        "reference=$hashes",
        'elif [ "$hashes" != "$reference" ]; then',
        "return 1",
    )
    tail = text.split("./tests/integration/evm_module/scripts/evm_giga_tests.sh", 1)[1]
    ordered(tail, "\ncheck_mixed_final_agreement\n", "All four mixed-mode validators advanced and agree")
    assert "|| true" not in body


def test_giga_mixed_final_agreement_transport_rejects_unready_or_bad_responses():
    text = source(SCRIPT_NAMES[0])
    rpc = commands(function_body(text, "validator_rpc"))
    assert rpc == [[
        "timeout", "10s", "docker", "exec", "pax-node-$1", "curl",
        "--fail", "--silent", "--show-error", "--max-time", "5",
        "http://localhost:26657/$2",
    ]]
    height = function_body(text, "validator_height")
    assert 'validator_rpc "$1" status | jq -er' in height
    assert ".error == null and .result.sync_info.catching_up == false" in height
    assert ".result.sync_info.latest_block_height" in height
    assert 'select(type == "string" and test("^[1-9][0-9]*$"))' in height


def test_giga_mixed_final_agreement_exit_trap_preserves_failure_and_logs():
    text = source(SCRIPT_NAMES[0])
    ordered(text, "trap mixed_failure_diagnostics EXIT", "make docker-cluster-stop", "DOCKER_DETACH=true make")
    trap = function_body(text, "mixed_failure_diagnostics")
    ordered(trap, "local status=$?", "trap - EXIT", 'if [ "$status" -ne 0 ]; then')
    assert "for node in 0 1 2 3; do" in trap
    assert 'timeout 10s docker logs --tail 200 "pax-node-${node}" >&2 || true' in trap
    assert "http://localhost:26657/status >&2 || true" in trap
    assert trap.rstrip().endswith('exit "$status"')
    # Best-effort commands are restricted to diagnostics and the old cluster stop.
    remainder = text.replace(trap, "")
    assert remainder.count("|| true") == 1
    assert "make docker-cluster-stop || true" in remainder
    assert "EXIT_CODE=$?" not in text


def test_giga_unfrozen_fallback_only_frozen_install_before_workload():
    text = source(SCRIPT_NAMES[1])
    argv = commands(text)
    npm = [line for line in argv if line[0] == "npm"]
    assert npm == [["npm", "ci", "--prefer-offline"]]
    assert "set -e" in text
    assert "npm install" not in text
    ordered(text, "cd contracts", "npm ci --prefer-offline", "npx hardhat test --network paxlocal test/EVMGigaTest.js")
    assert "||" not in text


def test_io_setup_suppressed_sender_and_sequence_fail_closed():
    text = source(SCRIPT_NAMES[2])
    assert "set -euo pipefail" in text
    ordered(
        text,
        'FROM_ADDR=$(run paxd keys show "$FROM" "${KEYRING_ARGS[@]}" -a)',
        'if [[ -z "$FROM_ADDR" ]]; then',
        "exit 1",
        "SEQ_BEFORE_ASSOC=$(get_from_seq)",
    )
    seq = function_body(text, "get_from_seq")
    assert 'run paxd q account "$FROM_ADDR" -o json | jq -er' in seq
    assert '.sequence | select(type == "string" and test("^[0-9]+$"))' in seq
    wait = function_body(text, "wait_from_seq_advance")
    ordered(wait, 'if [[ ! "$prev" =~ ^[0-9]+$ ]]; then', "return 1", "deadline=$((SECONDS + 5))")
    ordered(wait, 'while [ "$SECONDS" -lt "$deadline" ]; do', 'cur=$(get_from_seq) || return 1', '[ "$cur" -gt "$prev" ]', "return 0", "sleep 0.5", "done", "return 1")
    assert "|| true" not in text
    assert "2>/dev/null" not in text
    assert "skipped" not in text


def test_io_setup_suppressed_cli_to_container_to_receipt_wiring():
    text = source(SCRIPT_NAMES[2])
    run = function_body(text, "run")
    assert 'docker exec --env PAX_EVM_IO_PASSWORD="$PASSWORD" "$CONTAINER" /bin/bash -c' in run
    assert '| "$@"\' bash "$@"' in run
    assert 'KEYRING_ARGS+=(--keyring-backend "$PAX_EVM_IO_KEYRING_BACKEND")' in text
    cli = (ROOT / "modules/evm/client/cli/tx.go").read_text(encoding="utf-8")
    assert 'fmt.Println("Transaction hash:", resp.Hex())' in cli
    extract = function_body(text, "deploy_tx_hash")
    assert "awk '/^Transaction hash: / {print $3}'" in extract
    ordered(extract, 'if [[ ! "$hash" =~ ^0x[[:xdigit:]]{64}$ ]]; then', "return 1", "printf '%s\\n' \"$hash\"")
    wait = function_body(text, "wait_deploy_receipt")
    # Parse the actual curl argument; the request must query this deployment.
    curl_line = next(line for line in wait.replace("\\\n", " ").splitlines() if "response=$(docker exec" in line)
    tokens = shlex.split(curl_line)
    payload = json.loads(tokens[tokens.index("-d") + 1])
    assert payload == {"jsonrpc": "2.0", "id": 1, "method": "eth_getTransactionReceipt", "params": ["$tx"]}
    assert tokens[tokens.index("--max-time") + 1] == "5"
    assert "--fail" in tokens
    assert '"$EVM_RPC_URL") || return 1' in curl_line
    for output, path, tx_name in (
        ("DEPLOY_OUT", "minimal", "DEPLOY_TX"),
        ("REVERTER_OUT", "reverter", "REVERTER_TX"),
    ):
        deployment = next(line for line in text.splitlines() if line.startswith(output + "="))
        assert f"$(run paxd tx evm deploy /tmp/{path}_contract.hex" in deployment
        assert '--evm-rpc "$EVM_RPC_URL"' in deployment
        assert '"${KEYRING_ARGS[@]}"' in deployment
        assert "||" not in deployment
        assert f'{tx_name}=$(deploy_tx_hash "${output}")' in text
        assert f'$(wait_deploy_receipt "${tx_name}")' in text


def test_io_setup_suppressed_receipts_require_success_and_committed_contract():
    wait = function_body(source(SCRIPT_NAMES[2]), "wait_deploy_receipt")
    ordered(wait, "for attempt in {1..30}; do", '.error == null and has("result")', '.result != null', '.status == "0x1" and .transactionHash == $tx')
    assert 'test("^0x[1-9a-fA-F][0-9a-fA-F]*$")' in wait
    assert 'test("^0x[0-9a-fA-F]{40}$")' in wait
    assert '. != "0x0000000000000000000000000000000000000000"' in wait
    ordered(wait, "deployment receipt is unsuccessful or invalid", "return 1", "printf '%s\\n' \"$response\"", "return 0", "sleep 1", "done", "return 1")
    assert "|| true" not in wait


def test_io_setup_suppressed_all_waits_and_bindings_precede_both_suites():
    text = source(SCRIPT_NAMES[2])
    setup = text.split("# Poll sequences between signed submissions;", 1)[1]
    ordered(
        setup,
        "SEQ_BEFORE_ASSOC=$(get_from_seq)",
        "ASSOC_OUT=$(run paxd tx evm associate-address",
        '.error == null and has("result")',
        'wait_from_seq_advance "$SEQ_BEFORE_ASSOC"',
        "SEQ_BEFORE_SEND=$(get_from_seq)",
        "run paxd tx evm send",
        'wait_from_seq_advance "$SEQ_BEFORE_SEND"',
        "SEQ_BEFORE_MINIMAL=$(get_from_seq)",
        "DEPLOY_OUT=$(run paxd tx evm deploy",
        'RECEIPT=$(wait_deploy_receipt "$DEPLOY_TX")',
        "PAX_EVM_IO_SEED_BLOCK=$(printf '%s\\n' \"$RECEIPT\" | jq -er '.result.blockNumber')",
        "PAX_EVM_IO_DEPLOY_TX_HASH=$DEPLOY_TX",
        "export PAX_EVM_IO_SEED_BLOCK PAX_EVM_IO_DEPLOY_TX_HASH",
        'wait_from_seq_advance "$SEQ_BEFORE_MINIMAL"',
        "REVERTER_OUT=$(run paxd tx evm deploy",
        'REVERTER_RECEIPT=$(wait_deploy_receipt "$REVERTER_TX")',
        "PAX_EVM_IO_REVERTER_ADDRESS=$(printf '%s\\n' \"$REVERTER_RECEIPT\" | jq -er '.result.contractAddress')",
        "export PAX_EVM_IO_REVERTER_ADDRESS",
        ': "${PAX_EVM_IO_SEED_BLOCK:?',
        ': "${PAX_EVM_IO_DEPLOY_TX_HASH:?',
        ': "${PAX_EVM_IO_REVERTER_ADDRESS:?',
        "export PAX_EVM_IO_RUN_INTEGRATION=1",
        "go test ./tests/integration/evm_module/rpc_io_test/ -v -count=1",
        "export PAX_EVM_WS_RUN_INTEGRATION=1",
        "go test ./tests/integration/evm_module/ws_test/ -v -count=1",
    )
    consumer = (ROOT / "tests/integration/evm_module/rpc_io_test/rpc_io_test.go").read_text(encoding="utf-8")
    for binding in ("PAX_EVM_IO_SEED_BLOCK", "PAX_EVM_IO_DEPLOY_TX_HASH", "PAX_EVM_IO_REVERTER_ADDRESS"):
        assert f'os.Getenv("{binding}")' in consumer


def test_repair_record_maps_every_finding_to_declared_proof():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert set(record["finding"]) == {
        "giga-mixed-final-agreement", "giga-unfrozen-fallback", "io-setup-suppressed",
    }
    declared = {
        node.name for node in ast.walk(ast.parse(Path(__file__).read_text(encoding="utf-8")))
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    for finding in record["finding"].values():
        assert finding["audit"] == "22.30.172"
        assert finding["tests"]
        assert set(finding["tests"]) <= declared
        assert (ROOT / finding["file"]).is_file()
    task = record["task"]["22"]["33"]["744"]
    assert task["base_revision"] == "76c050242a1edcad6212aa52fac15670ecb5612d"
    assert "not run" in task["baseline"]
    assert "review" in task["review_only"]
    assert "contracts/package-lock.json" in task["wiring_requests"][0]
    assert "domain 16" in task["wiring_requests"][0]
