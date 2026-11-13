"""Keep site and repaired wiki citations tracked and product prose task-free."""

from pathlib import Path
import re
import subprocess


ROOT = Path(__file__).resolve().parents[2]
SITE = ROOT / "docs/site/paxeer-docs/src/app"
WIKI = ROOT / "docs/wiki"
WIKI_PAGES = (
    "Asset.md",
    "Budget.md",
    "Escrow.md",
    "Fees.md",
    "FiatRamps.md",
    "Governance.md",
    "LNI.md",
    "MarketMakerRamps.md",
    "Migration.md",
    "Perps.md",
    "Service.md",
    "StorageScan.md",
    "Stream.md",
    "Ucp.md",
    "VisaTap.md",
)
SPEC_FILE = re.compile(
    r"\bspec/(?:[\w.-]+/)*[\w-][\w.-]*\.[A-Za-z][\w]*(?![\w/-])"
)
SPEC_CLAUSE = re.compile(
    r"\[(?:req|task)\.[^\]\n]+\]|\b(?:ac|do)_\d+\b", re.IGNORECASE
)
TASK_STATUS = re.compile(
    r"\b(?:task|requirement|acceptance)s?\s+\d+(?:\.\d+)*\b"
    r"|\bspec(?:ification)?\s+`?reality`?\b",
    re.IGNORECASE,
)
SOURCE_FILE = re.compile(r"`((?:[\w.-]+/)+[\w.-]+\.[A-Za-z]\w*)(?::[\d,-]+)?`")


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
    pages = sorted(SITE.rglob("page.tsx"))
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
    page = SITE / "storage-scan/page.tsx"
    content = page.read_text(encoding="utf-8")
    assert content.strip(), "Storage scan page is empty"
    clauses = SPEC_CLAUSE.findall(content)
    assert not clauses, f"Specification clauses in storage scan prose: {clauses}"


def test_wiki_pages_cite_only_tracked_spec_files():
    """Check specification citations throughout the wiki pages repaired here."""
    tracked = set(
        subprocess.run(
            ["git", "ls-files", "-z", "--", "spec/"],
            cwd=ROOT,
            check=True,
            capture_output=True,
            text=True,
        ).stdout.split("\0")
    )
    missing = []
    for name in WIKI_PAGES:
        page = WIKI / name
        content = page.read_text(encoding="utf-8")
        assert content.strip(), f"Wiki page is empty: {name}"
        for number, line in enumerate(content.splitlines(), 1):
            for match in SPEC_FILE.finditer(line):
                path = match.group()
                if path not in tracked or not (ROOT / path).is_file():
                    missing.append(f"{page.relative_to(ROOT)}:{number}: {path}")

    assert not missing, "Untracked or missing specification files:\n" + "\n".join(missing)


def test_wiki_pages_carry_no_task_status_lines():
    clauses = []
    for name in WIKI_PAGES:
        content = (WIKI / name).read_text(encoding="utf-8")
        assert content.strip(), f"Wiki page is empty: {name}"
        for pattern in (SPEC_CLAUSE, TASK_STATUS):
            for match in pattern.finditer(content):
                number = content.count("\n", 0, match.start()) + 1
                clauses.append(f"{name}:{number}: {match.group()}")

    assert not clauses, "Specification or task clauses in wiki prose:\n" + "\n".join(clauses)


def test_wiki_lni_sources_exist():
    content = (WIKI / "LNI.md").read_text(encoding="utf-8")
    sources = SOURCE_FILE.findall(content)
    assert sources, "No LNI source file citations found"
    tracked = set(
        subprocess.run(
            ["git", "ls-files", "-z"],
            cwd=ROOT,
            check=True,
            capture_output=True,
            text=True,
        ).stdout.split("\0")
    )
    missing = sorted(
        {path for path in sources if path not in tracked or not (ROOT / path).is_file()}
    )
    assert not missing, "Untracked or missing LNI source files:\n" + "\n".join(missing)
