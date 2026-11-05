import json
from pathlib import Path


def test_example_pins_receipt_keys():
    example = Path(__file__).resolve().parents[1] / "config.example.json"
    config = json.loads(example.read_text(encoding="utf-8"))

    assert "kernel_network_id" not in config
    gateway = config["gateway"]
    assert "sequencer_id" not in gateway
    assert "sequencer_public_key" not in gateway

    receipt_keys = gateway["receipt_keys"]
    assert isinstance(receipt_keys, list)
    assert receipt_keys
    for key in receipt_keys:
        assert isinstance(key, dict)
        assert set(key) == {"key_id", "public_key", "first_height", "last_height"}
        for value in key.values():
            assert isinstance(value, str)
            assert value.startswith("REPLACE_WITH_")
