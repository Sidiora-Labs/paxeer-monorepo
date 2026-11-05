"""Owned-node upgrade rehearsal; the offline repair gate checks its source contract.

A release kit supplies real, source/digest-pinned executables, a disposable home
with a passed governance plan, signed state-producing transactions, receipt-key
pins and exact expected state. Nothing downloads or synthesizes a node/artifact.
Run with --kit and its independently recorded --kit-sha256 on a native host.
"""

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import time
from urllib.parse import urlsplit


ROOT = Path(__file__).resolve().parents[3]
LEDGER = "0x" + "00" * 18 + "101a"
PROGRAMS = "0x" + "00" * 18 + "101d"
STATE_KEYS = {"kernel_balance", "kernel_sequence", "kernel_work", "evm_balance", "evm_storage"}


def require(value, reason):
    if not value:
        raise ValueError(reason)


def load_module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def digest(path):
    with Path(path).open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def validate_kit(kit, kind):
    require(kit["kind"] == kind and kind in ("major", "minor"), "upgrade kind")
    require(kit["evm_chain_id"] == 125, "current network must use EVM chain 125")
    plan = kit["plan"]
    require(type(plan["height"]) is int and plan["height"] > 1, "scheduled height")
    if kind == "major":
        require(plan["name"] == "v7.0", "major must exercise the real merge plan")
        info = json.loads(plan["info"])
        require(info["leaders"] and info["receipt_keys"] and info.get("last_seq", 0) == 0,
                "merge plan must carry real sequencer genesis")
    else:
        require(re.fullmatch(r"v7\.[1-9][0-9]*\.[0-9]+", plan["name"]),
                "minor must use a supported post-merge release, not the historical fixture")
        require(json.loads(plan["info"])["upgradeType"] == "minor", "minor plan metadata")
    old, new = kit["old"], kit["new"]
    for row in (old, new):
        require(re.fullmatch(r"[0-9a-f]{40}", row["source_revision"]), "source revision pin")
        require(re.fullmatch(r"[0-9a-f]{64}", row["sha256"]), "artifact SHA-256 pin")
        require(row["version"] and row["path"].startswith("/"), "absolute versioned binary")
    require(old["sha256"] != new["sha256"] and old["source_revision"] != new["source_revision"],
            "two distinct source-bound binaries required")
    for key in ("rpc_url", "consensus_url"):
        url = urlsplit(kit[key])
        require(url.scheme == "http" and url.hostname == "127.0.0.1" and url.port
                and url.path in ("", "/") and not url.username and not url.password
                and not url.query and not url.fragment, "owned loopback root endpoint")
    home = Path(kit["home"])
    require(home.is_absolute() and home.name == "disposable-upgrade-home", "disposable home required")
    require(kit["receipt_keys"], "independently pinned receipt signing keys required")
    for phase in ("before", "after"):
        expected = kit[phase]
        require(set(expected) == STATE_KEYS, "complete kernel/EVM expected state")
        for value in expected.values():
            require(isinstance(value, str) and re.fullmatch(r"0x[0-9a-f]+", value)
                    and int(value, 16) > 0, "nonempty nonzero canonical state required")
        transactions = kit[phase + "_transactions"]
        require({row["role"] for row in transactions} == {"kernel", "work", "evm"},
                "real kernel, work and EVM producers required in each phase")
        for row in transactions:
            require(re.fullmatch(r"0x(?:[0-9a-f]{2})+", row["raw"]), "signed transaction bytes")
    require(kit["before"] != kit["after"], "post-upgrade work must change observable state")
    watch = kit["watch"]
    for key in ("account_id", "asset_id", "program_id", "storage_slot"):
        require(re.fullmatch(r"0x[0-9a-f]{64}", watch[key]), "32-byte state identity")
    for key in ("evm_account", "contract"):
        require(re.fullmatch(r"0x[0-9a-f]{40}", watch[key]), "EVM state identity")


def pinned_binary(row):
    path = Path(row["path"])
    require(path.resolve(strict=True) == path and path.is_file() and not path.is_symlink()
            and os.access(path, os.X_OK), "real executable path required")
    require(digest(path) == row["sha256"], "artifact digest differs from release pin")
    with path.open("rb") as stream:
        require(stream.read(4) == b"\x7fELF", "native paxd required")
    result = subprocess.run([str(path), "version"], check=True, capture_output=True,
                            text=True, timeout=15)
    require(result.stdout.strip() == row["version"], "binary version differs from release pin")
    return str(path)


