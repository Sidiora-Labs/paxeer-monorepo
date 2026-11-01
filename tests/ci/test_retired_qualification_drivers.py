from retired_tests import REPO_ROOT, assert_paths_absent, assert_symbols_unreferenced


def test_qualification_drivers_retired():
    assert_paths_absent(
        [
            "tests/qualification/paxeer_x_exchange_tif.py",
            "tests/qualification/paxeer_x_oracle_transport.py",
            "tests/qualification/paxeer_x_idempotency_lifecycle.py",
        ]
    )
    excluded = {
        "arith",
        "codec",
        "crypto",
        "fuzz",
        "chain",
        "daemon",
        "bridge",
        "explorer",
    }
    roots = [
        path.relative_to(REPO_ROOT)
        for path in (REPO_ROOT / "tests").iterdir()
        if path.name not in excluded
    ]
    assert_symbols_unreferenced(["lx_oracle_adapter_run"], roots)
