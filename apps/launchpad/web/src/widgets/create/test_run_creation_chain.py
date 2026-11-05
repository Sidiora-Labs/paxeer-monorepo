import importlib.util
from pathlib import Path
import sys

import pytest


spec = importlib.util.spec_from_file_location(
    'run_creation_chain', Path(__file__).with_name('run-creation-chain.py')
)
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


def test_anvil_binary_prefers_env(tmp_path):
    preferred = tmp_path / 'preferred-anvil'
    preferred.symlink_to(sys.executable)
    path_dir = tmp_path / 'bin'
    path_dir.mkdir()
    path_binary = path_dir / 'anvil'
    path_binary.symlink_to(sys.executable)
    env = {'ANVIL_BIN': str(preferred), 'PATH': str(path_dir)}

    assert runner.anvil_binary(env) == str(preferred)
    assert runner.anvil_binary({'PATH': str(path_dir)}) == str(path_binary)

    non_executable = tmp_path / 'non-executable'
    non_executable.touch(mode=0o600)
    for invalid in (tmp_path / 'missing', path_dir, non_executable):
        env['ANVIL_BIN'] = str(invalid)
        assert runner.anvil_binary(env) == str(path_binary)


def test_anvil_binary_fails_without_candidate(tmp_path):
    non_executable = tmp_path / 'anvil'
    non_executable.touch(mode=0o600)
    env = {'PATH': str(tmp_path)}

    candidates = (
        None, '', str(tmp_path / 'missing'), str(tmp_path), str(non_executable)
    )
    for candidate in candidates:
        if candidate is None:
            env.pop('ANVIL_BIN', None)
        else:
            env['ANVIL_BIN'] = candidate
        with pytest.raises(SystemExit) as error:
            runner.anvil_binary(env)
        assert error.value.code == 'anvil binary unavailable: set ANVIL_BIN or PATH'
