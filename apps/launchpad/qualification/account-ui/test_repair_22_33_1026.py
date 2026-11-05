"""Audit account-readme-path through real files, runner argv and loopback HTTP."""

import functools
import hashlib
import http.server
import json
import pathlib
import re
import runpy
import shlex
import threading
import urllib.request

import pytest


SOURCE = pathlib.Path(__file__).resolve().parent
ROOT = SOURCE.parents[3]
PREFIX = SOURCE.relative_to(ROOT).as_posix()
README = (SOURCE / 'README.md').read_text()


@pytest.fixture
def owned_http():
    requests = []

    class FileServer(http.server.SimpleHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.path, dict(self.headers.items())))
            super().do_GET()

        def log_message(self, format, *args):
            pass

    server = http.server.ThreadingHTTPServer(
        ('127.0.0.1', 0), functools.partial(FileServer, directory=str(ROOT))
    )
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f'http://127.0.0.1:{server.server_port}', requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()


def documented_commands():
    return [
        shlex.split(line)
        for block in re.findall(r'```sh\n(.*?)```', README, flags=re.S)
        for line in block.splitlines()
        if line.strip()
    ]


def test_account_readme_paths_over_real_http(owned_http, tmp_path):
    origin, requests = owned_http
    assert 'client/qualification' not in README
    assert 'client/apps/web' not in README
    for target in re.findall(r'\]\(([^)]+)\)', README):
        path = (SOURCE / target).resolve()
        assert path.is_relative_to(ROOT)
        with urllib.request.urlopen(origin + '/' + path.relative_to(ROOT).as_posix()) as response:
            assert response.status == 200
            assert response.read() == path.read_bytes()

    for argv in documented_commands():
        if argv[0] == 'python3':
            with urllib.request.urlopen(origin + '/' + argv[1]) as response:
                assert response.read() == (ROOT / argv[1]).read_bytes()

    phase4 = runpy.run_path(str(SOURCE / 'run-phase4.py'))
    typecheck = next(argv for argv in documented_commands() if argv[0].endswith('/tsc'))
    config_path = typecheck[typecheck.index('-p') + 1]
    client = phase4['ObservedHTTP'](tmp_path, timeout=5)
    config = client.get(origin + '/' + config_path, None, 'owned-typecheck-config')
    assert config == json.loads((ROOT / config_path).read_text())
    assert config['compilerOptions']['strict'] is True
    assert config['compilerOptions']['noEmit'] is True
    for included in config['include']:
        assert (SOURCE / included).is_file()
    sample, = client.samples
    requested_path, headers = requests[-1]
    assert requested_path == '/' + config_path
    assert headers['Accept'] == 'application/json'
    assert headers['X-Request-Id'] == sample['requestId']
    assert 'Authorization' not in headers
    assert re.fullmatch(r'[0-9a-f]{32}', sample['requestId'])
    captured = tmp_path / sample['bodyPath']
    assert captured.read_bytes() == (ROOT / config_path).read_bytes()
    assert captured.stat().st_mode & 0o777 == 0o600
    assert sample['status'] == 200
    assert sample['method'] == 'GET'
    assert sample['bytes'] == len(captured.read_bytes())
    assert sample['bodySha256'] == hashlib.sha256(captured.read_bytes()).hexdigest()


