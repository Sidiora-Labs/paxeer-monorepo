from retired_tests import assert_paths_absent


def test_batch_identity_retired():
    assert_paths_absent([
        "tests/protocol/lxp_test_batch_identity.c",
        "tests/protocol/lxp_test_terminal_rejection.c",
    ])
