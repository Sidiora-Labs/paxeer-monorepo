from kernel_retirement import assert_retired


def test_light_credit_is_retired():
    assert_retired(["tests/bridge/test_light_credit.c"])
