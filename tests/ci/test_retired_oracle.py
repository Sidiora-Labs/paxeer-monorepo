from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_oracle_retired():
    assert_paths_absent(
        [
            "tests/modules/test_oracle_transport.c",
            "tests/network/test_oracle_adapter.c",
            "tests/sequencer/test_oracle_root.c",
        ]
    )
    assert_symbols_unreferenced(
        [
            "lx_oracle_transport_encode",
            "lx_oracle_adapter_config",
            "lx_oracle_activity_encode_signed",
            "lx_oracle_activity_encode",
            "lx_oracle_adapter_isolation_check",
            "lx_oracle_availability_bundle",
        ],
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
