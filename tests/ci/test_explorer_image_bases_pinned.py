import re
import shlex
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[2]
DOCKERFILES = (
    ROOT / "docker/explorer-sig-provider/Dockerfile",
    ROOT / "docker/explorer-smart-contract-verifier/Dockerfile",
)
PINNED_IMAGE = re.compile(r"([^@\s]+):([^:@\s]+)@sha256:[0-9a-f]{64}")


def _instructions(path):
    contents = path.read_text(encoding="utf-8")
    contents = re.sub(r"\\\r?\n", " ", contents)
    for line in contents.splitlines():
        line = line.strip()
        if line and not line.startswith("#"):
            instruction, arguments = line.split(None, 1)
            yield instruction.upper(), shlex.split(arguments, comments=True)


@pytest.mark.parametrize("dockerfile", DOCKERFILES, ids=lambda path: path.parent.name)
def test_explorer_bases_pinned_by_digest(dockerfile):
    stages = set()
    external_bases = []
    for instruction, arguments in _instructions(dockerfile):
        if instruction != "FROM":
            continue
        base = next(arg for arg in arguments if not arg.startswith("--"))
        if base.lower() not in stages:
            external_bases.append(base)
            match = PINNED_IMAGE.fullmatch(base)
            assert match, f"{dockerfile}: base needs an exact tag and digest: {base}"
            tag = match.group(2)
            assert re.search(r"\d+\.\d+", tag), f"Unversioned base tag: {base}"
            assert not {"latest", "stable", "rolling", "edge"}.intersection(
                re.split(r"[._-]", tag.lower())
            ), f"Floating base tag: {base}"
        if len(arguments) >= 3 and arguments[-2].upper() == "AS":
            stages.add(arguments[-1].lower())
    assert len(external_bases) >= 2, f"{dockerfile}: builder and runtime are required"


@pytest.mark.parametrize("dockerfile", DOCKERFILES, ids=lambda path: path.parent.name)
def test_explorer_cargo_builds_locked(dockerfile):
    builds = []
    cooks = []
    installs = []
    for instruction, arguments in _instructions(dockerfile):
        if instruction != "RUN":
            continue
        commands = []
        command = []
        for argument in arguments:
            if argument in {"&&", "||", ";", "|"}:
                commands.append(command)
                command = []
            else:
                command.append(argument)
        commands.append(command)
        for command in commands:
            if command[:2] == ["cargo", "build"]:
                builds.append(command)
            elif command[:3] == ["cargo", "chef", "cook"]:
                cooks.append(command)
            elif command[:3] == ["cargo", "install", "cargo-chef"]:
                installs.append(command)
    assert builds, f"{dockerfile}: no cargo build found"
    assert cooks, f"{dockerfile}: no dependency cache build found"
    assert installs, f"{dockerfile}: cargo-chef must be installed"
    for command in builds + cooks + installs:
        assert "--locked" in command, f"{dockerfile}: unlocked command: {command}"
    for command in builds + cooks:
        assert "--release" in command, f"{dockerfile}: expected release build"
    for command in installs:
        assert "--version" in command, f"{dockerfile}: cargo-chef must be versioned"
        version = command[command.index("--version") + 1]
        assert re.fullmatch(r"\d+\.\d+\.\d+", version), version


@pytest.mark.parametrize("dockerfile", DOCKERFILES, ids=lambda path: path.parent.name)
def test_no_eol_runtime_base_in_published_images(dockerfile):
    instructions = list(_instructions(dockerfile))
    runtime_start = max(
        index
        for index, (instruction, _) in enumerate(instructions)
        if instruction == "FROM"
    )
    runtime = instructions[runtime_start:]
    base = runtime[0][1][0]
    assert base.split("@", 1)[0] == "ubuntu:24.04", (
        f"{dockerfile}: published runtime must use the supported Ubuntu 24.04 base"
    )
    packages = [
        argument
        for instruction, arguments in runtime
        if instruction == "RUN"
        for argument in arguments
    ]
    assert "libssl3" in packages, f"{dockerfile}: OpenSSL 3 runtime is required"
    assert "ca-certificates" in packages, f"{dockerfile}: TLS roots are required"
    assert not any(package.startswith("libssl1.") for package in packages), (
        f"{dockerfile}: end-of-life OpenSSL runtime"
    )
