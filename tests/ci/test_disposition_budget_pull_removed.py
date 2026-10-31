from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[2]


def test_no_queue_disposition_codec_case():
    source = (ROOT / "tests/protocol/lxp_test_terminal_rejection.c").read_text()
    retired = re.findall(
        r"\b(?:two_class_disposition_case|lxp_queue_disposition\w*|"
        r"LXP_QUEUE_DISPOSITION_\w+)\b",
        source,
    )
    assert not retired, f"Queue-disposition codec case remains: {retired}"


def test_no_budget_pull_request_case():
    source = (ROOT / "tests/modules/test_budget_delegate.c").read_text()
    retired = re.findall(
        r"\b(?:lx_budget_pull_request|lx_budget_pull_execute)\b", source
    )
    assert not retired, f"Budget pull request cases remain: {retired}"
