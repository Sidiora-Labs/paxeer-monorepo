import ast
import json
import os
import re
import runpy
import shlex
import subprocess
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[3]
JOURNEY = REPO_ROOT / "services/platform/ramps/sandbox-journey.sh"


def test_journey_help_names_no_on_account_sequence():
    environment = {
        key: value
        for key, value in os.environ.items()
        if not key.startswith(("LAYERX_RAMP_", "PAXEER_X_RAMP_"))
    }
    for full_contract in ("0", "1"):
        result = subprocess.run(
            ["bash", str(JOURNEY), "--help"],
            env={**environment, "PAXEER_X_RAMP_FULL_CONTRACT": full_contract},
            capture_output=True,
            text=True,
            check=False,
        )
        assert result.returncode == 0, result.stderr
        assert "Usage:" in result.stdout
        assert "LAYERX_RAMP_ON_ACCOUNT_SEQUENCE" not in result.stdout
        assert "LAYERX_RAMP_OFF_RECEIVER_SEQUENCE" not in result.stdout
        assert "customer-signed raw_transaction" in result.stdout
        assert "no separate sequence is accepted" in result.stdout
        assert "--transaction-hash" in result.stdout
        assert "authenticated committed receipts" in result.stdout
    source = JOURNEY.read_text()
    assert "LAYERX_RAMP_ON_ACCOUNT_SEQUENCE" not in source
    assert "LAYERX_RAMP_OFF_RECEIVER_SEQUENCE" not in source
    assert 'work "${on_digest}" submit_layerx >/dev/null' in source
    assert 'work "${off_digest}" submit_customer_payment >/dev/null' in source
    assert 'raw_transaction:.off.raw_transaction' in source
    assert 'work(on_digest, "submit_layerx")' in source
    assert 'work(off_digest, "submit_customer_payment")' in source
    contract = runpy.run_path(str(JOURNEY.with_name("journey_contract.py")))
    work_request = contract["work_request"]
    digest = [17] * 32
    assert work_request(digest, "submit_layerx") == {
        "order_digest": digest,
        "action": "submit_layerx",
        "account_sequence": None,
    }
    raw_transaction = "02" + "11" * 32
    assert work_request(digest, "submit_customer_payment", raw_transaction) == {
        "order_digest": digest,
        "action": "submit_customer_payment",
        "account_sequence": None,
        "raw_transaction": list(bytes.fromhex(raw_transaction)),
    }


def test_journey_verifies_by_transaction_hash():
    syntax = subprocess.run(
        ["bash", "-n", str(JOURNEY)], capture_output=True, text=True, check=False
    )
    assert syntax.returncode == 0, syntax.stderr
    source = JOURNEY.read_text()
    commands = [
        shlex.split(line)
        for line in source.replace("\\\n", " ").splitlines()
        if line.lstrip().startswith('"$LAYERX_RAMP_RECEIPT_VERIFIER_BINARY" ')
    ]
    assert commands == [
        [
            "$LAYERX_RAMP_RECEIPT_VERIFIER_BINARY",
            "--config-file", "$LAYERX_RAMP_RECEIPT_VERIFIER_CONFIG",
            "--order-file", "$order_file",
            "--transaction-hash", "$tx_hash",
            "--output-file", "$result_file",
        ]
    ]
    embedded = ast.parse(source.split("<<'PY'\n", 1)[1].split("\nPY\n", 1)[0])
    verifier = next(
        node
        for node in embedded.body
        if isinstance(node, ast.FunctionDef) and node.name == "verify_done"
    )
    invocations = [
        node
        for node in ast.walk(verifier)
        if isinstance(node, ast.Call) and ast.unparse(node.func) == "subprocess.run"
    ]
    assert len(invocations) == 1
    arguments = invocations[0].args[0].elts
    assert len(arguments) == 9
    assert ast.unparse(arguments[0]) == "env('LAYERX_RAMP_RECEIPT_VERIFIER_BINARY')"
    assert [arguments[index].value for index in (1, 3, 5, 7)] == [
        "--config-file", "--order-file", "--transaction-hash", "--output-file",
    ]
    assert isinstance(arguments[6], ast.Name) and arguments[6].id == "transaction"
    assert 'transaction = "0x" + octets(snapshot["evidence"]["tx_hash"]).hex()' in source
    assert 'verified["transaction_hash"] == transaction' in source
    assert 'view["transaction_hash"] == transaction' in source
    assert 'verified["receipt_digest"] == octets(view["receipt_digest"]).hex()' in source
    assert 'verified["rung"] == snapshot["evidence"]["rung"] == "committed"' in source
    assert 'verified["maintained_batch"] is True' in source
    assert "keys=$(printf '%s' \"$response\" | receipt_evidence)" in source
    assert re.search(r'tx_hash="0x\$\([^\n]+\$keys[^\n]+\.tx_hash\[\]', source)
    assert "$proof[0].transaction_hash == $tx_hash" in source
    assert "$proof[0].receipt_digest == $receipt_digest" in source
    assert '$proof[0].rung == "committed" and .evidence.rung == "committed"' in source
    assert '$proof[0].maintained_batch == true' in source
    assert '.presentation.transaction_hash == $tx_hash' in source

    function = re.search(r"^receipt_evidence\(\) \{\n.*?^\}", source, re.M | re.S)
    assert function is not None
    evidence = {"tx_hash": [17] * 32, "receipt_digest": [34] * 32}

    def read_evidence(value):
        return subprocess.run(
            ["bash", "-eu", "-c", function.group() + "\nreceipt_evidence"],
            input=json.dumps({"evidence": value}),
            capture_output=True,
            text=True,
            check=False,
        )

    value = {**evidence, "rung": "committed"}
    result = read_evidence(value)
    assert result.returncode == 0, result.stderr
    assert json.loads(result.stdout) == value
    for rung in (None, "submitted", "receipted", "final", "done"):
        assert read_evidence({**evidence, "rung": rung}).returncode != 0
    for field in ("tx_hash", "receipt_digest"):
        for invalid in (None, [], [0] * 32, [1] * 31, [256] * 32, [1.5] * 32):
            value = {**evidence, "rung": "committed", field: invalid}
            assert read_evidence(value).returncode != 0
    assert (
        read_evidence(
            {"activity_id": [17] * 32, "receipt_digest": [34] * 32, "rung": "committed"}
        ).returncode
        != 0
    )
