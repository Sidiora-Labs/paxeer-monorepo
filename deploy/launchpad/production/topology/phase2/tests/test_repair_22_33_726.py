from __future__ import annotations

import ast
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from importlib import import_module
from pathlib import Path
import re
import shlex
import sys
import threading

import pytest

TOPOLOGY = Path(__file__).resolve().parents[2]
SOURCE_ROOT = TOPOLOGY.parents[3]
RUNBOOK = TOPOLOGY / 'phase2/operator-runbook.md'
sys.path.insert(0, str(TOPOLOGY))

from phase2 import v1
from phase2.capacity import LOAD_FILES, load_capture_adapter
from phase2.operator import Journal
from phase2.providers import connected_plan, validate_connected_plan
from phase2.tests.test_repair_22_31_313 import actual_adapter, connected_inputs


def documented_commands():
    return [shlex.split(block.replace('\\\n', ' '))
            for block in re.findall(r'```sh\n(.*?)\n```', RUNBOOK.read_text(), re.S)]


def cli_declarations(module):
    main = next(node for node in ast.parse(Path(module.__file__).read_text()).body
                if isinstance(node, ast.FunctionDef) and node.name == 'main')
    return {
        ast.literal_eval(node.args[0]): {
            keyword.arg: ast.literal_eval(keyword.value) for keyword in node.keywords
            if keyword.arg in ('choices', 'required', 'default')}
        for node in ast.walk(main)
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute)
        and node.func.attr == 'add_argument'
    }


def test_topology_operator_command_paths_match_cli_declarations():
    commands = documented_commands()
    common = ['--inputs', '/reviewed/capacity-inputs.json', '--source-root', '$PWD']
    prefix = ['PYTHONPATH=deploy/launchpad/production/topology', 'python3', '-m']
    assert commands == [
        prefix + ['phase2.operator', 'ACTION', '--inputs', '/reviewed/operator-inputs.json',
                  '--source-root', '$PWD', '--journal', '/durable/topology/operator.db',
                  '--actor', 'OPERATOR', '--output', '/evidence/result.json'],
        prefix + ['phase2.capacity', 'capture'] + common
        + ['--output', '/durable/new-topology-capture'],
        prefix + ['phase2.capacity', 'check'] + common
        + ['--measurements', '/reviewed/signed-phase4-references.json',
           '--output', '/durable/new-result.json'],
    ]
    text = RUNBOOK.read_text()
    assert 'deploy/production/' not in text
    assert 'tools/production/' not in text
    actions = dict(re.findall(r'^\| `([^`]+)` \| (.*?) \|$', text, re.M))
    operator = cli_declarations(import_module('phase2.operator'))
    assert set(actions) == set(operator['action']['choices'])
    assert operator['--phase'] == {'choices': (2, 3), 'default': 2}
    assert {name for name, values in operator.items() if values.get('required')} == {
        '--inputs', '--source-root', '--journal', '--actor', '--output'}
    for action, flags in {
        'validate': ['--evidence'],
        'activate': ['--evidence', '--authorization'],
        'reject': ['--authorization'],
        'rehearse': ['--phase 3', '--rehearsal', '--authorization'],
        'recover-rehearsal': ['--phase 3', '--rehearsal', '--authorization'],
    }.items():
        for flag in flags:
            assert flag in actions[action]
            assert flag.split()[0] in operator
    capacity = cli_declarations(import_module('phase2.capacity'))
    assert capacity['action']['choices'] == ('capture', 'check')
    for command in commands:
        module = import_module(command[3])
        assert Path(module.__file__).resolve() == (
            SOURCE_ROOT / command[0].split('=', 1)[1]
            / (command[3].replace('.', '/') + '.py'))
        declarations = cli_declarations(module)
        flags = set(command[5::2])
        assert flags <= set(declarations)
        assert {name for name, values in declarations.items()
                if values.get('required')} <= flags


