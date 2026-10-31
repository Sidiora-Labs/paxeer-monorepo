#!/usr/bin/env python3
"""Report retirement of custody governance artifact production and execution."""
import argparse
from pathlib import Path
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--produce', type=Path, help='retired artifact production option')
    parser.add_argument('--manifest', type=Path, help='retired artifact execution option')
    parser.parse_args()
    parser.error('custody governance artifact production and execution have been retired')


if __name__ == '__main__':
    sys.exit(main())