class OwnedNode:
    def __init__(self, kit, chain):
        self.kit, self.chain = kit, chain
        self.process = None
        self.binary = None
        self.home = Path(kit["home"])
        require(self.home.resolve(strict=True) == self.home and self.home.is_dir(), "staged home required")
        self.rpc = chain.Node(kit["rpc_url"], kit["receipt_keys"])
        self.env = {key: value for key, value in os.environ.items()
                    if key != "UPGRADE_VERSION_LIST"}

    def start(self, row):
        require(self.process is None or self.process.poll() is not None, "already running")
        self.binary = pinned_binary(row)
        url = urlsplit(self.kit["rpc_url"])
        args = [self.binary, "start", "--home", str(self.home),
                "--rpc.laddr", self.kit["consensus_url"].replace("http://", "tcp://"),
                "--evm.http_enabled", "true", "--evm.http_address", "127.0.0.1",
                "--evm.http_port", str(url.port), "--sequencer.enable", "true",
                "--sequencer.wal-dir", str(self.home / "sequencer-wal"),
                "--sequencer.receipt-key-file", self.kit["receipt_key_file"]]
        with (self.home / "upgrade-proof.log").open("ab") as log:
            self.process = subprocess.Popen(args, env=self.env, stdout=log, stderr=log,
                                            start_new_session=True)
        self.wait(lambda: self.rpc.call("eth_chainId", []) == "0x7d")

    def wait(self, predicate, seconds=180):
        deadline = time.monotonic() + seconds
        while time.monotonic() < deadline:
            require(self.process.poll() is None, "paxd exited during state proof")
            try:
                if predicate():
                    return
            except (OSError, self.chain.RpcError):
                pass
            time.sleep(0.1)
        raise ValueError("node progress deadline")

    def stop(self):
        if self.process is not None and self.process.poll() is None:
            os.killpg(self.process.pid, signal.SIGTERM)
            self.process.wait(timeout=30)

    def scheduled(self):
        result = subprocess.run(
            [self.binary, "query", "upgrade", "plan", "--home", str(self.home),
             "--node", self.kit["consensus_url"].replace("http://", "tcp://"), "--output", "json"],
            check=True, capture_output=True, text=True, timeout=15,
        )
        actual = json.loads(result.stdout)
        expected = self.kit["plan"]
        require(actual["name"] == expected["name"] and int(actual["height"]) == expected["height"]
                and actual["info"] == expected["info"], "governance plan differs from pinned transition")

    def applied(self):
        plan = self.kit["plan"]
        result = subprocess.run(
            [self.binary, "query", "upgrade", "applied", plan["name"], "--home", str(self.home),
             "--node", self.kit["consensus_url"].replace("http://", "tcp://"), "--output", "json"],
            check=True, capture_output=True, text=True, timeout=15,
        )
        require(int(json.loads(result.stdout)["header"]["height"]) == plan["height"], "plan was not applied")


def snapshot(node, watch, evm):
    """All reads use one explicit committed height, never several moving heads."""
    height = node.call("eth_blockNumber", [])
    block = node.call("eth_getBlockByNumber", [height, False])
    require(block["number"] == height and int(block["hash"], 16) != 0, "committed block required")
    account, asset = bytes.fromhex(watch["account_id"][2:]), bytes.fromhex(watch["asset_id"][2:])

    def view(address, method, types, args):
        data = "0x" + evm.encode_call(method, types, args).hex()
        return node.call("eth_call", [{"to": address, "data": data}, height])

    state = {
        "kernel_balance": view(LEDGER, "balanceOf", ["bytes32", "bytes32"], [account, asset]),
        "kernel_sequence": view(LEDGER, "nextSequence", ["bytes32", "bytes32"], [account, asset]),
        "kernel_work": view(PROGRAMS, "getStateHead", ["bytes32"],
                            [bytes.fromhex(watch["program_id"][2:])]),
        "evm_balance": node.call("eth_getBalance", [watch["evm_account"], height]),
        "evm_storage": node.call("eth_getStorageAt", [watch["contract"], watch["storage_slot"], height]),
    }
    return {"height": height, "block": block, "state": state}


