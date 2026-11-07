#!/usr/bin/env python3
"""Refuse the replay qualification modes retired with the batch-WAL daemon."""

import argparse
import sys


RETIRED = "batch-WAL replay authority qualification has been retired"


def parser():
    command = argparse.ArgumentParser(
        description=__doc__,
        epilog=(
            "Compatibility options only: all qualification modes refuse. "
            "LAYERX_REPLAY_AUTHORITY_EVIDENCE is ignored; no evidence or fixtures "
            "are read or produced."
        ),
        allow_abbrev=False,
    )
    modes = command.add_mutually_exclusive_group()
    modes.add_argument("--build", action="store_true", help="retired; always refuses")
    modes.add_argument(
        "--prepare-fixture", metavar="PATH", help="retired; always refuses"
    )
    return command


def main():
    command = parser()
    command.parse_args()
    command.error(RETIRED)


if __name__ == "__main__":
    sys.exit(main())
