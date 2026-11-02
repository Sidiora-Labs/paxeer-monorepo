from hashlib import sha256
import importlib.util
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys


FIXTURES = Path(__file__).with_name("fixtures")
PROGRAMS = "0x000000000000000000000000000000000000101d"
ADDR = "0x0000000000000000000000000000000000001004"
LEDGER = "0x000000000000000000000000000000000000101a"
CALL_TYPES = (
    "bytes32", "uint16", "string", "bytes", "bytes", "bytes", "uint32",
    "uint64[7]", "uint128",
)
BINDINGS = {
    "0e0dd0e63aae0420ae0e2aa896013a983facb1ce537be3d368896d81412d7bb9":
        "0x9858effd232b4033e47d90003d41ec34ecaeda94",
    "aa92353b9469d1f56b28426dfa72ce3f3fabc7a5c41ee61a51f63bcbd98b942a":
        "0x6fac4d18c912343bf86fa7049364dd4e424ab9c0",
    "8020367c3d7d568d84ef8d5b2725431dc7bb21daf42630cceb95cc09cd9e30ba":
        "0x6006ef1944fb519a746d00cdaf715cbd27a5a008",
    "f4b28c8bd83c578c1a3b6243c773925511e74eb351384a499ec75f5c02bb838e":
        "0xa5cd5a7a353dad72643512f25f395ec1857bc1ca",
    "a4b827a1dde86a5ba3c209ee9e8827aed3b62a822bf7ace0f29d24f1e78e7d4f":
        "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
    "1357c9f8949c28208dea838a96363e35d3b6437ee31c78c3634acf09b4aa0ff1":
        "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266",
}


def read_fixture(name):
    return json.loads((FIXTURES / name).read_text())


def encode_static(kind, value):
    array = re.fullmatch(r"(uint\d+)\[(\d+)\]", kind)
    if array:
        assert len(value) == int(array[2])
        return b"".join(encode_static(array[1], item) for item in value)
    if kind == "bytes32":
        assert len(value) == 32
        return value
    assert re.fullmatch(r"uint\d+", kind)
    assert 0 <= value < 2 ** int(kind[4:])
    return value.to_bytes(32, "big")