def assert_state(actual, expected):
    require(set(actual) == STATE_KEYS and set(expected) == STATE_KEYS, "state fields")
    require(actual == expected, "kernel/EVM state changed or expected work was not committed")


def produce(node, rows, watch, evm):
    receipts = []
    for row in rows:
        raw = bytes.fromhex(row["raw"][2:])
        tx, _ = evm.decode_signed(raw)
        targets = {"kernel": LEDGER, "work": PROGRAMS, "evm": watch["contract"]}
        require(tx.chain_id == 125 and "0x" + tx.to.hex() == targets[row["role"]],
                "signed producer must bind chain and actual state consumer")
        reference = node.call("eth_sendRawTransaction", [row["raw"]])
        require(reference == "0x" + evm.keccak256(raw).hex(), "submission hash differs")
        receipts.append(node.wait_committed(reference))
    return receipts


def retained(node, saved, receipts):
    require(node.call("eth_getBlockByNumber", [saved["height"], False]) == saved["block"],
            "retained committed block changed")
    for receipt in receipts:
        require(node.committed(receipt["transaction_hash"]) == receipt,
                "committed receipt changed across upgrade/restart")


def run(kit, kind):
    validate_kit(kit, kind)
    # Validate both artifacts before starting a node or touching its state.
    pinned_binary(kit["old"])
    pinned_binary(kit["new"])
    chain = load_module("upgrade_chain", ROOT / "tools/qualification/paxeer-x/fixtures/merged_chain.py")
    node = OwnedNode(kit, chain)
    evidence = {"kind": kind, "old": kit["old"], "new": kit["new"], "plan": kit["plan"]}
    try:
        node.start(kit["old"])
        node.scheduled()
        before_receipts = produce(node.rpc, kit["before_transactions"], kit["watch"], chain)
        before = snapshot(node.rpc, kit["watch"], chain)
        assert_state(before["state"], kit["before"])
        require(int(before["height"], 16) < kit["plan"]["height"], "pre-upgrade capture too late")
        if kind == "major":
            node.process.wait(timeout=180)
            require(node.process.returncode != 0, "old binary must halt for the merge upgrade")
            halt = json.loads((node.home / "data/upgrade-info.json").read_text())
            require(halt["name"] == kit["plan"]["name"]
                    and int(halt["height"]) == kit["plan"]["height"]
                    and halt["info"] == kit["plan"]["info"], "wrong upgrade halt")
        else:
            node.stop()
        node.start(kit["new"])
        node.wait(lambda: int(node.rpc.call("eth_blockNumber", []), 16) > kit["plan"]["height"])
        node.applied()
        preserved = snapshot(node.rpc, kit["watch"], chain)
        assert_state(preserved["state"], before["state"])
        retained(node.rpc, before, before_receipts)
        after_receipts = produce(node.rpc, kit["after_transactions"], kit["watch"], chain)
        after = snapshot(node.rpc, kit["watch"], chain)
        assert_state(after["state"], kit["after"])
        node.stop()
        node.start(kit["new"])
        node.wait(lambda: int(node.rpc.call("eth_blockNumber", []), 16) > int(after["height"], 16))
        recovered = snapshot(node.rpc, kit["watch"], chain)
        assert_state(recovered["state"], after["state"])
        retained(node.rpc, before, before_receipts)
        retained(node.rpc, after, after_receipts)
        node.applied()
        evidence.update(before=before, preserved=preserved, after=after, recovered=recovered,
                        before_receipts=before_receipts, after_receipts=after_receipts)
        return evidence
    finally:
        node.stop()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--kind", choices=("major", "minor"), required=True)
    parser.add_argument("--kit", type=Path, required=True)
    parser.add_argument("--kit-sha256", required=True)
    parser.add_argument("--evidence", type=Path, required=True)
    args = parser.parse_args()
    require(re.fullmatch(r"[0-9a-f]{64}", args.kit_sha256)
            and digest(args.kit) == args.kit_sha256, "release kit digest mismatch")
    evidence = run(json.loads(args.kit.read_text()), args.kind)
    with args.evidence.open("x") as stream:
        json.dump(evidence, stream, sort_keys=True)
        stream.write("\n")
    print("PASS")


if __name__ == "__main__":
    main()
