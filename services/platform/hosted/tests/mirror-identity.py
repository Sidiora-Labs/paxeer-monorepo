#!/usr/bin/env python3
"""Derives the base58 Ed25519 public key of a 64-byte Solana keypair file.

The bring-up binds the rendered mirror configuration and the Solana deployment
record to this identity, so the publisher can never be pointed at a program
namespace reserved for another payer.
"""

import json
import pathlib
import sys
import subprocess

import beta_tool

BASE58_ALPHABET = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"


def refuse(message):
    raise SystemExit("mirror-identity: %s" % message)


def base58_encode(raw):
    number = int.from_bytes(raw, "big")
    text = ""
    while number:
        number, remainder = divmod(number, 58)
        text = BASE58_ALPHABET[remainder] + text
    return "1" * (len(raw) - len(raw.lstrip(b"\x00"))) + text


def solana_public_key(path):
    try:
        values = json.loads(pathlib.Path(path).read_bytes())
    except (OSError, ValueError):
        refuse("the keypair file is not a readable Solana keypair array")
    if (
        not isinstance(values, list)
        or len(values) != 64
        or any(type(value) is not int or not 0 <= value <= 255 for value in values)
    ):
        refuse("the keypair file is not 64 keypair bytes")
    try:
        # The shared producer derives the public key and compares the complete
        # seed/public pair before returning the deployment identity.
        return json.loads(beta_tool.run(["mirror-public", path]))
    except (OSError, ValueError, subprocess.CalledProcessError):
        refuse("the keypair public half does not match its signing seed")


def main(argv):
    if len(argv) != 2 or argv[0] != "solana-public-key":
        refuse("usage: mirror-identity.py solana-public-key <keypair-file>")
    print(solana_public_key(argv[1]))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
