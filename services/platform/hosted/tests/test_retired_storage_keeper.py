from pathlib import Path


def test_storage_keeper_crate_is_absent():
    platform = Path(__file__).resolve().parents[2]
    keeper_name = "paxai-storage-keeper"
    keeper = platform / "hosted" / keeper_name

    assert not keeper.exists(), f"Retired keeper crate still exists: {keeper}"
    assert not keeper.is_symlink(), f"Retired keeper path is a symlink: {keeper}"

    manifests = sorted(platform.rglob("Cargo.toml"))
    assert platform / "Cargo.toml" in manifests, "Platform manifest must exist"
    for manifest in manifests:
        assert keeper_name not in manifest.read_text(encoding="utf-8"), (
            f"Platform manifest still names the retired keeper: {manifest}"
        )
