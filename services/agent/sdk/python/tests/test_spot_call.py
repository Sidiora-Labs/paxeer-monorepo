from __future__ import annotations

import json
import re
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from layerx_sdk.exchange import PrecompileAbiError, PrecompileCall, abi_selector
from layerx_sdk.generated.precompile_writes import SPOT_PRECOMPILES, SPOT_WRITES
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
