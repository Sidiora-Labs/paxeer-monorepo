from kernel_retirement import assert_retired


def test_emergency_exit_is_retired():
    assert_retired(["contracts/EmergencyExit.sol"])
