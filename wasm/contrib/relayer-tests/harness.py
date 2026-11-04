"""Safety and receipt checks for the upstream two-chain relayer example."""

import base64
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import signal
import subprocess
import sys
import time
import tomllib


class Refusal(ValueError):
    """A harness observation is unsafe, incomplete, or unsuccessful."""


def require(condition, message):
    if not condition:
        raise Refusal(message)


def read_json(path):
    return json.loads(Path(path).read_text())


def check_homes(root):
    root = Path(root).absolute()
    for name in ("data", ".relayer"):
        home = root / name
        require(not home.is_symlink(), f"Refusing symlink home: {home}")
        require(home.resolve().parent == root.resolve(), "Home escapes harness root")
    return root


def chain_home(root, chain):
    root = Path(root).resolve()
    require(re.fullmatch(r"[A-Za-z0-9_-]+", chain), "Invalid chain identity")
    home = root / chain
    require(not home.is_symlink(), "Refusing symlink chain home")
    require(home.resolve().parent == root, "Chain home escapes data root")
    require(not (root / "pids").is_symlink(), "Refusing symlink PID directory")
    return home


def ports(values):
    require(len(values) == 4, "Four listener ports required")
    require(all(re.fullmatch(r"[0-9]+", str(p)) for p in values), "Invalid listener port")
    result = tuple(int(p) for p in values)
    require(all(1 <= p <= 65535 for p in result), "Listener port outside TCP range")
    require(len(set(result)) == 4, "Listener ports must be distinct")
    return result


def check_config(path, values):
    rpc, peer, profile, grpc = ports(values)
    with Path(path).open("rb") as stream:
        config = tomllib.load(stream)
    require(config["rpc"]["laddr"] == f"tcp://0.0.0.0:{rpc}", "RPC listener mismatch")
    require(config["p2p"]["laddr"] == f"tcp://0.0.0.0:{peer}", "Peer listener mismatch")
    require(config["rpc"]["pprof_laddr"] == f"localhost:{profile}", "Profiling listener mismatch")
    with Path(path).with_name("app.toml").open("rb") as stream:
        app = tomllib.load(stream)
    require(app["grpc"]["address"] == f"0.0.0.0:{grpc}", "gRPC listener mismatch")
    return grpc


def process_identity(pid):
    """Return a birth token, executable and exact argv; zombies are stopped."""
    require(isinstance(pid, int) and pid > 1, "Invalid tracked PID")
    proc = Path(f"/proc/{pid}")
    if sys.platform.startswith("linux"):
        try:
            stat = (proc / "stat").read_text().rsplit(")", 1)[1].split()
            if stat[0] == "Z":
                return None
            argv = (proc / "cmdline").read_bytes().rstrip(b"\0").split(b"\0")
            return {
                "birth": Path("/proc/sys/kernel/random/boot_id").read_text().strip() + ":" + stat[19],
                "binary": str((proc / "exe").resolve(strict=True)),
                "argv": [os.fsdecode(arg) for arg in argv],
            }
        except (FileNotFoundError, ProcessLookupError):
            return None
    # macOS has no /proc. lstart protects against PID reuse, while comm and
    # shlex-split args independently verify the executable and --home argument.
    def ps(field):
        result = subprocess.run(
            ["ps", "-ww", "-p", str(pid), "-o", field + "="],
            text=True, capture_output=True, check=False,
        )
        require(result.returncode in (0, 1), "Cannot inspect tracked process")
        return result.stdout.strip()

    birth = ps("lstart")
    if not birth or ps("stat").startswith("Z"):
        return None
    return {"birth": birth, "binary": str(Path(ps("comm")).resolve()), "argv": shlex.split(ps("args"))}


def validate_identity(identity, binary, home):
    require(identity is not None, "Child exited before PID registration")
    require(identity["binary"] == str(Path(binary).resolve()), "Tracked executable mismatch")
    argv = identity["argv"]
    require("--home" in argv and argv.count("--home") == 1, "Missing or ambiguous process home")
    index = argv.index("--home")
    require(index + 1 < len(argv) and argv[index + 1] == str(home), "Tracked process home mismatch")


