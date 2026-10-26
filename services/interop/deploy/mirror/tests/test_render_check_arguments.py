import ast
import re
import shlex
from pathlib import Path


def test_check_passes_only_arguments_the_renderer_declares():
    mirror = Path(__file__).resolve().parents[1]
    renderer = ast.parse((mirror / "render-config.py").read_text())
    declared = {
        argument.value
        for call in ast.walk(renderer)
        if isinstance(call, ast.Call)
        and isinstance(call.func, ast.Attribute)
        and call.func.attr == "add_argument"
        for argument in call.args
        if isinstance(argument, ast.Constant)
        and isinstance(argument.value, str)
        and argument.value.startswith("--")
    }
    assert declared, "No renderer argument declarations found"

    check = (mirror / "tests" / "render-config-check.sh").read_text()
    logical_lines = check.replace("\\\n", "").splitlines()
    invocation = re.compile(
        r'\b(?:python3\s+"\$RENDERER"|render(?:_ethereum_only)?)(?=\s)'
    )
    commands = [
        line[match.end():]
        for line in logical_lines
        if (match := invocation.search(line))
    ]
    assert any('python3 "$RENDERER"' in line for line in logical_lines)
    assert commands, "No renderer invocations found"

    # solana_target supplies additional arguments through render's array.
    solana_target = re.search(r"^solana_target\(\) \{\n(.*?)^\}", check, re.M | re.S)
    assert solana_target, "No Solana argument provider found"
    commands.append(solana_target.group(1).replace("\\\n", ""))

    passed = {
        token.split("=", 1)[0]
        for command in commands
        for token in shlex.split(command, comments=True)
        if token.startswith("--")
    }
    assert passed, "No renderer flags found"
    assert passed <= declared, f"Undeclared renderer flags: {sorted(passed - declared)}"