def test_topology_operator_command_paths_real_cli_transport_and_journal(connected_inputs, tmp_path):
    config, source_root, adapter_sha, documents, chain, chain_sha = connected_inputs
    assert source_root == SOURCE_ROOT
    text = RUNBOOK.read_text()
    adapter_path, = re.findall(r'`<source-root>/([^`]+)`', text)
    assert adapter_path == 'tools/launchpad/production/integration/adapters.py'
    assert hashlib.sha256((source_root / adapter_path).read_bytes()).hexdigest() == adapter_sha
    assert 'do not substitute synthetic documents or bypass the digest check' in text
    inputs = tmp_path / 'operator-inputs.json'
    inputs.write_bytes(v1.canonical({
        'schemaVersion': 2, 'topology': config, 'adapterSHA256': adapter_sha,
        'documents': documents, 'chainFile': str(chain), 'chainSHA256': chain_sha,
        'expectedSourceDigest': config['release']['sourceDigest'],
        'expectedInterfaceDigest': config['release']['interfaceDigest'],
        'kubectl': {}, 'openssl': {}, 'trust': {},
    }))
    journal_path = tmp_path / 'operator.db'
    output = tmp_path / 'result.json'
    command = documented_commands()[0]
    replacements = {
        '/reviewed/operator-inputs.json': str(inputs), '$PWD': str(source_root),
        '/durable/topology/operator.db': str(journal_path), 'OPERATOR': 'reviewed-operator',
        '/evidence/result.json': str(output),
    }
    args = [replacements.get(token, token) for token in command[4:]]
    cli = import_module(command[3])
    assert cli.main(['render', *args[1:]]) == 0
    rendered = v1.load(output)
    assert rendered == connected_plan(*connected_inputs)
    assert validate_connected_plan(rendered, config) is True
    payload = output.read_bytes()
    requests = []

    class RenderedPlan(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append((self.command, self.path, self.headers['Content-Type']))
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(('127.0.0.1', 0), RenderedPlan)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        port = server.server_address[1]
        client = actual_adapter().LocalHTTP('http://127.0.0.1:' + str(port), {port})
        status, received = client.request('GET', '/topology/connected')
        assert status == 200
        assert received == payload
        consumed = v1.decode(received)
        assert consumed == rendered
        assert validate_connected_plan(consumed, config) is True
        assert requests == [('GET', '/topology/connected', 'application/json')]
        assert cli.main(['propose', *args[1:]]) == 0
        proposal = v1.load(output)
        assert proposal['state'] == 'proposed'
        assert proposal['actor'] == 'reviewed-operator'
        reopened = Journal(journal_path)
        try:
            recovered = reopened.proposal(config['revision'])
            assert v1.decode(recovered['plan'].encode()) == consumed
            assert recovered['state'] == 'proposed'
            assert recovered['actor'] == 'reviewed-operator'
        finally:
            reopened.close()
    finally:
        server.shutdown()
        thread.join(timeout=5)
        server.server_close()
    assert not thread.is_alive()
    wrong_sha = ('0' if adapter_sha[0] != '0' else '1') + adapter_sha[1:]
    with pytest.raises(v1.Invalid, match='integrated runtime adapter digest mismatch'):
        connected_plan(config, source_root, wrong_sha, documents, chain, chain_sha)


def test_topology_operator_command_paths_capacity_sources_keep_digest_checks():
    text = RUNBOOK.read_text()
    load_path, = set(re.findall(r'`(tools/[^`]+/load)`', text))
    assert load_path == 'tools/launchpad/production/load'
    root = SOURCE_ROOT / load_path
    for filename in LOAD_FILES:
        assert '`' + filename + '`' in text
    references = {filename: hashlib.sha256((root / filename).read_bytes()).hexdigest()
                  for filename in LOAD_FILES}
    adapter = load_capture_adapter(SOURCE_ROOT, references)
    assert Path(adapter.__file__).resolve() == root / 'capture.py'
    assert adapter.SCHEMA_PATH == root / 'final-shared-capture.schema.json'
    for name in ('phase2_types', 'phase2_http', 'workload'):
        assert Path(sys.modules[name].__file__).resolve() == root / (name + '.py')
    changed = dict(references)
    observed = changed['capture.py']
    changed['capture.py'] = ('0' if observed[0] != '0' else '1') + observed[1:]
    with pytest.raises(v1.Invalid, match='D16 capture source changed: capture.py'):
        load_capture_adapter(SOURCE_ROOT, changed)
