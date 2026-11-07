"""Check the explorer backend's documented checkout and working directories."""

from pathlib import Path
import re
import shlex
import subprocess


ROOT = Path(__file__).resolve().parents[2]
EXPLORER = ROOT / "services/explorer"
BACKEND = EXPLORER / "backend"
REPOSITORY = "paxeer-monorepo"
DEVCONTAINER_README = BACKEND / ".devcontainer/README.md"
COMPOSE_README = BACKEND / "docker-compose/README.md"


def _command_blocks(content):
    block = None
    for number, line in enumerate(content.splitlines(), 1):
        if line.lstrip().startswith("```"):
            if block is None:
                block = []
            else:
                yield block
                block = None
        elif block is not None:
            block.append((number, line))
    assert block is None, "Unclosed Markdown code fence"


def test_doc_cd_targets_exist():
    devcontainer = DEVCONTAINER_README.read_text(encoding="utf-8")
    compose = COMPOSE_README.read_text(encoding="utf-8")
    assert (
        f"Run the clone command block from the parent directory of `{REPOSITORY}`."
        in devcontainer
    )
    assert (
        f"Run subsequent commands from `{REPOSITORY}/services/explorer/backend`."
        in devcontainer
    )
    context = re.search(r"Start each fenced command block below in `([^`]+)`", compose)
    assert context is not None, "Compose instructions must state a starting directory"
    compose_start = ROOT / context[1]
    assert compose_start == BACKEND
    assert compose_start.is_dir()

    for page, content in (
        (DEVCONTAINER_README, devcontainer),
        (COMPOSE_README, compose),
    ):
        checked = 0
        documented = len(re.findall(r"\bcd\s+", content))
        for block in _command_blocks(content):
            cwd = compose_start if page == COMPOSE_README else BACKEND
            cloned = False
            for number, line in block:
                if re.search(r"\bgit\s+clone\s+", line):
                    cloned = True
                    cwd = ROOT.parent
                for match in re.finditer(r"\bcd\s+([^;&|]+)", line):
                    args = shlex.split(match[1], comments=True)
                    assert len(args) == 1, f"{page}:{number}: invalid cd command"
                    target = Path(args[0])
                    if cloned and cwd == ROOT.parent:
                        # Map the documented checkout name to this checkout, whose
                        # directory may have a different name in CI or a worktree.
                        assert target.parts[0] == REPOSITORY, (
                            f"{page}:{number}: cd must enter the cloned repository"
                        )
                        destination = ROOT.joinpath(*target.parts[1:])
                    else:
                        destination = cwd / target
                    assert destination.is_dir(), (
                        f"{page.relative_to(ROOT)}:{number}: "
                        f"cd {args[0]} from {cwd} does not resolve to a directory"
                    )
                    cwd = destination.resolve()
                    checked += 1
        assert checked > 0, f"No cd commands checked in {page}"
        assert checked == documented, f"Unchecked cd commands in {page}"


def test_doc_clone_urls_name_this_repository():
    tracked = subprocess.run(
        ["git", "ls-files", "-z", "--", "services/explorer"],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.split("\0")
    pages = [ROOT / name for name in tracked if name.lower().endswith(".md")]
    assert DEVCONTAINER_README in pages
    assert COMPOSE_README in pages
    checked = 0
    failures = []
    for page in pages:
        for number, line in enumerate(page.read_text(encoding="utf-8").splitlines(), 1):
            for match in re.finditer(r"\bgit\s+clone\s+([^`]+)", line):
                args = shlex.split(match[1], comments=True)
                urls = [arg for arg in args if "://" in arg or arg.startswith("git@")]
                assert len(urls) == 1, f"{page}:{number}: expected one clone URL"
                checked += 1
                repository = urls[0].rstrip("/").rsplit("/", 1)[-1]
                if repository.removesuffix(".git") != REPOSITORY:
                    failures.append(f"{page.relative_to(ROOT)}:{number}: {urls[0]}")
    assert checked > 0, "No documented explorer clone URLs checked"
    assert not failures, "Incorrect clone repository names:\n" + "\n".join(failures)
