from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_relay_archive_retired():
    assert_paths_absent(
        [
            "tests/relay-archive/e2e.py",
            "tests/relay-archive/plain_listener.py",
            "tests/relay-archive/sign_activity.c",
            "tests/relay-archive/state_archive_lifecycle.py",
        ]
    )
    assert_symbols_unreferenced(
        ["layerx-archive-codec"],
        ["tests"],
        exclude=[
            "tests/arith",
            "tests/codec",
            "tests/crypto",
            "tests/fuzz",
            "tests/chain",
            "tests/daemon",
            "tests/bridge",
            "tests/explorer",
        ],
    )
