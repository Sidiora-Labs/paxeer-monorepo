from pathlib import Path
import re
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


def test_retired_kernel_crates_are_not_dependencies():
    workspace = Path(__file__).resolve().parents[2]
    retired = {
        "layerx-client",
        "layerx-programs-runtime",
        "layerx-programs-registry",
        "layerx-programs-protocol-adapter",
    }
    non_code = re.compile(
        r'//[^\n]*|/\*.*?\*/|(?:br|r)(?P<hashes>\#*)".*?"(?P=hashes)'
        r'|b?"(?:\\.|[^"\\])*"',
        re.DOTALL,
    )
    for member in ("gateway", "agent-boundary", "authority", "registry", "indexer"):
        crate = workspace / "hosted" / member
        with (crate / "Cargo.toml").open("rb") as manifest:
            contents = tomllib.load(manifest)
        source_paths = [
            path
            for directory in ("src", "tests", "examples", "benches")
            for path in (crate / directory).rglob("*.rs")
        ]
        if (crate / "build.rs").is_file():
            source_paths.append(crate / "build.rs")
        assert source_paths, f"{member} must have Rust sources to scan"
        sources = "\n".join(
            non_code.sub(" ", path.read_text()) for path in source_paths
        )
        for table in (contents, *contents.get("target", {}).values()):
            for kind in ("dependencies", "dev-dependencies", "build-dependencies"):
                for name, dependency in table.get(kind, {}).items():
                    package = (
                        dependency.get("package", name)
                        if isinstance(dependency, dict)
                        else name
                    )
                    if package not in retired:
                        continue
                    namespace = re.escape(name.replace("-", "_"))
                    assert re.search(
                        rf"\b{namespace}\s*::|\bextern\s+crate\s+{namespace}\b",
                        sources,
                    ), f"{member} lists unused retired dependency {package!r} as {name!r}"

    with (workspace / "Cargo.lock").open("rb") as lockfile:
        packages = {
            package["name"]: package for package in tomllib.load(lockfile)["package"]
        }
    toolkit_dependencies = {
        dependency.split()[0]
        for dependency in packages["layerx-ramp-toolkit"]["dependencies"]
    }
    assert toolkit_dependencies.isdisjoint({"layerx-intents", "layerx-paxeer-client"}), (
        "Ramp toolkit lock entry must not depend on layerx-intents or layerx-paxeer-client"
    )
