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