def abi_encode(types, values):
    assert len(types) == len(values)
    head = []
    tails = []
    for kind, value in zip(types, values):
        if kind in ("bytes", "string"):
            raw = value.encode("utf-8") if kind == "string" else value
            padded_size = ((len(raw) + 31) // 32) * 32
            tails.append(len(raw).to_bytes(32, "big") + raw.ljust(padded_size, b"\0"))
            head.append(None)
        else:
            head.append(encode_static(kind, value))
    offset = sum(32 if word is None else len(word) for word in head)
    tail_index = 0
    for index, word in enumerate(head):
        if word is None:
            head[index] = offset.to_bytes(32, "big")
            offset += len(tails[tail_index])
            tail_index += 1
    return b"".join(head + tails)


def call_arguments(fixture):
    payload = bytes.fromhex(fixture["payload_hex"])
    cursor = 0

    def take(size):
        nonlocal cursor
        result = payload[cursor : cursor + size]
        assert len(result) == size
        cursor += size
        return result

    def number(size):
        return int.from_bytes(take(size), "big")

    program = take(32)
    abi = number(2)
    sizes = [number(size) for size in (2, 4, 2, 4)]
    capacity = number(4)
    budget = [number(8) for _ in range(7)]
    entry, data, capabilities, access = [take(size) for size in sizes]
    assert cursor == len(payload)
    assert budget == [int(value) for value in fixture["resources"]]
    assert payload in bytes.fromhex(fixture["signed_activity_hex"])
    return (program, abi, entry.decode("utf-8"), data, capabilities, access,
            capacity, budget, int(fixture["fee_limit"]))


def assert_resolution(account):
    address = account["evm_address"]
    public = account["layerx_public_key"]
    assert re.fullmatch(r"0x[0-9a-f]{40}", address)
    assert address == BINDINGS[public]
    query = account["resolution"]
    assert query["to"] == ADDR
    assert query["chainId"] == 125
    expected = abi_encode(("bytes32",), (bytes.fromhex(public),))
    assert query["data"] == "0xc413bad3" + expected.hex()
    assert query["result"] == "0x" + bytes.fromhex(address[2:]).rjust(32, b"\0").hex()
    result = bytes.fromhex(query["result"][2:])
    assert len(result) == 32 and result[:12] == bytes(12)
    assert result[12:] == bytes.fromhex(address[2:])


def test_call_vectors_wrap_payload_in_layerxprograms_call(tmp_path):
    originals = {}
    for version in (3, 4):
        name = f"native-program-call-v{version}.json"
        fixture = read_fixture(name)
        originals[name] = fixture
        arguments = call_arguments(fixture)
        expected = "0x54bd2b48" + abi_encode(CALL_TYPES, arguments).hex()
        assert fixture["evm"] == {
            "to": PROGRAMS, "chainId": 125,
            "selector": "0x54bd2b48", "calldata": expected,
        }
        shutil.copyfile(FIXTURES / name, tmp_path / name)
    generator = FIXTURES / "generate_native_program_fixture.py"
    result = subprocess.run(
        [sys.executable, str(generator), "--check"], capture_output=True, text=True
    )
    assert result.returncode == 0, result.stderr
    copied = tmp_path / generator.name
    shutil.copyfile(generator, copied)
    changed = dict(originals["native-program-call-v4.json"])
    changed["evm"] = {**changed["evm"], "calldata": "0x54bd2b48"}
    (tmp_path / "native-program-call-v4.json").write_text(json.dumps(changed))
    result = subprocess.run(
        [sys.executable, str(copied), "--check"], capture_output=True, text=True
    )
    assert result.returncode != 0
    assert "EVM call fixture drift" in result.stderr
    result = subprocess.run(
        [sys.executable, str(copied)], capture_output=True, text=True
    )
    assert result.returncode == 0, result.stderr
    for name, original in originals.items():
        assert json.loads((tmp_path / name).read_text()) == original
    spec = importlib.util.spec_from_file_location("native_fixture", generator)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    for fixture in originals.values():
        assert module.evm(fixture) == fixture["evm"]
        payload = bytes.fromhex(fixture["payload_hex"])
        for damaged in (payload[:105], payload[:-1], payload + b"\0"):
            try:
                module.calldata(damaged, int(fixture["fee_limit"]))
            except ValueError:
                pass
            else:
                raise AssertionError("generator accepted malformed native payload")


def test_account_derivation_records_evm_address():
    fixture = read_fixture("account-derivation-v1.json")
    seen = set()
    for group in fixture["mnemonic_vectors"]:
        assert group["accounts"]
        for account in group["accounts"]:
            assert_resolution(account)
            public = account["layerx_public_key"]
            assert account["did"] == "did:layerx:" + public
            assert account["bind"]["to"] == ADDR
            assert account["bind"]["calldata"].startswith("dd9aa628" + public)
            seen.add(public)
    wallet = fixture["wallet_signature"]
    assert wallet["vectors"]
    for account in wallet["vectors"]:
        assert_resolution(account)
        assert account["evm_address"] == wallet["evm_address"]
        assert account["typed_data"]["message"]["address"] == account["evm_address"]
        assert account["did"] == "did:layerx:" + account["layerx_public_key"]
        seen.add(account["layerx_public_key"])
    assert seen == set(BINDINGS)


def main_account_id(endpoint):
    name = endpoint["account"].encode("utf-8")
    assert endpoint["account"] == "agent:did:layerx:" + endpoint["layerx_public_key"] + ":main"
    return sha256(b"LX:ACCOUNT:v1" + len(name).to_bytes(4, "big") + name).digest()


def test_intent_plan_legs_are_evm_calls():
    fixture = read_fixture("intent-plan-v1.json")
    request = fixture["plan_request"]
    result = fixture["plan_result"]
    legs = result["legs"]
    assert len(legs) == 2
    assert legs[0]["source"] == request["source"]
    assert legs[-1]["destination"] == request["destination"]
    calls = set()
    for index, leg in enumerate(legs):
        assert leg["index"] == index
        assert "account_sequence" not in leg
        assert leg["chainId"] == 125
        assert leg["to"] == LEDGER
        assert_resolution(leg["source"])
        assert_resolution(leg["destination"])
        if index:
            assert leg["source"]["account"] == legs[index - 1]["destination"]["account"]
            assert leg["source"]["evm_address"] == legs[index - 1]["destination"]["evm_address"]
        payment = leg["payment"]
        assert payment["from"] == main_account_id(leg["source"]).hex()
        assert payment["to"] == main_account_id(leg["destination"]).hex()
        assert payment["assetId"] == request["asset_id"]
        assert payment["amount"] == leg["money"]["amount"] == request["money"]["amount"]
        signing = result["signing_requirements"][index]
        assert signing["leg_index"] == index
        assert payment["paymentReference"] == signing["action_key"]
        expected = abi_encode(
            ("bytes32", "bytes32", "bytes32", "uint128", "uint64", "bytes32"),
            (main_account_id(leg["source"]), main_account_id(leg["destination"]),
             bytes.fromhex(request["asset_id"]), int(leg["money"]["amount"]),
             payment["sequence"], bytes.fromhex(signing["action_key"])),
        )
        assert leg["data"] == "0x78ea11db" + expected.hex()
        assert len(bytes.fromhex(leg["data"][2:])) == 196
        calls.add((leg["to"], leg["data"], leg["chainId"]))
    assert len(calls) == len(legs)
