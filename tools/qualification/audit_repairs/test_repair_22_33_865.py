"""Source binding proof; protected gateway/node qualification remains external."""

import ast
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import textwrap


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "scripts/paxeer-x/verify-sdk-program-python.sh"
CORPUS = [
    "tests/agent/sdk/python/test_native_program_call.py",
    "tests/agent/sdk/python/test_native_program_binding.py",
    "tests/agent/sdk/python/test_program_lifecycle.py",
    "tests/agent/sdk/python/test_program_executed_v3.py",
    "tests/agent/sdk/python/test_program_http.py",
]
EXEC = 'exec python3 - "$python_sdk_task_root" <<\'PY\'\n'


def script_parts():
    prefix, marker, body = SCRIPT.read_text().partition(EXEC)
    assert marker == EXEC
    assert body.endswith("\nPY\n")
    return prefix, ast.parse(body.removesuffix("PY\n"))


def clean_environment():
    return {"PATH": os.path.dirname(sys.executable) + os.pathsep + os.defpath}


def test_python_sdk_import_root_shell_contract():
    parsed = subprocess.run(
        ["bash", "-n", str(SCRIPT)], capture_output=True, text=True, timeout=10
    )
    assert parsed.returncode == 0, parsed.stderr
    prefix, tree = script_parts()
    export = next(line for line in prefix.splitlines() if line.startswith("export PYTHONPATH="))
    assert shlex.split(export) == [
        "export",
        "PYTHONPATH=$python_sdk_task_root/services/agent/sdk/python${PYTHONPATH:+:$PYTHONPATH}",
    ]
    assert "export PYTHONDONTWRITEBYTECODE=1" in prefix.splitlines()
    paths = next(
        node.value for node in tree.body
        if isinstance(node, ast.Assign)
        and any(isinstance(target, ast.Name) and target.id == "paths" for target in node.targets)
    )
    assert ast.literal_eval(paths) == CORPUS
    loop = next(node for node in tree.body if isinstance(node, ast.For))
    assert ast.unparse(loop.iter) == "enumerate(paths)"
    calls = [ast.unparse(node) for node in ast.walk(loop) if isinstance(node, ast.Call)]
    assert "spec.loader.exec_module(module)" in calls
    assert "suite.addTests(unittest.defaultTestLoader.loadTestsFromModule(module))" in calls
    guards = [node for node in tree.body if isinstance(node, ast.If)]
    assert ast.unparse(guards[0].test) == (
        "not os.environ.get('LAYERX_PYTHON_PROGRAM_HTTP_FIXTURE') or "
        "not os.environ.get('PAXEER_X_PROGRAM_TERMINAL_V5_CORPUS')"
    )
    assert ast.unparse(guards[1].test) == (
        "result.skipped or result.expectedFailures or result.unexpectedSuccesses"
    )
    assert ast.unparse(guards[1].body[0]) == "sys.exit(1)"
    assert ast.unparse(tree.body[-1]) == "sys.exit(0 if result.wasSuccessful() else 1)"


def test_python_sdk_import_root_preserves_prerequisite_refusal(tmp_path):
    for present in (None, "LAYERX_PYTHON_PROGRAM_HTTP_FIXTURE", "PAXEER_X_PROGRAM_TERMINAL_V5_CORPUS"):
        environment = clean_environment()
        if present:
            environment[present] = str(tmp_path / "unavailable")
        result = subprocess.run(
            ["sh", str(SCRIPT)], cwd=tmp_path, env=environment,
            capture_output=True, text=True, timeout=10,
        )
        assert result.returncode == 78, result.stderr
        refusal = json.loads(result.stdout)
        assert refusal["exit_code"] == 78
        assert refusal["status"] == "prerequisite-unavailable"
        assert "genuine signed native v5 corpus required" in refusal["reason"]


