import ast
import inspect
from pathlib import Path
import subprocess
import sys
from types import ModuleType


def test_cases_are_history_and_pagination_only(data_boundary: ModuleType):
    assert data_boundary.CASES == ("history-readiness", "unified-pagination")
    assert callable(data_boundary.history_readiness)
    assert callable(data_boundary.unified_pagination)
    for name in ("kernel_receipts", "KernelReceipts", "receipt_refresh"):
        assert not hasattr(data_boundary, name)
    assert not any(name.startswith("sr_") for name in vars(data_boundary))

    script = Path(data_boundary.__file__)
    help_result = subprocess.run(
        [sys.executable, str(script), "--help"], capture_output=True, text=True, timeout=10
    )
    assert help_result.returncode == 0, help_result.stderr
    assert "--case {history-readiness,unified-pagination}" in help_result.stdout
    for case in ("kernel-receipts", "receipt-refresh", "standalone-receipts"):
        result = subprocess.run(
            [sys.executable, str(script), "--case", case],
            capture_output=True,
            text=True,
            timeout=10,
        )
        assert result.returncode == 2, result.stderr
        assert "invalid choice" in result.stderr


def test_no_retired_binary_prerequisite(data_boundary: ModuleType):
    tree = ast.parse(Path(data_boundary.__file__).read_text())
    prerequisites = ast.parse(inspect.getsource(data_boundary.prerequisites))
    conditions = []
    for node in ast.walk(tree):
        if isinstance(node, ast.If):
            conditions.append(node.test)
        elif isinstance(node, ast.For):
            conditions.append(node.iter)
        elif isinstance(node, ast.Raise) and node.exc is not None:
            conditions.append(node.exc)
    for node in (prerequisites, *conditions):
        literals = [
            item.value for item in ast.walk(node)
            if isinstance(item, ast.Constant) and isinstance(item.value, str)
        ]
        for binary in ("layerxd", "layerx-archive-codec"):
            assert not any(binary in literal for literal in literals), literals

    retired_prefixes = ("LAYERX_KERNEL_RECEIPT_", "LAYERX_EXPLORER_", "LAYERX_RECEIPT_REFRESH_")
    retired_variables = {
        "LAYERX_NETWORK_GATEWAY_ENDPOINT",
        "PAXEER_X_LAYERX_SEND_CMD",
        "PAXEER_X_LAYERX_ASSETS",
        "PAXEER_X_STANDALONE_RECEIPTS_ACCOUNT",
        "PAXEER_X_NODE_PRIVATE_DIR",
    }
    for node in ast.walk(tree):
        if isinstance(node, ast.Constant) and isinstance(node.value, str):
            assert not node.value.startswith(retired_prefixes), node.value
            assert node.value not in retired_variables, node.value
