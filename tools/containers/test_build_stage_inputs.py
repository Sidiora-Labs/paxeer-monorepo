from __future__ import annotations

import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import tempfile

import pytest


ROOT = Path(__file__).resolve().parents[2]
IMAGE_DOCKERFILES = (
    ROOT / "docker/platform-webhooks/Dockerfile",
    ROOT / "docker/platform-core/Dockerfile",
)


def _build_stage(dockerfile: Path) -> str:
    recipe = dockerfile.read_text(encoding="utf-8")
    match = re.search(
        r"^FROM[^\n]+\s+AS\s+build\s*$\n(.*?)(?=^FROM|\Z)",
        recipe,
        re.MULTILINE | re.DOTALL,
    )
    assert match is not None, f"{dockerfile} must define a build stage"
    return match.group(1)


def _stage_copies(dockerfile: Path) -> list[tuple[str, str]]:
    copies = []
    for line in _build_stage(dockerfile).splitlines():
        if not line.startswith("COPY "):
            continue
        fields = shlex.split(line)
        fields = [field for field in fields[1:] if not field.startswith("--")]
        assert len(fields) >= 2, f"invalid build-stage COPY in {dockerfile}: {line}"
        copies.extend((source, fields[-1]) for source in fields[:-1])
    return copies


def _build_command(dockerfile: Path) -> list[str]:
    match = re.search(r"^RUN (cargo build .*)$", _build_stage(dockerfile), re.MULTILINE)
    assert match is not None, f"{dockerfile} must define a cargo build command"
    command = shlex.split(match.group(1))
    assert "--locked" in command, f"{dockerfile} build must use locked dependencies"
    assert "--release" in command, (
        f"{dockerfile} build must declare its release profile"
    )
    return [argument for argument in command if argument != "--release"]


def _copy_stage_sources(dockerfile: Path, tree: Path) -> None:
    for source, destination in _stage_copies(dockerfile):
        source_path = ROOT / source.rstrip("/")
        assert source_path.exists(), f"build-stage source does not exist: {source}"
        destination_path = tree / destination.rstrip("/")
        if source_path.is_dir():
            shutil.copytree(source_path, destination_path, dirs_exist_ok=True)
        else:
            destination_path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(source_path, destination_path)


def _run_build(
    dockerfile: Path, target: Path, *, include_headers: bool
) -> subprocess.CompletedProcess[str]:
    with tempfile.TemporaryDirectory(prefix="layerx-build-stage-") as temporary_tree:
        tree = Path(temporary_tree)
        _copy_stage_sources(dockerfile, tree)
        if not include_headers:
            shutil.rmtree(tree / "include" / "layerx")
        environment = os.environ.copy()
        environment["CARGO_TARGET_DIR"] = str(target)
        return subprocess.run(
            _build_command(dockerfile),
            cwd=tree,
            env=environment,
            capture_output=True,
            text=True,
            check=False,
        )


@pytest.fixture(scope="module")
def shared_target(tmp_path_factory: pytest.TempPathFactory) -> Path:
    return tmp_path_factory.mktemp("layerx-build-target")


@pytest.mark.parametrize(
    "dockerfile", IMAGE_DOCKERFILES, ids=lambda path: path.parent.name
)
def test_build_stage_sources_compile_in_debug_profile(
    dockerfile: Path, shared_target: Path
) -> None:
    result = _run_build(dockerfile, shared_target, include_headers=True)
    assert result.returncode == 0, result.stdout + result.stderr


def test_build_stage_without_include_layerx_fails(shared_target: Path) -> None:
    for dockerfile in IMAGE_DOCKERFILES:
        result = _run_build(dockerfile, shared_target, include_headers=False)
        assert result.returncode != 0, (
            f"{dockerfile} unexpectedly built without include/layerx:\n"
            f"{result.stdout}\n{result.stderr}"
        )
