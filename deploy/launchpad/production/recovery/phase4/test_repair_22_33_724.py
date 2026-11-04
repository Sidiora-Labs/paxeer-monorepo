"""Exercise the documented recovery commands and their real source bindings."""

import asyncio
from contextlib import contextmanager
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import json
from pathlib import Path
import re
import shlex
import sys
from threading import Thread
from urllib.parse import parse_qs, urlencode, urlsplit

import pytest


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[4]
RELEASE_PATH = 'deploy/launchpad/production/recovery/phase4/release.py'


def documented_commands():
    text = (HERE / 'runbook.md').read_text()
    return [shlex.split(block.replace('\\\n', ''))
            for block in re.findall(r'```sh\n(.*?)```', text, re.DOTALL)]


@pytest.fixture(scope='module')
def release_module():
    spec = importlib.util.spec_from_file_location('repair_22_33_724_release', ROOT / RELEASE_PATH)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


@contextmanager
def cli_arguments(arguments):
    previous = sys.argv
    sys.argv = arguments
    try:
        yield
    finally:
        sys.argv = previous


def test_recovery_release_command_paths(release_module):
    commands = documented_commands()
    assert commands == [
        ['python3', RELEASE_PATH, '--mode', 'collect', '--configuration', '<private-config>',
         '--configuration-sha256', '<admitted-sha>', '--output-directory', '<fresh-owned-directory>'],
        ['python3', RELEASE_PATH, '--mode', 'evaluate', '--configuration', '<private-evaluation-config>',
         '--configuration-sha256', '<admitted-evaluation-sha>', '--output-directory', '<fresh-owned-directory>'],
        ['python3', '-m', 'unittest', 'discover', '-s',
         'deploy/launchpad/production/recovery/phase4', '-p', 'test_release.py'],
    ]
    assert (ROOT / commands[0][1]).is_file()
    assert (ROOT / commands[2][5] / commands[2][7]).is_file()
    assert release_module.ROOT == ROOT
    assert release_module.HTTP.__module__ == 'phase2_http'
    for name in ('phase2_http', 'phase2_types', 'capture'):
        assert Path(sys.modules[name].__file__).resolve().parent == ROOT / 'tools/launchpad/production/load'
    assert Path(sys.modules['providers'].__file__).resolve().parent == ROOT / 'deploy/launchpad/production/recovery/phase2'
    text = (HERE / 'runbook.md').read_text()
    assert 'deploy/production/' not in text
    assert 'tools/production/' not in text
    for path in ('tools/launchpad/production/qualify.py', 'tools/launchpad/production/load/'):
        assert path in text
        assert (ROOT / path).exists()
    for target in re.findall(r'\]\(([^)]+)\)', text):
        assert (HERE / target).resolve().exists(), target


@pytest.mark.parametrize('command_index', [0, 1], ids=['collect', 'evaluate'])
@pytest.mark.parametrize('invalid_input', ['missing', 'changed', 'public'])
def test_documented_cli_retains_failed_input_evidence(
        release_module, tmp_path, capsys, command_index, invalid_input):
    command = documented_commands()[command_index]
    configuration = tmp_path / 'configuration.json'
    raw = b'{"version":4}\n'
    digest = hashlib.sha256(raw).hexdigest()
    if invalid_input != 'missing':
        configuration.write_bytes(raw if invalid_input == 'public' else b'{"version":3}\n')
        configuration.chmod(0o644 if invalid_input == 'public' else 0o600)
    output = tmp_path / 'evidence'
    substitutions = {
        '<private-config>': str(configuration),
        '<private-evaluation-config>': str(configuration),
        '<admitted-sha>': digest,
        '<admitted-evaluation-sha>': digest,
        '<fresh-owned-directory>': str(output),
    }
    arguments = [substitutions.get(token, token) for token in command[1:]]
    if invalid_input == 'changed':
        with pytest.raises(release_module.recovery.Invalid, match='^changed release input$'):
            release_module.bound({'path': str(configuration), 'sha256': digest})
    with cli_arguments(arguments):
        assert release_module.main() == 1
    summary = json.loads(capsys.readouterr().out)
    assert summary == {'outcome': 'fail', 'result_path': str(output / 'release-result.json'),
                       'capacity_qualified': False}
    report = json.loads((output / 'release-result.json').read_bytes())
    assert report['outcome'] == 'fail'
    expected_limitation = {'missing': 'FileNotFoundError', 'changed': 'Invalid', 'public': 'ValueError'}
    assert report['limitation'] == expected_limitation[invalid_input]
    assert {row['requirement'] for row in report['results']} == set(release_module.recovery.METRICS)
    assert len(report['results']) == 10
    assert all(row['outcome'] == 'fail' and len(row['criteria']) == 5 for row in report['results'])
    assert report['capacity_qualified'] is False
    assert report['monthly_availability_qualified'] is False
    assert report['final_shared_gate'] == '9.1'
    inventory = json.loads((output / 'artifact-sha256.json').read_bytes())
    assert inventory == {'release-result.json': hashlib.sha256((output / 'release-result.json').read_bytes()).hexdigest()}
    assert output.stat().st_mode & 0o777 == 0o700
    for path in output.iterdir():
        assert path.stat().st_mode & 0o777 == 0o600


@pytest.mark.parametrize('option', [
    '--mode', '--configuration', '--configuration-sha256', '--output-directory',
])
def test_documented_cli_requires_every_authority_argument(release_module, capsys, option):
    arguments = documented_commands()[0][1:]
    index = arguments.index(option)
    del arguments[index:index + 2]
    with cli_arguments(arguments), pytest.raises(SystemExit) as error:
        release_module.main()
    assert error.value.code == 2
    assert option in capsys.readouterr().err


def test_canonical_transport_reads_native_metrics(release_module, tmp_path):
    requests = []
    application_origin = 'https://recovery.example.invalid'
    metric = 'recovery_wal_age_seconds'
    labels = {'service': 'recovery', 'environment': 'qualification', 'zone': 'region-recovery'}
    selector = metric + '{' + ','.join(key + '="' + value + '"' for key, value in sorted(labels.items())) + '}'
    response = {'status': 'success', 'data': {'resultType': 'vector', 'result': [
        {'metric': {'__name__': metric, **labels}, 'value': [1000, '1']}]}}

    class NativeMetrics(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.path, dict(self.headers)))
            raw = release_module.recovery.canonical(response)
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(raw)))
            self.end_headers()
            self.wfile.write(raw)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(('127.0.0.1', 0), NativeMetrics)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        origin = 'http://127.0.0.1:' + str(server.server_port)
        with pytest.raises(ValueError):
            release_module.HTTP(origin, application_origin, owned_local=False)
        client = release_module.HTTP(origin, application_origin, owned_local=True)
        result = asyncio.run(client.call('GET', '/api/v1/query?' + urlencode({'query': selector})))
        assert result == response
        assert len(requests) == 1
        path, headers = requests[0]
        assert urlsplit(path).path == '/api/v1/query'
        assert parse_qs(urlsplit(path).query) == {'query': [selector]}
        assert headers['Origin'] == application_origin
        assert headers['Accept'] == 'application/json'
        assert 'Authorization' not in headers
        journal = release_module.Journal(tmp_path / 'native-evidence')
        binding = journal.write('native-response.json', result)
        assert release_module.bound(binding) == response
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
    assert not thread.is_alive()
