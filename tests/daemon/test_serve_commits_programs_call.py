from recorded_paxd import observe_committed_program, program_transaction, recovery_recording


def test_serve_commits_an_admitted_programs_call():
    recording = recovery_recording()
    case = recording["case"]
    _, call = program_transaction(case, response_capacity=16)
    assert call.guest_abi == 1
    assert call.response_capacity == 16
    verified, status, state = observe_committed_program(
        case, recording["phases"]["after_merge"], submit=True,
    )
    assert verified.height == int(status["blockNumber"], 16)
    assert state is not None
