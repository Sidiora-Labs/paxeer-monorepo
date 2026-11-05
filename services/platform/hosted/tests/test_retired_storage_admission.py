from pathlib import Path


def test_authority_has_no_storage_admission():
    platform = Path(__file__).resolve().parents[2]
    module_name = "ai_storage_admission"
    module = platform / "hosted" / "authority" / "src" / f"{module_name}.rs"

    assert not module.exists(), f"Retired storage admission module still exists: {module}"
    assert not module.is_symlink(), f"Retired module path is a symlink: {module}"

    sources = sorted(platform.rglob("*.rs"))
    assert platform / "hosted" / "authority" / "src" / "lib.rs" in sources, (
        "Authority Rust source must exist"
    )
    for source in sources:
        assert module_name not in source.read_text(encoding="utf-8"), (
            f"Platform Rust source still names the retired module: {source}"
        )