def record_child(root, chain, pid, binary):
    home = chain_home(root, chain)
    identity = process_identity(int(pid))
    validate_identity(identity, binary, home)
    directory = Path(root) / "pids"
    directory.mkdir(parents=True, exist_ok=True)
    record = directory / f"{chain}.json"
    require(not record.exists() and not record.is_symlink(), "Existing child record must be stopped first")
    value = {"pid": int(pid), "home": str(home), **identity}
    temporary = record.with_suffix(f".{os.getpid()}.tmp")
    temporary.write_text(json.dumps(value))
    temporary.replace(record)
    return value


def signal_owned(pid, identity, signum):
    if sys.platform.startswith("linux"):
        # A pidfd pins the process across the identity check and the signal,
        # including the otherwise dangerous exit/PID-reuse race.
        try:
            descriptor = os.pidfd_open(pid)
        except ProcessLookupError:
            return
        try:
            current = process_identity(pid)
            if current is None:
                return
            require(current == identity, "PID identity changed before signal")
            signal.pidfd_send_signal(descriptor, signum)
        except ProcessLookupError:
            pass
        finally:
            os.close(descriptor)
    else:
        current = process_identity(pid)
        if current is None:
            return
        require(current == identity, "PID identity changed before signal")
        try:
            os.kill(pid, signum)
        except ProcessLookupError:
            pass


def stop_children(root, chain=None, expected_pid=None, timeout=5):
    root = Path(root).resolve()
    require(not (root / "pids").is_symlink(), "Refusing symlink PID directory")
    records = [root / "pids" / f"{chain}.json"] if chain else sorted((root / "pids").glob("*.json"))
    for record in records:
        if not record.exists():
            continue
        require(not record.is_symlink(), "Refusing symlink PID record")
        saved = read_json(record)
        home = chain_home(root, record.stem)
        require(saved["home"] == str(home), "Recorded home mismatch")
        pid = saved["pid"]
        if expected_pid is not None:
            require(pid == int(expected_pid), "Record belongs to another child; refusing cleanup")

        def owned():
            current = process_identity(pid)
            if current is None:
                return False
            validate_identity(current, saved["binary"], home)
            require(current == {k: saved[k] for k in ("birth", "binary", "argv")}, "PID identity changed; refusing signal")
            return True

        if owned():
            identity = {k: saved[k] for k in ("birth", "binary", "argv")}
            signal_owned(pid, identity, signal.SIGTERM)
            deadline = time.monotonic() + timeout
            while owned() and time.monotonic() < deadline:
                time.sleep(0.02)
            if owned():
                signal_owned(pid, identity, signal.SIGKILL)
                deadline = time.monotonic() + timeout
                while owned() and time.monotonic() < deadline:
                    time.sleep(0.02)
                require(not owned(), "Tracked child did not stop")
        record.unlink(missing_ok=True)


def balance(text):
    """Parse legacy rly query balance's SDK Coins string without accepting noise."""
    text = text.strip()
    require(bool(text), "Empty balance response")
    result = {}
    for coin in text.split(","):
        match = re.fullmatch(r"([0-9]+)([A-Za-z][A-Za-z0-9/:._-]*)", coin)
        require(match is not None, "Unparseable balance response")
        amount, denom = match.groups()
        require(denom not in result, "Duplicate balance denomination")
        result[denom] = int(amount)
    return result


def path_ends(text):
    # Legacy rly paths show --json wraps Path in PathWithStatus.chains.
    value = json.loads(text)
    path = value["chains"]
    require(path["src"]["chain-id"] == "ibc-0" and path["dst"]["chain-id"] == "ibc-1", "Unexpected local chain path")
    for end in (path["src"], path["dst"]):
        require(end["port-id"] == "transfer", "Expected ICS20 transfer port")
        require(re.fullmatch(r"channel-[0-9]+", end["channel-id"]), "Missing linked channel")
        require(end["version"] == "ics20-1", "Unexpected transfer version")
    return path["src"], path["dst"]


