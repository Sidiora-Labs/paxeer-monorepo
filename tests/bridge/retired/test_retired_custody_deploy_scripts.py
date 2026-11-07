from kernel_retirement import assert_retired


def test_custody_deploy_scripts_is_retired():
    assert_retired(
        [
            "tests/bridge/custody_genesis.py",
            "tests/bridge/deploy_local_custody.py",
            "tests/bridge/owner-custody.py",
            "tests/bridge/test_evidence.py",
        ]
    )
