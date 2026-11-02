from __future__ import annotations

import json
import re
import urllib.parse
import urllib.request
from collections.abc import Sequence
from types import MappingProxyType
from typing import Literal, overload

from .account_derivation import keccak256
from .exchange import PrecompileCall
from .px import PxClient, PxRpcError, _MAX_REQUEST_BYTES

CHAINS_PRECOMPILE = "0x000000000000000000000000000000000000101e"
_METHOD_TYPES = {
    "createChain": ("string", "uint64", "uint64", "uint16"),
    "fundChain": ("uint32",),
    "pauseChain": ("uint32",),
    "resumeChain": ("uint32",),
    "retireChain": ("uint32",),
    "governancePause": ("uint32", "bool"),
    "setOperator": ("uint32", "address"),
    "getChain": ("uint32",),
    "chainByName": ("string",),
    "chainCount": (),
    "chainIdOf": ("uint32",),
    "params": (),
}
CHAINS_SIGNATURES = MappingProxyType(
    {name: f"{name}({','.join(types)})" for name, types in _METHOD_TYPES.items()}
)
CHAINS_SELECTORS = MappingProxyType(
    {
        name: "0x" + keccak256(sig.encode("ascii"))[:4].hex()
        for name, sig in CHAINS_SIGNATURES.items()
    }
)
_EVENT_FIELDS = {
    "ChainCreated": (
        ("id", "uint32", True),
        ("name", "string", False),
        ("operator", "address", True),
        ("chainId", "uint64", False),
    ),
    "ChainStatusChanged": (("id", "uint32", True), ("status", "uint8", False)),
    "ChainFunded": (
        ("id", "uint32", True),
        ("from", "address", True),
        ("amount", "uint256", False),
    ),
    "ChainOperatorChanged": (("id", "uint32", True), ("operator", "address", True)),
}
CHAINS_EVENT_SIGNATURES = MappingProxyType(
    {
        name: f"{name}({','.join(field[1] for field in fields)})"
        for name, fields in _EVENT_FIELDS.items()
    }
)
CHAINS_EVENT_TOPICS = MappingProxyType(
    {
        name: "0x" + keccak256(sig.encode("ascii")).hex()
        for name, sig in CHAINS_EVENT_SIGNATURES.items()
    }
)
_CHAIN_FIELDS = (
    ("id", "uint32"),
    ("name", "string"),
    ("operator", "address"),
    ("status", "uint8"),
    ("chainId", "uint64"),
    ("gasAllowance", "uint64"),
    ("byteAllowance", "uint64"),
    ("feeShareBps", "uint16"),
    ("bond", "uint256"),
    ("baseFeePerGas", "uint256"),
    ("createdHeight", "uint64"),
)
_PARAMS_FIELDS = (
    ("authority", "address"),
    ("minBond", "uint256"),
    ("creationFee", "uint256"),
    ("minGasAllowance", "uint64"),
    ("maxGasAllowance", "uint64"),
    ("totalGasAllowanceLeft", "uint64"),
    ("maxChains", "uint32"),
)
_READ_TYPES = {"chainByName": "uint32", "chainCount": "uint32", "chainIdOf": "uint64"}


def _uint(value: object, bits: int) -> int:
    if type(value) is not int or not 0 <= value < 1 << bits:
        raise ValueError(f"invalid-chains-uint{bits}")
    return value


def _unhex(value: str) -> bytes:
    if (
        not isinstance(value, str)
        or re.fullmatch(r"0x(?:[0-9a-fA-F]{2})*", value) is None
    ):
        raise ValueError("invalid-chains-hex")
    return bytes.fromhex(value[2:])


def _encode_abi(types: Sequence[str], values: Sequence[object]) -> bytes:
    if len(types) != len(values):
        raise ValueError("invalid-chains-argument-count")
    head: list[bytes] = []
    tail = b""
    for kind, value in zip(types, values):
        if kind == "string":
            if not isinstance(value, str):
                raise ValueError("invalid-chains-string")
            raw = value.encode("utf-8")
            head.append((len(types) * 32 + len(tail)).to_bytes(32, "big"))
            tail += len(raw).to_bytes(32, "big") + raw + bytes(-len(raw) % 32)
        elif kind == "address":
            if (
                not isinstance(value, str)
                or re.fullmatch(r"0x[0-9a-fA-F]{40}", value) is None
            ):
                raise ValueError("invalid-chains-address")
            head.append(bytes(12) + _unhex(value))
        elif kind == "bool":
            if type(value) is not bool:
                raise ValueError("invalid-chains-bool")
            head.append(int(value).to_bytes(32, "big"))
        else:
            head.append(_uint(value, int(kind[4:])).to_bytes(32, "big"))
    return b"".join(head) + tail


