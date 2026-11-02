from kernel_retirement import assert_retired


def test_semver_comp_is_retired():
    assert_retired(["contracts/libraries/SemverComp.sol"])
