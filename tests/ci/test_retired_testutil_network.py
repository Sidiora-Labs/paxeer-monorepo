from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_testutil_network_and_fuzzing_retired():
    assert_paths_absent(
        [
            "tests/testutil/network/network.go",
            "tests/testutil/fuzzing/cosmos.go",
        ]
    )
    assert_symbols_unreferenced(
        [
            "Paxeer-X-Network/tests/testutil/network",
            "Paxeer-X-Network/tests/testutil/fuzzing",
        ],
        ["."],
    )
