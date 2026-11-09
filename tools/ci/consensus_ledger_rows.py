#!/usr/bin/env python3
"""Print missing consensus ledger rows, oldest first in Git topological order."""

import argparse
from pathlib import Path
import subprocess
import sys

from consensus_commit_gate import DEFAULT_LIST, _git, unlisted_consensus_commits


def escape_cell(value):
    return (
        value.replace("|", "&#124;")
        .replace("\r\n", "\n")
        .replace("\r", "\n")
        .replace("\n", "<br>")
    )


def ledger_rows(repo, base, head, list_path=DEFAULT_LIST):
    base = _git(repo, "rev-parse", "--verify", f"{base}^{{commit}}").strip()
    head = _git(repo, "rev-parse", "--verify", f"{head}^{{commit}}").strip()
    unlisted = unlisted_consensus_commits(repo, base, head, list_path)
    history = _git(
        repo,
        "log",
        "--reverse",
        "--topo-order",
        "--format=%H",
        f"{base}..{head}",
        "--",
    )
    rows = []
    for sha in history.splitlines():
        if sha not in unlisted:
            continue
        subject = _git(repo, "show", "-s", "--format=%s", sha, "--").rstrip("\n")
        paths = ", ".join(f"`{escape_cell(path)}`" for path in unlisted[sha])
        rows.append(
            f"| `{sha}` | {escape_cell(subject)} | {paths} | `v7.0` (`MergeUpgrade`) |"
        )
    return rows


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("base", help="exclusive base revision")
    parser.add_argument("head", help="inclusive head revision")
    parser.add_argument("--list", default=DEFAULT_LIST, help="ledger path in head")
    args = parser.parse_args(argv)
    try:
        rows = ledger_rows(Path.cwd(), args.base, args.head, args.list)
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        detail = (
            error.stderr
            if isinstance(error, subprocess.CalledProcessError)
            else str(error)
        )
        print(f"Consensus ledger rows failed: {detail.strip()}", file=sys.stderr)
        return 2
    for row in rows:
        print(row)
    return 0


if __name__ == "__main__":
    sys.exit(main())
