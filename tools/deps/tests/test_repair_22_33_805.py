"""Check the real checksum-test command paths without invoking a Go toolchain."""

import ast
from pathlib import Path

import pytest


TESTS = Path(__file__).resolve().parent


def _function(filename, name):
    tree = ast.parse((TESTS / filename).read_text())
    functions = [
        node
        for node in tree.body
        if isinstance(node, ast.FunctionDef) and node.name == name
    ]
    assert len(functions) == 1
    function = functions[0]
    assert not function.decorator_list
    return function


def _assignment(body, name):
    assignments = [
        node
        for node in body
        if isinstance(node, ast.Assign)
        and any(
            isinstance(target, ast.Name) and target.id == name
            for target in node.targets
        )
    ]
    assert len(assignments) == 1, name
    return assignments[0].value


def _assertions(body):
    return {ast.unparse(node.test) for node in body if isinstance(node, ast.Assert)}


def _command_loop(function):
    guarded = [node for node in function.body if isinstance(node, ast.Try)]
    assert len(guarded) == 1
    protected = guarded[0]
    assert not protected.handlers
    assert not protected.orelse
    assert len(protected.body) == 1
    loop = protected.body[0]
    assert isinstance(loop, ast.For)
    assert not loop.orelse
    return protected, loop


def _commands(loop, expected, timeout=None):
    commands = []
    for index, node in enumerate(loop.body):
        if not isinstance(node, ast.Assign) or not isinstance(node.value, ast.Call):
            continue
        call = node.value
        if ast.unparse(call.func) != "subprocess.run":
            continue
        assert len(call.args) == 1
        commands.append(tuple(ast.literal_eval(call.args[0])))
        keywords = {keyword.arg: keyword.value for keyword in call.keywords}
        assert ast.unparse(keywords.pop("cwd")) == "module"
        assert ast.unparse(keywords.pop("env")) == "env"
        options = {name: ast.literal_eval(value) for name, value in keywords.items()}
        expected_options = {"capture_output": True, "text": True, "check": False}
        if timeout is not None:
            expected_options["timeout"] = timeout
        assert options == expected_options
        assert len(node.targets) == 1
        result = node.targets[0].id
        assertion = loop.body[index + 1]
        assert isinstance(assertion, ast.Assert)
        assert ast.unparse(assertion.test) == f"{result}.returncode == 0"
        diagnostics = {
            ast.unparse(child)
            for child in ast.walk(assertion.msg)
            if isinstance(child, (ast.Attribute, ast.Name))
        }
        assert {"module", f"{result}.stdout", f"{result}.stderr"} <= diagnostics
    assert commands == expected


def test_finding_27_readonly_builds_all_four_nested_modules():
    function = _function("test_go_root_checksums.py", "test_go_root_nested_checksums")
    modules = _assignment(function.body, "modules")
    assert isinstance(modules, ast.Tuple)
    assert [ast.unparse(module) for module in modules.elts] == [
        "root / 'tools/paxctl'",
        "root / 'tools/loadtest'",
        "root / 'services/wallet/ceremony'",
        "root / 'sdk/cosmovisor'",
    ]
    environment = _assignment(function.body, "env")
    assert isinstance(environment, ast.Call)
    assert ast.unparse(environment.func) == "dict"
    assert [ast.unparse(arg) for arg in environment.args] == ["os.environ"]
    assert {
        keyword.arg: ast.literal_eval(keyword.value)
        for keyword in environment.keywords
    } == {"GOWORK": "off", "GOFLAGS": "-mod=readonly", "GOTOOLCHAIN": "auto"}
    _, loop = _command_loop(function)
    assert ast.unparse(loop.target) == "module"
    assert ast.unparse(loop.iter) == "modules"
    _commands(
        loop,
        [
            ("go", "list", "-mod=readonly", "-deps", "-test", "./..."),
            ("go", "mod", "verify"),
            ("go", "build", "-mod=readonly", "./..."),
        ],
    )
    assert "packages.stdout.strip()" in _assertions(loop.body)


