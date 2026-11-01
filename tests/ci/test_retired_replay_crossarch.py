from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_replay_crossarch_retired():
    assert_paths_absent(
        [
            "tests/test_replay_crossarch.c",
            "tests/replica/lxp_replay_matrix.sh",
        ]
    )
    assert_symbols_unreferenced(
        [
            "lxp_replay_fixture",
            "lxp_replay_fixture_load",
            "lxp_replay_digest",
            "lxp_replay_crossarch_case",
        ],
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
