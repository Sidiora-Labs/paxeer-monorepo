#!/usr/bin/env python3
"""Render a manifest-bound BRIDGE_SIGNER_KEYS line for fly secrets import."""

import argparse
import json
from pathlib import Path
import sys
import tomllib


HERE = Path(__file__).resolve().parent
INVENTORY_PATH = "/etc/layerx/bridge-signer/keys.json"
SIGNER_DIRECTORY = "/run/secrets/bridge/signer"


def load_deployment(instance):
    if type(instance) is not int or instance not in range(1, 6):
        raise ValueError("instance must be an integer from 1 to 5")
    return tomllib.loads((HERE / f"relayer-{instance}.toml").read_text())


def render_inventory(instance, deployment=None):
    """Read public policy only; never open the key paths named by the policy."""
    if type(instance) is not int or instance not in range(1, 6):
        raise ValueError("instance must be an integer from 1 to 5")
    if deployment is None:
        deployment = load_deployment(instance)
    if deployment.get("app") != f"paxeer-bridge-relayer-{instance}":
        raise ValueError("deployment app does not match instance")
    expected_key = f"{SIGNER_DIRECTORY}/attestor-{instance}.key"
    environment = deployment.get("env", {})
    for name, expected in {
        "LAYERX_BRIDGE_ATTESTOR_INDEX": str(instance),
        "LAYERX_BRIDGE_ATTESTOR_SET_SIZE": "5",
        "LAYERX_BRIDGE_ATTESTOR_THRESHOLD": "3",
        "LAYERX_BRIDGE_ATTESTOR_KEY_FILE": expected_key,
    }.items():
        if environment.get(name) != expected:
            raise ValueError(f"deployment {name} does not match instance policy")

    files = deployment.get("files", [])
    mounts = {item["secret_name"]: item["guest_path"] for item in files}
    if len(mounts) != len(files) or len(set(mounts.values())) != len(files):
        raise ValueError("deployment has duplicate secret names or guest paths")
    if mounts.get("BRIDGE_SIGNER_KEYS") != INVENTORY_PATH:
        raise ValueError("BRIDGE_SIGNER_KEYS must mount at the signer inventory path")
    if mounts.get("BRIDGE_SIGNER_ATTESTOR_KEY") != expected_key:
        raise ValueError("attestor secret mount does not match instance")

    inventory = json.loads((HERE / "signer-keys.json").read_text())
    if set(inventory) != {"keys"} or not isinstance(inventory["keys"], list):
        raise ValueError("signer inventory must contain keys only")
    keys = inventory["keys"]
    if len(keys) != 11:
        raise ValueError("signer inventory must contain all eleven bridge policies")
    if any(set(key) != {"handle", "algorithm", "key_file", "domains"} for key in keys):
        raise ValueError("signer policies must contain exactly the four signer fields")
    if len({key["handle"] for key in keys}) != len(keys):
        raise ValueError("signer inventory has duplicate handles")
    attestors = [key for key in keys if key["handle"] == "bridge/attestor"]
    if len(attestors) != 1:
        raise ValueError("signer inventory must have exactly one bridge/attestor")
    attestors[0]["key_file"] = expected_key
    signer_mounts = {
        path for name, path in mounts.items()
        if name != "BRIDGE_SIGNER_KEYS" and path.startswith(SIGNER_DIRECTORY + "/")
    }
    paths = [key["key_file"] for key in keys]
    if len(set(paths)) != len(paths) or set(paths) != signer_mounts:
        raise ValueError("signer inventory paths must match the mounted signer keys exactly")
    return inventory


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--instance", type=int, choices=range(1, 6), required=True)
    args = parser.parse_args(argv)
    try:
        inventory = render_inventory(args.instance)
    except (ValueError, KeyError, TypeError, OSError) as error:
        print(f"render-signer-keys: {error}", file=sys.stderr)
        return 1
    print("BRIDGE_SIGNER_KEYS=" + json.dumps(inventory, separators=(",", ":")))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
