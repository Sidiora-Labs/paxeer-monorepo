import copy
import importlib.util
import json
import pathlib

import pytest


GATEWAY = pathlib.Path(__file__).resolve().parents[1]
ROOT = GATEWAY.parents[3]
SPEC = importlib.util.spec_from_file_location("gateway_render_m109", GATEWAY / "render.py")
RENDERER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RENDERER)


def example_environment():
    return {RENDERER.OVERRIDE_VARIABLE: str(GATEWAY / "trust-inputs.example.json")}


def example_keys():
    return json.loads((GATEWAY / "receipt-keys.example.json").read_text())


def test_repair_m109_example_matches_current_renderer():
    keys = RENDERER.receipt_keys(GATEWAY / "receipt-keys.example.json")
    actual = RENDERER.render(
        ROOT, example_environment(), "eip155-125",
        sorted({entry["publicKey"][2:] for entry in keys}),
    )
    example = json.loads((GATEWAY / "config.example.json").read_text())
    assert actual == example
    adapters, transports = RENDERER.derived(ROOT)
    for section, derived in (("adapters", adapters), ("transports", transports)):
        assert {entry["id"] for entry in example[section]} == set(derived)
        for entry in example[section]:
            for name, (value, _) in derived[entry["id"]].items():
                assert entry[name] == value, (entry["id"], name)


@pytest.mark.parametrize("check", [False, True])
@pytest.mark.parametrize("reverse", [False, True])
@pytest.mark.parametrize("same_public_key", [False, True])
def test_repair_m109_duplicate_key_id_refused_before_write(
    tmp_path, monkeypatch, capsys, check, reverse, same_public_key
):
    keys = example_keys()
    keys[1]["keyId"] = keys[0]["keyId"]
    if same_public_key:
        keys[1]["publicKey"] = keys[0]["publicKey"]
    if reverse:
        keys.reverse()
    source = tmp_path / "receipt-keys.json"
    source.write_text(json.dumps(keys))
    output = tmp_path / "config.json"
    previous = b"previous manifest must survive a refused render\n"
    output.write_bytes(previous)
    # No process or environment outside the test is needed for the real CLI.
    monkeypatch.setattr(RENDERER.os, "environ", example_environment())
    arguments = [
        "--network-id", "eip155-125", "--receipt-keys-file", str(source),
        "--out", str(output),
    ]
    if check:
        arguments.append("--check")
    assert RENDERER.main(arguments) == 2
    refusal = capsys.readouterr().err
    assert "repeats keyId " + keys[0]["keyId"] in refusal
    assert "must be unique" in refusal
    assert output.read_bytes() == previous
    output.unlink()
    assert RENDERER.main(arguments) == 2
    assert not output.exists()
    assert json.loads(source.read_text()) == keys


def test_repair_m109_distinct_ids_may_share_public_key(tmp_path, monkeypatch):
    keys = example_keys()
    keys[1]["publicKey"] = keys[0]["publicKey"]
    source = tmp_path / "receipt-keys.json"
    source.write_text(json.dumps(keys[::-1]))
    assert RENDERER.receipt_keys(source) == keys
    monkeypatch.setattr(RENDERER.os, "environ", example_environment())
    output = tmp_path / "config.json"
    assert RENDERER.main([
        "--network-id", "eip155-125", "--receipt-keys-file", str(source),
        "--out", str(output),
    ]) == 0
    assert json.loads(output.read_text())["x402_supported"]["signers"] == {
        "eip155:125": ["did:layerx:" + keys[0]["publicKey"][2:]]
    }
    assert output.stat().st_mode & 0o777 == 0o600


@pytest.mark.parametrize(
    "change, expected",
    [
        ("overlap", "overlapping height ranges"),
        ("reversed", "reversed height range"),
        ("zero", "nonzero"),
        ("boolean", "unsigned 64-bit integer"),
        ("overflow", "unsigned 64-bit integer"),
        ("malformed", "32-byte lowercase hexadecimal"),
        ("unknown", "must declare exactly"),
    ],
)
def test_repair_m109_preserves_key_and_range_checks(tmp_path, change, expected):
    keys = copy.deepcopy(example_keys())
    if change == "overlap":
        keys[1]["firstHeight"] = keys[0]["lastHeight"]
    elif change == "reversed":
        keys[1]["firstHeight"] = keys[1]["lastHeight"] + 1
    elif change == "zero":
        keys[1]["publicKey"] = "0x" + "00" * 32
    elif change == "boolean":
        keys[1]["firstHeight"] = True
    elif change == "overflow":
        keys[1]["lastHeight"] = 1 << 64
    elif change == "malformed":
        keys[1]["keyId"] = "0x" + "aa" * 31
    else:
        keys[1]["extra"] = 1
    source = tmp_path / "receipt-keys.json"
    source.write_text(json.dumps(keys))
    with pytest.raises(RENDERER.Refused, match=expected):
        RENDERER.receipt_keys(source)


def test_repair_m109_readme_matches_manifest_and_live_interfaces():
    readme = " ".join((GATEWAY / "README.md").read_text().split())
    assert "all five adapters and three transport bindings" in readme
    assert "counts are stale" not in readme
    for name in (
        "trust-inputs.example.json", "receipt-keys.example.json",
        "LAYERX_INTEROP_RECEIPT_KEYS_FILE", "px_getReceiptKeys",
        "complete signer set", "eth_sendRawTransaction", "pax_sequencerReceipt",
        "eth_getBlockByNumber", "repeating an id makes that selection ambiguous",
    ):
        assert name in readme
