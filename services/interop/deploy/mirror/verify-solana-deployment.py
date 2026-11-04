#!/usr/bin/env python3
"""Check the rooted loader accounts before finalization or deployment readiness."""

import argparse
import base64
import hashlib
import json
import ssl
import urllib.request
from pathlib import Path


BASE58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
LOADER = "BPFLoaderUpgradeab1e11111111111111111111111"


def public_key(value):
    number = 0
    for digit in value:
        number = number * 58 + BASE58.index(digit)
    decoded = b"\0" * (len(value) - len(value.lstrip("1")))
    decoded += number.to_bytes((number.bit_length() + 7) // 8, "big")
    if len(decoded) != 32:
        raise ValueError("invalid Solana public key")
    return decoded


class RootedDeployment:
    def __init__(self, endpoint):
        if not endpoint.startswith("https://"):
            raise ValueError("an https RPC endpoint is required")
        self.endpoint = endpoint
        self.context = ssl.create_default_context()
        self.request_id = 0

    def call(self, method, params):
        self.request_id += 1
        request = urllib.request.Request(
            self.endpoint,
            data=json.dumps(
                {"jsonrpc": "2.0", "id": self.request_id, "method": method, "params": params}
            ).encode(),
            headers={"Content-Type": "application/json"},
        )
        with urllib.request.urlopen(request, context=self.context, timeout=15) as response:
            raw = response.read(16 * 1024 * 1024 + 1)
        if len(raw) > 16 * 1024 * 1024:
            raise ValueError("RPC response exceeds bound")
        document = json.loads(raw)
        if (
            document.get("jsonrpc") != "2.0"
            or document.get("id") != self.request_id
            or "error" in document
            or "result" not in document
        ):
            raise ValueError("invalid RPC response")
        return document["result"]

    def account(self, address, root, executable):
        response = self.call(
            "getAccountInfo",
            [address, {"commitment": "finalized", "encoding": "base64", "minContextSlot": root}],
        )
        slot = response["context"]["slot"]
        if type(slot) is not int or slot < root:
            raise ValueError("account is behind the rooted observation")
        account = response["value"]
        if account["owner"] != LOADER or account["executable"] is not executable:
            raise ValueError("loader account identity mismatch")
        data, encoding = account["data"]
        if encoding != "base64":
            raise ValueError("account encoding mismatch")
        return base64.b64decode(data, validate=True)

    def verify(self, arguments):
        if self.call("getGenesisHash", []) != arguments.genesis_hash:
            raise ValueError("cluster genesis mismatch")
        root = self.call("getSlot", [{"commitment": "finalized"}])
        if type(root) is not int or root <= 0:
            raise ValueError("invalid finalized slot")
        status = self.call(
            "getSignatureStatuses", [[arguments.deployment_signature], {"searchTransactionHistory": True}]
        )
        if type(status["context"]["slot"]) is not int or status["context"]["slot"] < root:
            raise ValueError("signature observation is behind the root")
        if len(status["value"]) != 1:
            raise ValueError("invalid deployment signature response")
        deployed = status["value"][0]
        if (
            deployed is None
            or deployed["err"] is not None
            or deployed["confirmationStatus"] != "finalized"
            or type(deployed["slot"]) is not int
            or not 0 < deployed["slot"] <= root
        ):
            raise ValueError("deployment signature is not rooted successfully")
        program = self.account(arguments.program_id, root, True)
        if program != (2).to_bytes(4, "little") + public_key(arguments.program_data):
            raise ValueError("program data pointer mismatch")
        data = self.account(arguments.program_data, root, False)
        if len(data) <= 45 or data[:4] != (3).to_bytes(4, "little"):
            raise ValueError("invalid program data state")
        deployment_slot = int.from_bytes(data[4:12], "little")
        if not 0 < deployment_slot <= root or deployment_slot != deployed["slot"]:
            raise ValueError("program deployment slot mismatch")
        if arguments.expected_authority:
            if data[12] != 1 or data[13:45] != public_key(arguments.expected_authority):
                raise ValueError("upgrade authority is not the authorized publisher")
        elif data[12] != 0:
            raise ValueError("program remains mutable")
        elf = Path(arguments.elf).read_bytes()
        if not elf.startswith(b"\x7fELF") or data[45:] != elf:
            raise ValueError("rooted program ELF identity mismatch")
        return {
            "rooted_slot": root,
            "deployment_finalized_slot": deployment_slot,
            "upgrade_authority": arguments.expected_authority,
            "program_elf_sha256": hashlib.sha256(data[45:]).hexdigest(),
            "program_elf_bytes": len(elf),
            "on_chain_elf_verified": True,
        }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("rpc-url", "genesis-hash", "program-id", "program-data", "deployment-signature", "elf"):
        parser.add_argument("--" + name, required=True)
    parser.add_argument("--expected-authority")
    arguments = parser.parse_args()
    try:
        record = RootedDeployment(arguments.rpc_url).verify(arguments)
    except (ValueError, KeyError, TypeError, OSError) as error:
        parser.exit(1, f"verify-solana-deployment: {error}\n")
    print(json.dumps(record))


if __name__ == "__main__":
    main()
