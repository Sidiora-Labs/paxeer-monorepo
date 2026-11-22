import json
import re
import shlex
from fnmatch import fnmatchcase
from pathlib import Path, PurePosixPath


ROOT = Path(__file__).resolve().parents[2]
DOCKERFILE = ROOT / "docker/explorer-elixir-builder/Dockerfile"
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


def build_context_patterns():
    patterns = [
        line.strip()
        for line in DOCKERIGNORE.read_text(encoding="utf-8").splitlines()
        if line.strip() and not line.strip().startswith("#")
    ]
    assert patterns, "No build context patterns found"
    return patterns


def path_matches(path, pattern):
    path = PurePosixPath(path)
    return any(
        fnmatchcase(str(candidate), pattern.strip("/"))
        for candidate in [path, *path.parents]
    )


def is_in_build_context(path, patterns):
    included = True
    for pattern in patterns:
        reincluded = pattern.startswith("!")
        if path_matches(path, pattern[1:] if reincluded else pattern):
            included = reincluded
    return included


def test_every_dockerfile_copy_source_is_reincluded():
    patterns = build_context_patterns()
    missing = [
        source
        for source in local_copy_sources()
        if not is_in_build_context(source, patterns)
    ]
    assert not missing, "COPY sources missing from build context allowlist: " + ", ".join(
        missing
    )


def test_only_copied_builder_files_are_reincluded():
    patterns = build_context_patterns()
    sources = local_copy_sources()
    unexpected = []
    for file in sorted(DOCKERFILE.parent.rglob("*")):
        if not file.is_file():
            continue
        source = file.relative_to(ROOT).as_posix()
        if is_in_build_context(source, patterns) and not any(
            path_matches(source, copied) for copied in sources
        ):
            unexpected.append(source)
    assert not unexpected, (
        "Builder files not copied by Dockerfile in build context: "
        + ", ".join(unexpected)
    )
