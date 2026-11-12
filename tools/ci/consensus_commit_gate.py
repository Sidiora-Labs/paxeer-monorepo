#!/usr/bin/env python3
"""Refuse consensus-changing commits missing from the release ledger at head."""

import argparse
from pathlib import Path
import re
import subprocess
import sys


CONSENSUS_PATHS = (
    "precompiles/perps/",
    "precompiles/bank/",
    "sdk/x/bank/keeper/",
    "sdk/x/bank/types/",
    "node/app.go",
    "node/sequencer/",
    "programs/crates/layerx-programs-market/src/",
)
CONSENSUS_MODULE_PATH = re.compile(r"^modules/[^/]+/(?:keeper|types)/")
DEFAULT_LIST = "docs/release/consensus-commits.md"
HISTORY_ARGS = (
    "log",
    "--format=%H",
    "--raw",
    "--no-abbrev",
    "--root",
    "--no-renames",
    "-c",
    "-z",
)


def _table_commits(document):
    commits = set()
    for line in document.splitlines():
        cells = line.strip().split("|")
        if len(cells) < 6 or cells[0]:
            continue
        cell = cells[1].strip()
        if cell.startswith("`") and cell.endswith("`"):
            cell = cell[1:-1]
        if re.fullmatch(r"[0-9a-f]{40}", cell):
            commits.add(cell)
    return commits


def listed_commits(path):
    return _table_commits(Path(path).read_text(encoding="utf-8"))


def _git(repo, *args):
    return subprocess.run(
        ["git", "-C", str(repo), *args],
        check=True,
        capture_output=True,
        text=True,
    ).stdout


def consensus_changes_from_history(history):
    changes = {}
    sha = None
    fields = iter(history.split("\0"))
    for field in fields:
        header = field.lstrip("\n")
        if not header:
            continue
        if re.fullmatch(r"[0-9a-f]{40}", header):
            sha = header
            changes[sha] = []
            continue
        if sha is None or not header.startswith(":"):
            raise ValueError("Invalid raw consensus history header")
        path = next(fields, None)
        if not path:
            raise ValueError("Missing raw consensus history path")
        if not path.endswith("_test.go") and (
            CONSENSUS_MODULE_PATH.match(path)
            or any(
                path.startswith(prefix) if prefix.endswith("/") else path == prefix
                for prefix in CONSENSUS_PATHS
            )
        ):
            changes[sha].append(path)
    return changes


def consensus_paths_touched(repo, sha):
    history = _git(repo, *HISTORY_ARGS, "--max-count=1", sha, "--")
    changes = consensus_changes_from_history(history)
    if len(changes) != 1:
        raise ValueError("Expected exactly one consensus history revision")
    return next(iter(changes.values()))


def unlisted_from_history(document, history):
    listed = _table_commits(document)
    return {
        sha: paths
        for sha, paths in consensus_changes_from_history(history).items()
        if paths and sha not in listed
    }


def unlisted_consensus_commits(repo, base, head, list_path):
    repo = Path(_git(repo, "rev-parse", "--show-toplevel").strip())
    path = Path(list_path)
    if path.is_absolute():
        path = path.relative_to(repo)
    head_sha = _git(repo, "rev-parse", "--verify", f"{head}^{{commit}}").strip()
    base_sha = _git(repo, "rev-parse", "--verify", f"{base}^{{commit}}").strip()
    document = _git(repo, "show", f"{head_sha}:{path.as_posix()}")
    history = _git(repo, *HISTORY_ARGS, f"{base_sha}..{head_sha}", "--")
    return unlisted_from_history(document, history)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("base", help="exclusive base revision")
    parser.add_argument("head", help="inclusive head revision")
    parser.add_argument("--list", default=DEFAULT_LIST, help="ledger path in head")
    args = parser.parse_args(argv)
    try:
        unlisted = unlisted_consensus_commits(
            Path.cwd(), args.base, args.head, args.list
        )
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        detail = (
            error.stderr
            if isinstance(error, subprocess.CalledProcessError)
            else str(error)
        )
        print(f"Consensus commit gate failed: {detail.strip()}", file=sys.stderr)
        return 2
    for sha, paths in unlisted.items():
        print(f"Unlisted consensus commit {sha}:")
        for path in paths:
            print(f"  {path}")
    return 1 if unlisted else 0


if __name__ == "__main__":
    sys.exit(main())
