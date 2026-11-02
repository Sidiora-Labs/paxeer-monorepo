import json
import os
import re
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
        assert "LAYERX_RAMP_OFF_RECEIVER_SEQUENCE" in result.stdout
    source = JOURNEY.read_text()
    assert "LAYERX_RAMP_ON_ACCOUNT_SEQUENCE" not in source
    assert 'work "${on_digest}" submit_layerx null' in source
    assert (
        'work "${off_digest}" submit_layerx "${LAYERX_RAMP_OFF_RECEIVER_SEQUENCE}"'
        in source
    )


def test_journey_verifies_by_transaction_hash():
    syntax = subprocess.run(
        ["bash", "-n", str(JOURNEY)], capture_output=True, text=True, check=False
    )
    assert syntax.returncode == 0, syntax.stderr
    source = JOURNEY.read_text()
    commands = [
        shlex.split(line)
        for line in source.replace("\\\n", " ").splitlines()
        if " --verify-receipt " in line
    ]
    assert commands
    for command in commands:
        option = command.index("--verify-receipt")
        assert command[option + 3] == "$tx_hash", command
        assert not any("activity" in argument for argument in command)
    assert "keys=$(printf '%s' \"$response\" | receipt_evidence)" in source
    assert re.search(r'tx_hash="0x\$\([^\n]+\$keys[^\n]+\.tx_hash\[\]', source)
    assert "$proof[0].transaction_hash == $tx_hash" in source
    assert "$proof[0].receipt_digest == $receipt_digest" in source
    assert '($proof[0].rung == "receipted" or $proof[0].rung == "committed")' in source

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

    for rung in ("receipted", "committed"):
        value = {**evidence, "rung": rung}
        result = read_evidence(value)
        assert result.returncode == 0, result.stderr
        assert json.loads(result.stdout) == value
    for rung in (None, "submitted", "final", "done"):
        assert read_evidence({**evidence, "rung": rung}).returncode != 0
    for field in ("tx_hash", "receipt_digest"):
        for invalid in (None, [], [0] * 32, [1] * 31, [256] * 32, [1.5] * 32):
            value = {**evidence, "rung": "receipted", field: invalid}
            assert read_evidence(value).returncode != 0
    assert (
        read_evidence(
            {"activity_id": [17] * 32, "receipt_digest": [34] * 32, "rung": "receipted"}
        ).returncode
        != 0
    )
