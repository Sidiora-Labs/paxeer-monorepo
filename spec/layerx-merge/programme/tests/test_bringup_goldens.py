import re
import shlex
from pathlib import Path

from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
GOLDEN_PATH = Path(__file__).resolve().parents[1] / "data/goldens/merge-bringup.kvx"
REFUSALS = {
    "no-leaders": "carries no leader list",
    "lastseq": "want 0",
    "no-key": "pins no receipt key for first sequenced height",
    "bad-genesis": "sequencer genesis",
}
CASE_TESTS = {
    "accepted": "TestMergePlanRequiresKeyForFirstSequencedHeight",
    "no-leaders": "TestUpgradesSequencerGenesisAtActivation",
    "lastseq": "TestUpgradesSequencerGenesisAtActivation",
    "no-key": "TestMergePlanRefusesKeyCoveringOnlyActivationHeight",
    "bad-genesis": "TestUpgradesSequencerGenesisAtActivation",
}


def test_golden_cases_cover_every_refusal() -> None:
    goldens = load(GOLDEN_PATH)
    assert set(goldens) == {f"golden.{case}" for case in CASE_TESTS}
    domain_tests = (ROOT / "node/upgrades_sequencer_test.go").read_text(
        encoding="utf-8"
    )
    for case, test in CASE_TESTS.items():
        row = goldens[f"golden.{case}"]
        assert row["test"] == test, case
        assert shlex.split(row["command"]) == [
            "go",
            "test",
            "./node",
            "-run",
            f"^{test}$",
            "-count=1",
        ], case
        assert re.search(
            rf"^func {re.escape(test)}\(t \*testing\.T\) \{{",
            domain_tests,
            re.MULTILINE,
        ), case
        assert type(row["expect_exit"]) is int, case
        assert row["expect_exit"] == 0, case

    accepted = goldens["golden.accepted"]
    assert "expect_text" not in accepted
    assert accepted["activation_height"] == 42
    assert accepted["first_sequenced_height"] == accepted["activation_height"] + 1
    assert accepted["leaders"] == ["51" * 32, "52" * 32]
    assert accepted["last_seq"] == 0
    assert accepted["last_seq_hash"] == "00" * 32
    first_height = accepted["first_sequenced_height"]
    assert (
        accepted["receipt_key_first_height"]
        <= first_height
        <= accepted["receipt_key_last_height"]
    )
    assert accepted["receipt_key_first_height"] > accepted["activation_height"]
    for case, text in REFUSALS.items():
        assert goldens[f"golden.{case}"]["expect_text"] == text, case


def test_refusal_texts_match_handler() -> None:
    goldens = load(GOLDEN_PATH)
    source = (ROOT / "node/upgrades.go").read_text(encoding="utf-8")
    handlers = {}
    for name in (
        "mergeUpgradePlan",
        "continueMergeUpgradePlan",
        "migrateMergeState",
    ):
        match = re.search(
            rf"^func \(app \*App\) {name}\([^\n]* \{{\n.*?^\}}",
            source,
            re.MULTILINE | re.DOTALL,
        )
        assert match is not None, name
        handlers[name] = match[0]
    assert (
        "return app.continueMergeUpgradePlan(ctx, plan, fromVM)"
        in handlers["mergeUpgradePlan"]
    )
    assert (
        "versions, err := app.migrateMergeState(cached, plan, fromVM)"
        in handlers["continueMergeUpgradePlan"]
    )
    handler = "\n".join(handlers.values())
    assert "func (app *App) mergeUpgradePlan(" in handler
    messages = re.findall(r'(?:fmt\.Errorf|\.Wrapf)\("([^"\n]*)"', handler)
    assert len(messages) == 8
    for case, text in REFUSALS.items():
        expect_text = goldens[f"golden.{case}"]["expect_text"]
        assert expect_text == text, case
        assert expect_text in handler, case
        assert any(expect_text in message for message in messages), case
