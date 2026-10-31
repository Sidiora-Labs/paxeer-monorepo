from pathlib import Path


def test_authority_has_no_kernel_node_test():
    authority_tests = Path(__file__).resolve().parents[1] / "authority" / "tests"
    assert not (authority_tests / "real_node.rs").exists()

    sources = sorted(authority_tests.rglob("*.rs"))
    assert sources, "Authority tests must remain present"
    for source in sources:
        assert "tests/daemon/fixtures" not in source.read_text(), (
            f"{source} still references retired daemon fixtures"
        )

    for fixture in ("real-handover-genesis.lxt", "real-program-deploy-receipt.json"):
        assert (authority_tests / "fixtures" / fixture).is_file(), (
            f"Shared authority fixture must remain present: {fixture}"
        )
