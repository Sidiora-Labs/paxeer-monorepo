from retired_tests import REPO_ROOT, assert_paths_absent, assert_symbols_unreferenced


def test_module_drivers_retired():
    assert_paths_absent(
        [
            "tests/modules/paxeer_x_budget_lifecycle.py",
            "tests/modules/paxeer_x_escrow_replay_binding.py",
        ]
    )
    readme = "tests/fixtures/custody/paxeer-light-v1/README"
    assert_symbols_unreferenced(["custody-proof-build"], [readme])
    assert "custody-proof-build" not in (REPO_ROOT / readme).read_text(
        encoding="utf-8"
    ), f"{readme} still references custody-proof-build"
