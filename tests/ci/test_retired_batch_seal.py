from retired_tests import assert_paths_absent


def test_batch_seal_retired():
    assert_paths_absent(["tests/test_batch_seal.c"])
