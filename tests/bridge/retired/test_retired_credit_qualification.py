from kernel_retirement import assert_retired


def test_credit_qualification_is_retired():
    assert_retired(
        [
            "tests/bridge/qualify_credit.py",
            "tests/bridge/custody_credit.py",
            "tests/bridge/comet_credit.py",
            "tests/bridge/local_credit_evidence.py",
        ]
    )
