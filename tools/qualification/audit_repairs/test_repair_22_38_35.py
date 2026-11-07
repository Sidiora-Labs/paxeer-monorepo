"""Check the real inference-market pages against their declared source contract."""

from pathlib import Path
import re
import runpy
import tomllib
from urllib.parse import unquote, urlsplit

import pytest


ROOT = Path(__file__).resolve().parents[3]
DOCS = ROOT / "docs/agent/inference-market"
PAGES = ("README.md", "protocol.md", "operator.md", "client.md", "threats.md")
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_38_35.kvx"
DEVELOPMENT_ENVELOPE_SENTENCE = (
    "Only the development attestation envelope is accepted today."
)
DOCS_CHECK = runpy.run_path(
    str(ROOT / "spec/layerx-merge/programme/docs_check.py")
)


@pytest.mark.parametrize("page", PAGES)
def test_inference_market_page_exists_with_content(page):
    path = DOCS / page
    assert path.is_file(), f"Missing inference-market page: {page}"
    content = path.read_text(encoding="utf-8")
    assert content.strip(), f"Empty inference-market page: {page}"
    assert re.search(r"^# \S", content, re.MULTILINE), page


def test_protocol_contains_every_f11_operation_and_selector():
    specification = (ROOT / "spec/layerx-merge/spec.kvx").read_text(
        encoding="utf-8"
    )
    task = re.search(
        r"^\[task\.22\.38\.1\]\n(.*?)(?=^\[|\Z)",
        specification,
        re.MULTILINE | re.DOTALL,
    )
    assert task is not None, "Task 22.38.1 must define the operation table"
    operation_step = re.search(r'^do_1 = "(.*)"$', task.group(1), re.MULTILINE)
    assert operation_step is not None
    operations = re.findall(
        r"\b([A-Z][A-Za-z]+) (0x0B[0-9A-F]{2})\b", operation_step.group(1)
    )
    assert len(operations) == 17, "Expected twelve F11 writes and five reads"
    assert len({name for name, _ in operations}) == 17
    protocol = (DOCS / "protocol.md").read_text(encoding="utf-8")
    for name, selector in operations:
        assert re.search(
            rf"^\| `{selector}` \| {re.escape(name)} \|",
            protocol,
            re.MULTILINE,
        ), f"Missing or incorrect F11 operation: {selector} {name}"


def test_threat_statement_states_development_envelope_and_limits():
    threats = (DOCS / "threats.md").read_text(encoding="utf-8")
    assert DEVELOPMENT_ENVELOPE_SENTENCE in threats
    assert "Canaries bound but do not eliminate wrong answers." in threats
    assert "## Liveness and settlement" in threats
    assert "## Collusion and economic limits" in threats


@pytest.mark.parametrize("page", PAGES)
def test_inference_market_pages_have_no_retired_references(page):
    path = DOCS / page
    content = path.read_text(encoding="utf-8")
    # Use the repository's actual patterns; even exempt mentions are unnecessary.
    assert DOCS_CHECK["RETIRED"], "The canonical retired-reference list is empty"
    for pattern, label in DOCS_CHECK["RETIRED"]:
        assert pattern.search(content) is None, f"{page}: retired {label}"
    assert DOCS_CHECK["check_page"](str(path)) == []


def test_repair_record_lists_the_five_pages():
    assert RECORD.is_file()
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    pages = record["page"]
    assert len(pages) == len(PAGES)
    paths = [entry["path"] for entry in pages.values()]
    expected = [(DOCS / page).relative_to(ROOT).as_posix() for page in PAGES]
    assert sorted(paths) == sorted(expected)
    for path in paths:
        assert (ROOT / path).is_file(), f"Record lists a missing page: {path}"


def test_compatibility_links_to_inference_market():
    content = (ROOT / "docs/agent/COMPATIBILITY.md").read_text(encoding="utf-8")
    assert re.search(r"\[[^\]]+\]\(inference-market/(?:README\.md)?\)", content)


@pytest.mark.parametrize("page", PAGES)
def test_inference_market_relative_links_resolve(page):
    source = DOCS / page
    for destination in re.findall(
        r"\[[^\]]+\]\(([^\s)]+)\)", source.read_text(encoding="utf-8")
    ):
        url = urlsplit(destination)
        if url.scheme or url.netloc or url.path.startswith("/"):
            continue
        target = source.parent / unquote(url.path) if url.path else source
        assert target.exists(), f"{page}: broken link {destination}"
        if url.fragment and target.suffix == ".md":
            headings = re.findall(
                r"^#{1,6}\s+(.+)$", target.read_text(encoding="utf-8"), re.MULTILINE
            )
            anchors = {
                re.sub(r"[^\w\- ]", "", heading.lower()).replace(" ", "-")
                for heading in headings
            }
            assert unquote(url.fragment) in anchors, destination
