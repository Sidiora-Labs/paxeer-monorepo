"""Committed file-backed peers for the backup recovery contract test.

These fixture stores are not paxd/kernel databases. Their offline validator checks
matching committed generations, private-key presence and payload integrity.
"""

import hashlib
import json
from pathlib import Path
import sys


def seed(root, generation):
    root = Path(root)
    if (root / ".held").exists():
        raise ValueError("fixture writers are quiesced")
    node, kernel = root / "validator", root / "kernel"
    (node / "config").mkdir(parents=True, exist_ok=True)
    (node / "data").mkdir(exist_ok=True)
    (kernel / "volume").mkdir(parents=True, exist_ok=True)
    for name in ("priv_validator_key", "node_key"):
        key = node / "config" / (name + ".json")
        key.write_text(json.dumps({"priv_key": {"value": name + "-fixture-private"}}))
        key.chmod(0o600)
    payload = f"committed fixture generation {generation}\n".encode() * 1024
    (kernel / "volume" / "state.bin").write_bytes(payload)
    state = {"height": generation, "root": hashlib.sha256(payload).hexdigest()}
    (node / "data" / "committed.json").write_text(json.dumps(state))
    (kernel / "committed.json").write_text(json.dumps(state))
    (node / "data" / "priv_validator_state.json").write_text(
        json.dumps({"height": str(generation), "round": 0, "step": 0})
    )
    (root / "dump.sql").write_text(f"INSERT INTO wallets VALUES ({generation});\n")


def validate(node, kernel):
    node, kernel = Path(node), Path(kernel)
    state = json.loads((node / "data" / "committed.json").read_text())
    if state != json.loads((kernel / "committed.json").read_text()):
        raise ValueError("recovered node/kernel generation mismatch")
    if state["root"] != hashlib.sha256((kernel / "volume" / "state.bin").read_bytes()).hexdigest():
        raise ValueError("recovered payload root mismatch")
    signing = json.loads((node / "data" / "priv_validator_state.json").read_text())
    if signing["height"] != str(state["height"]):
        raise ValueError("recovered signing state mismatch")
    for name in ("priv_validator_key", "node_key"):
        key = json.loads((node / "config" / (name + ".json")).read_text())
        if not key["priv_key"]["value"]:
            raise ValueError("recovered private key missing")
    return state


def churn(path, ready, stop):
    """An intentionally unquiesced source writer, never a replacement capture tool."""
    with Path(path).open("r+b", buffering=0) as stream:
        generation = 0
        while not Path(stop).exists():
            generation += 1
            stream.seek(0)
            stream.write(generation.to_bytes(8, "little"))
            if generation == 1:
                Path(ready).touch()


def main():
    command, *args = sys.argv[1:]
    if command == "seed":
        seed(args[0], int(args[1]))
    elif command == "validate":
        validate(*args)
    elif command == "churn":
        churn(*args)
    else:
        raise ValueError(f"unknown fixture command: {command}")


if __name__ == "__main__":
    main()
