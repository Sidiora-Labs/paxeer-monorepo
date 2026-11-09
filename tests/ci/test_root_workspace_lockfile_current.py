import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
DEPENDENCY_GROUPS = (
    "dependencies",
    "devDependencies",
    "optionalDependencies",
    "peerDependencies",
)


def test_root_lock_matches_workspace_manifests():
    root_manifest = json.loads((ROOT / "package.json").read_text())
    lock = json.loads((ROOT / "package-lock.json").read_text())
    assert lock["lockfileVersion"] == 3
    packages = lock["packages"]
    assert packages[""]["workspaces"] == root_manifest["workspaces"]
    assert root_manifest["workspaces"]

    workspaces = set()
    for pattern in root_manifest["workspaces"]:
        matches = {
            path.relative_to(ROOT).as_posix()
            for path in ROOT.glob(pattern)
            if (path / "package.json").is_file()
        }
        assert matches, f"Workspace pattern has no manifests: {pattern}"
        workspaces.update(matches)

    assert {path for path in packages if "node_modules" not in Path(path).parts} == {
        "",
        *workspaces,
    }
    errors = []
    for workspace in ["", *sorted(workspaces)]:
        manifest = json.loads((ROOT / workspace / "package.json").read_text())
        locked = packages[workspace]
        for field in ("name", "version"):
            assert locked.get(field) == manifest.get(field), (workspace, field)
        for group in (*DEPENDENCY_GROUPS, "peerDependenciesMeta"):
            expected = manifest.get(group, {})
            actual = locked.get(group, {})
            if actual != expected:
                errors.append(
                    f"{workspace or '.'}: {group}: "
                    f"manifest={expected!r}, lockfile={actual!r}"
                )
        if workspace:
            link = packages[f"node_modules/{manifest['name']}"]
            assert link["link"] is True, workspace
            assert link["resolved"] == workspace, workspace
    assert not errors, "\n".join(errors)


def test_root_has_one_package_manager_lockfile():
    manifest = json.loads((ROOT / "package.json").read_text())
    assert manifest["packageManager"] == "npm@11.19.1"
    lockfiles = {
        name
        for name in (
            "package-lock.json",
            "npm-shrinkwrap.json",
            "pnpm-lock.yaml",
            "yarn.lock",
            "bun.lock",
            "bun.lockb",
        )
        if (ROOT / name).exists()
    }
    assert lockfiles == {"package-lock.json"}
    assert not (ROOT / "pnpm-workspace.yaml").exists()
