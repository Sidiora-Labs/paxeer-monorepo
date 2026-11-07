import json
from pathlib import Path
import re
import subprocess

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[2]
PNPM_PIN = re.compile(r"pnpm@(\d+\.\d+\.\d+)(?:\+sha(?:224|256|384|512)\.[0-9a-f]+)?")
DOCKERFILE_WORKSPACES = (
    ("docker/web/Dockerfile", "build", "services/wallet"),
    ("docker/wallet-gateway/Dockerfile", "build", "services/wallet"),
    ("docker/wallet-pwa/Dockerfile", "sdk", "services/wallet"),
    ("docker/wallet-pwa/Dockerfile", "builder", "apps/wallet"),
    ("apps/perps/Dockerfile", "deps", "apps/perps"),
    ("apps/perps/Dockerfile", "builder", "apps/perps"),
    ("deploy/launchpad/Dockerfile.web", "build", "apps/launchpad"),
)
IMAGE_DEPENDENCIES_FIXTURE = (
    ROOT / "apps/launchpad/qualification/image-dependencies/fixture"
)


def _tracked_paths(pattern):
    return [
        ROOT / path
        for path in subprocess.check_output(
            ["git", "ls-files", "-z", "--", pattern], cwd=ROOT
        ).decode().split("\0")
        if path
    ]


def _manifest(directory):
    return json.loads((directory / "package.json").read_text())


def _release(pin, path):
    match = PNPM_PIN.fullmatch(pin)
    assert match, f"{path}: packageManager must pin an exact pnpm release: {pin!r}"
    return match[1]


def _workspaces():
    workspaces = []
    for path in _tracked_paths("**/pnpm-workspace.yaml"):
        # This npm image fixture uses the file only to supply pnpm overrides.
        if path.parent != IMAGE_DEPENDENCIES_FIXTURE:
            workspaces.append(path.parent)
    assert workspaces, "No pnpm workspaces found"
    return workspaces


def _lock_importers(path):
    importers = {}
    for document in yaml.safe_load_all(path.read_text()):
        assert isinstance(document, dict) and "lockfileVersion" in document, path
        for name, groups in document.get("importers", {}).items():
            importer = importers.setdefault(name, {})
            for group, dependencies in groups.items():
                assert group not in importer, (path, name, group)
                importer[group] = dependencies
    assert "." in importers, f"{path}: missing root importer"
    return importers


@pytest.mark.parametrize("dockerfile,stage,owner", DOCKERFILE_WORKSPACES)
def test_dockerfile_pnpm_matches_package_manager(dockerfile, stage, owner):
    manifest_path = ROOT / owner / "package.json"
    expected = _release(_manifest(manifest_path.parent)["packageManager"], manifest_path)
    assert (manifest_path.parent / "pnpm-lock.yaml").is_file(), owner
    text = (ROOT / dockerfile).read_text()
    stages = re.split(r"(?im)^FROM\s+", text)[1:]
    matching = [
        section.partition("\n")[2]
        for section in stages
        if re.search(rf"\sAS\s+{re.escape(stage)}\s*$", section.splitlines()[0], re.I)
    ]
    assert len(matching) == 1, (dockerfile, stage)
    commands = "\n".join(
        line for line in matching[0].splitlines() if not line.lstrip().startswith("#")
    ).replace("\\\n", " ")
    pins = re.findall(r"\bpnpm@([^\s\"';]+)", commands)
    assert pins, f"{dockerfile}: {stage} must activate a pinned pnpm release"
    assert re.search(
        r"corepack prepare pnpm@\S+ --activate|"
        r"npm install -g pnpm@\S+|"
        r"npm exec --yes --package=pnpm@\S+ -- pnpm\b",
        commands,
    ), f"{dockerfile}: {stage} does not activate pnpm"
    for pin in pins:
        assert _release(f"pnpm@{pin}", dockerfile) == expected, (
            f"{dockerfile}: {stage} activates pnpm@{pin}, "
            f"but {owner}/package.json pins pnpm@{expected}"
        )


def test_each_pnpm_workspace_has_a_lockfile():
    for workspace in _workspaces():
        _release(_manifest(workspace).get("packageManager", ""), workspace)
        lockfile = workspace / "pnpm-lock.yaml"
        assert lockfile.is_file(), f"{workspace}: missing pnpm-lock.yaml"
        _lock_importers(lockfile)


def test_shared_lockfile_has_one_pnpm_release():
    workspaces = set(_workspaces())
    for path in _tracked_paths("**/pnpm-lock.yaml"):
        if _manifest(path.parent).get("packageManager", "").startswith("pnpm@"):
            workspaces.add(path.parent)
    releases_by_member = {}
    for workspace in sorted(workspaces):
        expected = _release(_manifest(workspace).get("packageManager", ""), workspace)
        for importer in _lock_importers(workspace / "pnpm-lock.yaml"):
            member = (workspace / importer).resolve()
            manifest = _manifest(member)
            if "packageManager" in manifest:
                actual = _release(manifest["packageManager"], member)
                assert actual == expected, (
                    f"{workspace}/pnpm-lock.yaml: {member} pins pnpm@{actual}, "
                    f"but the lockfile owner pins pnpm@{expected}"
                )
            previous = releases_by_member.setdefault(member, expected)
            assert previous == expected, (
                f"{member} is installed by shared lockfiles using "
                f"pnpm@{previous} and pnpm@{expected}"
            )
