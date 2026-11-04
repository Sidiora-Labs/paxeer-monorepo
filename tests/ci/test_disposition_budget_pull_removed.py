from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[2]


def assert_no_queue_disposition(root):
    path = root / "tests/protocol/lxp_test_terminal_rejection.c"
    assert not path.exists() and not path.is_symlink(), (
        "Retired terminal rejection suite remains"
    )
    sources = sorted(
        source for source in (root / "tests/protocol").rglob("*")
        if source.is_file() and source.suffix in {".c", ".h"}
    )
    assert sources, "No protocol test sources found"
    retired = [
        f"{source.relative_to(root)}: {name}"
        for source in sources
        for name in re.findall(
            r"\b(?:two_class_disposition_case|lxp_queue_disposition\w*|"
            r"LXP_QUEUE_DISPOSITION_\w+)\b",
            source.read_text(),
        )
    ]
    assert not retired, f"Queue-disposition codec case remains: {retired}"


def test_no_queue_disposition_codec_case():
    assert_no_queue_disposition(ROOT)


def test_no_budget_pull_request_case():
    source = (ROOT / "tests/modules/test_budget_delegate.c").read_text()
    retired = re.findall(
        r"\b(?:lx_budget_pull_request|lx_budget_pull_execute)\b", source
    )
    assert not retired, f"Budget pull request cases remain: {retired}"
