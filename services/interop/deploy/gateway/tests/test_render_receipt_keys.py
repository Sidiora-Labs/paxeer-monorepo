import json
import pathlib
import subprocess
import sys

import pytest


RENDERER = pathlib.Path(__file__).resolve().parents[1] / "render.py"


@pytest.mark.parametrize("check", [False, True])
@pytest.mark.parametrize(
    "ranges, conflicting",
    [
        ([(10, 20), (15, 25)], (0, 1)),
        ([(15, 25), (10, 20)], (1, 0)),
        ([(10, 20), (20, 30)], (0, 1)),
        ([(10, 30), (15, 20)], (0, 1)),
        ([(10, 20), (10, 30)], (0, 1)),
        ([(10, 20), (10, 20)], (0, 1)),
        ([(30, 40), (10, 20), (15, 25)], (1, 2)),
        ([((1 << 64) - 1, (1 << 64) - 1), (0, (1 << 64) - 1)], (1, 0)),
    ],
)
def test_overlapping_receipt_keys_are_refused(tmp_path, ranges, conflicting, check):
    keys = [
        {
            "keyId": "0x%064x" % (index + 1),
            "publicKey": "0x" + "ab" * 32,
            "firstHeight": first,
            "lastHeight": last,
        }
        for index, (first, last) in enumerate(ranges)
    ]
    keys_file = tmp_path / "receipt-keys.json"
    keys_file.write_text(json.dumps(keys))
    output = tmp_path / "gateway.json"
    command = [
        sys.executable,
        str(RENDERER),
        "--receipt-keys-file",
        str(keys_file),
        "--network-id",
        "layerx-beta",
        "--out",
        str(output),
    ]
    if check:
        command.append("--check")
    example = json.loads(RENDERER.with_name("config.example.json").read_text())
    environ = {
        "LAYERX_BETA_INTEROP_" + name.upper(): json.dumps(example[name])
        for name in ("ap2_keys", "ap2_assets", "visa_agents", "visa_targets", "fiat_providers")
    }
    refused = subprocess.run(command, env=environ, capture_output=True, text=True, timeout=30)
    assert refused.returncode == 2, refused.stderr
    assert "overlapping height ranges" in refused.stderr
    assert all(
        keys[index]["keyId"] in refused.stderr for index in conflicting
    ), refused.stderr
    assert not output.exists()

    for entry, (first, last) in zip(keys, [(11, (1 << 64) - 1), (0, 0), (1, 10)]):
        entry["firstHeight"], entry["lastHeight"] = first, last
    keys_file.write_text(json.dumps(keys))
    accepted = subprocess.run(command, env=environ, capture_output=True, text=True, timeout=30)
    assert accepted.returncode == 0, accepted.stderr
    if check:
        assert not output.exists()
    else:
        document = json.loads(output.read_text())
        assert document["x402_supported"]["signers"] == {
            "layerx:beta": ["did:layerx:" + "ab" * 32]
        }
        assert output.stat().st_mode & 0o777 == 0o600


@pytest.mark.parametrize("mode", [None, "--check", "--self-test"])
def test_single_key_argument_is_refused(tmp_path, mode):
    sequencer_file = tmp_path / "sequencer-public-key"
    sequencer_file.write_text("ab" * 32 + "\n")
    keys_file = tmp_path / "receipt-keys.json"
    keys_file.write_text(
        json.dumps([
            {
                "keyId": "0x" + "01" * 32,
                "publicKey": "0x" + "ab" * 32,
                "firstHeight": 0,
                "lastHeight": (1 << 64) - 1,
            }
        ])
    )
    output = tmp_path / "gateway.json"
    command = [
        sys.executable,
        str(RENDERER),
        "--network-id",
        "layerx-beta",
        "--receipt-keys-file",
        str(keys_file),
        "--sequencer-public-key-file",
        str(sequencer_file),
        "--out",
        str(output),
    ]
    if mode:
        command.append(mode)
    refused = subprocess.run(command, env={}, capture_output=True, text=True, timeout=30)
    assert refused.returncode == 2, refused.stderr
    assert "unrecognized arguments: --sequencer-public-key-file" in refused.stderr
    assert not output.exists()
