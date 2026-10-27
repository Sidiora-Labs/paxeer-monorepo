import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
DOCUMENTS = ("CODEMAP.md", "CONTRIBUTING.md", "docs/MONOREPO.md")


def test_sdk_js_documented_first_party() -> None:
    for name in DOCUMENTS:
        document = (ROOT / name).read_text(encoding="utf-8")
        descriptions = [
            line for line in document.splitlines() if "sdk/js" in line.lower()
        ]
        assert descriptions, f"{name}: missing sdk/js documentation"
        for description in descriptions:
            normalized = description.lower().replace("`", "")
            assert "first-party @pax-js/cosmos package" in normalized, (
                f"{name}: sdk/js must name the first-party @pax-js/cosmos package"
            )
            assert not re.search(r"\b(?:vendored|snapshots?)\b", normalized), (
                f"{name}: sdk/js must not be described as vendored or a snapshot"
            )
            assert "vendor.kvx" not in normalized, (
                f"{name}: sdk/js must not claim a vendor pin record"
            )
            assert "edited in place" in normalized, (
                f"{name}: sdk/js must permit in-place contributions"
            )
            assert "aligned to the protos of this repository" in normalized, (
                f"{name}: sdk/js must follow this repository's protos"
            )
