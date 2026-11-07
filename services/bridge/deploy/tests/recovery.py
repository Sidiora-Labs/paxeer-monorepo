"""Disposable node configuration and immutable signed-release inputs for dry runs."""

import json
import os
from pathlib import Path
import re
import sys
import tomllib


def set_toml(path, section, key, value):
    """Change one existing key without rewriting the node's other settings."""
    path = Path(path)
    text = path.read_text()
    parsed = tomllib.loads(text)
    table = parsed
    for part in section.split(".") if section else []:
        table = table[part]
    if key not in table:
        raise ValueError(f"{path}: missing {section}.{key}")
    current = ""
    count = 0
    lines = []
    for line in text.splitlines(keepends=True):
        heading = re.fullmatch(r"\s*\[([^\]]+)\]\s*(?:#.*)?\n?", line)
        if heading:
            current = heading[1]
        if current == section and re.match(rf"^\s*{re.escape(key)}\s*=", line):
            line = f"{key} = {json.dumps(value)}\n"
            count += 1
        lines.append(line)
    if count != 1:
        raise ValueError(f"{path}: expected exactly one {section}.{key}")
    result = "".join(lines)
    tomllib.loads(result)
    path.write_text(result)


def configure(home, rpc_port, p2p_port, evm_port):
    home = Path(home)
    genesis_path = home / "config/genesis.json"
    genesis = json.loads(genesis_path.read_text())
    state = genesis["app_state"]
    bridge = state["layerxbridge"]
    for key in ("chains", "assets", "caps", "nullifiers", "in_flight", "outbound_nonces"):
        if bridge.get(key):
            raise ValueError(f"dry run requires dormant bridge genesis: {key}")
    denom = state["staking"]["params"]["bond_denom"]
    state["staking"]["params"]["max_voting_power_ratio"] = "1.000000000000000000"
    state["gov"]["deposit_params"]["min_deposit"] = [{"denom": denom, "amount": "1"}]
    state["gov"]["voting_params"]["voting_period"] = "4s"
    state["gov"]["voting_params"]["expedited_voting_period"] = "2s"
    genesis_path.write_text(json.dumps(genesis) + "\n")
    config = home / "config/config.toml"
    for section, key, value in (
        ("rpc", "laddr", f"tcp://127.0.0.1:{rpc_port}"),
        ("p2p", "laddr", f"tcp://127.0.0.1:{p2p_port}"),
        ("p2p", "bootstrap-peers", ""),
        ("p2p", "persistent-peers", ""),
        ("p2p", "pex", False),
    ):
        set_toml(config, section, key, value)
    app = home / "config/app.toml"
    for section, key, value in (
        ("", "minimum-gas-prices", f"0{denom}"),
        ("api", "enable", False),
        ("grpc", "enable", False),
        ("grpc-web", "enable", False),
        ("evm", "http_enabled", True),
        ("evm", "http_address", "127.0.0.1"),
        ("evm", "http_port", int(evm_port)),
        ("evm", "ws_enabled", False),
    ):
        set_toml(app, section, key, value)
    return denom


class ReleaseJournal:
    """Reopen the exact attestor-signed input; never regenerate it on retry."""

    def __init__(self, directory):
        self.directory = Path(directory)
        self.directory.mkdir(mode=0o700, parents=True, exist_ok=True)

    def path(self, nonce):
        if not re.fullmatch(r"[1-9][0-9]*", str(nonce)):
            raise ValueError("release nonce must be a positive integer")
        return self.directory / f"{nonce}.json"

    def save(self, nonce, raw):
        json.loads(raw)
        with self.path(nonce).open("xb") as handle:
            os.chmod(handle.name, 0o600)
            handle.write(raw)
            handle.flush()
            os.fsync(handle.fileno())
        descriptor = os.open(self.directory, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(descriptor)
        finally:
            os.close(descriptor)

    def read(self, nonce):
        raw = self.path(nonce).read_bytes()
        json.loads(raw)
        return raw


def main(args):
    if args[0] == "configure":
        print(configure(args[1], *map(int, args[2:5])))
    elif args[0] == "save":
        ReleaseJournal(args[1]).save(args[2], Path(args[3]).read_bytes())
    elif args[0] == "read":
        sys.stdout.buffer.write(ReleaseJournal(args[1]).read(args[2]))
    else:
        raise ValueError(f"unknown recovery command: {args[0]}")


if __name__ == "__main__":
    main(sys.argv[1:])
