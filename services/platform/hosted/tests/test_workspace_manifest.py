from pathlib import Path
import tomllib


def test_workspace_members_are_unique():
    workspace = Path(__file__).resolve().parents[2]
    with (workspace / "Cargo.toml").open("rb") as manifest:
        members = tomllib.load(manifest)["workspace"]["members"]

    assert len(members) == len(set(members)), "Workspace members must be unique"
    for member in members:
        assert (workspace / member / "Cargo.toml").is_file(), (
            f"Workspace member {member!r} must contain a Cargo.toml"
        )


def test_paxeer_verifier_is_not_a_platform_dependency():
    workspace = Path(__file__).resolve().parents[2]
    manifest_path = workspace / "hosted" / "indexer" / "Cargo.toml"
    with manifest_path.open("rb") as manifest:
        contents = tomllib.load(manifest)
    for table in (contents, *contents.get("target", {}).values()):
        for kind in ("dependencies", "dev-dependencies", "build-dependencies"):
            for name, dependency in table.get(kind, {}).items():
                package = (
                    dependency.get("package", name)
                    if isinstance(dependency, dict)
                    else name
                )
                assert "layerx-paxeer-verifier" not in (name, package), (
                    "Indexer manifest must not depend on layerx-paxeer-verifier"
                )
    with (workspace / "Cargo.lock").open("rb") as lockfile:
        packages = {
            package["name"]: package for package in tomllib.load(lockfile)["package"]
        }
    assert all(
        dependency.split()[0] != "layerx-paxeer-verifier"
        for dependency in packages["layerx-indexer"]["dependencies"]
    ), "Indexer lock entry must not depend on layerx-paxeer-verifier"
    assert "layerx-paxeer-verifier" in packages, (
        "The verifier package must remain for crates that still depend on it"
    )
