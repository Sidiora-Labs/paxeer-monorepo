from pathlib import Path


def test_agent_boundary_has_no_kernel_node_test():
    tests = Path(__file__).resolve().parents[1] / "agent-boundary" / "tests"

    assert tests.is_dir(), "Agent boundary integration tests directory must exist"
    assert not (tests / "real_node.rs").exists(), "Retired kernel node harness remains"
    assert not (tests / "real_node").exists(), "Retired kernel node cases remain"
    assert (tests / "chain_receipt.rs").is_file(), "Chain receipt tests must remain"

    for path in sorted(tests.rglob("*")):
        if path.is_file():
            contents = path.read_bytes()
            for retired in (b"layerx_wire::activity", b"tests/bridge"):
                assert retired not in contents, (
                    f"{path.relative_to(tests)} still references {retired.decode()}"
                )
