import json
import re
import shlex
from fnmatch import fnmatchcase
from pathlib import Path, PurePosixPath


ROOT = Path(__file__).resolve().parents[2]
DOCKERFILE = ROOT / "docker/explorer-frontend/Dockerfile"
DOCKERIGNORE = DOCKERFILE.with_name("Dockerfile.dockerignore")


def local_copy_sources():
    dockerfile = re.sub(r"\\\r?\n", " ", DOCKERFILE.read_text(encoding="utf-8"))
    sources = []
    for line in dockerfile.splitlines():
        instruction = line.strip().split(maxsplit=1)
        if not instruction or instruction[0].upper() != "COPY":
            continue
        arguments = instruction[1]
        options = []
        while arguments.startswith("--"):
            option, arguments = arguments.split(maxsplit=1)
            options.append(option)
        if any(option.startswith("--from=") for option in options):
            continue
        paths = (
            json.loads(arguments)
            if arguments.startswith("[")
            else shlex.split(arguments)
        )
        assert len(paths) >= 2, f"COPY needs a source and destination: {line}"
        sources.extend(paths[:-1])
    assert sources, "No local COPY sources checked"
    return sources


def test_every_dockerfile_copy_source_is_reincluded():
    patterns = [
        line.strip()[1:].strip("/")
        for line in DOCKERIGNORE.read_text(encoding="utf-8").splitlines()
        if line.strip().startswith("!")
    ]
    assert patterns, "No build context allowlist entries found"
    missing = []
    for source in local_copy_sources():
        path = PurePosixPath(source)
        candidates = [path, *path.parents]
        if not any(
            fnmatchcase(str(candidate), pattern)
            for candidate in candidates
            for pattern in patterns
        ):
            missing.append(source)
    assert not missing, "COPY sources missing from build context allowlist: " + ", ".join(
        missing
    )


def test_envs_md_exists():
    assert (ROOT / "docs/explorer/ENVS.md").is_file()
