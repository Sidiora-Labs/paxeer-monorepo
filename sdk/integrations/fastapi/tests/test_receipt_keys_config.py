import json
from base64 import b64encode
from dataclasses import fields

import pytest
from layerx_sdk.px import PxReceiptKeyEntry

from layerx_fastapi import config, parse_receipt_keys


def _environment() -> dict[str, str]:
    return {
        "LAYERX_PRINCIPAL": "merchant",
        "LAYERX_PROTECTED_PATH": "/paid",
        "LAYERX_RESOURCE_URL": "https://merchant.example/paid",
        "LAYERX_X402_SCHEME": "exact",
        "LAYERX_X402_NETWORK": "eip155:125",
        "LAYERX_PRICE": "100",
        "LAYERX_ASSET": "0x" + "11" * 20,
        "LAYERX_PAY_TO": "0x" + "22" * 20,
        "LAYERX_PAYMENT_TIMEOUT_SECONDS": "60",
        "LAYERX_WEBHOOK_PATH": "/webhook",
        "LAYERX_WEBHOOK_PUBLIC_KEYS_JSON": json.dumps(
            {"webhook-key": b64encode(bytes.fromhex("33" * 32)).decode("ascii")}
        ),
    }


def test_load_config_reads_receipt_keys() -> None:
    environment = _environment()
    pins = [
        {
            "keyId": b64encode(bytes.fromhex("44" * 32)).decode("ascii"),
            "publicKey": b64encode(bytes.fromhex("55" * 32)).decode("ascii"),
            "firstHeight": 0,
            "lastHeight": 100,
        },
        {
            "keyId": b64encode(bytes.fromhex("66" * 32)).decode("ascii"),
            "publicKey": b64encode(bytes.fromhex("77" * 32)).decode("ascii"),
            "firstHeight": 101,
            "lastHeight": 18446744073709551615,
        },
    ]
    environment["LAYERX_RECEIPT_KEYS_JSON"] = json.dumps(pins)

    loaded = config.read_declared_config(environment)

    assert isinstance(loaded.receipt_keys, tuple)
    assert loaded.receipt_keys == (
        PxReceiptKeyEntry("44" * 32, "55" * 32, 0, 100),
        PxReceiptKeyEntry("66" * 32, "77" * 32, 101, 18446744073709551615),
    )
    assert parse_receipt_keys(environment["LAYERX_RECEIPT_KEYS_JSON"]) == loaded.receipt_keys
    assert loaded.principal == "merchant"
    assert loaded.protected_path == "/paid"
    assert "LAYERX_RECEIPT_KEYS_JSON" in config.DECLARED_KEYS
    assert "LAYERX_AUTHORIZED_BATCH_JSON" not in config.DECLARED_KEYS
    assert "authorized_batch" not in {field.name for field in fields(config.DeclaredConfig)}
    assert not hasattr(loaded, "authorized_batch")
    assert not hasattr(config, "parse_authorized_batch")

    invalid_values = [
        "{",
        "null",
        "[]",
        json.dumps({"keys": pins}),
        json.dumps([None]),
        json.dumps([{}]),
        json.dumps([pins[0], pins[0]]),
        json.dumps([{**pins[0], "keyId": "0x" + "44" * 32}]),
        json.dumps([{**pins[0], "publicKey": "!" * 44}]),
        json.dumps([{**pins[0], "publicKey": b64encode(bytes(31)).decode("ascii")}]),
        json.dumps([{**pins[0], "keyId": b64encode(bytes(32)).decode("ascii")}]),
        json.dumps([{**pins[0], "publicKey": b64encode(bytes(32)).decode("ascii")}]),
        json.dumps([{**pins[0], "firstHeight": 101}]),
        json.dumps([{**pins[0], "firstHeight": -1}]),
        json.dumps([{**pins[0], "firstHeight": True}]),
        json.dumps([{**pins[0], "lastHeight": 18446744073709551616}]),
        json.dumps([{**pins[0], "extra": "refuse"}]),
    ]
    for value in invalid_values:
        environment["LAYERX_RECEIPT_KEYS_JSON"] = value
        with pytest.raises(config.IntegrationError) as raised:
            config.read_declared_config(environment)
        assert raised.value.code is config.IntegrationErrorCode.INVALID_DECLARED_KEY


def test_load_config_refuses_authorized_batch_only() -> None:
    environment = _environment()
    environment["LAYERX_AUTHORIZED_BATCH_JSON"] = json.dumps(
        {
            "batchId": "0x" + "11" * 32,
            "asset": "0x" + "22" * 32,
            "previousStateRoot": "0x" + "33" * 32,
            "resultingStateRoot": "0x" + "44" * 32,
            "sequencerPublicKey": "0x" + "55" * 32,
        }
    )

    with pytest.raises(config.IntegrationError) as raised:
        config.read_declared_config(environment)

    assert raised.value.code is config.IntegrationErrorCode.MISSING_DECLARED_KEY
    assert raised.value.key == "LAYERX_RECEIPT_KEYS_JSON"
    assert "LAYERX_RECEIPT_KEYS_JSON" in str(raised.value)
    assert "LAYERX_RECEIPT_KEYS_JSON" not in environment

    environment["LAYERX_RECEIPT_KEYS_JSON"] = ""
    with pytest.raises(config.IntegrationError) as raised:
        config.read_declared_config(environment)
    assert raised.value.code is config.IntegrationErrorCode.MISSING_DECLARED_KEY
    assert raised.value.key == "LAYERX_RECEIPT_KEYS_JSON"
