from pathlib import Path

from programme.parity import acceptance_coverage, uncovered

ROOT = Path(__file__).resolve().parents[4]


def test_installed_spec_has_no_uncovered_acceptance() -> None:
    spec_path = ROOT / "spec/layerx-merge/spec.kvx"
    missing, stale = acceptance_coverage(spec_path)
    diagnostics = (
        f"Uncovered acceptance selectors ({len(missing)}):\n"
        + "\n".join(missing)
        + f"\nStale acceptance selectors ({len(stale)}):\n"
        + "\n".join(stale)
    )
    assert missing == [], diagnostics
    assert stale == [], diagnostics
    assert uncovered(spec_path) == [], diagnostics