def _decode_abi(types: Sequence[str], raw: bytes) -> tuple[str | int | bool, ...]:
    if len(raw) < len(types) * 32 or len(raw) % 32:
        raise ValueError("invalid-chains-abi-length")
    values: list[str | int | bool] = []
    for index, kind in enumerate(types):
        word = raw[index * 32 : (index + 1) * 32]
        number = int.from_bytes(word, "big")
        if kind == "string":
            if number < len(types) * 32 or number % 32 or number + 32 > len(raw):
                raise ValueError("invalid-chains-string-offset")
            length = int.from_bytes(raw[number : number + 32], "big")
            start = number + 32
            if start + length > len(raw):
                raise ValueError("invalid-chains-string-length")
            values.append(raw[start : start + length].decode("utf-8"))
        elif kind == "address":
            if any(word[:12]):
                raise ValueError("invalid-chains-address")
            values.append("0x" + word[12:].hex())
        elif kind == "bool":
            if number not in (0, 1):
                raise ValueError("invalid-chains-bool")
            values.append(bool(number))
        else:
            values.append(_uint(number, int(kind[4:])))
    if _encode_abi(types, values) != raw:
        raise ValueError("noncanonical-chains-abi")
    return tuple(values)


def chains_call(method: str, *args: object, value: int = 0) -> PrecompileCall:
    if method not in _METHOD_TYPES:
        raise ValueError("unknown-chains-method")
    _uint(value, 256)
    if value and method not in ("createChain", "fundChain"):
        raise ValueError("nonpayable-chains-method")
    data = CHAINS_SELECTORS[method] + _encode_abi(_METHOD_TYPES[method], args).hex()
    return PrecompileCall(CHAINS_PRECOMPILE, data, value)


def decode_chains_call(data: str) -> tuple[str, tuple[object, ...]]:
    raw = _unhex(data)
    for method, selector in CHAINS_SELECTORS.items():
        if raw[:4] == _unhex(selector):
            return method, _decode_abi(_METHOD_TYPES[method], raw[4:])
    raise ValueError("unknown-chains-selector")


def _decode_fields(fields: Sequence[tuple[str, str]], data: str) -> dict[str, str | int]:
    values = _decode_abi(tuple(kind for _, kind in fields), _unhex(data))
    return {
        name: str(value) if kind == "uint256" else value
        for (name, kind), value in zip(fields, values)
    }


def decode_get_chain(data: str) -> dict[str, str | int]:
    return _decode_fields(_CHAIN_FIELDS, data)


def decode_chains_params(data: str) -> dict[str, str | int]:
    return _decode_fields(_PARAMS_FIELDS, data)


def decode_chains_event(topics: Sequence[str], data: str) -> dict[str, str | int]:
    if isinstance(topics, (str, bytes)) or not topics:
        raise ValueError("invalid-chains-event-topics")
    topic0 = _unhex(topics[0])
    for name, topic in CHAINS_EVENT_TOPICS.items():
        if topic0 != _unhex(topic):
            continue
        fields = _EVENT_FIELDS[name]
        indexed = tuple(field for field in fields if field[2])
        plain = tuple(field for field in fields if not field[2])
        if len(topics) != len(indexed) + 1:
            raise ValueError("invalid-chains-event-topics")
        decoded = _decode_fields(tuple((key, kind) for key, kind, _ in plain), data)
        for (key, kind, _), value in zip(indexed, topics[1:]):
            decoded[key] = _decode_abi((kind,), _unhex(value))[0]
        return {"event": name, **decoded}
    raise ValueError("unknown-chains-event")


def virtual_chain_id(id: int) -> int:
    return 125000000 + _uint(id, 32)


