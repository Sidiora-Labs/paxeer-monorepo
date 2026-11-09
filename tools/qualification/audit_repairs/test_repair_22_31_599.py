from fnmatch import fnmatchcase
import os
from pathlib import Path
import pwd
import re
import shlex
import shutil
import subprocess
import tempfile

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "apps/explorer/deploy/scripts/collect_envs.sh"
CATALOGUE = "docs/explorer/ENVS.md"
REQUIRED_ENTRIES = {
    "NEXT_PUBLIC_APP_HOST=__",
    "NEXT_PUBLIC_NETWORK_ID=__",
    "NEXT_PUBLIC_API_HOST=__",
}


def _context_includes(path, rules):
    candidates = [
        path,
        *(str(parent) for parent in Path(path).parents if str(parent) != "."),
    ]
    included = True
    for rule in rules:
        rule = rule.strip()
        if not rule or rule.startswith("#"):
            continue
        pattern = rule.lstrip("!").strip("/")
        if any(fnmatchcase(candidate, pattern) for candidate in candidates):
            included = rule.startswith("!")
    return included


def _prepare_outputs(directory, existing_outputs):
    before = {}
    for name in (".env.registry", ".env"):
        output = directory / name
        if existing_outputs:
            output.write_text(f"Preserve existing {name}\n")
            before[name] = (output.read_bytes(), output.stat().st_mtime_ns)
    return before


def _assert_outputs_unchanged(directory, before):
    for name in (".env.registry", ".env"):
        output = directory / name
        if name in before:
            assert (output.read_bytes(), output.stat().st_mtime_ns) == before[name]
        else:
            assert not output.exists()


def test_builder_collects_envs_from_consolidated_docs(tmp_path):
    dockerfile = (ROOT / "docker/explorer-frontend/Dockerfile").read_text()
    assert "./docs/ENVS.md" not in dockerfile
    builder = dockerfile.split(" AS builder\n", 1)[1].split("\nFROM ", 1)[0]
    copy = "COPY docs/explorer/ENVS.md ./docs/explorer/ENVS.md"
    collect = "./deploy/scripts/collect_envs.sh ./docs/explorer/ENVS.md"
    assert copy in builder
    assert collect in builder
    assert builder.index(copy) < builder.index(collect)

    invocation = re.search(r"(\./\S*collect_envs\.sh)\s+(\S+)", builder)
    assert invocation
    workdir = Path(
        re.findall(r"^WORKDIR\s+(\S+)$", builder[: invocation.start()], re.M)[-1]
    )
    collector_path = workdir / invocation[1]
    catalogue_path = workdir / invocation[2]
    rules = (
        (ROOT / "docker/explorer-frontend/Dockerfile.dockerignore")
        .read_text()
        .splitlines()
    )
    staged = set()
    for match in re.finditer(r"^COPY\s+([^\n]+)$", builder[: invocation.start()], re.M):
        operands = shlex.split(match[1])
        if operands[0].startswith("--"):
            continue
        source, destination = operands
        target = workdir / destination
        if source == CATALOGUE:
            assert target == catalogue_path
            inputs = [(source, target)]
        elif (ROOT / source).is_dir() and collector_path.is_relative_to(target):
            inputs = [
                (str(Path(source) / collector_path.relative_to(target)), collector_path)
            ]
        else:
            continue
        for context_path, image_path in inputs:
            assert _context_includes(context_path, rules), context_path
            destination_path = tmp_path / image_path.relative_to(workdir)
            destination_path.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(ROOT / context_path, destination_path)
            staged.add(image_path)
    assert staged == {collector_path, catalogue_path}
    result = subprocess.run(
        ["bash", str(tmp_path / collector_path.relative_to(workdir)), invocation[2]],
        cwd=tmp_path,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stderr
    assert REQUIRED_ENTRIES <= set((tmp_path / ".env.registry").read_text().splitlines())
    assert (tmp_path / ".env").is_file()


def test_collect_envs_writes_registry_from_consolidated_docs(tmp_path):
    result = subprocess.run(
        ["bash", str(SCRIPT), str(ROOT / "docs/explorer/ENVS.md")],
        cwd=tmp_path,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stderr
    registry = tmp_path / ".env.registry"
    assert registry.is_file()
    lines = registry.read_text().splitlines()
    assert lines
    assert REQUIRED_ENTRIES <= set(lines)
    assert all(line.startswith("NEXT_PUBLIC_") for line in lines)
    assert all(re.fullmatch(r"NEXT_PUBLIC_[A-Za-z0-9_]+=__", line) for line in lines)
    assert lines == sorted(set(lines))
    assert (tmp_path / ".env").is_file()


@pytest.mark.parametrize("existing_outputs", [False, True])
def test_collect_envs_rejects_missing_document(tmp_path, existing_outputs):
    document = tmp_path / "missing.md"
    before = _prepare_outputs(tmp_path, existing_outputs)
    result = subprocess.run(
        ["bash", str(SCRIPT), str(document)],
        cwd=tmp_path,
        capture_output=True,
        text=True,
    )
    assert result.returncode == 1
    assert f"Environment documentation not found: {document}" in result.stderr
    _assert_outputs_unchanged(tmp_path, before)


@pytest.mark.parametrize("event", ["push", "pull_request"])
def test_explorer_catalogue_changes_trigger_images(event):
    workflow = yaml.load(
        (ROOT / ".github/workflows/explorer-images.yml").read_text(),
        Loader=yaml.BaseLoader,
    )
    filters = workflow["on"][event]["paths"]
    assert CATALOGUE in filters
    selected = False
    for pattern in filters:
        if fnmatchcase(CATALOGUE, pattern.lstrip("!")):
            selected = not pattern.startswith("!")
    assert selected, f"A catalogue-only {event} must regenerate Explorer images"


@pytest.mark.parametrize("existing_outputs", [False, True])
def test_collect_envs_rejects_unreadable_document(existing_outputs):
    credentials = {}
    if os.geteuid() == 0:
        unprivileged = pwd.getpwnam("nobody")
        assert unprivileged.pw_uid != 0
        credentials = {
            "user": unprivileged.pw_uid,
            "group": unprivileged.pw_gid,
            "extra_groups": [],
        }
    with tempfile.TemporaryDirectory() as directory:
        workdir = Path(directory)
        workdir.chmod(0o777)
        script = workdir / "collect_envs.sh"
        shutil.copyfile(SCRIPT, script)
        script.chmod(0o644)
        document = workdir / "ENVS.md"
        shutil.copyfile(ROOT / CATALOGUE, document)
        document.chmod(0o000)
        before = _prepare_outputs(workdir, existing_outputs)
        for name in before:
            (workdir / name).chmod(0o666)
        environment = {**os.environ, "LC_ALL": "C"}
        denied = subprocess.run(
            ["cat", str(document)],
            cwd=workdir,
            env=environment,
            capture_output=True,
            text=True,
            **credentials,
        )
        assert denied.returncode != 0
        assert "Permission denied" in denied.stderr
        result = subprocess.run(
            ["bash", str(script), str(document)],
            cwd=workdir,
            env=environment,
            capture_output=True,
            text=True,
            **credentials,
        )
        assert result.returncode != 0
        assert f"Environment documentation is not readable: {document}" in result.stderr
        _assert_outputs_unchanged(workdir, before)
