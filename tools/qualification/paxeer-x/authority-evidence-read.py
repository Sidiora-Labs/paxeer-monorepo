#!/usr/bin/env python3
import argparse
import sys


def main():
    parser = argparse.ArgumentParser(
        description="The authority evidence-read qualification producer is retired."
    )
    parser.parse_args()
    print(
        "authority-evidence-read: refusal: this qualification producer is retired",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main())
