"""Private EVM deployment checkpoints. The shell holds the per-chain flock.

A broadcasting checkpoint never authorizes a second forge deployment. Incomplete
Foundry artifacts require operator recovery of that exact run, then readback.
"""

import hashlib
import json
import os
from pathlib import Path
import re
import sys
import tempfile


ADDRESS = re.compile(r"0x[0-9a-fA-F]{40}")
VARIABLE = re.compile(r"[A-Z][A-Z0-9]*(_[A-Z0-9]+)*")


def address(value):
    if not isinstance(value, str) or not ADDRESS.fullmatch(value) or int(value, 16) == 0:
        raise ValueError("expected a nonzero 20-byte address")
    return value.lower()


def require_signer(configured, signer):
    signer = address(signer)
    if configured and address(configured) != signer:
        raise ValueError("local signer differs from configured_deployer; refusing before broadcast")
    return signer


def sync_directory(path):
    descriptor = os.open(path, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(descriptor)
    finally:
        os.close(descriptor)


def atomic_json(path, value):
    """Flush a private sibling and rename; readers see the old or complete JSON."""
    path = Path(path)
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w") as output:
            json.dump(value, output, sort_keys=True, indent=2)
            output.write("\n")
            output.flush()
            os.fsync(output.fileno())
        os.replace(temporary, path)
        sync_directory(path.parent)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def resolved_configuration(config):
    """Keep deployment settings, never arbitrary overlay fields or credential values."""
    result = {key: config[key] for key in (
        "chain", "kind", "chain_id", "owner", "attestors", "threshold", "finality_depth"
    )}
    if not re.fullmatch(r"[a-z][a-z0-9]*", result["chain"]) or result["kind"] != "evm":
        raise ValueError("not an EVM chain configuration")
    for key in ("chain_id", "threshold", "finality_depth"):
        if type(result[key]) is not int or result[key] <= 0:
            raise ValueError(f"{key} must be a positive integer")
    address(result["owner"])
    for signer in result["attestors"]:
        address(signer)
    if "deployer" in config:
        address(config["deployer"])
        result["deployer"] = config["deployer"]
    result["environment"] = {}
    for key in ("rpc_url", "deploy_key", "explorer_key"):
        if key in config["environment"]:
            name = config["environment"][key]
            if not isinstance(name, str) or not VARIABLE.fullmatch(name):
                raise ValueError("configuration may contain environment variable names only")
            result["environment"][key] = name
    result["native"] = {key: config["native"][key] for key in ("symbol", "decimals")}
    result["assets"] = [{key: asset[key] for key in (
        "symbol", "address", "asset_id", "decimals", "per_tx_cap", "total_cap"
    )} for asset in config["assets"]]
    for item in [result["native"], *result["assets"]]:
        if not re.fullmatch(r"[A-Za-z0-9._-]{1,32}", item["symbol"]):
            raise ValueError("invalid public asset symbol")
        if type(item["decimals"]) is not int or not 0 <= item["decimals"] <= 255:
            raise ValueError("invalid decimals")
    for asset in result["assets"]:
        for key in ("address", "asset_id"):
            if not ADDRESS.fullmatch(asset[key]):
                raise ValueError("invalid public asset identity")
        for key in ("per_tx_cap", "total_cap"):
            if not re.fullmatch(r"[1-9][0-9]*", asset[key]):
                raise ValueError("invalid cap")
    if "big_blocks" in config:
        result["big_blocks"] = {}
        for key in ("required", "acknowledged"):
            value = config["big_blocks"][key]
            if type(value) is not bool:
                raise ValueError("big-block settings must be booleans")
            result["big_blocks"][key] = value
    return result


class RunJournal:
    def __init__(self, directory, record, chain):
        self.directory = Path(directory).absolute()
        self.record = Path(record).absolute()
        self.chain = chain
        self.path = self.directory / "run.json"
        self.snapshot = self.directory / "resolved-config.json"

    def load(self):
        state = json.loads(self.path.read_text())
        if state["chain"] != self.chain or state["record"] != str(self.record):
            raise ValueError("journal belongs to another chain or deployment record")
        if state["configuration"] != str(self.snapshot):
            raise ValueError("journal snapshot path changed")
        if hashlib.sha256(self.snapshot.read_bytes()).hexdigest() != state["configuration_sha256"]:
            raise ValueError("resolved configuration digest changed")
        if self.directory.stat().st_mode & 0o077:
            raise ValueError("run journal must be private (0700)")
        return state

    def prepare(self, config, signer):
        signer = require_signer(config.get("deployer", ""), signer)
        snapshot = resolved_configuration(config)
        if self.directory.exists():
            state = self.load()
            if state["signer"] != signer:
                raise ValueError("signer differs from the retained deployment")
            if json.loads(self.snapshot.read_text()) != snapshot:
                raise ValueError("configuration differs from the retained deployment")
            return "broadcast" if state["phase"] == "prepared" else "readback"
        if self.record.exists():
            raise ValueError("deployment record already exists without its run journal")
        self.directory.mkdir(mode=0o700)
        sync_directory(self.directory.parent)
        atomic_json(self.snapshot, snapshot)
        atomic_json(self.path, {
            "version": 1, "chain": self.chain, "chain_id": snapshot["chain_id"],
            "record": str(self.record), "phase": "prepared", "signer": signer,
            "configuration": str(self.snapshot),
            "configuration_sha256": hashlib.sha256(self.snapshot.read_bytes()).hexdigest(),
            "transactions": [], "vault": None,
        })
        return "broadcast"

    def begin_broadcast(self):
        state = self.load()
        if state["phase"] != "prepared":
            raise ValueError("this run already attempted broadcast; reopen for readback only")
        state["phase"] = "broadcasting"
        atomic_json(self.path, state)

    def retain_vault(self, vault):
        state = self.load()
        vault = address(vault)
        if state["phase"] == "prepared":
            raise ValueError("no broadcast has been attempted")
        if state["vault"] and state["vault"] != vault:
            raise ValueError("broadcast changed the retained vault identity")
        state["vault"] = vault
        atomic_json(self.path, state)

    def capture(self):
        """Retain Foundry's exact artifact even when receipts are incomplete."""
        state = self.load()
        run = self.directory / "broadcast/DeployPaxeerXVault.s.sol" / str(state["chain_id"]) / "run-latest.json"
        if not run.is_file():
            raise ValueError(f"broadcast uncertain; recover the original run under {self.directory}; never deploy again")
        # Foundry owns this file; flush it before accepting it as durable evidence.
        with run.open("rb") as source:
            raw = source.read()
            os.fsync(source.fileno())
        for parent in (run.parent, run.parent.parent, run.parent.parent.parent, self.directory):
            sync_directory(parent)
        broadcast = json.loads(raw)
        transactions = broadcast["transactions"]
        receipts = broadcast["receipts"]
        if not transactions:
            raise ValueError("broadcast contains no transactions")
        for transaction in transactions:
            if address(transaction["transaction"]["from"]) != state["signer"]:
                raise ValueError("broadcast deployer differs from the local signer")
        creates = [tx for tx in transactions if tx["transactionType"] == "CREATE"]
        if len(creates) != 1:
            raise ValueError("broadcast must name exactly one created vault")
        vault = address(creates[0]["contractAddress"])
        self.retain_vault(vault)
        hashes = [tx.get("hash") for tx in transactions]
        retained = [tx.get("hash") for tx in state["transactions"] if tx.get("hash")]
        if any(value not in hashes for value in retained):
            raise ValueError("broadcast lost a retained submitted transaction")
        state.update(phase="recovery_required", vault=vault, transactions=transactions,
                     receipts=receipts, broadcast_record=str(run))
        atomic_json(self.path, state)
        if any(not isinstance(value, str) or not re.fullmatch(r"0x[0-9a-fA-F]{64}", value) for value in hashes):
            raise ValueError("broadcast has unsubmitted transactions; recover this exact run")
        if len(set(hashes)) != len(hashes):
            raise ValueError("broadcast repeats a transaction hash")
        by_hash = {receipt["transactionHash"]: receipt for receipt in receipts}
        if len(by_hash) != len(receipts) or set(by_hash) != set(hashes):
            raise ValueError("broadcast receipts are incomplete; recover this exact run")
        if any(int(receipt["status"], 16) != 1 for receipt in receipts):
            raise ValueError("broadcast contains an unsuccessful transaction")
        created = by_hash[creates[0]["hash"]]
        if address(created["contractAddress"]) != vault:
            raise ValueError("receipt differs from the retained vault")
        state.update(phase="readback", block=int(created["blockNumber"], 16))
        atomic_json(self.path, state)
        return state

    def publish(self, candidate):
        state = self.load()
        if state["phase"] not in ("readback", "complete"):
            raise ValueError("broadcast has not qualified for readback")
        for key, expected in (("chain", self.chain), ("chain_id", state["chain_id"]),
                              ("vault", state["vault"]), ("deployer", state["signer"]),
                              ("block", state["block"])):
            actual = candidate[key]
            if key in ("vault", "deployer"):
                actual = address(actual)
            if actual != expected:
                raise ValueError(f"readback changed {key}")
        result = dict(candidate, configuration=str(self.snapshot),
                      configuration_sha256=state["configuration_sha256"],
                      run_journal=str(self.path))
        atomic_json(self.record, result)
        state["phase"] = "complete"
        atomic_json(self.path, state)


def main(args):
    if args[0] == "signer":
        print(require_signer(args[1], args[2]))
        return
    operation, directory, record, chain, *rest = args
    journal = RunJournal(directory, record, chain)
    if operation == "config":
        journal.load()
        print(journal.snapshot)
    elif operation == "prepare":
        print(journal.prepare(json.loads(Path(rest[0]).read_text()), rest[1]))
    elif operation == "begin":
        journal.begin_broadcast()
    elif operation == "capture":
        journal.capture()
    elif operation == "publish":
        journal.publish(json.loads(Path(rest[0]).read_text()))
    else:
        raise ValueError(f"unknown journal operation {operation}")


if __name__ == "__main__":
    try:
        main(sys.argv[1:])
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"EVM run journal: {error}", file=sys.stderr)
        sys.exit(1)
