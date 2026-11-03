from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))

from layerx_sdk import (
    CHAINS_EVENT_SIGNATURES,
    CHAINS_EVENT_TOPICS,
    CHAINS_PRECOMPILE,
    CHAINS_SELECTORS,
    CHAINS_SIGNATURES,
    ChainsClient,
    chain_rpc_url,
    chains_call,
    decode_chains_call,
    decode_chains_event,
    decode_chains_params,
    decode_get_chain,
    virtual_chain_id,
)
from layerx_sdk.chains import _CHAIN_FIELDS, _PARAMS_FIELDS, _encode_abi


def test_chains_vectors_match_and_round_trip() -> None:
    vectors = json.loads((ROOT / "tests/vectors/chains/abi-vectors.json").read_text())
    assert CHAINS_PRECOMPILE == vectors["address"]
    assert set(CHAINS_SELECTORS) == {item["name"] for item in vectors["methods"]}
    assert len(CHAINS_SELECTORS) == 12
    for item in vectors["methods"]:
        name = item["name"]
        assert CHAINS_SIGNATURES[name] == item["signature"]
        assert CHAINS_SELECTORS[name] == item["selector"]
        call = chains_call(name, *item["args"])
        assert call.to == vectors["address"]
        assert call.value == 0
        assert call.data == item["calldata"]
        assert bytes.fromhex(call.data[2:]) == bytes.fromhex(item["calldata"][2:])
        method, args = decode_chains_call(call.data)
        assert method == name
        assert args == tuple(item["args"])
        assert chains_call(method, *args) == call

    for key, decode, fields in (
        ("getChain", decode_get_chain, _CHAIN_FIELDS),
        ("params", decode_chains_params, _PARAMS_FIELDS),
    ):
        example = vectors[key]
        decoded = decode(example["returndata"])
        assert decoded == example["decoded"]
        values = tuple(int(decoded[name]) if kind.startswith("uint") else decoded[name]
                       for name, kind in fields)
        encoded = _encode_abi(tuple(kind for _, kind in fields), values)
        assert encoded == bytes.fromhex(example["returndata"][2:])
        assert decode("0x" + encoded.hex()) == decoded

    assert set(CHAINS_EVENT_TOPICS) == {item["name"] for item in vectors["events"]}
    assert len(CHAINS_EVENT_TOPICS) == 4
    for item in vectors["events"]:
        assert CHAINS_EVENT_SIGNATURES[item["name"]] == item["signature"]
        assert CHAINS_EVENT_TOPICS[item["name"]] == item["topic0"]
        assert bytes.fromhex(CHAINS_EVENT_TOPICS[item["name"]][2:]) == bytes.fromhex(item["topic0"][2:])

    chain = vectors["getChain"]["decoded"]
    address = chain["operator"]
    id_topic = "0x" + _encode_abi(("uint32",), (chain["id"],)).hex()
    address_topic = "0x" + _encode_abi(("address",), (address,)).hex()
    events = (
        ("ChainCreated", [id_topic, address_topic], ("string", "uint64"),
         (chain["name"], chain["chainId"]),
         {"id": 1, "name": "demo", "operator": address, "chainId": 125000001}),
        ("ChainStatusChanged", [id_topic], ("uint8",), (chain["status"],),
         {"id": 1, "status": 1}),
        ("ChainFunded", [id_topic, address_topic], ("uint256",), (int(chain["bond"]),),
         {"id": 1, "from": address, "amount": chain["bond"]}),
        ("ChainOperatorChanged", [id_topic, address_topic], (), (),
         {"id": 1, "operator": address}),
    )
    for name, indexed, types, values, expected in events:
        topics = [CHAINS_EVENT_TOPICS[name], *indexed]
        data = "0x" + _encode_abi(types, values).hex()
        assert decode_chains_event(topics, data) == {"event": name, **expected}
        with pytest.raises(ValueError):
            decode_chains_event(topics[:-1], data)
        with pytest.raises(ValueError):
            decode_chains_event(topics, data + "00" * 32)

    assert virtual_chain_id(chain["id"]) == chain["chainId"]
    assert virtual_chain_id(0) == 125000000
    assert virtual_chain_id((1 << 32) - 1) == 125000000 + (1 << 32) - 1
    assert chain_rpc_url("https://rpc.example/", 1) == "https://rpc.example/chain/1"
    assert chain_rpc_url("https://rpc.example/evm///", 1) == "https://rpc.example/evm/chain/1"
    assert ChainsClient("https://rpc.example").endpoint == "https://rpc.example"
    for text in ("", "a" * 31, "a" * 32, "a" * 33, "链🌍" * 20):
        call = chains_call("createChain", text, (1 << 64) - 1, 0, (1 << 16) - 1,
                           value=(1 << 256) - 1)
        assert call.value == (1 << 256) - 1
        method, args = decode_chains_call(call.data)
        assert method == "createChain"
        assert args == (text, (1 << 64) - 1, 0, (1 << 16) - 1)
        assert chains_call(method, *args, value=call.value) == call
    for paused in (False, True):
        call = chains_call("governancePause", 1, paused)
        assert decode_chains_call(call.data) == ("governancePause", (1, paused))
    assert chains_call("fundChain", 1, value=123).value == 123

    for invalid in (-1, 1 << 32, True, "1"):
        with pytest.raises(ValueError):
            chains_call("getChain", invalid)
        with pytest.raises(ValueError):
            virtual_chain_id(invalid)
        with pytest.raises(ValueError):
            chain_rpc_url("https://rpc.example", invalid)
    for args in (("demo", 1 << 64, 1, 1), ("demo", 1, 1 << 64, 1),
                 ("demo", 1, 1, 1 << 16), ("demo", True, 1, 1)):
        with pytest.raises(ValueError):
            chains_call("createChain", *args)
    for invalid in ("0x", "0x1234", "0x" + "z" * 40, "0x" + "1" * 42):
        with pytest.raises(ValueError):
            chains_call("setOperator", 1, invalid)
    for invalid in (0, 1, "true"):
        with pytest.raises(ValueError):
            chains_call("governancePause", 1, invalid)
    with pytest.raises(ValueError):
        chains_call("getChain", 1, value=1)
    with pytest.raises(ValueError):
        chains_call("fundChain", 1, value=-1)
    with pytest.raises(ValueError):
        chains_call("chainCount", 1)
    with pytest.raises(ValueError):
        chains_call("missing")
    for invalid in ("0x", "0xdeadbeef", "0xac6fc81a00", "0xzz", "not hex"):
        with pytest.raises(ValueError):
            decode_chains_call(invalid)
    for key, decode in (("getChain", decode_get_chain), ("params", decode_chains_params)):
        data = vectors[key]["returndata"]
        for invalid in ("0x", data[:-2], data + "00" * 32):
            with pytest.raises(ValueError):
                decode(invalid)
    raw = bytearray.fromhex(vectors["getChain"]["returndata"][2:])
    raw[32:64] = (33).to_bytes(32, "big")
    with pytest.raises(ValueError):
        decode_get_chain("0x" + raw.hex())
    raw = bytearray.fromhex(vectors["getChain"]["returndata"][2:])
    raw[-1] = 1
    with pytest.raises(ValueError):
        decode_get_chain("0x" + raw.hex())
    with pytest.raises(ValueError):
        decode_chains_event(["0x" + "00" * 32], "0x")
    with pytest.raises(ValueError):
        decode_chains_event([], "0x")
    for base in ("", "/relative", "https://rpc.example/?q=1", "https://rpc.example/#fragment"):
        with pytest.raises(ValueError):
            chain_rpc_url(base, 1)
