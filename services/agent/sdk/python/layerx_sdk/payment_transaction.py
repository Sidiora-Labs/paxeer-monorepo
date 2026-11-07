"""Validated EVM transactions used by the ledger payment producers."""
from dataclasses import dataclass

from .account_derivation import _N, _recover, keccak256
from .evm_transaction import EvmTransactionError, _rlp, _rlp_decode, decode_signed


@dataclass(frozen=True)
class PaymentTransaction:
    chain_id: int
    nonce: int
    to: bytes
    value: int
    data: bytes
    sender: bytes


def decode_payment_transaction(raw: bytes) -> PaymentTransaction:
    if not isinstance(raw, bytes) or not 0 < len(raw) <= 524288:
        raise EvmTransactionError("invalid_transaction")
    if raw[0] == 2:
        tx, signature = decode_signed(raw)
        sender = bytes.fromhex(tx.recover_signer(signature)[2:])
        return PaymentTransaction(tx.chain_id, tx.nonce, tx.to, tx.value, tx.data, sender)
    fields = _rlp_decode(raw)
    if (len(fields) != 9 or any(type(field) is not bytes for field in fields)
            or _rlp(fields) != raw or len(fields[3]) != 20):
        raise EvmTransactionError("invalid_transaction")
    for index in (0, 1, 2, 4, 6, 7, 8):
        if len(fields[index]) > 32 or fields[index][:1] == b"\0":
            raise EvmTransactionError("invalid_transaction")
    nonce, gas, v, r, s = (int.from_bytes(fields[index], "big") for index in (0, 2, 6, 7, 8))
    if v < 35 or nonce >= 1 << 64 or not 0 < gas < 1 << 64 or not 0 < r < _N or not 0 < s <= _N // 2:
        raise EvmTransactionError("invalid_signature")
    chain_id, parity = (v - 35) // 2, (v - 35) % 2
    if not 0 < chain_id < 1 << 64:
        raise EvmTransactionError("invalid_transaction")
    encoded_chain = chain_id.to_bytes((chain_id.bit_length() + 7) // 8, "big")
    digest = keccak256(_rlp(fields[:6] + (encoded_chain, b"", b"")))
    public = _recover(digest, r, s, parity)
    return PaymentTransaction(chain_id, nonce, fields[3], int.from_bytes(fields[4], "big"), fields[5], keccak256(public)[-20:])
