"""Joined service builds use standalone modules and report safe diagnostics."""

import ast
import importlib.util
import os
from pathlib import Path
import sys

import pytest


ROOT = Path(__file__).resolve().parents[2]
PREPARE = ROOT / 'apps/launchpad/web/scripts/prepare-test-environment.py'
JOINED = ROOT / 'apps/launchpad/web/src/widgets/trading/run-joined-services.py'


def environment(output):
    spec = importlib.util.spec_from_file_location('prepare_test_environment', PREPARE)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module.Environment(output, dict(os.environ))


def test_run_reports_status_and_log_lines_without_urls(tmp_path):
    owner = environment(tmp_path)
    command = [sys.executable, '-c',
               "import sys; print('first diagnostic postgresql://'); "
               "print('second diagnostic redis://', file=sys.stderr); sys.exit(3)"]

    with pytest.raises(RuntimeError) as raised:
        owner.run(command, 'joined-build', cwd=tmp_path)

    message = str(raised.value)
    assert 'joined-build: exit 3;' in message
    assert str(tmp_path / 'joined-build.log') in message
    assert 'first diagnostic URL' in message
    assert 'second diagnostic URL' in message
    assert '://' not in message
    log = (tmp_path / 'joined-build.log').read_text()
    assert 'first diagnostic postgresql://' in log
    assert 'second diagnostic redis://' in log


def test_run_reports_only_the_last_40_log_lines(tmp_path):
    owner = environment(tmp_path)
    command = [sys.executable, '-c',
               "import sys; [print(f'diagnostic {i}') for i in range(45)]; sys.exit(3)"]

    with pytest.raises(RuntimeError) as raised:
        owner.run(command, 'joined-build', cwd=tmp_path)

    assert str(raised.value).splitlines()[1:] == [
        f'diagnostic {i}' for i in range(5, 45)
    ]


def test_go_builds_use_the_workspace_disabled_helper():
    tree = ast.parse(JOINED.read_text())
    parents = {child: node for node in ast.walk(tree)
               for child in ast.iter_child_nodes(node)}
    helper = next(node for node in tree.body
                  if isinstance(node, ast.FunctionDef) and node.name == 'build_go')
    builds = [node for node in ast.walk(tree)
              if isinstance(node, ast.Call)
              and any(isinstance(arg, (ast.List, ast.Tuple))
                      and len(arg.elts) >= 2
                      and all(isinstance(value, ast.Constant)
                              for value in arg.elts[:2])
                      and [value.value for value in arg.elts[:2]] == ['go', 'build']
                      for arg in node.args)]
    assert builds, 'The joined fixture must build its Go services'
    for build in builds:
        enclosing = parents[build]
        while not isinstance(enclosing, (ast.FunctionDef, ast.AsyncFunctionDef, ast.Module)):
            enclosing = parents[enclosing]
        assert enclosing is helper, 'Go builds must use build_go'
        assert isinstance(build.func, ast.Attribute)
        assert isinstance(build.func.value, ast.Name)
        assert (build.func.value.id, build.func.attr) == ('owner', 'run')

    assignments = [node for node in helper.body
                   if isinstance(node, ast.Assign)
                   and any(isinstance(target, ast.Attribute)
                           and isinstance(target.value, ast.Name)
                           and (target.value.id, target.attr) == ('owner', 'env')
                           for target in node.targets)]
    assert len(assignments) == 1
    build_env = assignments[0].value
    assert isinstance(build_env, ast.Dict)
    assert any(isinstance(key, ast.Constant) and key.value == 'GOWORK'
               and isinstance(value, ast.Constant) and value.value == 'off'
               for key, value in zip(build_env.keys, build_env.values))

    calls = [node for node in ast.walk(tree)
             if isinstance(node, ast.Call) and isinstance(node.func, ast.Name)
             and node.func.id == helper.name]
    assert len(calls) == 2
    assert any(isinstance(arg, ast.Constant) and arg.value == 'joined-build-stats-migrate'
               for call in calls for arg in call.args)
