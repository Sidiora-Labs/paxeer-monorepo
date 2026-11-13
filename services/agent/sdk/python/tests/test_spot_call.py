from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import get_args

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from layerx_sdk.exchange import (
    AbiType,
    EventInput,
    EventSpec,
    PrecompileAbiError,
    PrecompileCall,
    abi_selector,
    decode_event_with_spec,
    encode_abi_call,
    encode_signature_call,
)
from layerx_sdk.generated.precompile_writes import (
    LAUNCHPAD_EVENT_INPUTS,
    LAUNCHPAD_PRECOMPILE,
    SPOT_PRECOMPILES,
    SPOT_WRITES,
)
from layerx_sdk.launchpad import LAUNCHPAD_EVENTS, decode_launchpad_event, launchpad_call
from layerx_sdk.spot import spot_call


def _typescript_table(name: str) -> dict:
    source = (
        Path(__file__).resolve().parents[2] / "typescript/src/generated/precompile-writes.ts"
    ).read_text()
    match = re.search(rf"export const {name} = (\{{.*?\n\}}) as const", source, re.DOTALL)
    assert match is not None, name
    table = re.sub(r"(?m)^(\s*)(\w+):", r'\1"\2":', match[1])
    return json.loads(re.sub(r",\s*([}\]])", r"\1", table))


def _argument(kind: str) -> object:
    return {
        "address": "0x" + "12" * 20,
        "uint256": 7,
        "bool": True,
        "address[]": ["0x" + "12" * 20, "0x" + "34" * 20],
        "uint256[]": [7, 11],
        "bytes[]": ["0x010203", "0x" + "ab" * 33],
    }[kind]


def test_spot_call_matches_typescript_tables() -> None:
    addresses = _typescript_table("SPOT_PRECOMPILES")
    writes = _typescript_table("SPOT_WRITES")
    assert set(addresses) == {"pecor", "orders", "stopOrders", "quoter", "vault"}
    assert SPOT_PRECOMPILES == addresses
    assert SPOT_WRITES == {
        contract: {signature: write["payable"] for signature, write in table.items()}
        for contract, table in writes.items()
    }
    assert set(SPOT_WRITES) == set(SPOT_PRECOMPILES)
    for contract, table in writes.items():
        assert table, contract
        golden = json.loads(
            (
                Path(__file__).resolve().parents[5]
                / f"precompiles/spot/testdata/{contract.lower()}.golden.json"
            ).read_text()
        )
        selectors = {entry["signature"]: entry["id"] for entry in golden if entry["kind"] == "function"}
        for signature, write in table.items():
            values = [_argument(kind) for kind in write["types"]]
            call = spot_call(contract, signature, values)
            assert isinstance(call, PrecompileCall)
            assert call.to == addresses[contract], (contract, signature)
            assert call.data[:10] == selectors[signature] == abi_selector(signature), (contract, signature)
            assert (len(bytes.fromhex(call.data[2:])) - 4) % 32 == 0
            assert call.value == 0
            if write["payable"]:
                assert spot_call(contract, signature, values, value=17) == PrecompileCall(
                    addresses[contract], call.data, 17
                )
            else:
                with pytest.raises(PrecompileAbiError, match="^invalid_value:"):
                    spot_call(contract, signature, values, value=17)

    assert spot_call("vault", "depositNative()", [], value=17) == PrecompileCall(
        "0x0000000000000000000000000000000000001025", "0xdb6b5246", 17
    )
    assert spot_call("orders", "cancelLimitOrder(uint256)", [7]).data == (
        abi_selector("cancelLimitOrder(uint256)") + f"{7:064x}"
    )


def test_spot_call_refuses_unknown_write() -> None:
    for contract, signature, values in (
        ("unknown", "depositNative()", []),
        (SPOT_PRECOMPILES["vault"], "depositNative()", []),
        ("pecor", "unknown()", []),
        ("pecor", "depositNative()", []),
        ("vault", "swapExactInNative(address,uint256,uint256)", ["0x" + "12" * 20, 1, 2]),
        ("orders", "cancelLimitOrder(uint)", [7]),
        ("quoter", "owner()", []),
    ):
        with pytest.raises(PrecompileAbiError, match="^invalid_value:") as caught:
            spot_call(contract, signature, values)
        assert caught.value.code == "invalid_value"

    for contract in SPOT_PRECOMPILES:
        with pytest.raises(PrecompileAbiError, match="^invalid_value:"):
            spot_call(contract, "unknown()", [])

    for value in (-1, 0.5, "17", None):
        with pytest.raises(PrecompileAbiError, match="^invalid_value:"):
            spot_call("vault", "depositNative()", [], value=value)

    for values in ([], [-1], [1 << 256], ["7"]):
        with pytest.raises(PrecompileAbiError, match="^invalid_value:"):
            spot_call("orders", "cancelLimitOrder(uint256)", values)


