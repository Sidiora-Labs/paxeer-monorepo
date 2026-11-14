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
    assert re.fullmatch(r"v\d+\.\d+\.\d+", release["version"])
    prose = " ".join(readme.split())
    assert "[tools/release/version.json](../release/version.json)" in prose
    assert "the staged executable's `version --long --output json` output and the supplied build provenance." in prose
    assert "Their versions must agree with `HPX_PAXD_VERSION` when supplied." in prose
    assert "Provenance contains `version`, `source_revision` and `paxd_sha256`; its digest must match the exact staged executable." in prose
