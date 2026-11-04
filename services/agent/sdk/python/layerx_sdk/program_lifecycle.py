from __future__ import annotations

import struct
from dataclasses import dataclass
from hashlib import sha256

from .generated.receipt import supports_program_guest_abi
from .program_wire import DecodedSignedProgramCall, bind_signed_program_lifecycle


MAX_DEPLOY_PAYLOAD_BYTES = 524_288
DEPLOY_ENVELOPE_BYTES = 104


@dataclass(frozen=True)
class NativeProgramLifecycleRequest:
    ordinal: int
    payload: bytes
    signed_activity: bytes

    def __post_init__(self) -> None:
        object.__setattr__(self, "payload", bytes(self.payload))
        object.__setattr__(self, "signed_activity", bytes(self.signed_activity))
        decoders = {1: NativeProgramDeploy.decode, 2: NativeProgramUpgrade.decode, 7: NativeProgramWindDown.decode}
        if type(self.ordinal) is not int or self.ordinal not in decoders or decoders[self.ordinal](self.payload).encode() != self.payload:
            raise ValueError("lifecycle payload")
        self.bind()

    @classmethod
    def deploy(cls, value: NativeProgramDeploy, signed: bytes) -> NativeProgramLifecycleRequest:
        return cls(1, value.encode(), bytes(signed))

    @classmethod
    def upgrade(cls, value: NativeProgramUpgrade, signed: bytes) -> NativeProgramLifecycleRequest:
        return cls(2, value.encode(), bytes(signed))

    @classmethod
    def wind_down(cls, value: NativeProgramWindDown, signed: bytes) -> NativeProgramLifecycleRequest:
        return cls(7, value.encode(), bytes(signed))

    def bind(self, key: str | None = None) -> DecodedSignedProgramCall:
        return bind_signed_program_lifecycle(self.signed_activity, self.payload, self.ordinal, key)


def _fixed(value: bytes) -> bytes:
    if not isinstance(value, bytes) or len(value) != 32:
        raise ValueError("expected 32 bytes")
    return value


def _code(program: bytes, abi: int, wasm: bytes, digest: bytes) -> None:
    if (_fixed(program) == bytes(32) or not supports_program_guest_abi(abi)
            or not wasm.startswith(b"\0asm\x01\0\0\0") or len(wasm) > 1_048_576
            or _fixed(digest) != sha256(wasm).digest()):
        raise ValueError("invalid program code")


@dataclass(frozen=True)
class NativeProgramDeploy:
    program_id: bytes
    guest_abi: int
    authority: bytes
    new_hash: bytes
    wasm: bytes
    interface: bytes | None = None
    policy: int = 0

    @property
    def wasm_budget(self) -> int:
        return MAX_DEPLOY_PAYLOAD_BYTES - DEPLOY_ENVELOPE_BYTES - (
            0 if self.interface is None else 4 + len(self.interface)
        )

    def encode(self) -> bytes:
        if (type(self.policy) is not int or self.policy not in (0, 1)
                or (self.policy == 0) != (_fixed(self.authority) == bytes(32))
                or self.interface is not None and not 0 < len(self.interface) <= 952):
            raise ValueError("invalid deploy policy or interface")
        if len(self.wasm) > self.wasm_budget:
            raise ValueError(
                f"deployment payload exceeds {MAX_DEPLOY_PAYLOAD_BYTES} bytes; "
                f"remaining WASM budget is {self.wasm_budget} bytes after envelope/interface"
            )
        _code(self.program_id, self.guest_abi, self.wasm, self.new_hash)
        return (self.program_id + struct.pack(">HBB", self.guest_abi, self.policy, 0)
                + self.authority + self.new_hash + struct.pack(">I", len(self.wasm))
                + (b"" if self.interface is None else struct.pack(">I", len(self.interface)) + self.interface) + self.wasm)

    @classmethod
    def decode(cls, payload: bytes) -> NativeProgramDeploy:
        if len(payload) < 104 or payload[35] != 0:
            raise ValueError("invalid deploy framing")
        length = int.from_bytes(payload[100:104], "big")
        interface = None
        offset = 104
        if len(payload) != offset + length:
            if len(payload) < 108:
                raise ValueError("truncated interface")
            size = int.from_bytes(payload[104:108], "big")
            offset = 108 + size
            interface = payload[108:offset]
        value = cls(payload[:32], int.from_bytes(payload[32:34], "big"), payload[36:68], payload[68:100], payload[offset:], interface, payload[34])
        if len(value.wasm) != length or value.encode() != payload:
            raise ValueError("noncanonical deploy")
        return value


@dataclass(frozen=True)
class NativeProgramUpgrade:
    program_id: bytes
    guest_abi: int
    old_hash: bytes
    new_hash: bytes
    wasm: bytes
    migration_hook: bytes = b""
    interface: bytes | None = None
    clear_interface: bool = False

    def encode(self) -> bytes:
        _code(self.program_id, self.guest_abi, self.wasm, self.new_hash)
        if (len(self.migration_hook) > 65535 or type(self.clear_interface) is not bool
                or self.clear_interface and self.interface is None
                or self.interface is not None and (len(self.interface) > 952 or not self.interface and not self.clear_interface)):
            raise ValueError("invalid upgrade flags or interface")
        flags = int(bool(self.migration_hook)) | (int(self.clear_interface) << 1)
        return (self.program_id + struct.pack(">HBB", self.guest_abi, flags, 0) + _fixed(self.old_hash)
                + self.new_hash + struct.pack(">HI", len(self.migration_hook), len(self.wasm))
                + (b"" if self.interface is None else struct.pack(">I", len(self.interface)))
                + self.migration_hook + (self.interface or b"") + self.wasm)

    @classmethod
    def decode(cls, payload: bytes) -> NativeProgramUpgrade:
        if len(payload) < 106 or payload[35] != 0 or payload[34] & 0xfc:
            raise ValueError("invalid upgrade framing")
        hook_length, wasm_length = struct.unpack(">HI", payload[100:106])
        clear = bool(payload[34] & 2)
        offset, interface_length = 106, None
        if len(payload) != 106 + hook_length + wasm_length or clear:
            if len(payload) < 110:
                raise ValueError("truncated interface")
            interface_length = int.from_bytes(payload[106:110], "big")
            offset = 110
        hook = payload[offset:offset + hook_length]
        offset += hook_length
        interface = None if interface_length is None else payload[offset:offset + interface_length]
        offset += interface_length or 0
        value = cls(payload[:32], int.from_bytes(payload[32:34], "big"), payload[36:68], payload[68:100], payload[offset:], hook, interface, clear)
        if len(value.wasm) != wasm_length or value.encode() != payload:
            raise ValueError("noncanonical upgrade")
        return value


