from kernel_retirement import assert_retired


def test_credit_c_harness_is_retired():
    assert_retired(
        [
            "tests/bridge/sign_credit.c",
            "tests/bridge/test_credit.c",
            "tests/bridge/test_credit_admission.c",
            "tests/bridge/sole_writer.sh",
        ]
    )
