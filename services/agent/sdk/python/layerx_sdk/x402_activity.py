from .account_derivation import keccak256
from .evm_transaction import encode_call
from .payment_transaction import decode_payment_transaction
from .x402_receive import decode_receive


def bind_receive_activity(canonical: bytes, receive: bytes, actor: str, network: int, key: str) -> str:
    transaction = decode_payment_transaction(canonical)
    value = decode_receive(receive)
    grant = value["payer_grant"]
    expected = encode_call("draw", ("bytes32", "bytes32", "uint128", "bytes32", "uint64", "bytes32", "bytes32"),
        (bytes.fromhex(value["grant_id"]), bytes.fromhex(value["to"]), int(value["amount"]), bytes.fromhex(key),
         int(value["receiver_sequence"]), bytes.fromhex(grant["purpose_hash"]), bytes.fromhex(value["context_hash"])))
    if (type(network) is not int or network != 125 or transaction.chain_id != network
            or transaction.to != bytes.fromhex("000000000000000000000000000000000000101a")
            or transaction.value or transaction.data != expected or value["idempotency_key"] != key
            or "0x" + transaction.sender.hex() != actor.lower()):
        raise ValueError("grant draw transaction binding")
    return "0x" + keccak256(canonical).hex()
