import ast
import importlib.util
import json
import pathlib
import shlex
import subprocess
import sys

import pytest


RENDERER = pathlib.Path(__file__).resolve().parents[1] / "render.py"
SPEC = importlib.util.spec_from_file_location("gateway_receipt_keys_renderer", RENDERER)
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def render_command(keys_file, output, check):
    command = [
        sys.executable, str(RENDERER), "--receipt-keys-file", str(keys_file),
        "--network-id", "layerx-beta", "--out", str(output),
    ]
    if check:
        command.append("--check")
    example = json.loads(RENDERER.with_name("config.example.json").read_text())
    environ = {
        "LAYERX_BETA_INTEROP_" + name.upper(): json.dumps(example[name])
        for name in ("ap2_keys", "ap2_assets", "visa_agents", "visa_targets", "fiat_providers")
    }
    return subprocess.run(command, env=environ, capture_output=True, text=True, timeout=30)


@pytest.mark.parametrize("check", [False, True])
@pytest.mark.parametrize("same_public_key", [False, True])
@pytest.mark.parametrize(
    "ranges",
    [
        [(10, 20), (15, 25)],
        [(15, 25), (10, 20)],
        [(10, 20), (20, 30)],
        [(10, 30), (15, 20)],
        [(10, 20), (10, 30)],
        [(10, 20), (10, 20)],
        [(30, 40), (10, 20), (15, 25)],
        [((1 << 64) - 1, (1 << 64) - 1), (0, (1 << 64) - 1)],
        [(11, (1 << 64) - 1), (0, 0), (1, 10)],
    ],
)
def test_distinct_receipt_keys_preserve_order(tmp_path, ranges, same_public_key, check):
    keys = [
        {
            "keyId": "0x%064x" % (index + 1),
            "publicKey": "0x" + ("ab" if same_public_key else "%02x" % (index + 10)) * 32,
            "firstHeight": first,
            "lastHeight": last,
        }
        for index, (first, last) in enumerate(ranges)
    ]
    keys_file = tmp_path / "receipt-keys.json"
    keys_file.write_text(json.dumps(keys))
    assert MODULE.receipt_keys(keys_file) == keys
    output = tmp_path / "gateway.json"
    accepted = render_command(keys_file, output, check)
    assert accepted.returncode == 0, accepted.stderr
    assert json.loads(keys_file.read_text()) == keys
    if check:
        assert not output.exists()
    else:
        document = json.loads(output.read_text())
        assert document["x402_supported"]["signers"] == {
            "layerx:beta": sorted({"did:layerx:" + key["publicKey"][2:] for key in keys})
        }
        assert output.stat().st_mode & 0o777 == 0o600


@pytest.mark.parametrize("check", [False, True])
@pytest.mark.parametrize("reverse", [False, True])
@pytest.mark.parametrize("same_public_key", [False, True])
@pytest.mark.parametrize("window", [(10, 20), (15, 25), (21, 30)])
def test_duplicate_receipt_key_ids_are_refused(tmp_path, check, reverse, same_public_key, window):
    first = {
        "keyId": "0x" + "ab" * 32,
        "publicKey": "0x" + "ab" * 32,
        "firstHeight": 10,
        "lastHeight": 20,
    }
    keys = [first, {
        **first, "publicKey": first["publicKey"] if same_public_key else "0x" + "cd" * 32,
        "firstHeight": window[0], "lastHeight": window[1],
    }]
    if reverse:
        keys.reverse()
    keys_file = tmp_path / "receipt-keys.json"
    keys_file.write_text(json.dumps(keys))
    with pytest.raises(MODULE.Refused, match="repeats keyId"):
        MODULE.receipt_keys(keys_file)
    output = tmp_path / "gateway.json"
    for existing in (True, False):
        previous = b"existing manifest\n"
        if existing:
            output.write_bytes(previous)
        refused = render_command(keys_file, output, check)
        assert refused.returncode == 2, refused.stderr
        assert "repeats keyId " + first["keyId"] in refused.stderr
        assert "must be unique" in refused.stderr
        if existing:
            assert output.read_bytes() == previous
            output.unlink()
        else:
            assert not output.exists()
        assert json.loads(keys_file.read_text()) == keys


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


def test_hosted_renderer_options_are_declared():
    parser_source = ast.parse(RENDERER.read_text())
    declared = {
        argument.value
        for call in ast.walk(parser_source)
        if isinstance(call, ast.Call)
        and isinstance(call.func, ast.Attribute)
        and isinstance(call.func.value, ast.Name)
        and call.func.value.id == "parser"
        and call.func.attr == "add_argument"
        for argument in call.args
        if isinstance(argument, ast.Constant)
        and isinstance(argument.value, str)
        and argument.value.startswith("--")
    }
    assert declared
    root = RENDERER.parents[4]
    scripts = sorted((root / "services/platform/hosted/tests").glob("beta-*.sh"))
    assert len(scripts) == 3
    renderer_path = "/services/interop/deploy/gateway/render.py"
    calls = 0
    for script in scripts:
        lines = script.read_text().replace("\\\n", " ").splitlines()
        for line in lines:
            if renderer_path not in line or line.lstrip().startswith("#"):
                continue
            command = shlex.split(line, comments=True)
            for index, word in enumerate(command):
                if not word.endswith(renderer_path):
                    continue
                arguments = command[index + 1:]
                if "||" in arguments:
                    arguments = arguments[:arguments.index("||")]
                options = {
                    argument.split("=", 1)[0]
                    for argument in arguments
                    if argument.startswith("--")
                }
                assert options, script.name
                assert options <= declared, (script.name, sorted(options - declared))
                calls += 1
    assert calls > 0
