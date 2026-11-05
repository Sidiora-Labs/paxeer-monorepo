from retired_tests import assert_paths_absent


def test_verify_cli_retired():
    assert_paths_absent(["tests/tools/test_verify_cli.c"])