@dataclass(frozen=True)
class NativeProgramWindDown:
    program_id: bytes
    operation: int
    account: bytes = bytes(32)
    asset: bytes = bytes(32)
    destination: bytes = bytes(32)
    seed: bytes = b""
    exit_program: bytes = bytes(32)
    deadline_batch: int = 0

    def encode(self) -> bytes:
        if _fixed(self.program_id) == bytes(32) or type(self.operation) is not int:
            raise ValueError("invalid wind-down")
        prefix = self.program_id + bytes([self.operation])
        if self.operation == 1 and len(self.seed) <= 128:
            return prefix + _fixed(self.account) + _fixed(self.asset) + _fixed(self.destination) + struct.pack(">H", len(self.seed)) + self.seed
        if self.operation == 2 and type(self.deadline_batch) is int and 0 <= self.deadline_batch < 1 << 64:
            return prefix + _fixed(self.exit_program) + struct.pack(">Q", self.deadline_batch)
        if self.operation == 3:
            return prefix
        if self.operation == 4:
            return prefix + _fixed(self.account)
        raise ValueError("invalid wind-down operation")

    @classmethod
    def decode(cls, payload: bytes) -> NativeProgramWindDown:
        if len(payload) < 33:
            raise ValueError("truncated wind-down")
        operation = payload[32]
        if operation == 1 and len(payload) >= 131:
            value = cls(payload[:32], operation, payload[33:65], payload[65:97], payload[97:129], payload[131:])
        elif operation == 2 and len(payload) == 73:
            value = cls(payload[:32], operation, exit_program=payload[33:65], deadline_batch=int.from_bytes(payload[65:], "big"))
        elif operation == 3 and len(payload) == 33:
            value = cls(payload[:32], operation)
        elif operation == 4 and len(payload) == 65:
            value = cls(payload[:32], operation, account=payload[33:])
        else:
            raise ValueError("invalid wind-down framing")
        if value.encode() != payload:
            raise ValueError("noncanonical wind-down")
        return value


def decode_lifecycle_calldata(calldata: bytes, ordinal: int):
    from .evm_transaction import encode_call
    signatures = {
        1: (("deploy", ("bytes32", "uint16", "uint8", "bytes32", "bytes32", "bytes", "bytes")),),
        2: (("upgrade", ("bytes32", "uint16", "uint8", "bytes32", "bytes32", "bytes", "bytes", "bytes", "uint128")),),
        7: (("setExitRoute", ("bytes32", "bytes32", "bytes32", "bytes32", "bytes")),
            ("deprecate", ("bytes32", "bytes32", "uint64")),
            ("tombstone", ("bytes32",)), ("exitValue", ("bytes32", "bytes32"))),
    }
    for name, types in signatures.get(ordinal, ()):
        zeros = tuple(b"" if kind == "bytes" else bytes(32) if kind == "bytes32" else 0 for kind in types)
        if calldata[:4] != encode_call(name, types, zeros)[:4]:
            continue
        body = calldata[4:]
        if len(body) < len(types) * 32 or len(body) % 32:
            raise ValueError("lifecycle ABI framing")
        values = []
        for index, kind in enumerate(types):
            word = body[index * 32:(index + 1) * 32]
            number = int.from_bytes(word, "big")
            if kind == "bytes32":
                values.append(word)
            elif kind == "bytes":
                if number + 32 > len(body):
                    raise ValueError("lifecycle ABI offset")
                size = int.from_bytes(body[number:number + 32], "big")
                if size > len(body) - number - 32:
                    raise ValueError("lifecycle ABI length")
                values.append(body[number + 32:number + 32 + size])
            else:
                values.append(number)
        if encode_call(name, types, values) != calldata:
            raise ValueError("noncanonical lifecycle ABI")
        if ordinal == 1:
            program, abi, policy, authority, digest, interface, wasm = values
            value = NativeProgramDeploy(program, abi, authority, digest, wasm, interface or None, policy)
        elif ordinal == 2:
            program, abi, flags, old, digest, hook, interface, wasm, fee = values
            if flags & ~3 or bool(flags & 1) != bool(hook):
                raise ValueError("lifecycle upgrade flags")
            value = NativeProgramUpgrade(program, abi, old, digest, wasm, hook, interface if interface or flags & 2 else None, bool(flags & 2))
        elif name == "setExitRoute":
            value = NativeProgramWindDown(values[0], 1, *values[1:])
        elif name == "deprecate":
            value = NativeProgramWindDown(values[0], 2, exit_program=values[1], deadline_batch=values[2])
        elif name == "tombstone":
            value = NativeProgramWindDown(values[0], 3)
        else:
            value = NativeProgramWindDown(values[0], 4, account=values[1])
        value.encode()
        return value
    raise ValueError("lifecycle method mismatch")
