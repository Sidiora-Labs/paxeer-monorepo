import ast
import importlib.util
import inspect
from pathlib import Path


def test_handover_chain_needs_no_read_consumer_binary():
    path = Path(__file__).with_name('handover-chain.py')
    spec = importlib.util.spec_from_file_location('handover_chain', path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)

    tree = ast.parse(inspect.getsource(module))
    retired = {
        'LAYERX_TEST_HANDOVER_READ_CONSUMER_BIN',
        'native-handover-reads',
    }
    assert all(
        node.value not in retired
        for node in ast.walk(tree)
        if isinstance(node, ast.Constant) and isinstance(node.value, str)
    )
