from pathlib import Path


def test_core_has_no_kernel_daemon_test():
    tests = Path(__file__).resolve().parents[1] / "core" / "tests"

    assert tests.is_dir(), "Core integration tests directory must exist"
    for retired in (
        "boundary.rs",
        "custody_asset_send.rs",
        "core_chain.py",
        "qualify.py",
        "receipt_retention.py",
    ):
        assert not (tests / retired).exists(), f"Retired core test remains: {retired}"

    for retained in ("send.rs", "signer.rs"):
        assert (tests / retained).is_file(), f"Core library test must remain: {retained}"

    for path in sorted(tests.rglob("*")):
        if path.is_file():
            contents = path.read_bytes()
            for retired in (b"layerxd", b"checkpoint-settlement.json"):
                assert retired not in contents, (
                    f"{path.relative_to(tests)} still references {retired.decode()}"
                )
