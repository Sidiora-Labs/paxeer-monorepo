#!/usr/bin/env python3
"""Fail closed when an HPX artifact disagrees with the reviewed build identity."""
import hashlib
import json
from pathlib import Path
import re
import sys


RELEASE_IDENTITY_POLICY = {"schema": 1, "provenance_required": True}


def validate_release(manifest, binary_info, provenance, digest, revision, advertised):
    version = manifest["version"]
    if RELEASE_IDENTITY_POLICY != {"schema": 1, "provenance_required": True}:
        raise ValueError("unsupported HPX release identity policy")
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", version):
        raise ValueError("invalid release version")
    if not re.fullmatch(r"[0-9a-f]{40}", revision):
        raise ValueError("full source revision required")
    if not re.fullmatch(r"[0-9a-f]{64}", digest):
        raise ValueError("invalid artifact digest")
    if advertised != version or binary_info.get("version") != version or provenance.get("version") != version:
        raise ValueError("manifest, advertised, executable and provenance versions must agree")
    if binary_info.get("commit") != revision or provenance.get("source_revision") != revision:
        raise ValueError("executable and build provenance must match the checkout revision")
    if provenance.get("paxd_sha256") != digest:
        raise ValueError("artifact does not match build provenance digest")
    if binary_info.get("server_name") != "paxd":
        raise ValueError("unexpected executable identity")
    return version


def main():
    manifest, info, provenance, artifact, revision, advertised = sys.argv[1:]
    digest = hashlib.sha256(Path(artifact).read_bytes()).hexdigest()
    print(validate_release(json.loads(Path(manifest).read_text()), json.loads(Path(info).read_text()), json.loads(Path(provenance).read_text()), digest, revision, advertised))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError) as error:
        raise SystemExit(str(error))
