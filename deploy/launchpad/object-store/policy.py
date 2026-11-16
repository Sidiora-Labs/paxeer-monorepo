#!/usr/bin/env python3
"""Render the least-privilege object policy for one launchpad account."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any


ACCOUNTS = Path(__file__).with_name("accounts.json")
BUCKET_NAME = re.compile(r"^[a-z0-9-]+$")
BUCKET_ACTIONS = {"s3:GetBucketAcl", "s3:GetBucketPolicyStatus", "s3:ListBucket"}


class PolicyError(ValueError):
    """An account or bucket argument cannot produce a policy."""


def _accounts() -> dict[str, Any]:
    try:
        document = json.loads(ACCOUNTS.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise PolicyError("cannot read object account document") from exc
    accounts = document.get("accounts")
    if not isinstance(accounts, dict):
        raise PolicyError("object account document has no accounts")
    return accounts


def _validate_buckets(service: str, entry: dict[str, Any], supplied: list[str]) -> list[tuple[dict[str, Any], str]]:
    expected = entry.get("buckets")
    if not isinstance(expected, list):
        raise PolicyError(f"{service}: invalid bucket table")
    if len(supplied) != len(expected):
        raise PolicyError(f"{service}: expected {len(expected)} bucket names, got {len(supplied)}")
    pairs: list[tuple[dict[str, Any], str]] = []
    for definition, bucket in zip(expected, supplied):
        if not isinstance(bucket, str) or BUCKET_NAME.fullmatch(bucket) is None:
            raise PolicyError(f"{service}: invalid bucket name {bucket!r}")
        pairs.append((definition, bucket))
    return pairs


def render_policy(service: str, supplied: list[str]) -> dict[str, Any]:
    accounts = _accounts()
    if service not in accounts:
        raise PolicyError(f"unknown service {service}")
    entry = accounts[service]
    if not isinstance(entry, dict):
        raise PolicyError(f"{service}: invalid account")

    statements: list[dict[str, Any]] = []
    for definition, bucket in _validate_buckets(service, entry, supplied):
        actions = definition.get("actions", [])
        prefixes = definition.get("prefixes", [])
        if not isinstance(actions, list) or not isinstance(prefixes, list):
            raise PolicyError(f"{service}: invalid bucket definition")
        object_actions = sorted(set(actions) - BUCKET_ACTIONS)
        if object_actions and prefixes:
            resources = [
                f"arn:aws:s3:::{bucket}/{prefix}*" if prefix else f"arn:aws:s3:::{bucket}/*"
                for prefix in prefixes
            ]
            statements.append({"Effect": "Allow", "Action": object_actions, "Resource": resources})
        for action in sorted(set(actions) & BUCKET_ACTIONS):
            statement: dict[str, Any] = {
                "Effect": "Allow",
                "Action": [action],
                "Resource": [f"arn:aws:s3:::{bucket}"],
            }
            if action == "s3:ListBucket" and prefixes:
                statement["Condition"] = {"StringLike": {"s3:prefix": prefixes}}
            statements.append(statement)

    return {"Version": "2012-10-17", "Statement": statements}


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("service")
    parser.add_argument("buckets", nargs="*")
    args = parser.parse_args(argv)
    try:
        policy = render_policy(args.service, args.buckets)
    except PolicyError as exc:
        print(f"policy: {exc}", file=sys.stderr)
        return 2
    print(json.dumps(policy, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
