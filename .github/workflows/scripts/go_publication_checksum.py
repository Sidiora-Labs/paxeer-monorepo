"""Validate and retain the installed module's exact checksum-database record."""

import base64
import binascii
import json
from pathlib import Path
import sys


def publication_checksum(module, version, sums, download):
    selected = [
        line for line in sums.splitlines()
        if line.split()[:2] == [module, version]
    ]
    if len(selected) != 1:
        raise ValueError("expected exactly one module/version checksum")
    fields = selected[0].split()
    if len(fields) != 3 or not fields[2].startswith("h1:"):
        raise ValueError("expected an h1 checksum")
    try:
        digest = base64.b64decode(fields[2][3:], validate=True)
    except (ValueError, binascii.Error) as error:
        raise ValueError("invalid h1 digest") from error
    if len(digest) != 32 or base64.b64encode(digest).decode() != fields[2][3:]:
        raise ValueError("invalid h1 digest")
    if (download.get("Path"), download.get("Version"), download.get("Sum")) != (
        module, version, fields[2]
    ) or download.get("Error"):
        raise ValueError("download checksum or module identity differs")
    return selected[0]


def retain_checksum(module, version, sums_path, download_path, evidence_path):
    checksum = publication_checksum(
        module, version, Path(sums_path).read_text(encoding="utf-8"),
        json.loads(Path(download_path).read_text(encoding="utf-8")),
    )
    with Path(evidence_path).open("a", encoding="utf-8") as evidence:
        evidence.write(checksum + "\n")
    return checksum


def main():
    print(retain_checksum(*sys.argv[1:]))


if __name__ == "__main__":
    main()
