"""Durable Solana write intent and finalized custody-account reconciliation.

The deployment shell holds the record lock across this helper and every writer.
An unanswered write is never retried merely because its account is still absent.
"""

import base64
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile
from urllib.request import Request, urlopen


ALPHABET = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
FIELD = 2**255 - 19
EDWARDS_D = -121665 * pow(121666, FIELD - 2, FIELD) % FIELD


def decode_key(text):
    number = 0
    for character in text:
        if character not in ALPHABET:
            raise ValueError("invalid base58 public key")
        number = number * 58 + ALPHABET.index(character)
    raw = number.to_bytes((number.bit_length() + 7) // 8, "big")
    raw = b"\0" * (len(text) - len(text.lstrip("1"))) + raw
    if len(raw) != 32 or not any(raw):
        raise ValueError("expected a nonzero 32-byte public key")
    return raw


def encode_key(raw):
    number = int.from_bytes(raw, "big")
    text = ""
    while number:
        number, remainder = divmod(number, 58)
        text = ALPHABET[remainder] + text
    return "1" * (len(raw) - len(raw.lstrip(b"\0"))) + text


def derive_address(program, seeds):
    """Pubkey::find_program_address, using compressed Edwards-y decompression."""
    for bump in range(255, -1, -1):
        raw = hashlib.sha256(
            b"".join(seeds) + bytes([bump]) + decode_key(program) + b"ProgramDerivedAddress"
        ).digest()
        y = (int.from_bytes(raw, "little") & ((1 << 255) - 1)) % FIELD
        square = (y * y - 1) * pow(EDWARDS_D * y * y + 1, FIELD - 2, FIELD) % FIELD
        if square != 0 and pow(square, (FIELD - 1) // 2, FIELD) != 1:
            return encode_key(raw), bump
    raise ValueError("no off-curve program address")


def require_program_identity(configured, supplied):
    decode_key(supplied)
    if not configured.startswith("PLACEHOLDER:"):
        decode_key(configured)
        if configured != supplied:
            raise ValueError("program keypair differs from solana.program_id; refusing before broadcast")
    return supplied


def atomic_json(path, value):
    path = Path(path)
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w") as output:
            json.dump(value, output, sort_keys=True, indent=2)
            output.write("\n")
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
        descriptor = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(descriptor)
        finally:
            os.close(descriptor)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def desired_configuration(config, program):
    require_program_identity(config["solana"]["program_id"], program)
    # Only public deployment facts are retained, never endpoint or key material.
    return {
        "chain_id": config["chain_id"], "program_id": program,
        "owner": config["owner"], "attestors": [x.lower() for x in config["attestors"]],
        "threshold": config["threshold"], "commitment": config["solana"]["commitment"],
        "assets": [{
            "symbol": asset["symbol"], "mint": asset["address"],
            "asset_id": asset["asset_id"].lower(), "decimals": asset["decimals"],
            "per_tx_cap": asset["per_tx_cap"], "total_cap": asset["total_cap"],
        } for asset in config["assets"]],
    }


class Rpc:
    def __init__(self, endpoint):
        self.endpoint = endpoint

    def call(self, method, params):
        request = Request(self.endpoint, json.dumps({
            "jsonrpc": "2.0", "id": 1, "method": method, "params": params,
        }).encode(), {"Content-Type": "application/json"})
        try:
            with urlopen(request, timeout=30) as response:
                document = json.load(response)
        except Exception:
            # URL credentials must not appear in diagnostics.
            raise ValueError("Solana RPC transport failed") from None
        if document.get("jsonrpc") != "2.0" or document.get("id") != 1:
            raise ValueError("invalid Solana RPC response")
        if "error" in document or "result" not in document:
            raise ValueError("Solana RPC refused account reconciliation")
        return document["result"]


class RunJournal:
    def __init__(self, path):
        self.path = Path(path)

    def load(self):
        state = json.loads(self.path.read_text())
        if state["version"] != 1:
            raise ValueError("unknown Solana journal version")
        if self.path.stat().st_mode & 0o077:
            raise ValueError("Solana journal must be private (0600)")
        return state

    def prepare(self, config, program, genesis, publisher, elf_sha256, elf_bytes):
        binding = {
            "configuration": desired_configuration(config, program),
            "genesis_hash": genesis, "publisher": publisher,
            "elf_sha256": elf_sha256, "elf_bytes": int(elf_bytes),
        }
        if self.path.exists():
            state = self.load()
            if state["binding"] != binding:
                raise ValueError("deployment inputs differ from retained Solana intent")
            if not state.get("deployment"):
                raise ValueError("deployment intent is unresolved; reconcile the original transaction before retrying")
            return "resume"
        atomic_json(self.path, {
            "version": 1, "binding": binding, "phase": "deploy_intent",
            "intent": {"kind": "deploy", "program_id": program, "elf_sha256": elf_sha256},
            "deployment": None, "observed_slot": 0,
        })
        return "deploy"

    def deployed(self, deployment):
        state = self.load()
        if state["phase"] != "deploy_intent":
            raise ValueError("no pending deployment intent")
        program = state["binding"]["configuration"]["program_id"]
        require_program_identity(program, deployment["programId"])
        if not isinstance(deployment.get("signature"), str) or not deployment["signature"]:
            raise ValueError("deployment response has no signature")
        state.update(deployment=deployment, phase="deployed", intent=None)
        atomic_json(self.path, state)

    def reconcile(self, config, rpc, minimum_slot=0):
        state = self.load()
        desired = state["binding"]["configuration"]
        program = desired["program_id"]
        if desired_configuration(config, program) != desired:
            raise ValueError("custody settings differ from retained Solana intent")
        if not state["deployment"]:
            raise ValueError("deployment has not been acknowledged")
        if rpc.call("getGenesisHash", []) != state["binding"]["genesis_hash"]:
            raise ValueError("Solana genesis differs from retained intent")
        config_address, config_bump = derive_address(program, [b"config"])
        _, vault_bump = derive_address(program, [b"vault-authority"])
        asset_addresses = [derive_address(program, [b"asset", decode_key(x["mint"])])
                           for x in desired["assets"]]
        minimum_slot = max(state["observed_slot"], int(minimum_slot))
        result = rpc.call("getMultipleAccounts", [
            [config_address, *[x[0] for x in asset_addresses]],
            {"encoding": "base64", "commitment": "finalized", "minContextSlot": minimum_slot},
        ])
        slot = result["context"]["slot"]
        if type(slot) is not int or slot < minimum_slot:
            raise ValueError("stale Solana account snapshot")
        accounts = result["value"]
        if not isinstance(accounts, list) or len(accounts) != 1 + len(asset_addresses):
            raise ValueError("incomplete Solana account snapshot")
        present = []
        for index, account in enumerate(accounts):
            present.append(account is not None)
            if account is None:
                continue
            if account["owner"] != program or account["executable"] is not False:
                raise ValueError("custody account has foreign owner or executable flag")
            data, encoding = account["data"]
            if encoding != "base64":
                raise ValueError("custody account encoding is not base64")
            raw = base64.b64decode(data, validate=True)
            if index == 0:
                signers = b"".join(bytes.fromhex(x[2:]) for x in desired["attestors"])
                if (len(raw) != 1367 or raw[:10] != b"PXBRCFG0\0\1"
                        or raw[10:42] != decode_key(desired["owner"])
                        or raw[42:75] != b"\0" * 33
                        or raw[75:79] != bytes([len(desired["attestors"]), desired["threshold"], config_bump, vault_bump])
                        or raw[87:87 + len(signers)] != signers):
                    raise ValueError("existing custody config differs from requested owner, signers or policy")
            else:
                asset = desired["assets"][index - 1]
                expected = (
                    b"PXBRAST0\0\1" + decode_key(asset["mint"]) + bytes.fromhex(asset["asset_id"][2:])
                    + bytes([asset["decimals"], 1, asset_addresses[index - 1][1]])
                    + int(asset["per_tx_cap"]).to_bytes(8, "big")
                    + int(asset["total_cap"]).to_bytes(8, "big")
                )
                if len(raw) != 89 or raw[:81] != expected:
                    raise ValueError("existing custody asset differs from requested identity, decimals or caps")
        if not present[0] and any(present[1:]):
            raise ValueError("assets exist without custody config")
        intent = state["intent"]
        if intent:
            target = 0 if intent["kind"] == "initialise" else intent["index"] + 1
            if not present[target]:
                raise ValueError("admin intent is unresolved; wait for finalized readback before any further write")
        if not present[0]:
            intent = {"kind": "initialise", "owner": desired["owner"],
                      "attestors": desired["attestors"], "threshold": desired["threshold"]}
            action, phase = "initialise", "initialise_intent"
        elif not all(present):
            index = present.index(False) - 1
            intent = {"kind": "register-asset", "index": index, **desired["assets"][index]}
            action, phase = str(index), "asset_intent"
        else:
            intent, action, phase = None, "ready", "ready"
        state.update(intent=intent, phase=phase, observed_slot=slot)
        atomic_json(self.path, state)
        return action

    def publish(self, record_path, record, phase):
        state = self.load()
        if phase not in ("deployed", "ready") or state["phase"] != phase:
            raise ValueError("deployment record phase is not reconciled")
        if record["program_id"] != state["binding"]["configuration"]["program_id"]:
            raise ValueError("deployment record program differs from intent")
        record.update(version=1, phase=phase)
        atomic_json(record_path, record)


def main(argv):
    command, *args = argv
    if command == "identity":
        print(require_program_identity(*args))
        return
    journal = RunJournal(args[0])
    if command == "prepare":
        config = json.loads(Path(args[1]).read_text())
        print(journal.prepare(config, *args[2:]))
    elif command == "deployed":
        journal.deployed(json.loads(Path(args[1]).read_text()))
    elif command == "deployment":
        print(json.dumps(journal.load()["deployment"]))
    elif command == "next":
        print(journal.reconcile(json.loads(Path(args[1]).read_text()), Rpc(args[2]), args[3]))
    elif command == "publish":
        journal.publish(args[1], json.loads(Path(args[2]).read_text()), args[3])
    else:
        raise ValueError("unknown Solana journal command")


if __name__ == "__main__":
    try:
        main(sys.argv[1:])
    except Exception as error:
        # Transport errors are redacted inside Rpc; no endpoint is journaled.
        print(f"deploy-solana-program: {error}", file=sys.stderr)
        raise SystemExit(1)
