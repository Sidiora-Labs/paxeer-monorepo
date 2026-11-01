from kernel_retirement import assert_retired


def test_message_types_is_retired():
    assert_retired(["contracts/libraries/MessageTypes.sol"])