def chain_rpc_url(base: str, id: int) -> str:
    _uint(id, 32)
    if not isinstance(base, str):
        raise ValueError("invalid-chains-rpc-base")
    url = urllib.parse.urlsplit(base)
    if (
        url.scheme not in ("http", "https")
        or not url.hostname
        or url.username
        or url.password
        or url.query
        or url.fragment
    ):
        raise ValueError("invalid-chains-rpc-base")
    return f"{base.rstrip('/')}/chain/{id}"


class ChainsClient(PxClient):
    def request(self, method: str, params: list[object]) -> str:
        if method != "eth_call" or not isinstance(params, list):
            raise ValueError("invalid-chains-rpc-method")
        self._id += 1
        request_id = self._id
        body = json.dumps(
            {"jsonrpc": "2.0", "id": request_id, "method": method, "params": params},
            ensure_ascii=True,
            allow_nan=False,
            separators=(",", ":"),
        ).encode("utf-8")
        if len(body) > _MAX_REQUEST_BYTES:
            raise ValueError("px-request-too-large")
        headers = {
            "Accept": "application/json",
            "Content-Type": "application/json",
            "Content-Length": str(len(body)),
            "User-Agent": "layerx-python/0.1.0",
        }
        if self._credential is not None:
            headers["Authorization"] = self._credential.use()
        request = urllib.request.Request(
            self.endpoint, method="POST", headers=headers, data=body
        )
        with self._opener.open(request, timeout=self._timeout) as response:
            if (
                response.status != 200
                or response.headers.get_content_type() != "application/json"
            ):
                raise ValueError("invalid-px-http-answer")
            raw = response.read(self._maximum_response_bytes + 1)
        if len(raw) > self._maximum_response_bytes:
            raise ValueError("px-body-too-large")
        answer = json.loads(raw.decode("utf-8"))
        if (
            not isinstance(answer, dict)
            or answer.get("jsonrpc") != "2.0"
            or type(answer.get("id")) is not int
            or answer["id"] != request_id
            or ("error" in answer) == ("result" in answer)
        ):
            raise ValueError("invalid-px-envelope")
        if "error" in answer:
            error = answer["error"]
            if (
                not isinstance(error, dict)
                or type(error.get("code")) is not int
                or not isinstance(error.get("message"), str)
            ):
                raise ValueError("invalid-px-error")
            raise PxRpcError(error["code"], error["message"], error.get("data"))
        result = answer["result"]
        _unhex(result)
        return result

    @overload
    def read(
        self, method: Literal["getChain", "params"], *args: object, block: str = "latest"
    ) -> dict[str, str | int]: ...

    @overload
    def read(
        self,
        method: Literal["chainByName", "chainCount", "chainIdOf"],
        *args: object,
        block: str = "latest",
    ) -> int: ...

    def read(
        self, method: str, *args: object, block: str = "latest"
    ) -> dict[str, str | int] | int:
        if method not in ("getChain", "params", *_READ_TYPES):
            raise ValueError("invalid-chains-read-method")
        if (
            not isinstance(block, str)
            or re.fullmatch(
                r"latest|earliest|pending|safe|finalized|0x(?:0|[1-9a-fA-F][0-9a-fA-F]*)",
                block,
            ) is None
        ):
            raise ValueError("invalid-chains-block")
        call = chains_call(method, *args)
        data = self.request("eth_call", [{"to": call.to, "data": call.data}, block])
        if method == "getChain":
            return decode_get_chain(data)
        if method == "params":
            return decode_chains_params(data)
        result = _decode_abi((_READ_TYPES[method],), _unhex(data))[0]
        if type(result) is not int:
            raise ValueError("invalid-chains-read-result")
        return result

    def get_chain(self, id: int, *, block: str = "latest") -> dict[str, str | int]:
        return self.read("getChain", id, block=block)

    def params(self, *, block: str = "latest") -> dict[str, str | int]:
        return self.read("params", block=block)

    def chain_by_name(self, name: str, *, block: str = "latest") -> int:
        return self.read("chainByName", name, block=block)

    def chain_count(self, *, block: str = "latest") -> int:
        return self.read("chainCount", block=block)

    def chain_id_of(self, id: int, *, block: str = "latest") -> int:
        return self.read("chainIdOf", id, block=block)
