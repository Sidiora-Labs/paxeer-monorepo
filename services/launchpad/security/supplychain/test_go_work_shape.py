import re
import shlex
from pathlib import Path


LAUNCHPAD = Path(__file__).resolve().parents[2]
WORKSPACE = LAUNCHPAD / 'go.work'


def _workspace_modules():
    workspace = WORKSPACE.read_text()
    use_match = re.search(r'^use\s*\(\s*\n(.*?)^\)', workspace, re.MULTILINE | re.DOTALL)
    assert use_match is not None, 'go.work must declare a use block'
    modules = [
        (LAUNCHPAD / entry).resolve()
        for line in use_match.group(1).splitlines()
        for entry in shlex.split(line.split('//', 1)[0])
    ]
    assert modules, 'go.work must declare workspace modules'
    assert len(modules) == len(set(modules)), 'go.work must not repeat use entries'
    return set(modules)


def _go_directive(path):
    directives = re.findall(
        r'^go[ \t]+(\d+\.\d+(?:\.\d+)?)[ \t]*(?://[^\n]*)?$',
        path.read_text(),
        re.MULTILINE,
    )
    assert len(directives) == 1, f'{path} must declare exactly one go directive'
    return directives[0]


def test_go_work_uses_every_launchpad_module():
    workspace_modules = _workspace_modules()
    launchpad_modules = {
        module.parent.resolve()
        for module in LAUNCHPAD.rglob('go.mod')
        if module.is_file()
    }
    missing = launchpad_modules - workspace_modules
    assert not missing, f'Launchpad modules missing from go.work: {sorted(missing)}'
    for module in sorted(workspace_modules):
        assert (module / 'go.mod').is_file(), f'go.work use entry lacks go.mod: {module}'


def test_go_directives_match_workspace():
    workspace_go = _go_directive(WORKSPACE)
    assert workspace_go == '1.25.0', 'go.work must pin go 1.25.0'
    for module in sorted(_workspace_modules()):
        module_go = _go_directive(module / 'go.mod')
        assert module_go == workspace_go, (
            f'{module / "go.mod"} declares go {module_go}; '
            f'go.work declares go {workspace_go}'
        )