ISOLATED_PROBE = textwrap.dedent("""\
    import ast
    import importlib
    import io
    import json
    import os
    from pathlib import Path
    import sys
    import unittest

    root = Path(sys.argv[1])
    sdk = root / "services/agent/sdk/python"
    corpus = root / "tests/agent/sdk/python"
    assert sys.flags.no_site == 1
    assert "site" not in sys.modules
    assert not any("site-packages" in path or "dist-packages" in path for path in sys.path)
    assert os.environ["PYTHONDONTWRITEBYTECODE"] == "1"
    exported = os.environ["PYTHONPATH"].split(os.pathsep)
    assert Path(exported[0]) == sdk
    assert exported[1:] == [str(corpus)]
    assert Path(sys.path[1]) == sdk

    script = (root / "scripts/paxeer-x/verify-sdk-program-python.sh").read_text()
    body = script.split("<<'PY'\\n", 1)[1].removesuffix("PY\\n")
    tree = ast.parse(body)
    paths = next(ast.literal_eval(node.value) for node in tree.body
                 if isinstance(node, ast.Assign)
                 and any(isinstance(target, ast.Name) and target.id == "paths"
                         for target in node.targets))
    modules = []
    for relative in paths:
        source = root / relative
        module = importlib.import_module(source.stem)
        assert Path(module.__file__).resolve() == source
        modules.append(module)
    import layerx_sdk
    from layerx_sdk.agent_http import AgentHttpTransport
    from layerx_sdk.native_program_call import encode_native_program_call
    from layerx_sdk.production import ProductionClient
    from layerx_sdk.programs import ProgramOperations
    http = modules[-1]
    assert http.AgentHttpTransport is AgentHttpTransport
    assert http.ProductionClient is ProductionClient
    assert http.ProgramOperations is ProgramOperations
    assert http.encode_native_program_call is encode_native_program_call
    assert modules[0].encode_native_program_call is encode_native_program_call
    assert modules[1].encode_native_program_call is encode_native_program_call
    origins = {}
    for name, module in tuple(sys.modules.items()):
        if name == "layerx_sdk" or name.startswith("layerx_sdk."):
            origin = Path(module.__file__).resolve()
            assert origin.is_relative_to(sdk / "layerx_sdk"), (name, origin)
            origins[name] = str(origin)
    assert "layerx_sdk.agent_http" in origins
    assert "layerx_sdk.programs" in origins
    assert "layerx_sdk.program_wire" in origins

    # Execute the existing native payload and signed-corpus cases unchanged.
    suite = unittest.TestSuite(
        unittest.defaultTestLoader.loadTestsFromModule(module) for module in modules[:2]
    )
    result = unittest.TextTestRunner(stream=io.StringIO()).run(suite)
    assert result.testsRun == 2
    assert result.wasSuccessful(), (result.errors, result.failures)
    assert not result.skipped and not result.expectedFailures and not result.unexpectedSuccesses
    print(json.dumps({"modules": [module.__name__ for module in modules],
                      "tests_run": result.testsRun, "origins": origins}))
    """)


def test_python_sdk_import_root_isolated_http_and_corpus(tmp_path):
    prefix, _ = script_parts()
    environment = clean_environment()
    # Only committed corpus helpers are ambient; no installed SDK or pytest.
    environment["PYTHONPATH"] = str(ROOT / "tests/agent/sdk/python")
    command = prefix + '\nexec "$1" -S - "$2"\n'
    result = subprocess.run(
        ["sh", "-c", command, str(SCRIPT), sys.executable, str(ROOT)],
        input=ISOLATED_PROBE, cwd=tmp_path, env=environment,
        capture_output=True, text=True, timeout=30,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    proof = json.loads(result.stdout)
    assert proof["modules"] == [Path(path).stem for path in CORPUS]
    assert proof["tests_run"] == 2
    assert proof["origins"]["layerx_sdk"] == str(
        ROOT / "services/agent/sdk/python/layerx_sdk/__init__.py"
    )
