#!/usr/bin/env python3
"""Check retained node admission, fee, committed execution and replay evidence.

Usage: paxeer_x_rejection_policy.py --evidence FILE --revision SOURCE_REVISION
The shared in-process checker also backs repair_m730_test.go. Evidence must
cover every retained policy case; missing or stale evidence is a refusal.
"""
import os
import sys

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))


def main():
    os.chdir(ROOT)
    command = ["go", "run", "./tests/protocol/cmd/protocol-policy", "rejection"]
    os.execvp(command[0], command + sys.argv[1:])


if __name__ == "__main__":
    main()
