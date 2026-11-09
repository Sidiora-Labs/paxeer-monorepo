"""Offline checks for the explorer documentation move."""

from pathlib import Path
import re
import shlex
import subprocess
from urllib.parse import unquote, urlsplit


ROOT = Path(__file__).resolve().parents[3]
DOCS = ROOT / "docs/explorer"
EXPECTED_FILES = {
    "BUILD-TIME_ENVS.md",
    "CONTRIBUTING.md",
    "CUSTOM_BUILD.md",
    "DEPRECATED_ENVS.md",
    "ENVS.md",
    "PULL_REQUEST_TEMPLATE.md",
    "RELEASE_NOTES.md",
    "CODE_OF_CONDUCT.md",
    "UPSTREAM.md",
}
INLINE_LINK = re.compile(
    r"\]\(\s*(?:<([^>\n]+)>|((?:\\.|[^()\s]|\([^()\n]*\))+))"
    r"(?:\s+(?:\"[^\"]*\"|'[^']*'|\([^)]*\)))?\s*\)"
)
REFERENCE_LINK = re.compile(
    r"^ {0,3}\[[^]\n]+\]:\s*(?:<([^>\n]+)>|(\S+))", re.MULTILINE
)


def markdown_prose(text):
    """Exclude fenced examples from link and heading parsing."""
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
    for match in re.finditer(
        r"^ {0,3}#{1,6}\s+(.+?)(?:\s+#+)?\s*$", text, re.MULTILINE
    ):
        heading = re.sub(r"<[^>]*>", "", match[1]).lower()
        slug = re.sub(r"[^\w\- ]", "", heading).replace(" ", "-")
        count = counts.get(slug, 0)
        counts[slug] = count + 1
        ids.add(f"{slug}-{count}" if count else slug)
    ids.update(re.findall(r"\b(?:id|name)=[\"']([^\"']+)[\"']", text))
    return ids


def test_original_explorer_documentation_locations_removed():
    for relative in (
        "apps/explorer/docs",
        "apps/explorer/RELEASE_NOTES.md",
        "apps/explorer/CODE_OF_CONDUCT.md",
        "services/explorer/UPSTREAM.md",
    ):
        path = ROOT / relative
        assert not path.exists() and not path.is_symlink(), relative


def test_all_nine_explorer_documents_moved():
    actual = {
        p.relative_to(DOCS).as_posix() for p in DOCS.rglob("*") if p.is_file()
    }
    assert actual == EXPECTED_FILES
    for relative in sorted(EXPECTED_FILES):
        path = DOCS / relative
        assert not path.is_symlink(), relative
        assert path.read_text(encoding="utf-8").startswith("# "), relative


def test_every_relative_explorer_markdown_link_resolves():
    checked = 0
    failures = []
    for path in sorted(DOCS.rglob("*.md")):
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
                if url.fragment and target.suffix.lower() == ".md":
                    target_prose = markdown_prose(target.read_text(encoding="utf-8"))
                    if unquote(url.fragment) not in heading_ids(target_prose):
                        failures.append(
                            f"{path.relative_to(ROOT)}: missing anchor {destination}"
                        )
    assert checked > 0, "The documentation check must inspect real relative links"
    assert not failures, "\n".join(failures)


def test_envs_validator_collects_the_moved_environment_reference():
    validator = ROOT / "apps/explorer/deploy/tools/envs-validator"
    script = (validator / "test.sh").read_text(encoding="utf-8")
    collectors = [
        shlex.split(line)
        for line in script.splitlines()
        if line.startswith("../../scripts/collect_envs.sh ")
    ]
    assert len(collectors) == 1
    assert len(collectors[0]) == 2
    document = collectors[0][1]
    assert document == "../../../../../docs/explorer/ENVS.md"
    assert (validator / document).resolve() == DOCS / "ENVS.md"
    assert (validator / document).is_file()


def test_validator_and_feature_reporter_name_the_moved_environment_reference():
    for relative, reference in (
        (
            "apps/explorer/deploy/tools/envs-validator/index.ts",
            "docs/explorer/ENVS.md",
        ),
        (
            "apps/explorer/deploy/tools/feature-reporter/entry.js",
            "docs/explorer/ENVS.md#app-features",
        ),
    ):
        source = (ROOT / relative).read_text(encoding="utf-8")
        assert reference in source, relative
        assert "blockscout/frontend/blob/main/docs/ENVS.md" not in source, relative
        url = urlsplit(reference)
        document = ROOT / url.path
        assert document.is_file(), relative
        if url.fragment:
            assert url.fragment in heading_ids(markdown_prose(document.read_text()))


def test_no_tracked_references_to_retired_explorer_documentation_paths():
    result = subprocess.run(
        [
            "git", "grep", "-l", "-a", "-F",
            "-e", "apps/explorer/docs/",
            "-e", "apps/explorer/RELEASE_NOTES.md",
            "-e", "services/explorer/UPSTREAM.md",
            "--", ".", ":(exclude)spec/**", ":(exclude)GOTCHA.kvx",
            ":(exclude)tools/qualification/audit_repairs/test_repair_22_36_4.py",
        ],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 1, result.stdout + result.stderr
