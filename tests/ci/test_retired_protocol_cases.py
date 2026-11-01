from retired_tests import assert_symbols_unreferenced, build_and_run_c_suite


# Retired symbol identifiers used by the source absence checks.
lxp_checkpoint_maximum_attestation_delay_ms = (
    "lxp_checkpoint_maximum_attestation_delay_ms"
)
lxp_guarantor_attestation = "lxp_guarantor_attestation"
lxp_checkpoint_certificate = "lxp_checkpoint_certificate"
lxp_guarantor_key_record = "lxp_guarantor_key_record"


def test_protocol_cases_retired():
    assert_symbols_unreferenced(
        [
            lxp_checkpoint_maximum_attestation_delay_ms,
            lxp_guarantor_attestation,
            lxp_checkpoint_certificate,
            lxp_guarantor_key_record,
            "lxp_guarantor_cert",
            "lxp_guarantor_set",
            "lxp_finalisation_state",
            "lxp_finalisation_requirements",
        ],
        ["tests/protocol/lxp_test_protocol.c"],
    )
    assert_symbols_unreferenced(
        [
            "lx_batch_header",
            "lx_web_observation_encode",
            "lx_web_root",
            "lx_batch_header_set_web_root",
        ],
        ["tests/protocol/lxp_test_dispatch.c"],
    )
    build_and_run_c_suite("build/tests/lxp_test_protocol")
    build_and_run_c_suite("build/tests/lxp_test_dispatch")
