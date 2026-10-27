import ast
import hashlib
import json
import re
import shlex
import subprocess
import sys
from pathlib import Path


def test_check_passes_only_arguments_the_renderer_declares():
    mirror = Path(__file__).resolve().parents[1]
    renderer = ast.parse((mirror / "render-config.py").read_text())
    declared = {
        argument.value
        for call in ast.walk(renderer)
        if isinstance(call, ast.Call)
        and isinstance(call.func, ast.Attribute)
        and call.func.attr == "add_argument"
        for argument in call.args
        if isinstance(argument, ast.Constant)
        and isinstance(argument.value, str)
        and argument.value.startswith("--")
    }
    assert declared, "No renderer argument declarations found"

    check = (mirror / "tests" / "render-config-check.sh").read_text()
    logical_lines = check.replace("\\\n", "").splitlines()
    invocation = re.compile(
        r'\b(?:python3\s+"\$RENDERER"|render(?:_ethereum_only)?)(?=\s)'
    )
    commands = [
        line[match.end():]
        for line in logical_lines
        if (match := invocation.search(line))
    ]
    assert any('python3 "$RENDERER"' in line for line in logical_lines)
    assert commands, "No renderer invocations found"

    # solana_target supplies additional arguments through render's array.
    solana_target = re.search(r"^solana_target\(\) \{\n(.*?)^\}", check, re.M | re.S)
    assert solana_target, "No Solana argument provider found"
    commands.append(solana_target.group(1).replace("\\\n", ""))

    passed = {
        token.split("=", 1)[0]
        for command in commands
        for token in shlex.split(command, comments=True)
        if token.startswith("--")
    }
    assert passed, "No renderer flags found"
    assert passed <= declared, f"Undeclared renderer flags: {sorted(passed - declared)}"


def test_rendered_keys_match_the_example(tmp_path):
    mirror = Path(__file__).resolve().parents[1]
    example = json.loads((mirror / "config.example.json").read_text())
    keys = [
        {
            "keyId": "0x" + hashlib.sha256(b"first receipt key").hexdigest(),
            "publicKey": "0x8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e",
            "firstHeight": 1000,
            "lastHeight": 1999,
        },
        {
            "keyId": "0x" + hashlib.sha256(b"second receipt key").hexdigest(),
            "publicKey": "0x3d4017c3e843895a92b70aa74d1b7ebc9c982ccf2ec4968cc0cd55f12af4660c",
            "firstHeight": 2000,
            "lastHeight": 2999,
        },
    ]
    keys_file = tmp_path / "receipt keys.json"
    keys_file.write_text(json.dumps(keys))
    output = tmp_path / "rendered.json"
    command = [
        sys.executable,
        str(mirror / "render-config.py"),
        "--output",
        str(output),
        "--state-directory",
        str(tmp_path / "state"),
        "--first-height",
        "1000",
        "--status-listen",
        "127.0.0.1:9091",
        "--paxeer-chain-id",
        "125",
        "--receipt-keys-file",
        str(keys_file),
        "--ethereum-chain-id",
        "11155111",
        "--ethereum-genesis-hash",
        "11" * 32,
        "--ethereum-archive-contract",
        "22" * 20,
        "--ethereum-archive-code-hash",
        "33" * 32,
        "--ethereum-signer-public-key",
        "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
    ]
    for chain in ("paxeer", "ethereum"):
        for backend in ("a", "b"):
            command.extend(
                [
                    "--%s-endpoint" % chain,
                    "https://%s-%s.example/rpc,%s-%s,/run/secrets/ca.der,/run/secrets/token"
                    % (chain, backend, chain, backend),
                ]
            )

    def key_sets(value):
        if isinstance(value, dict):
            return {key: key_sets(item) for key, item in value.items()}
        if isinstance(value, list):
            assert value
            shape = key_sets(value[0])
            assert all(key_sets(item) == shape for item in value)
            return [shape]
        return None

    for with_solana in (False, True):
        arguments = list(command)
        if with_solana:
            for backend in ("a", "b"):
                arguments.extend(
                    [
                        "--solana-endpoint",
                        "https://solana-%s.example/rpc,solana-%s,/run/secrets/ca.der,/run/secrets/token"
                        % (backend, backend),
                    ]
                )
            for name in (
                "genesis-hash",
                "archive-program",
                "upgradeable-loader",
                "program-data-account",
                "signer-public-key",
            ):
                arguments.extend(
                    ["--solana-" + name, "11111111111111111111111111111112"]
                )
            arguments.extend(["--solana-program-code-hash", "44" * 32])
        result = subprocess.run(arguments, capture_output=True, text=True)
        assert result.returncode == 0, result.stderr
        rendered = json.loads(output.read_text())
        expected = dict(example)
        if not with_solana:
            expected.pop("solana")
        assert key_sets(rendered) == key_sets(expected)
        assert rendered["first_height"] == 1000
        assert (
            rendered["freshness_budget_blocks"]
            == example["freshness_budget_blocks"]
            == 64
        )
        assert set(rendered["paxeer"]) == {"rpc", "chain_id", "receipt_keys"}
        assert rendered["paxeer"]["chain_id"] == 125
        assert rendered["paxeer"]["receipt_keys"] == keys
        assert json.loads(keys_file.read_text()) == keys
        assert "node" not in rendered
        assert "first_batch_number" not in rendered
        assert "checkpoint_freshness_budget_batches" not in rendered
        assert "checkpoint_policy" not in output.read_text()
        assert output.stat().st_mode & 0o777 == 0o600
