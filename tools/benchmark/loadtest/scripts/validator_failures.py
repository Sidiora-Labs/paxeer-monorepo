"""Run bounded validator faults or recover a retained controller record."""

import argparse
import os

from operations_cli import run


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("-s", "--ssh-key")
    parser.add_argument("-f", "--failure", choices=["restart", "delegation-change", "slow-network"])
    parser.add_argument("-v", "--validators")
    parser.add_argument("-c", "--chain-id")
    parser.add_argument("--concurrent", type=int, default=2)
    parser.add_argument("--hold", default="10m")
    parser.add_argument("--recovery")
    parser.add_argument("--recover")
    args = parser.parse_args()
    arguments = ["--paxd", os.environ.get("PAXD_BIN", "paxd"),
                 "--concurrent", str(args.concurrent), "--hold", args.hold]
    for name in ["ssh_key", "failure", "validators", "chain_id", "recovery", "recover"]:
        value = getattr(args, name)
        if value is not None:
            arguments.extend(["--" + name.replace("_", "-"), value])
    run("validator-faults", arguments)


if __name__ == "__main__":
    main()
