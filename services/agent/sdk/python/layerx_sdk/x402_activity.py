from .account_derivation import keccak256
from .evm_transaction import encode_call
from .payment_transaction import decode_payment_transaction
from .x402 import LAYERX_LEDGER_ADDRESS, LEDGER_DRAW_TYPES
from .x402_receive import decode_receive


def bind_receive_activity(canonical: bytes, receive: bytes, actor: str, network: int, key: str) -> str:
    if (not isinstance(key, str) or len(key) != 64
            or any(character not in "0123456789abcdef" for character in key)
            or not isinstance(actor, str) or len(actor) != 42 or actor[:2] != "0x"
            or any(character not in "0123456789abcdefABCDEF" for character in actor[2:])):
        raise ValueError("grant draw transaction binding")
    transaction = decode_payment_transaction(canonical)
    value = decode_receive(receive)
    grant = value["payer_grant"]
    expected = encode_call(
        "draw", LEDGER_DRAW_TYPES,
        (bytes.fromhex(value["grant_id"]), bytes.fromhex(value["to"]), int(value["amount"]),
         bytes.fromhex(key), int(value["receiver_sequence"]),
         bytes.fromhex(grant["purpose_hash"]), bytes.fromhex(value["context_hash"])),
    )
    if (type(network) is not int or network != 125 or transaction.chain_id != network
            or transaction.to != LAYERX_LEDGER_ADDRESS
            or transaction.value or transaction.data != expected or value["idempotency_key"] != key
            or "0x" + transaction.sender.hex() != actor.lower()
            or value["grant_id"] != grant["grant_id"]
            or value["from"] != grant["from"] or value["to"] != grant["recipient"]
            or value["asset"] != grant["asset"]
            or value["receiver_authorization"]["network_id"] != network
            or value["receiver_authorization"]["controller"] != value["to"]
            or value["receiver_authorization"]["signed_context_hash"] != value["context_hash"]):
        raise ValueError("grant draw transaction binding")
    return "0x" + keccak256(canonical).hex()
