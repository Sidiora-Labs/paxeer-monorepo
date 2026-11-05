from pathlib import Path


def test_kernel_node_drivers_are_absent():
    source = Path(__file__).resolve()
    tests = source.parent
    retired = (
        'reset_recovery.py',
        'node-test.sh',
        'bootstrap-test.py',
        'node-artifacts.py',
    )
    remaining = [
        name for name in retired
        if (tests / name).exists() or (tests / name).is_symlink()
    ]
    assert not remaining, f'Retired kernel node drivers remain: {remaining}'

    for launcher in ('bootstrap.sh', 'supervisor.sh'):
        assert (tests.parent / launcher).is_file(), f'Node launcher missing: {launcher}'

    forbidden = tuple(name.encode() for name in retired) + (
        b'layerx-custody-proof',
        b'tests/bridge',
    )
    references = []
    for path in sorted(tests.rglob('*')):
        relative = path.relative_to(tests)
        if source == path or not path.is_file():
            continue
        if {'__pycache__', '.pytest_cache', 'target'}.intersection(relative.parts):
            continue
        for number, line in enumerate(path.read_bytes().splitlines(), start=1):
            for name in forbidden:
                if name in line:
                    references.append(f'{relative}:{number}: {name.decode()}')
    assert not references, 'Retired kernel dependencies remain:\n' + '\n'.join(references)
