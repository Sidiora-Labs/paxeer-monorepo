import ast
from pathlib import Path


def test_payment_journey_calls_no_lx_method():
    root = Path(__file__).parent
    methods = []
    for name in ("payment-journey.py", "journey-credentials.py", "journey_common.py"):
        source = (root / name).read_text()
        tree = ast.parse(source, filename=name)
        for node in ast.walk(tree):
            if not isinstance(node, ast.Call):
                continue
            function = node.func
            is_rpc = (isinstance(function, ast.Name) and function.id == "rpc") or (
                isinstance(function, ast.Attribute) and function.attr == "rpc"
            )
            if not is_rpc:
                continue
            method = (
                node.args[0]
                if node.args
                else next(
                    (argument.value for argument in node.keywords if argument.arg == "method"),
                    None,
                )
            )
            assert isinstance(method, ast.Constant) and isinstance(method.value, str), (
                f"{name}:{node.lineno}: RPC method must be an auditable literal"
            )
            methods.append((name, node.lineno, method.value))
        assert "program-journey.py" not in source, name
    assert methods, "journey RPC inventory must not be empty"
    inventory = "\n".join(f"{name}:{line}: {method}" for name, line, method in methods)
    print(inventory)
    assert all(not method.startswith("lx_") for _, _, method in methods), inventory
    assert all(
        method.startswith(("px_", "eth_", "pax_")) for _, _, method in methods
    ), inventory
    payment_methods = {
        method for name, _, method in methods if name == "payment-journey.py"
    }
    assert {
        "px_requestFunds",
        "px_getBalances",
        "eth_sendRawTransaction",
        "pax_sequencerReceipt",
    } <= payment_methods, inventory
