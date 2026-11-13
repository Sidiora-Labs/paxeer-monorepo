"""Exercise the catalog's filesystem and JSON interfaces without native toolchains."""
import importlib.util
import json
from pathlib import Path
import re

import pytest


SPEC = importlib.util.spec_from_file_location(
    'repair_catalog_22_33_940', Path(__file__).with_name('catalog.py'))
catalog = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(catalog)

OMITTED = (
    'services/launchpad/discovery',
    'services/launchpad/notifications',
    'services/launchpad/security',
    'services/launchpad/security/control',
)


@pytest.fixture
def workspace(tmp_path):
    target = tmp_path / catalog.WORKSPACE
    target.parent.mkdir(parents=True)
    target.write_text((catalog.ROOT / catalog.WORKSPACE).read_text())
    for name in catalog.MODULES:
        directory = tmp_path / name
        directory.mkdir(parents=True, exist_ok=True)
        module = 'example.test/' + name
        (directory / 'go.mod').write_text('module ' + module + '\n\ngo 1.25.0\n')
        (directory / 'boundary_test.go').write_text(
            'package boundary\nimport "testing"\n'
            'func TestBoundary(t *testing.T) {\n'
            '    if 2 + 3 != 5 { t.Fatal("sum") }\n}\n')
    web = tmp_path / 'apps/launchpad/web/src/boundary.test.ts'
    web.parent.mkdir(parents=True)
    web.write_text("test('boundary', () => { expect(2 + 3).toBe(5); });\n")
    return tmp_path


def invoke(capsys, *args):
    code = catalog.main(list(args))
    report = json.loads(capsys.readouterr().out)
    return code, report


def go_report(root, module, events=None):
    package = 'example.test/' + module
    if events is None:
        events = [
            {'Action': 'run', 'Package': package, 'Test': 'TestBoundary'},
            {'Action': 'pass', 'Package': package, 'Test': 'TestBoundary'},
            {'Action': 'pass', 'Package': package},
        ]
    path = root / 'go-test.jsonl'
    path.write_text(''.join(json.dumps(event) + '\n' for event in events))
    return path


def test_workspace_membership_and_regenerated_catalog():
    workspace_path = catalog.ROOT / catalog.WORKSPACE
    block = re.search(r'^use\s*\((.*?)\)', workspace_path.read_text(), re.M | re.S)
    assert block is not None
    canonical = {
        (workspace_path.parent / path.strip()).resolve().relative_to(catalog.ROOT).as_posix()
        for path in block.group(1).splitlines() if path.strip()
    }
    production = {name for name in canonical if name.startswith(
        ('services/launchpad/', 'sdk/launchpad/'))}
    assert len(production) == 17
    assert set(catalog.MODULES) == production
    assert len(catalog.MODULES) == 17
    assert set(OMITTED) <= production
    assert canonical - production == {
        'deploy/launchpad/production/recovery/phase2/broker',
        'tools/launchpad/production/load/eventdriver',
    }
    current = catalog.inventory(catalog.ROOT)
    assert current['issues'] == []
    assert current['workspace']['modules'] == sorted(production)
    saved = json.loads(Path(__file__).with_name('source-catalog.json').read_text())
    assert saved == dict(current, status='inventoried', mode='inventory')
    assert saved['evidence_level'] == 'source_inventory_only'
    for module in saved['go_modules']:
        directory = catalog.ROOT / module['path']
        for source in module['files']:
            path = catalog.ROOT / source['path']
            owner = next(parent for parent in path.parents if (parent / 'go.mod').is_file())
            assert owner == directory
    for name in OMITTED:
        module = next(row for row in saved['go_modules'] if row['path'] == name)
        assert module['manifest_exists']
        assert module['declared_test_count'] > 0


def test_inventory_cli_round_trip(workspace, capsys):
    code, report = invoke(capsys, 'inventory', '--root', str(workspace))
    assert code == 0
    assert report['status'] == 'inventoried'
    assert report['issues'] == []
    assert report['testing_gaps'] == []
    assert {row['path'] for row in report['go_modules']} == set(catalog.MODULES)
    assert sum(row['declared_test_count'] for row in report['go_modules']) == 17
    security = next(row for row in report['go_modules'] if row['path'] == OMITTED[2])
    assert [row['path'] for row in security['files']] == [OMITTED[2] + '/boundary_test.go']


@pytest.mark.parametrize('module', OMITTED)
def test_omitted_modules_require_named_json_evidence(workspace, capsys, module):
    transcript = go_report(workspace, module)
    args = ('verify-go-json', '--root', str(workspace), '--module', module,
            '--input', str(transcript))
    code, report = invoke(capsys, *args)
    assert code == 0
    assert report['status'] == 'passed'
    assert report['expected_top_level_tests'] == 1
    assert report['executed_named_tests'] == 1
    assert report['passing_packages'] == ['example.test/' + module]
    # A successful package alone must never stand in for its named tests.
    go_report(workspace, module, [{'Action': 'pass', 'Package': 'example.test/' + module}])
    code, report = invoke(capsys, *args)
    assert code == 2
    assert report['status'] == 'failed'
    assert 'missing executed passing tests' in report['error']


