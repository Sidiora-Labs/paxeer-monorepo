"""Task 22.33.916 repair record.

serial-control-source-path is proven by test_serial_control_source_path through
the production Store, signed acceptance artifacts and serial qualification gate.
The existing exact-v2 proof retains the frozen fixture paths, fifty authenticated
acceptances, canonical equality and refusal of changed control bytes and hashes.

Baseline revision: 0e1d5eb94cabe89e2260279a849b7154911ef8ee. No baseline
test was run: the task's ordered gates prohibit verification before implementation.
The audit records the bad source path; no baseline execution result is claimed.
Wiring requests: none. The runner's hf-done evidence records the verified candidate
revision, command, exit code and private log location.
"""

import pytest

import test_serial_gate as serial_gate_tests


@pytest.fixture(scope="module")
def serial_qualification():
    qualification = serial_gate_tests.SerialQualification
    qualification.setUpClass()
    try:
        yield qualification
    finally:
        qualification.tearDownClass()


def test_serial_control_source_path(serial_qualification):
    case = serial_qualification(
        "test_exact_v2_full_gate_retains_signed_fifty_and_candidate_equality"
    )
    case.debug()
