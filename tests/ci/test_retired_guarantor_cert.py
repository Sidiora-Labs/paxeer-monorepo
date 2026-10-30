from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_guarantor_cert_retired():
    assert_paths_absent(
        ["tests/test_guarantor_cert.c", "tests/test_receipt_offline.c"]
    )
    assert_symbols_unreferenced(
        ["lxp_augmented_receipt", "lxp_receipt_augment"],
        ["tests"],
        exclude=[
            "tests/arith",
            "tests/codec",
            "tests/crypto",
            "tests/fuzz",
            "tests/chain",
            "tests/daemon",
            "tests/bridge",
            "tests/explorer",
        ],
    )