@pytest.mark.parametrize('mode', (
    'inventory', 'discover', 'run-go', 'run-vitest', 'verify-go-json', 'verify-vitest-json'))
@pytest.mark.parametrize('change', ('added', 'removed', 'duplicate', 'missing', 'truncated'))
def test_workspace_drift_rejected_before_execution(workspace, capsys, mode, change):
    path = workspace / catalog.WORKSPACE
    text = path.read_text()
    if change == 'added':
        path.write_text(text + '\nuse ./new-service\n')
    elif change == 'removed':
        path.write_text(text.replace('\t./discovery\n', ''))
    elif change == 'duplicate':
        path.write_text(text + '\nuse ./discovery\n')
    elif change == 'missing':
        path.unlink()
    else:
        path.write_text('go 1.25.0\nuse (\n./discovery\n')
    code, report = invoke(capsys, mode, '--root', str(workspace))
    assert code == 2
    assert report['status'] == 'failed'
    assert 'workspace ' in report['error']
    if change == 'added':
        assert "missing=['services/launchpad/new-service']" in report['error']
    elif change == 'removed':
        assert "unlisted=['services/launchpad/discovery']" in report['error']
    elif change == 'duplicate':
        assert 'duplicate module path' in report['error']
    elif change == 'truncated':
        assert 'unterminated use block' in report['error']
    assert 'stdout_log' not in report


def test_workspace_accepts_single_use_quoted_paths_and_comments(workspace, capsys):
    path = workspace / catalog.WORKSPACE
    text = path.read_text().replace('\t./discovery\n', '')
    path.write_text(text + '\n// use ./ignored\nuse "./discovery" // retained service\n')
    code, report = invoke(capsys, 'inventory', '--root', str(workspace))
    assert code == 0
    assert report['workspace']['issues'] == []
    assert report['workspace']['modules'] == sorted(catalog.MODULES)


@pytest.mark.parametrize('module', OMITTED)
def test_missing_module_manifest_is_reported(workspace, capsys, module):
    (workspace / module / 'go.mod').unlink()
    code, report = invoke(capsys, 'inventory', '--root', str(workspace))
    assert code == 2
    assert 'missing module: ' + module in report['issues']
    row = next(row for row in report['go_modules'] if row['path'] == module)
    assert row['manifest_exists'] is False


def test_build_only_module_is_explicit_and_cannot_qualify_as_tests(workspace, capsys):
    module = OMITTED[0]
    (workspace / module / 'boundary_test.go').unlink()
    code, report = invoke(capsys, 'inventory', '--root', str(workspace))
    assert code == 0
    assert report['testing_gaps'] == ['build-only module with no declared Go tests: ' + module]
    row = next(row for row in report['go_modules'] if row['path'] == module)
    assert row['declared_test_count'] == 0
    assert row['files'] == []
    transcript = go_report(workspace, module)
    code, report = invoke(capsys, 'verify-go-json', '--root', str(workspace),
                          '--module', module, '--input', str(transcript))
    assert code == 2
    assert 'no expected named tests' in report['error']


@pytest.mark.parametrize('case, message', (
    ('empty', 'missing executed'),
    ('skip', 'runner reported skip'),
    ('fail', 'runner reported fail'),
    ('package', 'missing final package pass'),
    ('subtest', 'without final pass'),
))
def test_native_go_rejections_retained(workspace, capsys, case, message):
    module = OMITTED[3]
    transcript = go_report(workspace, module)
    events = [json.loads(line) for line in transcript.read_text().splitlines()]
    if case == 'empty':
        events = []
    elif case in ('skip', 'fail'):
        events[1]['Action'] = case
    elif case == 'package':
        events.pop()
    else:
        events.append({'Action': 'run', 'Package': 'example.test/' + module,
                       'Test': 'TestBoundary/unfinished'})
    go_report(workspace, module, events)
    code, report = invoke(capsys, 'verify-go-json', '--root', str(workspace),
                          '--module', module, '--input', str(transcript))
    assert code == 2
    assert message in report['error']


def test_vitest_json_rejections_retained(workspace, capsys):
    path = workspace / 'vitest.json'
    report = {'success': True, 'numTotalTests': 1, 'numPassedTests': 1,
              'testResults': [{'name': 'src/boundary.test.ts', 'status': 'passed',
                               'assertionResults': [{'status': 'passed'}]}]}
    path.write_text(json.dumps(report))
    args = ('verify-vitest-json', '--root', str(workspace), '--input', str(path))
    code, result = invoke(capsys, *args)
    assert code == 0
    assert result['executed_assertions'] == 1
    report['testResults'][0]['assertionResults'][0]['status'] = 'pending'
    path.write_text(json.dumps(report))
    code, result = invoke(capsys, *args)
    assert code == 2
    assert 'assertion failed/skipped/pending' in result['error']
    report['numTotalTests'] = 0
    path.write_text(json.dumps(report))
    code, result = invoke(capsys, *args)
    assert code == 2
    assert 'failure or no tests' in result['error']
