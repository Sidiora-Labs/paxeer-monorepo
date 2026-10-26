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
