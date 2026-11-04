#!/usr/bin/env python3
"""Run a verified Ignite faucet for a disposable local paxd chain.

Example: start_faucet.sh --disposable --account admin --recipient pax1...
The recipient must be a disposable probe account. Startup makes one real transfer
and verifies its committed transaction before reporting readiness.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shutil
import signal
import stat
import subprocess
import sys
import tarfile
import tempfile
import time
import tomllib
import urllib.error
import urllib.parse
import urllib.request

RELEASE = "0.0.3"
# Independently pinned from the release's published checksums, not downloaded at runtime.
RELEASE_SHA256 = {
    ("Linux", "x86_64"): "02c815c127939a4996bcdb6c7a543cc099907e82426b427b5a03de9fe34270ab",
    ("Linux", "aarch64"): "490c04909a1cfc64ceca2a0d1bae8d3ff620581359fcb80fccdf8b2129e82976",
    ("Darwin", "x86_64"): "7337fbcba05e9db93f97a565d03629137bbb763857a530fabe48fa330628d124",
    ("Darwin", "arm64"): "562f0620b5f2edf1d2aba53b5bbece99db04285ff8335804bf03b25b3e1b78af",
}
MAX_ARTIFACT = 256 * 1024 * 1024


def require(condition, message):
    if not condition:
        raise ValueError(message)


def release_artifact(system=None, machine=None):
    system, machine = system or platform.system(), machine or platform.machine()
    key = (system, machine)
    require(key in RELEASE_SHA256, "unsupported faucet release platform")
    arch = {"x86_64": "amd64", "aarch64": "arm64", "arm64": "arm64"}[machine]
    name = f"faucet_{RELEASE}_{system.lower()}_{arch}.tar.gz"
    return f"https://github.com/ignite/faucet/releases/download/v{RELEASE}/{name}", RELEASE_SHA256[key]


def private_directory(path):
    info = Path(path).lstat()
    require(stat.S_ISDIR(info.st_mode) and info.st_uid == os.getuid(), "unsafe staging directory ownership")
    require(stat.S_IMODE(info.st_mode) == 0o700, "staging directory must have mode 0700")


def owned_file(path, executable=False):
    path = Path(path)
    private_directory(path.parent)
    info = path.lstat()
    require(stat.S_ISREG(info.st_mode) and info.st_uid == os.getuid() and info.st_nlink == 1, "unsafe artifact ownership or type")
    require(stat.S_IMODE(info.st_mode) == (0o500 if executable else 0o600), "unsafe artifact permissions")


def download(url, destination):
    require(urllib.parse.urlsplit(url).scheme == "https", "artifact requires HTTPS")
    private_directory(Path(destination).parent)
    # urlopen enforces TLS certificate and hostname verification by default.
    with urllib.request.urlopen(url, timeout=60) as response:
        require(response.status == 200 and urllib.parse.urlsplit(response.url).scheme == "https", "artifact transfer failed or downgraded")
        fd = os.open(destination, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "wb") as output:
            total = 0
            while block := response.read(1024 * 1024):
                total += len(block)
                require(total <= MAX_ARTIFACT, "artifact is oversized")
                output.write(block)
            require(total > 0, "empty artifact download")
            output.flush()
            os.fsync(output.fileno())
    owned_file(destination)


def verify_archive(archive, digest, destination):
    archive, destination = Path(archive), Path(destination)
    owned_file(archive)
    private_directory(destination.parent)
    with archive.open("rb") as stream:
        require(hashlib.file_digest(stream, "sha256").hexdigest() == digest, "faucet release digest mismatch")
    with tarfile.open(archive, "r:gz") as bundle:
        members = bundle.getmembers()
        require(all(m.isfile() or m.isdir() for m in members), "release contains a link or special file")
        require(all(not Path(m.name).is_absolute() and ".." not in Path(m.name).parts for m in members), "release contains an unsafe path")
        binaries = [m for m in members if m.name == "faucet" and m.isfile()]
        require(len(binaries) == 1 and 0 < binaries[0].size <= MAX_ARTIFACT, "missing, duplicate or oversized faucet binary")
        # Extract only the authenticated member; archive permissions and paths are never applied.
        with bundle.extractfile(binaries[0]) as source:
            fd = os.open(destination, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
            with os.fdopen(fd, "wb") as output:
                shutil.copyfileobj(source, output)
                require(output.tell() == binaries[0].size, "incomplete faucet binary")
                output.flush()
                os.fsync(output.fileno())
    os.chmod(destination, 0o500)
    owned_file(destination, executable=True)
    return destination


def local_chain(home, binary):
    home = Path(home).expanduser().resolve(strict=True)
    executable = shutil.which(str(binary))
    require(executable is not None, "paxd executable was not found")
    executable = Path(executable).resolve(strict=True)
    require(executable.is_file() and os.access(executable, os.X_OK), "paxd is not executable")
    config = tomllib.loads((home / "config/client.toml").read_text())
    genesis = json.loads((home / "config/genesis.json").read_text())
    chain = config["chain-id"]
    require(isinstance(chain, str) and chain and genesis["chain_id"] == chain, "client and genesis chain IDs differ")
    require(config["keyring-backend"] == "test", "faucet requires a disposable test keyring")
    node = urllib.parse.urlsplit(config["node"])
    require(node.scheme in ("tcp", "http") and node.hostname in ("localhost", "127.0.0.1", "::1") and node.port is not None, "faucet requires an explicit local node endpoint")
    require(not node.username and not node.password and not node.query and not node.fragment and node.path in ("", "/"), "invalid local node endpoint")
    return {"home": str(home), "binary": str(executable), "chain": chain, "node": config["node"]}


def query_command(config, *arguments):
    return [config["binary"], *arguments, "--home", config["home"], "--node", config["node"], "--chain-id", config["chain"], "--output", "json"]


def faucet_command(executable, config, account, denom, amount, fee, port):
    return [
        str(executable), "--cli-name", config["binary"], "--home", config["home"],
        "--node", config["node"], "--account-name", account, "--keyring-backend", "test",
        "--sdk-version", "stargate", "--denoms", denom, "--credit-amount", str(amount),
        "--max-credit", str(amount * 10), "--fee-amount", str(fee), "--port", str(port),
        "--mnemonic", "", "--keyring-password", "",
    ]


def json_command(command):
    result = subprocess.run(command, check=True, capture_output=True, text=True, timeout=60)
    return json.loads(result.stdout)


def http_json(url, value=None):
    data = None if value is None else json.dumps(value).encode()
    request = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(request, timeout=60) as response:
        require(response.status == 200, "faucet request failed")
        return json.load(response)


def request_funding(url, recipient, denom, amount):
    response = http_json(url, {"address": recipient, "coins": [f"{amount}{denom}"]})
    require(isinstance(response, dict) and not response.get("error"), "faucet refused the probe transfer")


def funding_transaction(value, sender, recipient, denom, amount, after_height):
    require(isinstance(value, dict) and type(value.get("code")) is int and value["code"] == 0, "funding transaction failed")
    require(int(value["height"]) > after_height, "funding transaction is not newly committed")
    require(re.fullmatch(r"[0-9A-Fa-f]{64}", value["txhash"]) is not None, "invalid transaction hash")
    logs = json.loads(value["raw_log"])
    for log in logs:
        for event in log["events"]:
            if event["type"] == "transfer":
                attributes = {a["key"]: a["value"] for a in event["attributes"]}
                if attributes.get("sender") == sender and attributes.get("recipient") == recipient and attributes.get("amount") == f"{amount}{denom}":
                    return value["txhash"]
    raise ValueError("committed transaction does not contain the requested funding transfer")


def stop_child(child):
    if child.poll() is None:
        child.terminate()
        try:
            child.wait(timeout=30)
        except subprocess.TimeoutExpired:
            child.kill()
            child.wait()


def launch(args):
    require(args.disposable, "acknowledge a disposable local chain with --disposable")
    require(0 < args.amount < 2**60 and 0 <= args.fee < 2**63 and 0 < args.port < 65536, "invalid amount, fee or port")
    require(re.fullmatch(r"[a-zA-Z][a-zA-Z0-9/:._-]{2,127}", args.denom) is not None, "invalid denomination")
    require(re.fullmatch(r"[a-zA-Z0-9_-]+", args.account) is not None, "invalid funding account name")
    require(re.fullmatch(r"[a-z0-9]+1[a-z0-9]+", args.recipient) is not None, "invalid probe recipient")
    config = local_chain(args.home, args.paxd)
    # The chain is resolved from the same home and endpoint passed to the faucet.
    node = urllib.parse.urlsplit(config["node"])
    rpc = urllib.parse.urlunsplit(("http", node.netloc, "", "", ""))
    status = http_json(rpc + "/status")
    require(status["node_info"]["network"] == config["chain"] and status["sync_info"]["catching_up"] is False, "local node is on another chain or still syncing")
    sender_result = subprocess.run(
        [config["binary"], "keys", "show", args.account, "-a", "--home", config["home"], "--keyring-backend", "test"],
        check=True, capture_output=True, text=True, timeout=30,
    )
    sender = sender_result.stdout.strip()
    require(re.fullmatch(r"[a-z0-9]+1[a-z0-9]+", sender) is not None and sender != args.recipient, "probe must fund another disposable account")
    url, digest = release_artifact()
    with tempfile.TemporaryDirectory(prefix="pax-faucet-") as temporary:
        staging = Path(temporary)
        private_directory(staging)
        archive = staging / "release.tar.gz"
        download(url, archive)
        executable = verify_archive(archive, digest, staging / "faucet")
        owned_file(executable, executable=True)
        command = faucet_command(executable, config, args.account, args.denom, args.amount, args.fee, args.port)
        child = subprocess.Popen(command, env={"PATH": os.defpath, "HOME": str(Path.home()), "LANG": "C.UTF-8"})
        previous = {}
        def interrupted(signum, frame):
            raise KeyboardInterrupt
        try:
            for signum in (signal.SIGINT, signal.SIGTERM):
                previous[signum] = signal.signal(signum, interrupted)
            endpoint = f"http://127.0.0.1:{args.port}"
            deadline = time.monotonic() + 30
            while True:
                require(child.poll() is None, "faucet exited before readiness")
                try:
                    info = http_json(endpoint + "/info")
                    require(info.get("is_a_faucet") is True and info.get("chain_id") == config["chain"], "faucet selected the wrong chain")
                    break
                except (urllib.error.URLError, TimeoutError):
                    require(time.monotonic() < deadline, "faucet readiness timed out")
                    time.sleep(0.2)
            before_height = int(http_json(rpc + "/status")["sync_info"]["latest_block_height"])
            request_funding(endpoint + "/", args.recipient, args.denom, args.amount)
            matches = json_command(query_command(
                config, "query", "txs", "--events", f"message.sender={sender}&transfer.recipient={args.recipient}",
                "--page", "1", "--limit", "100",
            ))
            hashes = []
            for value in matches["txs"]:
                if int(value["height"]) > before_height:
                    hashes.append(funding_transaction(value, sender, args.recipient, args.denom, args.amount, before_height))
            require(len(hashes) == 1, "probe did not yield exactly one committed funding transaction")
            confirmed = json_command(query_command(config, "query", "tx", hashes[0]))
            require(funding_transaction(confirmed, sender, args.recipient, args.denom, args.amount, before_height) == hashes[0], "funding hash changed on query")
            require(child.poll() is None, "faucet exited during the funding probe")
            print(f"Faucet ready on port {args.port}; committed funding transaction {hashes[0]}", flush=True)
            require(child.wait() == 0, "faucet exited unsuccessfully")
        finally:
            stop_child(child)
            for signum, handler in previous.items():
                signal.signal(signum, handler)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--disposable", action="store_true")
    parser.add_argument("--home", default=os.environ.get("PAX_HOME", str(Path.home() / ".pax")))
    parser.add_argument("--paxd", default=os.environ.get("PAXD_BIN", "paxd"))
    parser.add_argument("--account", required=True)
    parser.add_argument("--recipient", required=True)
    parser.add_argument("--denom", default="uhpx")
    parser.add_argument("--amount", type=int, default=10000000)
    parser.add_argument("--fee", type=int, default=0)
    parser.add_argument("--port", type=int, default=8000)
    launch(parser.parse_args(argv))


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)
    except (ValueError, OSError, KeyError, TypeError, subprocess.SubprocessError) as error:
        sys.exit(f"faucet: {error}")
