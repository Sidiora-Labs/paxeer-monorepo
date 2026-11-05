import ast
import importlib.util

from recorded_paxd import ROOT, RecordedPaxd, programs_execution_case, verify_program_answer


def test_handover_chain_needs_no_read_consumer_binary():
    path = ROOT / "cmd/retirement/test_daemon_tests_handover_finality.py"
    spec = importlib.util.spec_from_file_location("retained_handover_retirement", path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    assert "tests/daemon/handover-chain.py" in module.RETIRED_PATHS
    for relative in module.RETIRED_PATHS:
        retired_path = ROOT / relative
        assert not retired_path.exists() and not retired_path.is_symlink()

    retired = {"LAYERX_TEST_HANDOVER_READ_CONSUMER_BIN", "native-handover-reads"}
    tree = ast.parse(path.read_text())
    assert all(
        node.value not in retired
        for node in ast.walk(tree)
        if isinstance(node, ast.Constant) and isinstance(node.value, str)
    )
    makefile = (ROOT / "Makefile").read_text()
    for name in retired:
        assert name not in makefile

    case = programs_execution_case()
    peer = RecordedPaxd.execution(case)
    with peer.serving() as client:
        tx_hash = client.send(case["rawTransaction"])
        answer = client.receipt(tx_hash)
        assert answer is not None
        assert verify_program_answer(case, answer).receipt.activity_id.hex() == tx_hash[2:]
