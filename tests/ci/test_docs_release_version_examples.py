"""Keep documented build provenance aligned with the release identity."""

import json
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[2]


def test_hpx_provenance_example_matches_release_version():
    release = json.loads(
        (ROOT / "tools/release/version.json").read_text(encoding="utf-8")
    )
    readme = (ROOT / "tools/hpx/README.md").read_text(encoding="utf-8")
    examples = [
        json.loads(block)
        for block in re.findall(r"^```json\s*\n(.*?)^```\s*$", readme, re.M | re.S)
    ]
    provenance = [
        example
        for example in examples
        if isinstance(example, dict)
        and {"source_revision", "paxd_sha256"} <= example.keys()
    ]

    assert len(provenance) == 1, "Expected one hpx build provenance JSON example"
    assert provenance[0]["version"] == release["version"], (
        "The hpx provenance example must use the version in tools/release/version.json"
    )
    assert "The `version` value is read from `tools/release/version.json`." in readme
