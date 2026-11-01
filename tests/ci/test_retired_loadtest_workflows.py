from retired_tests import assert_paths_absent


def test_nested_loadtest_workflows_retired():
    assert_paths_absent(
        [
            "tools/loadtest/.github/workflows/build-and-test.yml",
            "tools/loadtest/.github/workflows/containerize.yaml",
        ]
    )
