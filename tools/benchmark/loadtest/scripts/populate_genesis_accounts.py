"""Create an exact genesis inventory after privately persisting every seed."""

import argparse
import os
from pathlib import Path

from operations_cli import run


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("count", type=int)
    parser.add_argument("mode", nargs="?", choices=["loc"])
    parser.add_argument("--accounts", default=str(Path.home() / "test_accounts"))
    args = parser.parse_args()
    arguments = [
        "--count", str(args.count),
        "--home", os.environ.get("PAX_HOME", str(Path.home() / ".pax")),
        "--paxd", os.environ.get("PAXD_BIN", str(Path.home() / "go/bin/paxd")),
        "--accounts", args.accounts,
    ]
    if args.mode == "loc":
        arguments.append("--local")
    run("populate-genesis", arguments)


if __name__ == "__main__":
    main()
