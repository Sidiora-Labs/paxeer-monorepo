from kernel_retirement import assert_retired


def test_delegate_call_aware_is_retired():
    assert_retired(["contracts/security/DelegateCallAware.sol"])
