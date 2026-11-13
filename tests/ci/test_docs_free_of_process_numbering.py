"""Product guides describe behavior without numbered development history."""

from functools import lru_cache
from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
CONTRIBUTOR_PAGES = {
    "CONTRIBUTING.md",
}
PAGE_SUFFIXES = {".md", ".markdown", ".mdx", ".rst", ".txt", ".html", ".htm", ".adoc"}
VENDOR_COMPONENTS = {"vendor", "vendored", "third_party", "third-party", "node_modules"}
# Hyperfide's upstream documentation is mirrored alongside its source snapshot.
VENDOR_ROOTS = {"tools/hyperfide", "docs/hyperfide"}
TASK_NUMBER = re.compile(r"\btasks?[\s`*#:_]+\d+\.\d+\.\d+\b", re.IGNORECASE)
GOTCHA_OR_DECISION_NUMBER = re.compile(
    r"\b(?:gotchas?|decisions?)[\s`*#:.()_-]+\d+\b", re.IGNORECASE
)
REPAIR_RECORD_PROSE = re.compile(
    r"\b(?:repair[\s-]+records?|audit[\s-]+findings?)\b", re.IGNORECASE
)
INLINE_CODE = re.compile(r"(?P<ticks>`+)(?P<value>[^`\n]+)(?P=ticks)")
FILE_NAME = re.compile(r"(?:[\w.@+-]+/)*[\w.@+-]+\.[A-Za-z][\w.-]*(?::\d+)?")


@lru_cache(maxsize=1)
def _product_pages():
    tracked = subprocess.run(
        ["git", "ls-files", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.split("\0")
    assert any(tracked), "No tracked repository paths found"
    vendor_roots = VENDOR_ROOTS | {
        Path(name).parent.as_posix()
        for name in tracked
        if Path(name).name == "VENDOR.kvx"
    }
    pages = []
    for name in sorted(set(tracked) - {""} - CONTRIBUTOR_PAGES):
        path = Path(name)
        # Specifications are contributor engineering records, not product guides.
        if path.parts[0] == "spec":
            continue
        if VENDOR_COMPONENTS.intersection(path.parts[:-1]) or any(
            name.startswith(root + "/") for root in vendor_roots
        ):
            continue
        is_readme = path.stem.lower() == "readme" or path.name.lower().startswith(
            "readme."
        )
        is_docs_page = "docs" in path.parts[:-1] and path.suffix.lower() in PAGE_SUFFIXES
        if is_readme or is_docs_page:
            page = ROOT / path
            assert page.is_file(), f"Tracked documentation page is missing: {name}"
            content = page.read_text(encoding="utf-8")
            # Mask only filename spans, preserving offsets and all surrounding prose.
            # Backticks around a task/decision number do not exempt the reference.
            content = INLINE_CODE.sub(
                lambda match: (
                    " " * len(match.group())
                    if FILE_NAME.fullmatch(match.group("value"))
                    else match.group()
                ),
                content,
            )
            pages.append((name, content))
    assert pages, "No tracked README or documentation pages found"
    return pages


def _references(pattern):
    references = []
    for name, content in _product_pages():
        for match in pattern.finditer(content):
            line = content.count("\n", 0, match.start()) + 1
            references.append(f"{name}:{line}: {match.group()}")
    return references


def test_docs_have_no_task_numbers():
    references = _references(TASK_NUMBER)
    assert not references, "Task numbers in product documentation:\n" + "\n".join(references)


def test_docs_have_no_gotcha_or_decision_numbers():
    references = _references(GOTCHA_OR_DECISION_NUMBER)
    assert not references, "Gotcha or decision numbers in product documentation:\n" + "\n".join(
        references
    )


def test_docs_have_no_repair_record_prose():
    references = _references(REPAIR_RECORD_PROSE)
    assert not references, "Repair or audit history in product documentation:\n" + "\n".join(
        references
    )
