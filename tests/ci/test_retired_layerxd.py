from retired_tests import assert_paths_absent


def test_layerxd_retired():
    assert_paths_absent(
        [
            "tests/test_layerxd.c",
            "tests/test_daemon_bootstrap_artifact.c",
        ]
    )
