"""Keep programme records and untracked implementation paths out of product docs."""

from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
DOCS = ROOT / "docs"
PROGRAMME_TASK_ID = re.compile(r"\btasks?\s+22\.\d+(?:\.\d+)*\b", re.IGNORECASE)
DECISION_ID = re.compile(r"\bdecisions?[\s.#-]+\d+\b", re.IGNORECASE)
INLINE_CODE = re.compile(r"(?<!`)`([^`\n]+)`(?!`)")
PATH_TOKEN = re.compile(r"[\w.@+/-]+(?::\d+)?")


def _product_pages():
    pages = []
    for page in sorted(DOCS.rglob("*")):
        if not page.is_file() or page.suffix.lower() not in {
            ".md",
            ".mdx",
            ".rst",
            ".html",
        }:
            continue
        relative = page.relative_to(DOCS)
        if relative.parts[0] == "release" and (
            relative.parts[1] == "7.0.0"
            or relative.parts[1] == "7.0.0.md"
            or relative.parts[1].startswith("7.0.0-")
        ):
            continue
        pages.append(page)
    assert pages, "No product documentation pages found"
    return pages


def _references(pattern):
    references = []
    for page in _product_pages():
        text = page.read_text(encoding="utf-8")
        for match in pattern.finditer(text):
            line = text.count("\n", 0, match.start()) + 1
            references.append(f"{page.relative_to(ROOT)}:{line}: {match.group()}")
    return references


def test_docs_pages_cite_no_programme_task_ids():
    references = _references(PROGRAMME_TASK_ID)
    assert not references, "Programme task IDs in product docs:\n" + "\n".join(references)


def test_docs_pages_cite_no_decision_ids():
    references = _references(DECISION_ID)
    assert not references, "Decision IDs in product docs:\n" + "\n".join(references)


def test_inference_market_guides_name_only_tracked_files():
    result = subprocess.run(
        ["git", "ls-files", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    )
    tracked_files = set(result.stdout.rstrip("\0").split("\0"))
    assert tracked_files, "No tracked repository files found"
    repository_roots = {path.split("/", 1)[0] for path in tracked_files}
    guides = sorted((DOCS / "agent/inference-market").glob("*.md"))
    assert guides, "No inference market guides found"
    untracked = []
    checked = 0
    for guide in guides:
        text = guide.read_text(encoding="utf-8")
        for match in INLINE_CODE.finditer(text):
            token = match.group(1)
            if PATH_TOKEN.fullmatch(token) is None:
                continue
            path = re.sub(r":\d+$", "", token)
            if path.startswith(("./", "../")):
                target = guide.parent / path
            elif path.split("/", 1)[0] in repository_roots:
                target = ROOT / path
            else:
                continue
            checked += 1
            resolved = target.resolve()
            if (
                not resolved.is_relative_to(ROOT)
                or resolved.relative_to(ROOT).as_posix() not in tracked_files
                or not resolved.is_file()
            ):
                line = text.count("\n", 0, match.start()) + 1
                untracked.append(f"{guide.relative_to(ROOT)}:{line}: {token}")
    assert checked, "No backticked repository paths checked in inference market guides"
    assert not untracked, "Untracked inference market paths:\n" + "\n".join(untracked)
