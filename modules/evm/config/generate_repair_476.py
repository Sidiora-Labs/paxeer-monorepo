"""Regenerate the published account and xweb documentation vectors for chain 125.

Run from the repository root:
uv run --with eth-account==0.13.7 --with cryptography==46.0.3 \
    modules/evm/config/generate_repair_476.py
"""

import json
from pathlib import Path
import sys

from eth_account import Account
from eth_account.messages import encode_typed_data

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))
from layerx_sdk import account_derivation as derivation  # noqa: E402

CHAIN_ID = 125
ORDER = 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141
FIXTURE = ROOT / "sdk/platform/conformance/fixtures/account-derivation-v1.json"
DOC = ROOT / "modules/xweb/ATTESTATION.md"


def update_resolution(vector, derived):
    vector["evm_address"] = derived.evm_address.lower()
    vector["layerx_public_key"] = derived.layerx_public_key
    vector["did"] = derived.did
    resolution = vector["resolution"]
    resolution["chainId"] = CHAIN_ID
    resolution["data"] = "0xc413bad3" + derived.layerx_public_key
    resolution["result"] = "0x" + "00" * 12 + derived.evm_address[2:].lower()


def regenerate_account_fixture():
    fixture = json.loads(FIXTURE.read_text())
    fixture["chain_id"] = CHAIN_ID
    for mnemonic in fixture["mnemonic_vectors"]:
        for vector in mnemonic["accounts"]:
            derived = derivation.derive_from_mnemonic(
                mnemonic["mnemonic"], mnemonic["passphrase"], vector["index"]
            )
            update_resolution(vector, derived)
            bind = vector["bind"]
            call = derivation.plan_layerx_bind(
                derived, CHAIN_ID, derivation.LayerXBindState(None, bind["nonce"])
            ).call
            if call is None:
                raise ValueError("unbound fixture must produce a bind call")
            bind["chain_id"] = CHAIN_ID
            bind["to"] = call.to
            bind["calldata"] = call.data[2:]
            transaction = bind["transaction"]
            transaction["raw"] = Account.sign_transaction(
                {
                    "type": 2,
                    "chainId": CHAIN_ID,
                    "nonce": transaction["evm_nonce"],
                    "maxPriorityFeePerGas": int(transaction["max_priority_fee_per_gas"]),
                    "maxFeePerGas": int(transaction["max_fee_per_gas"]),
                    "gas": transaction["gas_limit"],
                    "to": call.to,
                    "value": 0,
                    "data": call.data,
                },
                derived.evm_private_key,
            ).raw_transaction.hex()
    wallet = fixture["wallet_signature"]
    for vector in wallet["vectors"]:
        index = vector["index"]
        typed_data = derivation.key_derivation_typed_data(
            CHAIN_ID, wallet["evm_address"], index, fixture["origin"]
        )
        signed = Account.sign_message(
            encode_typed_data(full_message=typed_data), wallet["private_key"]
        )
        signature = bytes(signed.signature)
        parity = signature[64] - 27
        high_s = (
            signature[:32]
            + (ORDER - int.from_bytes(signature[32:64], "big")).to_bytes(32, "big")
            + bytes([27 + (parity ^ 1)])
        )
        vector["typed_data"] = typed_data
        vector["eip712_hash"] = signed.message_hash.hex()
        vector["signature"] = signature.hex()
        vector["equivalent_signatures"] = [
            high_s.hex(),
            (signature[:64] + bytes([parity])).hex(),
        ]
        derived = derivation.derive_from_wallet_signature(
            CHAIN_ID, wallet["evm_address"], signature, index, fixture["origin"]
        )
        update_resolution(vector, derived)
    fixture["resolution_provenance"] = (
        "Regenerated for chain 125 by modules/evm/config/generate_repair_476.py "
        "using the production Python account derivation and binding code; "
        "expected getEvmAddrByLayerX ABI answers, no live-chain execution claim."
    )
    FIXTURE.write_text(json.dumps(fixture, indent=2) + "\n")


def regenerate_attestation_doc():
    vectors = json.loads(
        (ROOT / "modules/xweb/types/testdata/preimage-vectors.json").read_text()
    )["vectors"]
    text = DOC.read_text()
    for marker in (
        "\nThe shared\n[`types/testdata/preimage-vectors.json`]",
        "\nThe full preimage bytes of each vector are in",
    ):
        if marker in text:
            text = text[:text.index(marker)]
    start = text.index("## Vectors")
    text = text[:start] + text[start:].replace("713714", str(CHAIN_ID))
    text = text.replace(
        "These are frozen compatibility vectors, not deployment settings. In particular,\n"
        "`125` is the historical origin-1 test chain id, and origin-2 network id `1`\n"
        "is a fixture value. Live signing uses the chain state and governed configuration\n"
        "described above. The bytes, digests and signatures below remain unchanged.",
        "Origin-1 vectors below are regenerated for Paxeer chain `125`; origin-2\n"
        "network id `1` is a fixture value. Live signing uses chain state and governed\n"
        "configuration. The generator is\n"
        "[`generate_repair_476.py`](../evm/config/generate_repair_476.py).",
    )
    legacy = []
    for vector in vectors:
        if vector["origin"] != 1:
            continue
        preimage = bytearray.fromhex(vector["preimage"][2:])
        preimage[15:47] = CHAIN_ID.to_bytes(32, "big")
        digest = derivation.keccak256(bytes(preimage))
        if vector["name"] == "evm-fetch":
            key = bytes([0x3C]) + bytes(30) + bytes([1])
        else:
            key = derivation.keccak256(
                b"PAXEERX_WEB_API_ENVELOPE_V1 vector attestor 1"
            )
        signer = Account.from_key(key).address.lower()
        if vector["name"] == "evm-api-single" and signer != vector["signer"]:
            raise ValueError("API fixture attestor key does not match its payload")
        if vector["name"] == "evm-fetch":
            begin = text.index("Vector `evm-fetch`")
            end = text.index("Vector `program-search`")
            text = (
                text[:begin]
                + text[begin:end].replace(
                    '- signer `' + vector["signer"] + '`, signature',
                    '- signer `' + signer + '`, signature',
                )
                + text[end:]
            )
        signature = "0x" + Account.unsafe_sign_hash(digest, key).signature.hex()
        text = text.replace(vector["digest"], "0x" + digest.hex())
        text = text.replace(vector["signature"], signature)
        legacy.append(
            f'- `{vector["name"]}`: digest `{vector["digest"]}`, '
            f'signature `{vector["signature"]}`.'
        )
    text = text.rstrip() + (
        "\n\nThe shared\n"
        "[`types/testdata/preimage-vectors.json`](types/testdata/preimage-vectors.json)\n"
        "still contains the historical origin-1 compatibility corpus. Its legacy\n"
        "digests and signatures below identify those archived bytes; they are not\n"
        "chain-125 signing examples. Updating that corpus and its cross-language\n"
        "consumers requires a separate scope.\n\n" + "\n".join(legacy) + "\n"
    )
    DOC.write_text(text)


if __name__ == "__main__":
    regenerate_account_fixture()
    regenerate_attestation_doc()
