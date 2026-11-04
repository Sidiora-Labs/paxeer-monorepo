#!/usr/bin/env python3
"""Durable HPX fleet roster and funding journal, shared by both shell entrypoints."""
import argparse
import base64
import contextlib
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import sys
import time


class FleetError(RuntimeError):
    pass


def require(condition, message):
    if not condition:
        raise FleetError(message)


def atomic(path, value):
    path = Path(path)
    fd, name = tempfile.mkstemp(dir=path.parent)
    try:
        with os.fdopen(fd, "w") as stream:
            stream.write(value)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(name, path)
        directory = os.open(path.parent, os.O_DIRECTORY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        if os.path.exists(name):
            os.unlink(name)


class FleetState:
    def __init__(self, directory):
        self.directory = Path(directory)
        self.directory.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.path = self.directory / "state.json"
        self.data = json.loads(self.path.read_text()) if self.path.exists() else {"hosts": {}}
        legacy = self.directory / "operators.tsv"
        if not self.path.exists() and legacy.exists():
            # Existing deployments may only have the old public roster. Import it
            # without granting verification, so temporary SSH loss cannot erase it.
            for row in legacy.read_text().splitlines():
                fields = row.split("\t")
                require(len(fields) == 4, "invalid legacy roster row")
                mon, host, acc, val = fields
                require(host not in self.data["hosts"], "duplicate legacy host")
                self.data["hosts"][host] = {"identity": [mon, acc, val], "verified": False}

    def save(self):
        atomic(self.path, json.dumps(self.data, sort_keys=True, indent=2) + "\n")
        rows = []
        for host, result in sorted(self.data["hosts"].items()):
            if "identity" in result:
                mon, acc, val = result["identity"]
                rows.append(f"{mon}\t{host}\t{acc}\t{val}\n")
        atomic(self.directory / "operators.tsv", "".join(rows))

    def begin(self, hosts):
        hosts = sorted(set(hosts) | set(self.data["hosts"]))
        require(hosts, "empty intended roster")
        require(all(re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9.:-]*", h) for h in hosts), "invalid host")
        if "plan" in self.data:
            require(set(hosts) == set(self.data["hosts"]), "allocation plan freezes the roster")
        for host in hosts:
            self.data["hosts"].setdefault(host, {})["verified"] = False
        self.save()
        return hosts

    def record(self, host, line):
        require(host in self.data["hosts"], "host outside intended roster")
        result = self.data["hosts"][host]
        fields = line.split()
        require(len(fields) in (4, 5) and fields[0] == "OPERATOR", "invalid OPERATOR result")
        mon, acc, val = fields[1:4]
        # hpx emits SDK Bech32 account and validator addresses (20-byte payloads).
        require(re.fullmatch(r"pax1[023456789acdefghjklmnpqrstuvwxyz]{38}", acc), "invalid account address")
        require(re.fullmatch(r"paxvaloper1[023456789acdefghjklmnpqrstuvwxyz]{38}", val), "invalid validator address")
        identity = [mon, acc, val]
        require("identity" not in result or result["identity"] == identity, "host identity changed")
        require(all(h == host or r.get("identity", [None, None])[1] != acc for h, r in self.data["hosts"].items()), "duplicate operator account")
        result.update(identity=identity, verified=True)
        self.save()

    def complete(self):
        hosts = self.data["hosts"]
        require(hosts and all(r.get("verified") and r.get("identity") for r in hosts.values()), "incomplete intended roster; retry keygen")
        self.save()  # Consumers receive a fresh projection of the canonical roster.
        return [r["identity"][1] for _, r in sorted(hosts.items())]

    def plan(self, sender, balance, buffer, context):
        accounts = self.complete()
        if "plan" not in self.data:
            require(balance > buffer >= 0, "insufficient funding balance after fee reserve")
            allocation = (balance - buffer) // len(accounts)
            require(allocation > 0, "empty allocation")
            self.data["plan"] = {"sender": sender, "context": context, "targets": {a: allocation for a in accounts}}
            self.data["payments"] = []
            self.save()  # The original target is durable before signing or broadcast.
        plan = self.data["plan"]
        require(plan["sender"] == sender and plan["context"] == context and set(plan["targets"]) == set(accounts), "funding plan identity/context changed")
        return plan

    def deficit(self, account, balance):
        require(balance >= 0, "negative recipient balance")
        return max(0, self.data["plan"]["targets"][account] - balance)

    def prepare(self, account, amount, signed, encoded):
        require(account in self.data["plan"]["targets"], "recipient outside allocation plan")
        require(0 < amount <= self.data["plan"]["targets"][account], "invalid payment amount")
        require(all(p["state"] == "committed" for p in self.data["payments"]), "unresolved payment blocks new signing")
        raw = base64.b64decode(encoded, validate=True)
        require(raw, "empty encoded transaction")
        payment = {"account": account, "amount": amount, "signed": signed, "hash": hashlib.sha256(raw).hexdigest().upper(), "state": "prepared"}
        self.data["payments"].append(payment)
        self.save()  # Preserve the exact signed bytes and hash before submission.
        return payment

    def receipt(self, payment, response, balance):
        tx = response.get("tx_response", response)
        require(tx.get("txhash", "").upper() == payment["hash"], "transaction hash mismatch")
        require(int(tx.get("height", 0)) > 0 and "code" in tx, "transaction not committed")
        require(int(tx["code"]) == 0, "committed transaction failed")
        require(balance >= self.data["plan"]["targets"][payment["account"]], "recipient balance below allocation")
        payment.update(state="committed", receipt=tx, balance=balance)
        self.save()


class Paxd:
    """Production CLI transport; only signed transactions and public queries cross it."""
    def __init__(self, executable, rpc, chain, key, keyring, gas, backend):
        self.executable, self.rpc, self.chain, self.key = executable, rpc, chain, key
        self.kb = ["--keyring-backend", backend, "--keyring-dir", keyring]
        self.gas = gas

    def run(self, *args):
        return subprocess.check_output([self.executable, *args], text=True, timeout=90).strip()

    def balance(self, address):
        data = json.loads(self.run("q", "bank", "balances", address, "--node", self.rpc, "-o", "json"))
        return sum(int(c["amount"]) for c in data["balances"] if c["denom"] == "uhpx")

    def sign(self, account, amount, directory):
        unsigned = json.loads(self.run("tx", "bank", "send", self.key, account, f"{amount}uhpx", "--from", self.key, "--node", self.rpc, "--chain-id", self.chain, "--gas", "auto", "--gas-adjustment", "1.5", "--gas-prices", self.gas, *self.kb, "--generate-only", "-o", "json"))
        path = Path(directory) / "unsigned.json"
        atomic(path, json.dumps(unsigned))
        signed = json.loads(self.run("tx", "sign", str(path), "--from", self.key, "--node", self.rpc, "--chain-id", self.chain, *self.kb, "-o", "json"))
        atomic(Path(directory) / "signed.json", json.dumps(signed))
        encoded = self.run("tx", "encode", str(Path(directory) / "signed.json"))
        return signed, encoded

    def submit(self, payment, directory):
        path = Path(directory) / "signed.json"
        atomic(path, json.dumps(payment["signed"]))
        result = json.loads(self.run("tx", "broadcast", str(path), "--node", self.rpc, "--broadcast-mode", "sync", "-o", "json"))
        tx = result.get("tx_response", result)
        require(tx.get("txhash", "").upper() == payment["hash"], "broadcast hash mismatch")
        require("code" in tx and int(tx["code"]) == 0, "CheckTx refused payment")

    def transaction(self, txhash):
        try:
            return json.loads(self.run("q", "tx", txhash, "--node", self.rpc, "-o", "json"))
        except subprocess.CalledProcessError:
            return None  # Not indexed yet; never considered a successful payment.


def fund(state, peer, sender, buffer, context, broadcast, attempts=30, pause=time.sleep):
    # Plan creation and deficit computation are identical for preview and broadcast.
    plan = state.plan(sender, peer.balance(sender), buffer, context)
    for payment in state.data["payments"]:
        if payment["state"] != "committed":
            require(broadcast, "pending payment requires committed reconciliation")
            settle(state, peer, payment, attempts, pause, submit=False)
    for account, target in plan["targets"].items():
        deficit = state.deficit(account, peer.balance(account))
        if not deficit:
            continue
        if not broadcast:
            print(f"[dry-run] {account}: outstanding {deficit}uhpx")
            continue
        signed, encoded = peer.sign(account, deficit, state.directory)
        payment = state.prepare(account, deficit, signed, encoded)
        settle(state, peer, payment, attempts, pause, submit=True)
    if broadcast:
        require(all(peer.balance(a) >= t for a, t in plan["targets"].items()), "unresolved funding allocation")


def settle(state, peer, payment, attempts, pause, submit):
    result = peer.transaction(payment["hash"])
    if result is None:
        # On recovery rebroadcast the same signed bytes, never create a second payment.
        try:
            peer.submit(payment, state.directory)
        except (FleetError, subprocess.CalledProcessError) as error:
            if submit:
                raise
            # A duplicate CheckTx refusal still requires a committed query below.
            print(f"Rebroadcast unresolved for {payment['hash']}: {error}", file=sys.stderr)
    for _ in range(attempts):
        result = peer.transaction(payment["hash"])
        if result is not None:
            state.receipt(payment, result, peer.balance(payment["account"]))
            return
        pause(1)
    raise FleetError(f"unresolved payment {payment['hash']}; retry the same journal")


@contextlib.contextmanager
def locked(directory):
    Path(directory).mkdir(mode=0o700, parents=True, exist_ok=True)
    with open(Path(directory) / "state.lock", "a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        yield FleetState(directory)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("directory")
    parser.add_argument("action", choices=["begin", "record", "complete", "fund"])
    parser.add_argument("values", nargs="*")
    args = parser.parse_args()
    os.umask(0o077)
    with locked(args.directory) as state:
        if args.action == "begin":
            print("\n".join(state.begin(args.values)))
        elif args.action == "record":
            state.record(*args.values)
        elif args.action == "complete":
            state.complete()
        else:
            executable, rpc, chain, key, keyring, gas, backend, sender, buffer, yes = args.values
            peer = Paxd(executable, rpc, chain, key, keyring, gas, backend)
            fund(state, peer, sender, int(buffer), {"rpc": rpc, "chain": chain, "denom": "uhpx"}, yes == "1")


if __name__ == "__main__":
    try:
        main()
    except (FleetError, OSError, ValueError, subprocess.SubprocessError) as error:
        raise SystemExit(str(error))
