#!/usr/bin/env python3
"""Report retirement of custody governance artifact production and execution."""
import argparse
from pathlib import Path
import sys


def parser():
    command = argparse.ArgumentParser(description=__doc__, allow_abbrev=False)
    command.add_argument('--produce', type=Path, help='retired artifact production option')
    command.add_argument('--manifest', type=Path, help='retired artifact execution option')
    return command


def main():
    command = parser()
    command.parse_args()
    command.error('custody governance artifact production and execution have been retired')


if __name__ == '__main__':
    sys.exit(main())
