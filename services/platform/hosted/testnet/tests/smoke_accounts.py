"""Separate hosted identity keys from the native EVM payment signers."""

import argparse
import json
from pathlib import Path
import shlex
import sys

from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec, ed25519
from cryptography.hazmat.primitives.serialization import load_pem_private_key

from journey_common import protected, write

sys.path.insert(0, str(Path(__file__).resolve().parents[5] / "services/agent/sdk/python"))
from layerx_sdk.account_derivation import (
    DerivedAccount, LayerXBindState, plan_layerx_bind, keccak256,
)

NATIVE_DENOM = "uhpx"


def generate_accounts(directory):
    accounts = {}
    for name in ("source", "destination"):
        identity = load_pem_private_key(
            protected(directory / f"test-{name}-signer.key"), password=None
        )
        assert isinstance(identity, ed25519.Ed25519PrivateKey)
        public = identity.public_key().public_bytes(
            serialization.Encoding.Raw, serialization.PublicFormat.Raw
        ).hex()
        signer = ec.generate_private_key(ec.SECP256K1())
        public_evm = signer.public_key().public_numbers()
        address = "0x" + keccak256(
            public_evm.x.to_bytes(32, "big") + public_evm.y.to_bytes(32, "big")
        )[-20:].hex()
        path = directory / f"test-{name}-evm.key"
        write(path, signer.private_bytes(
            serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8,
            serialization.NoEncryption(),
        ))
        derived = DerivedAccount(
            index=0, evm_address=address, layerx_public_key=public,
            did="did:layerx:" + public,
            layerx_seed=identity.private_bytes(
                serialization.Encoding.Raw, serialization.PrivateFormat.Raw,
                serialization.NoEncryption(),
            ),
            evm_private_key=signer.private_numbers().private_value.to_bytes(32, "big"),
        )
        # The native bootstrap owns submission; never claim a local declaration is a chain binding.
        plan = plan_layerx_bind(derived, 125, LayerXBindState(nonce=0, bound_did_public_key=None))
        accounts[name] = {
            "did": derived.did, "evm_address": address,
            "bind_nonce": plan.call.nonce,
            "bind_to": plan.call.to, "bind_data": plan.call.data,
        }
    document = {"evm_chain_id": 125, "denomination": NATIVE_DENOM, "accounts": accounts}
    write(directory / "smoke-accounts.json", json.dumps(document, indent=2) + "\n")
    return document


def smoke_environment(directory):
    # Only paths and public data are exported; private keys stay in protected files.
    protected(directory / "test-source-evm.key")
    protected(directory / "smoke-accounts.json")
    return {
        "LAYERX_TEST_SOURCE_EVM_KEY_FILE": str(directory / "test-source-evm.key"),
        "LAYERX_TEST_ACCOUNTS_FILE": str(directory / "smoke-accounts.json"),
        "LAYERX_TEST_ASSET": NATIVE_DENOM,
    }


def main(argv=None):
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=("generate", "env"))
    parser.add_argument("directory", type=Path)
    args = parser.parse_args(argv)
    if args.mode == "generate":
        generate_accounts(args.directory)
    else:
        for name, value in smoke_environment(args.directory).items():
            print(f"export {name}={shlex.quote(value)}")


if __name__ == "__main__":
    main()
