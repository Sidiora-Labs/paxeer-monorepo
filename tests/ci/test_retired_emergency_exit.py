from retired_tests import assert_paths_absent


def test_emergency_exit_retired():
    assert_paths_absent(["tests/test_emergency_exit.c"])