def test_precompile_event_types_match_generated_tables() -> None:
    typescript = (
        Path(__file__).resolve().parents[2]
        / "typescript/src/generated/precompile-writes.ts"
    ).read_text()
    typescript_event_types = set(re.findall(r'"type":"([^"]+)"', typescript))
    python_event_types = {
        kind for _, inputs in LAUNCHPAD_EVENT_INPUTS for _, kind, _ in inputs
    }
    abi_types = set(get_args(AbiType))
    assert typescript_event_types == {
        "address", "bool", "bytes16", "bytes32", "int256", "string", "uint8",
        "uint32", "uint64", "uint256",
    }
    assert python_event_types == {
        "address", "bool", "bytes32", "string", "uint8", "uint32", "uint64",
        "uint256",
    }
    assert typescript_event_types <= abi_types
    assert python_event_types <= abi_types


def test_uint32_launchpad_chain_events_round_trip() -> None:
    maximum = (1 << 32) - 1
    signature = "openChainRevenueEpoch(uint32)"
    assert launchpad_call(signature, [maximum]).data == (
        abi_selector(signature) + f"{maximum:064x}"
    )
    assert encode_abi_call("openChainRevenueEpoch", ["uint32"], [maximum]) == (
        abi_selector(signature) + f"{maximum:064x}"
    )
    for call in (
        lambda: launchpad_call(signature, [maximum + 1]),
        lambda: encode_abi_call("openChainRevenueEpoch", ["uint32"], [maximum + 1]),
    ):
        with pytest.raises(PrecompileAbiError, match="^invalid_value:"):
            call()

    names = (
        "ChainLaunched",
        "ChainRevenueEpochOpened",
        "ChainRevenueClaimed",
        "LaunchedChainRetired",
    )
    specs = tuple(spec for spec in LAUNCHPAD_EVENTS if spec.name in names)
    assert tuple(spec.name for spec in specs) == names
    for spec in specs:
        assert spec.inputs[0] == EventInput("chain", "uint32", True)
        fields = {
            item.name: (
                "0x" + "ab" * 20
                if item.type == "address"
                else maximum
                if item.name == "chain"
                else 9
                if item.type == "uint64"
                else 10
            )
            for item in spec.inputs
        }
        encoded = {
            item.name: (
                "00" * 12 + "ab" * 20
                if item.type == "address"
                else f"{fields[item.name]:064x}"
            )
            for item in spec.inputs
        }
        topics = [spec.topic0] + [
            "0x" + encoded[item.name] for item in spec.inputs if item.indexed
        ]
        data = "0x" + "".join(
            encoded[item.name] for item in spec.inputs if not item.indexed
        )
        assert decode_launchpad_event(LAUNCHPAD_PRECOMPILE, topics, data).fields == fields
        with pytest.raises(PrecompileAbiError, match="^non_canonical_word:"):
            decode_launchpad_event(
                LAUNCHPAD_PRECOMPILE,
                [topics[0], "0x" + f"{maximum + 1:064x}", *topics[2:]],
                data,
            )


def test_precompile_signed_and_fixed_bytes_round_trip() -> None:
    value = "0x" + "ab" * 16
    encoded = "ff" * 32 + "ab" * 16 + "00" * 16
    assert encode_abi_call("signedBytes", ["int256", "bytes16"], [-1, value]) == (
        abi_selector("signedBytes(int256,bytes16)") + encoded
    )
    assert encode_signature_call("signedBytes(int256,bytes16)", [-1, value]) == (
        abi_selector("signedBytes(int256,bytes16)") + encoded
    )
    spec = EventSpec(
        "SignedBytes",
        LAUNCHPAD_PRECOMPILE,
        (EventInput("signed", "int256"), EventInput("fixed", "bytes16")),
    )
    assert decode_event_with_spec(
        spec, LAUNCHPAD_PRECOMPILE, [spec.topic0], "0x" + encoded
    ).fields == {"signed": -1, "fixed": value}
    with pytest.raises(PrecompileAbiError, match="^invalid_value:"):
        encode_abi_call("signedBytes", ["int256", "bytes16"], [1 << 255, value])
    with pytest.raises(PrecompileAbiError, match="^non_canonical_word:"):
        decode_event_with_spec(
            spec, LAUNCHPAD_PRECOMPILE, [spec.topic0], "0x" + encoded[:-2] + "01"
        )