def test_account_readme_commands_match_owned_runners():
    assert (ROOT / 'apps/launchpad/web/package.json').is_file()
    commands = documented_commands()
    synthetic = next(argv for argv in commands if argv[0].endswith('/vitest'))
    assert synthetic == [
        'apps/launchpad/web/node_modules/.bin/vitest',
        'run', '--config', PREFIX + '/vitest.config.mts',
        PREFIX + '/disclosure.test.tsx', PREFIX + '/rewards-failure.test.tsx',
    ]
    for name in ('run-check.py', 'run-phase2.py'):
        runner = runpy.run_path(str(SOURCE / name))
        assert runner['root'] == ROOT
        assert callable(runner['main'])
        assert ['python3', PREFIX + '/' + name] in commands
        argv = runner['command']
        assert pathlib.Path(argv[0]) == ROOT / synthetic[0]
        assert argv[1:4] == synthetic[1:4]
        assert (ROOT / argv[3]).is_file()
        if name == 'run-check.py':
            assert len(argv) == 4
        else:
            assert argv[4:] == [PREFIX + '/account-journeys.test.tsx']
            assert (ROOT / argv[4]).is_file()
            assert 'L31_REAL_ACCESS_TOKEN' in runner['required']
            assert 'L31_REAL_EXPIRED_ACCESS_TOKEN' in runner['required']
    phase4 = runpy.run_path(str(SOURCE / 'run-phase4.py'))
    assert phase4['ROOT'] == ROOT
    assert phase4['SOURCE'] == SOURCE
    blocks = re.findall(r'```sh\n(.*?)```', README, flags=re.S)
    historical = next(block for block in blocks if '/run-check.py' in block)
    assert shlex.split(historical.splitlines()[0]) == [
        'mkdir', '-p', 'launchpad/evidence/production-system/L31/phase1'
    ]
    disclosure = next(block for block in blocks if '/vitest run' in block)
    assert shlex.split(disclosure.splitlines()[0]) == [
        'mkdir', '-p', 'launchpad/evidence/production-system/L31/phase1'
    ]


def test_account_disclosure_imports_resolve_to_exported_primitives(owned_http):
    origin, _ = owned_http
    with urllib.request.urlopen(origin + '/' + PREFIX + '/disclosure.test.tsx') as response:
        assert response.status == 200
        consumer = response.read().decode('utf-8')
    config = json.loads((SOURCE / 'tsconfig.json').read_text())
    included = {(SOURCE / path).resolve() for path in config['include']}
    imports = re.findall(
        r"import\s+\{([^}]+)\}\s+from\s+['\"](../../web/src/[^'\"]+)['\"]",
        consumer,
    )
    assert len(imports) == 3
    declared = set()
    for names, specifier in imports:
        candidates = [
            (SOURCE / specifier).with_suffix(suffix).resolve()
            for suffix in ('.ts', '.tsx')
        ]
        paths = [path for path in candidates if path.is_file()]
        assert len(paths) == 1, specifier
        path, = paths
        assert path in included
        with urllib.request.urlopen(origin + '/' + path.relative_to(ROOT).as_posix()) as response:
            assert response.status == 200
            producer = response.read().decode('utf-8')
            assert producer == path.read_text()
        for name in names.split(','):
            name = name.strip()
            assert re.search(
                r'export\s+(?:function|class|const)\s+' + re.escape(name) + r'\b',
                producer,
            )
            declared.add(name)
    assert {'portfolioLabels', 'sharePreview', 'exactAmount', 'rewardAmount'} <= declared


def test_account_readme_discovery_and_qualification_boundaries():
    config = (SOURCE / 'vitest.config.mts').read_text()
    assert "include: ['" + PREFIX + "/*.test.tsx']" in config
    assert {path.name for path in SOURCE.glob('*.test.tsx')} == {
        'disclosure.test.tsx', 'rewards-failure.test.tsx', 'account-journeys.test.tsx'
    }
    synthetic, service, staging = (
        README.split('## Disclosure primitives and typechecking')[1].split('## Real account service journeys')[0],
        README.split('## Real account service journeys')[1].split('## Staging browser and physical-device runner')[0],
        README.split('## Staging browser and physical-device runner')[1],
    )
    assert 'synthetic financial inputs' in synthetic
    assert 'no successful provider response' in synthetic
    assert '**every** `*.test.tsx`' in service
    assert 'including real-service journeys' in service
    assert 'native-exit.json' in service
    assert 'Node/Vitest, not a native device' in service
    assert 'Missing inputs fail explicitly' in service
    assert 'physical Android device' in staging
    assert 'fresh private output path outside the checkout' in staging
    assert '`qualificationCapacityClaim` false' in staging
    assert 'deferred to serial task 9.1' in staging
    assert 'do not run the service journeys, browser or physical-device qualification' in staging
