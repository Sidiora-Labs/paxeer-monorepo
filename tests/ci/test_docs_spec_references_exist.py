"""Keep site specification citations tracked and storage scan prose task-free."""

from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
SITE = ROOT / "docs/site/docs"
SPEC_FILE = re.compile(
    r"\bspec/(?:[\w.-]+/)*[\w-][\w.-]*\.[A-Za-z][\w]*(?![\w/-])"
)
SPEC_CLAUSE = re.compile(
    r"\[(?:req|task)\.[^\]\n]+\]|\b(?:ac|do)_\d+\b", re.IGNORECASE
)


def test_site_pages_cite_only_tracked_spec_files():
    tracked = set(
        subprocess.run(
            ["git", "ls-files", "-z", "--", "spec/"],
            cwd=ROOT,
            check=True,
            capture_output=True,
            text=True,
        ).stdout.split("\0")
    )
    pages = sorted(SITE.rglob("*.md"))
    assert pages, "No site pages found"
    references = []
    missing = []
    for page in pages:
        for number, line in enumerate(page.read_text(encoding="utf-8").splitlines(), 1):
            for match in SPEC_FILE.finditer(line):
                path = match.group()
                reference = f"{page.relative_to(ROOT)}:{number}: {path}"
                references.append(reference)
                if path not in tracked or not (ROOT / path).is_file():
                    missing.append(reference)

    assert references, "No specification file citations found in site pages"
    assert not missing, "Untracked or missing specification files:\n" + "\n".join(missing)


def test_storage_scan_page_has_no_task_clauses():
    page = SITE / "programs/storage-scan.md"
    content = page.read_text(encoding="utf-8")
    assert content.strip(), "Storage scan page is empty"
    clauses = SPEC_CLAUSE.findall(content)
    assert not clauses, f"Specification clauses in storage scan prose: {clauses}"
