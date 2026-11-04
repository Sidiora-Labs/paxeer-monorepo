"""The installed inspector owns the foundation wire schema for all consumers."""
import hashlib
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[3]


def qualify_foundation(manifest, directory, revision, consumer):
    paths = manifest.get("source_paths")
    if not isinstance(paths, list) or not paths or any(not isinstance(p, str) for p in paths):
        raise ValueError("foundation source closure is absent")
    closure = subprocess.run(
        ["git", "ls-tree", "-r", "--full-tree", "-z", revision, "--", *paths],
        cwd=ROOT, capture_output=True, check=True,
    ).stdout
    if not closure:
        raise ValueError("foundation source closure is empty")
    result = subprocess.run(
        [os.environ.get("BRINGUP_INSPECT_BIN", "bringup-inspect"),
         "native-foundation-provenance" if manifest.get("version") == 2 else "foundation-provenance", str(directory), revision,
         hashlib.sha256(closure).hexdigest(), consumer],
        input=json.dumps(manifest), capture_output=True, text=True, timeout=30,
        check=False,
    )
    if result.returncode:
        raise ValueError("foundation provenance refused: " + result.stderr.strip())
    if json.loads(result.stdout) != manifest:
        raise ValueError("foundation transport changed the produced manifest")
    return manifest
