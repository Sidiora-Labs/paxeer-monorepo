import re
import shlex
import subprocess
from pathlib import Path


def test_makefile_modules_match_go_work():
    root = Path(__file__).resolve().parents[4]
    result = subprocess.run(
        ['make', '-C', 'services/launchpad', '-pn'],
        cwd=root,
        check=True,
        capture_output=True,
        text=True,
    )
    modules_match = re.search(r'^MODULES := (.+)$', result.stdout, re.MULTILINE)
    assert modules_match is not None, 'Make database must declare expanded MODULES'
    modules = shlex.split(modules_match.group(1))

    workspace = (root / 'services/launchpad/go.work').read_text()
    use_match = re.search(r'^use\s*\(\s*\n(.*?)^\)', workspace, re.MULTILINE | re.DOTALL)
    assert use_match is not None, 'go.work must declare a use block'
    workspace_modules = {
        Path(entry).as_posix()
        for line in use_match.group(1).splitlines()
        for entry in shlex.split(line.split('//', 1)[0])
    }
    excluded = {
        '../../deploy/launchpad/production/recovery/phase2/broker',
        '../../tools/launchpad/production/load/eventdriver',
    }

    assert len(modules) == len(set(modules)), 'MODULES must not contain duplicates'
    assert set(modules) == workspace_modules - excluded


def test_fmt_target_does_not_mask_goimports_failure():
    root = Path(__file__).resolve().parents[4]
    result = subprocess.run(
        ['make', '-C', 'services/launchpad', '-n', 'fmt'],
        cwd=root,
        check=True,
        capture_output=True,
        text=True,
    )
    recipe_lines = result.stdout.splitlines()
    assert all('|| true' not in line for line in recipe_lines)
    goimports_lines = [line.strip() for line in recipe_lines if 'goimports -w' in line]
    assert len(goimports_lines) == 1, 'fmt must invoke goimports once'
    assert re.fullmatch(
        r'if command -v goimports >/dev/null; then goimports -w .+; fi',
        goimports_lines[0],
    ), 'goimports must run inside the command -v guard'


def test_test_module_target_rejects_unknown_module():
    root = Path(__file__).resolve().parents[4]
    for module in ['nope', '', 'discovery api', '%']:
        result = subprocess.run(
            ['make', '-C', 'services/launchpad', 'test-module', f'MODULE={module}', 'RUN=^TestX$'],
            cwd=root,
            check=False,
            capture_output=True,
            text=True,
        )
        assert result.returncode != 0, f'test-module must reject MODULE={module!r}'
        assert 'MODULE must be one of ' in result.stdout + result.stderr


def test_test_module_target_runs_only_the_named_module():
    root = Path(__file__).resolve().parents[4]
    launchpad = root / 'services/launchpad'
    makefile = (launchpad / 'Makefile').read_text()
    phony_match = re.search(r'^\.PHONY:\s*(.+)$', makefile, re.MULTILINE)
    assert phony_match is not None
    assert 'test-module' in phony_match.group(1).split()
    assert (
        'test-module: ## go test -count=1 one module with an anchored -run '
        '(MODULE=<dir> RUN=<regex>)'
    ) in makefile

    for run in ['', 'TestX', '^TestX', 'TestX$']:
        result = subprocess.run(
            ['make', 'test-module', 'MODULE=discovery', f'RUN={run}'],
            cwd=launchpad,
            check=False,
            capture_output=True,
            text=True,
        )
        assert result.returncode != 0, f'test-module must reject RUN={run!r}'
        assert 'RUN must be an anchored regex' in result.stdout + result.stderr

    result = subprocess.run(
        ['make', '-n', 'test-module', 'MODULE=discovery', 'RUN=^TestX$'],
        cwd=launchpad,
        check=True,
        capture_output=True,
        text=True,
    )
    test_lines = [line.strip() for line in result.stdout.splitlines() if 'go test' in line]
    assert len(test_lines) == 1, 'test-module must invoke go test exactly once'
    assert "-run '^TestX$'" in test_lines[0]
    assert shlex.split(test_lines[0]) == [
        'cd', 'discovery', '&&', 'go', 'test', '-count=1', './...', '-run', '^TestX$',
    ]