def voucher_denom(destination):
    trace = f'{destination["port-id"]}/{destination["channel-id"]}/test'
    return "ibc/" + hashlib.sha256(trace.encode()).hexdigest().upper()


def transactions(text):
    # Legacy rly query txs marshals []*Tendermint ResultTx directly, not an
    # SDK SearchTxsResult wrapper. A full page cannot prove complete history.
    value = json.loads(text)
    require(isinstance(value, list), "Unparseable transaction search")
    require(len(value) < 100, "Incomplete transaction search; refusing full history page")
    for tx in value:
        require(re.fullmatch(r"[A-Fa-f0-9]{64}", tx["hash"]), "Invalid transaction hash")
        # Protobuf ABCI ResponseDeliverTx omits its default zero code.
        require(str(tx["tx_result"].get("code", 0)) == "0", "Chain transaction refused")
        require(isinstance(tx["tx_result"].get("events"), list), "Missing transaction events")
    return value


def events(txs, kind):
    for tx in txs:
        for event in tx["tx_result"]["events"]:
            if event["type"] == kind:
                attributes = event["attributes"]
                pairs = {
                    base64.b64decode(item["key"], validate=True).decode("utf-8"):
                    base64.b64decode(item["value"], validate=True).decode("utf-8")
                    for item in attributes
                }
                require(len(pairs) == len(attributes), "Duplicate event attribute")
                yield tx["hash"], pairs


def packet_matches(event, src, dst):
    return all(event.get(key) == value for key, value in {
        "packet_src_port": src["port-id"], "packet_src_channel": src["channel-id"],
        "packet_dst_port": dst["port-id"], "packet_dst_channel": dst["channel-id"],
    }.items())


def find_packet(txs, previous, src, dst, sender, receiver):
    packets = []
    for txhash, event in events(txs, "send_packet"):
        if txhash in previous or not packet_matches(event, src, dst):
            continue
        data = json.loads(event["packet_data"])
        require(data == {"denom": "test", "amount": "1000000", "sender": sender, "receiver": receiver}, "Unexpected transfer packet data")
        sequence = event["packet_sequence"]
        require(re.fullmatch(r"[1-9][0-9]*", sequence), "Invalid packet sequence")
        packets.append({"txhash": txhash, "sequence": sequence})
    require(len(packets) <= 1, "Ambiguous new transfer packet")
    return packets[0] if packets else None


def receipt_complete(packet, src, dst, writes, acknowledgements, initial_src, initial_dst, current_src, current_dst):
    successful = False
    for _, event in events(writes, "write_acknowledgement"):
        if packet_matches(event, src, dst) and event["packet_sequence"] == packet["sequence"]:
            ack = json.loads(event["packet_ack"])
            require(ack == {"result": "AQ=="}, "ICS20 acknowledgement refused")
            successful = True
    acknowledged = any(
        packet_matches(event, src, dst) and event["packet_sequence"] == packet["sequence"]
        for _, event in events(acknowledgements, "acknowledge_packet")
    )
    denom = voucher_denom(dst)
    require("test" in initial_src and "test" in current_src, "Missing source native balance")
    delta = current_dst.get(denom, 0) - initial_dst.get(denom, 0)
    debit = initial_src["test"] - current_src["test"]
    require(delta <= 1000000 and debit <= 1000000, "Unexpected balance movement")
    return successful and acknowledged and delta == 1000000 and debit == 1000000


class Relayer:
    def __init__(self, home, evidence, timeout=60):
        self.home = str(Path(home).resolve())
        self.evidence = Path(evidence)
        self.evidence.mkdir(parents=True)
        self.deadline = time.monotonic() + timeout
        self.counter = 0

    def call(self, *args):
        left = self.deadline - time.monotonic()
        require(left > 0, "IBC transfer deadline expired")
        self.counter += 1
        prefix = self.evidence / f"{self.counter:03d}"
        command = ["rly", "--home", self.home, *args]
        prefix.with_suffix(".command.json").write_text(json.dumps(command))
        with prefix.with_suffix(".stdout").open("w") as stdout, prefix.with_suffix(".stderr").open("w") as stderr:
            result = subprocess.run(command, stdout=stdout, stderr=stderr, text=True, timeout=min(10, left), check=False)
        require(result.returncode == 0, f"Relayer refused command; evidence: {prefix}")
        return prefix.with_suffix(".stdout").read_text()


