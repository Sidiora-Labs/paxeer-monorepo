"""Keep public repository links, clone instructions and translations canonical."""

from pathlib import Path
import re
import shlex
import subprocess


ROOT = Path(__file__).resolve().parents[2]
REPOSITORY = "paxeer-monorepo"
CANONICAL_URL = f"https://github.com/Sidiora-Labs/{REPOSITORY}"
GO_MODULE_REPOSITORIES = {"Paxeer-X-Network", "LayerX-Network"}
REPOSITORY_REFERENCE = re.compile(
    r"(?P<transport>(?:https?|git|ssh)://(?:git@)?|git@)?"
    r"github\.com[/:]Sidiora-Labs/(?P<repository>[A-Za-z0-9_.-]+)",
    re.IGNORECASE,
)
BADGE_PATHS = (
    "actions/workflow/status",
    "license",
    "last-commit",
    "stars",
)


def _tracked_markdown_pages():
    tracked = subprocess.run(
        ["git", "ls-files", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.split("\0")
    pages = [
        ROOT / name
        for name in tracked
        if name.lower().endswith(".md")
        and not name.startswith("tools/jev/tests/fixtures/")
    ]
    assert ROOT / "README.md" in pages, "Tracked Markdown inventory lacks README.md"
    return pages


def _readme_translations():
    pages = sorted((ROOT / "docs/readme").glob("README.*.md"))
    expected = {
        f"README.{locale}.md"
        for locale in ("de", "es", "fr", "ja", "pt-BR", "ru", "zh-CN")
    }
    assert expected <= {page.name for page in pages}, "Missing README translations"
    return pages


def test_docs_link_only_the_canonical_repository_url():
    scoped_pages = {
        "CODE_OF_CONDUCT.md",
        "README.md",
        "RELEASE.md",
        "consensus/README.md",
        "contracts/bridge/README.md",
        "docs/platform/beta-environment.md",
        "docs/readme/README.de.md",
        "docs/readme/README.es.md",
        "docs/readme/README.fr.md",
        "docs/readme/README.pt-BR.md",
        "docs/readme/README.zh-CN.md",
        "docs/wiki/Home.md",
        "programs/README.md",
        "services/explorer/backend/.devcontainer/README.md",
    }
    # Repository-wide coverage belongs to task 22.39.64. Preserve upstream
    # documentation and provenance instead of treating them as product links.
    allowlist = {
        "out_of_scope_directories": (
            "tools/hyperfide/",
            "docs/hyperfide/",
            "programs/vendor/",
            "spec/",
        ),
        "provenance_repositories": {"hyperfide", "LayerX-Network", "Launch-x"},
    }
    pages = _tracked_markdown_pages()
    assert scoped_pages <= {page.relative_to(ROOT).as_posix() for page in pages}, (
        "Missing tracked Markdown pages in task scope"
    )
    failures = []
    references = 0
    for page in pages:
        name = page.relative_to(ROOT).as_posix()
        if name.startswith(allowlist["out_of_scope_directories"]):
            continue
        if name not in scoped_pages:
            continue
        for number, line in enumerate(page.read_text(encoding="utf-8").splitlines(), 1):
            for match in REPOSITORY_REFERENCE.finditer(line):
                repository = match["repository"].rstrip(".").removesuffix(".git")
                if match["transport"] is None and repository in GO_MODULE_REPOSITORIES:
                    # These scheme-less names are the declared Go module import paths.
                    continue
                if repository in allowlist["provenance_repositories"]:
                    continue
                references += 1
                if repository != REPOSITORY:
                    failures.append(
                        f"{page.relative_to(ROOT)}:{number}: {match.group()}"
                    )

    assert references, "No repository URLs found in tracked Markdown"
    assert not failures, "Noncanonical repository URLs:\n" + "\n".join(failures)


def test_clone_commands_use_canonical_directory():
    failures = []
    clones = 0
    for page in _tracked_markdown_pages():
        lines = page.read_text(encoding="utf-8").splitlines()
        for index, line in enumerate(lines):
            clone = re.search(r"\bgit\s+clone\s+(.+)", line)
            if clone is None:
                continue
            repository = REPOSITORY_REFERENCE.search(clone[1])
            if repository is None:
                continue
            name = repository["repository"].rstrip(".").removesuffix(".git")
            if name not in GO_MODULE_REPOSITORIES | {REPOSITORY}:
                continue
            clones += 1
            location = f"{page.relative_to(ROOT)}:{index + 1}"
            if name != REPOSITORY:
                failures.append(f"{location}: clone must use {CANONICAL_URL}")
            following = next(
                (
                    candidate.strip()
                    for candidate in lines[index + 1 :]
                    if candidate.strip() and not candidate.lstrip().startswith("#")
                ),
                "",
            )
            command = shlex.split(following.removeprefix("$ "))
            if not (
                len(command) == 2
                and command[0] == "cd"
                and (
                    command[1] == REPOSITORY
                    or command[1].startswith(f"{REPOSITORY}/")
                )
            ):
                failures.append(
                    f"{location}: expected cd {REPOSITORY}, found {following!r}"
                )

    assert clones, "No repository clone commands found in tracked Markdown"
    assert not failures, "Noncanonical clone directories:\n" + "\n".join(failures)


def test_readme_translations_badges_use_canonical_repository():
    reference = None
    for page in _readme_translations():
        content = page.read_text(encoding="utf-8")
        badges = re.findall(r'<img\b[^>]*\bsrc="([^"]+)"[^>]*>', content)
        repository_badges = [
            badge for badge in badges if badge.startswith("https://img.shields.io/github/")
        ]
        assert len(repository_badges) == 4, f"{page.name}: expected four GitHub badges"
        for badge, path in zip(repository_badges, BADGE_PATHS):
            prefix = f"https://img.shields.io/github/{path}/Sidiora-Labs/{REPOSITORY}"
            suffix = "/ci.yml?" if path == "actions/workflow/status" else "?"
            assert badge.startswith(prefix + suffix), f"{page.name}: {badge}"
        if reference is None:
            reference = repository_badges
        assert repository_badges == reference, f"{page.name}: inconsistent badge block"


def test_readme_translations_source_link_is_canonical():
    for page in _readme_translations():
        content = page.read_text(encoding="utf-8")
        source_links = re.findall(
            r"\[(github\.com/Sidiora-Labs/[^\]]+)\]\(([^)]+)\)", content
        )
        assert source_links == [(CANONICAL_URL.removeprefix("https://"), CANONICAL_URL)], (
            f"{page.name}: expected a canonical repository source label and link"
        )
