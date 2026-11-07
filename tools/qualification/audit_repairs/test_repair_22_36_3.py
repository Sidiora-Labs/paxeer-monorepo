"""Offline checks for the wallet documentation move."""

from pathlib import Path
import re
import subprocess
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[3]
DOCS = ROOT / "docs/wallet"
EXPECTED_FILES = {
    "app/CHANGELOG.md",
    "app/README.md",
    "app/api/README.md",
    "app/architecture/README.md",
    "app/architecture/storage-forensics.md",
    "app/deployment/README.md",
    "app/development/README.md",
    "app/operations/runbooks.md",
    "service/cutover.md",
    "service/security-review.md",
    "service/runbooks/backup-restore.md",
    "service/runbooks/gateway-rollback.md",
    "service/runbooks/identity-provider-outage.md",
    "service/runbooks/node-replacement.md",
    "service/runbooks/refresh-cadence.md",
    "service/runbooks/rpc-pool-failure.md",
}
INLINE_LINK = re.compile(
    r"\]\(\s*(?:<([^>\n]+)>|((?:\\.|[^()\s]|\([^()\n]*\))+))"
    r"(?:\s+(?:\"[^\"]*\"|'[^']*'|\([^)]*\)))?\s*\)"
)
REFERENCE_LINK = re.compile(
    r"^ {0,3}\[[^]\n]+\]:\s*(?:<([^>\n]+)>|(\S+))", re.MULTILINE
)


def markdown_prose(text):
    """Ignore fenced examples so shell and JSON syntax are not links."""
    lines = []
    fence = None
    for line in text.splitlines():
        marker = re.match(r"^ {0,3}(`{3,}|~{3,})", line)
        if fence is None and marker:
            fence = marker[1]
        elif fence is not None:
            closing = r" {0,3}" + re.escape(fence[0]) + "{" + str(len(fence)) + r",}\s*"
            if re.fullmatch(closing, line):
                fence = None
        else:
            lines.append(line)
    return "\n".join(lines)


def heading_ids(text):
    ids = set()
    counts = {}
    headings = re.finditer(
        r"^ {0,3}#{1,6}\s+(.+?)(?:\s+#+)?\s*$", text, re.MULTILINE
    )
    for match in headings:
        heading = re.sub(r"<[^>]*>", "", match[1]).lower()
        slug = re.sub(r"[^\w\- ]", "", heading).replace(" ", "-")
        count = counts.get(slug, 0)
        counts[slug] = count + 1
        ids.add(f"{slug}-{count}" if count else slug)
    ids.update(re.findall(r"\b(?:id|name)=[\"']([^\"']+)[\"']", text))
    return ids


def test_original_wallet_documentation_locations_removed():
    for relative in (
        "apps/wallet/docs", "apps/wallet/CHANGELOG.md", "services/wallet/docs"
    ):
        path = ROOT / relative
        assert not path.exists() and not path.is_symlink(), relative


def test_all_sixteen_wallet_documents_moved():
    actual = {
        p.relative_to(DOCS).as_posix() for p in DOCS.rglob("*") if p.is_file()
    }
    assert actual == EXPECTED_FILES
    for relative in sorted(EXPECTED_FILES):
        path = DOCS / relative
        assert not path.is_symlink(), relative
        assert path.read_text(encoding="utf-8").startswith("# "), relative


def assert_relative_markdown_links_resolve(paths, tracked_files=None):
    checked = 0
    failures = []
    for path in paths:
        prose = markdown_prose(path.read_text(encoding="utf-8"))
        for pattern in (INLINE_LINK, REFERENCE_LINK):
            for match in pattern.finditer(prose):
                destination = match[1] or match[2]
                url = urlsplit(destination)
                if url.scheme or url.netloc:
                    continue
                checked += 1
                decoded = unquote(
                    re.sub(
                        r"\\([!\"#$%&'()*+,\-./:;<=>?@\[\]^_`{|}~])",
                        r"\1",
                        url.path,
                    )
                )
                if decoded.startswith("/"):
                    target = ROOT / decoded.lstrip("/")
                else:
                    target = path.parent / decoded if decoded else path
                if not target.exists():
                    failures.append(f"{path.relative_to(ROOT)}: missing {destination}")
                    continue
                if tracked_files is not None:
                    assert target.is_file(), f"{path.relative_to(ROOT)}: not a file {destination}"
                    assert target.resolve().relative_to(ROOT).as_posix() in tracked_files, (
                        f"{path.relative_to(ROOT)}: untracked {destination}"
                    )
                if url.fragment and target.suffix.lower() == ".md":
                    target_prose = markdown_prose(target.read_text(encoding="utf-8"))
                    if unquote(url.fragment) not in heading_ids(target_prose):
                        failures.append(f"{path.relative_to(ROOT)}: missing anchor {destination}")
    assert checked > 0, "The documentation link check must inspect real relative links"
    assert not failures, "\n".join(failures)


def test_every_relative_wallet_markdown_link_resolves():
    assert_relative_markdown_links_resolve(sorted(DOCS.rglob("*.md")))


def test_every_relative_wallet_readme_link_resolves_to_tracked_file():
    result = subprocess.run(
        ["git", "ls-files", "-z"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    )
    tracked_files = set(result.stdout.split("\0")) - {""}
    for relative in ("apps/wallet/README.md", "services/wallet/README.md"):
        assert_relative_markdown_links_resolve([ROOT / relative], tracked_files)


def test_no_tracked_references_to_retired_wallet_documentation_paths():
    result = subprocess.run(
        [
            "git", "grep", "-n", "-I", "-F",
            "-e", "apps/wallet/docs", "-e", "services/wallet/docs",
            "--", ".", ":(exclude)spec/**", ":(exclude)GOTCHA.kvx",
            ":(exclude)**/repair*.kvx", ":(exclude)**/CHANGELOG*",
            ":(exclude)tools/qualification/audit_repairs/test_repair_22_36_3.py",
        ],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1, result.stdout + result.stderr