def run_transfer(home, root):
    evidence = Path(root) / f"transfer-{time.time_ns()}"
    relayer = Relayer(home, evidence)
    print(f"Transfer evidence: {evidence}", flush=True)
    relayer.call("tx", "link", "demo", "-d")
    src, dst = path_ends(relayer.call("paths", "show", "demo", "--json"))
    sender = relayer.call("chains", "address", "ibc-0").strip()
    receiver = relayer.call("chains", "address", "ibc-1").strip()
    require(re.fullmatch(r"wasm1[0-9a-z]+", sender) and re.fullmatch(r"wasm1[0-9a-z]+", receiver), "Invalid relayer address")
    initial_src = balance(relayer.call("q", "bal", "ibc-0", "--ibc-denoms"))
    initial_dst = balance(relayer.call("q", "bal", "ibc-1", "--ibc-denoms"))
    query = f"send_packet.packet_src_channel={src['channel-id']}"
    previous = {tx["hash"] for tx in transactions(relayer.call("q", "txs", "ibc-0", query, "--limit", "100"))}
    relayer.call("tx", "transfer", "ibc-0", "ibc-1", "1000000test", receiver, "--path", "demo")
    packet = None
    while time.monotonic() < relayer.deadline:
        if packet is None:
            packet = find_packet(transactions(relayer.call("q", "txs", "ibc-0", query, "--limit", "100")), previous, src, dst, sender, receiver)
            if packet:
                (evidence / "packet.json").write_text(json.dumps(packet))
        if packet:
            relayer.call("tx", "relay-packets", "demo")
            relayer.call("tx", "relay-acknowledgements", "demo")
            write_query = f"write_acknowledgement.packet_dst_channel={dst['channel-id']}&write_acknowledgement.packet_sequence={packet['sequence']}"
            ack_query = f"acknowledge_packet.packet_src_channel={src['channel-id']}&acknowledge_packet.packet_sequence={packet['sequence']}"
            writes = transactions(relayer.call("q", "txs", "ibc-1", write_query, "--limit", "100"))
            acks = transactions(relayer.call("q", "txs", "ibc-0", ack_query, "--limit", "100"))
            current_src = balance(relayer.call("q", "bal", "ibc-0", "--ibc-denoms"))
            current_dst = balance(relayer.call("q", "bal", "ibc-1", "--ibc-denoms"))
            if receipt_complete(packet, src, dst, writes, acks, initial_src, initial_dst, current_src, current_dst):
                (evidence / "receipt.json").write_text(json.dumps({"packet": packet, "denom": voucher_denom(dst), "source_before": initial_src, "source_after": current_src, "destination_before": initial_dst, "destination_after": current_dst}))
                print("IBC transfer executed successfully")
                return
        time.sleep(min(1, max(0, relayer.deadline - time.monotonic())))
    raise Refusal(f"Timed out awaiting successful acknowledgement and exact balances; evidence: {evidence}")


def main(args):
    action, *values = args
    if action == "homes":
        check_homes(*values)
    elif action == "chain-home":
        chain_home(*values)
    elif action == "ports":
        ports(values)
    elif action == "config":
        check_config(values[0], values[1:])
    elif action == "record":
        record_child(*values)
    elif action == "stop":
        stop_children(*values)
    elif action == "transfer":
        run_transfer(*values)
    else:
        raise Refusal("Unknown harness operation")


if __name__ == "__main__":
    try:
        main(sys.argv[1:])
    except (Refusal, KeyError, TypeError, ValueError, OSError, subprocess.SubprocessError) as error:
        print(f"Harness refused: {error}", file=sys.stderr)
        sys.exit(1)
