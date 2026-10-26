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