def test_finding_28_readonly_builds_both_database_modules_and_keeps_imports():
    function = _function("test_vendored_checksums.py", "test_vendored_database_checksums")
    modules = _assignment(function.body, "modules")
    assert isinstance(modules, ast.Dict)
    assert {
        ast.unparse(key): ast.literal_eval(value)
        for key, value in zip(modules.keys, modules.values)
    } == {
        "ROOT / 'core/deps/db-backend'": {
            "google.golang.org/grpc",
            "google.golang.org/grpc/credentials",
        },
        "ROOT / 'core/deps/tm-db'": {
            "github.com/syndtr/goleveldb/leveldb",
            "github.com/tendermint/tm-db",
        },
    }
    environment = _assignment(function.body, "env")
    assert isinstance(environment, ast.Dict)
    assert environment.keys[0] is None
    assert ast.unparse(environment.values[0]) == "os.environ"
    assert {
        ast.literal_eval(key): ast.literal_eval(value)
        for key, value in zip(environment.keys[1:], environment.values[1:])
    } == {"GOWORK": "off", "GOFLAGS": "-mod=readonly"}
    _, loop = _command_loop(function)
    assert ast.unparse(loop.target) == "(module, required_imports)"
    assert ast.unparse(loop.iter) == "modules.items()"
    _commands(
        loop,
        [
            ("go", "mod", "verify"),
            ("go", "list", "-mod=readonly", "-deps", "./..."),
            ("go", "build", "-mod=readonly", "./..."),
        ],
        timeout=300,
    )
    assert ast.unparse(_assignment(loop.body, "packages")) == (
        "set(dependencies.stdout.splitlines())"
    )
    assert {"packages", "required_imports <= packages"} <= _assertions(loop.body)


@pytest.mark.parametrize(
    "filename,name,snapshots,original",
    [
        (
            "test_go_root_checksums.py",
            "test_go_root_nested_checksums",
            "snapshots",
            "snapshot",
        ),
        (
            "test_vendored_checksums.py",
            "test_vendored_database_checksums",
            "before",
            "original",
        ),
    ],
)
def test_findings_27_28_keep_checksum_and_exact_manifest_guards(
    filename, name, snapshots, original
):
    function = _function(filename, name)
    assert ast.literal_eval(_assignment(function.body, snapshots)) == {}
    snapshot_loops = [node for node in function.body if isinstance(node, ast.For)]
    assert len(snapshot_loops) == 1
    loop = snapshot_loops[0]
    assert ast.unparse(loop.target) == "module"
    assert ast.unparse(loop.iter) == "modules"
    assert ast.unparse(_assignment(loop.body, "checksum")) == "module / 'go.sum'"
    assert {"checksum.is_file()", "checksum.read_bytes().strip()"} <= _assertions(
        loop.body
    )
    files = [node for node in loop.body if isinstance(node, ast.For)]
    assert len(files) == 1
    assert ast.literal_eval(files[0].iter) == ("go.mod", "go.sum")
    assert ast.unparse(files[0].target) == "name"
    assert ast.unparse(_assignment(files[0].body, "path")) == "module / name"
    assert any(
        isinstance(node, ast.Assign)
        and [ast.unparse(target) for target in node.targets] == [f"{snapshots}[path]"]
        and ast.unparse(node.value) == "path.read_bytes()"
        for node in files[0].body
    )
    guarded, _ = _command_loop(function)
    assert function.body.index(loop) < function.body.index(guarded)
    assert len(guarded.finalbody) == 1
    final = guarded.finalbody[0]
    assert isinstance(final, ast.For)
    assert ast.unparse(final.iter) == f"{snapshots}.items()"
    assert ast.unparse(final.target) == f"(path, {original})"
    assert f"path.read_bytes() == {original}" in _assertions(final.body)
