from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_da_retrieval_retired():
    assert_paths_absent(
        ["tests/test_da_retrieval.c", "tests/storage/lxp_test_da_verified.c"]
    )
    assert_symbols_unreferenced(
        [
            "lxp_da_retrieval_request",
            "lxp_da_serve_chunk",
            "lxp_da_fetch",
            "lxp_da_verify_served_bytes",
            "lxp_da_lookup_kind",
            "lxp_da_serve_chunk_proof",
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
