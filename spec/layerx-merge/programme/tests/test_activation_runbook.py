from pathlib import Path

from programme.activation import (
    GOLDEN_PATH,
    KNOWN_UPGRADE_PLANS,
    load_plans,
    render_runbook,
)
from programme.kvx import load

RUNBOOK_PATH = Path(__file__).resolve().parents[2] / "docs/activation-runbook.md"


def test_runbook_matches_render() -> None:
    plans = load_plans()
    goldens = load(GOLDEN_PATH)
    rendered = render_runbook(plans, goldens)

    assert RUNBOOK_PATH.read_bytes() == rendered.encode("utf-8")
    for section in ("Plans", "Bring-up checks", "Refusals"):
        assert f"\n## {section}\n" in rendered
    assert len([plan for plan in plans if plan["name"] in KNOWN_UPGRADE_PLANS]) == 4
    for plan in plans:
        assert plan["name"] in rendered
        assert plan["handler"] in rendered
        for store in [*plan["store_added"], *plan["store_deleted"]]:
            assert store in rendered
    assert len(goldens) == 5
    assert sum("expect_text" in fields for fields in goldens.values()) == 4
    for section, fields in goldens.items():
        assert f"\n### {section.removeprefix('golden.')}\n" in rendered
        assert fields["command"] in rendered
        assert fields["test"] in rendered
        if "expect_text" in fields:
            assert fields["expect_text"] in rendered
