"""Exercise production bundle consumers with private, digest-pinned input bundles."""
import contextlib
import hashlib
import importlib.util
import io
import json
import pathlib
import runpy
import shutil
import sys

import pytest


ROOT = pathlib.Path(__file__).resolve().parents[5]
SCRIPTS = pathlib.Path('sdk/launchpad/packages/sdk/scripts')
BUNDLES = pathlib.Path('launchpad/spec/production-system/integration/contracts')
SDK = pathlib.Path('sdk/launchpad/packages/sdk')
V2_RELEASE = pathlib.Path('sdk/launchpad/packages/sdk-v2/release-manifest.json')
GENERATOR = pathlib.Path('tools/launchpad/production/interfaces/codegen.py')
BUNDLE_FILES = ('codegen-manifest.json', 'schema-definitions.json', 'openapi.json', 'asyncapi.json')


def copy_file(root, relative):
    target = root / relative
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(ROOT / relative, target)
    return target


def load_module(path, name):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def load_script(root, name):
    return load_module(copy_file(root, SCRIPTS / (name + '.py')), 'repair_' + name)


def write_json(path, document):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(document, sort_keys=True, indent=2) + '\n')


def copy_bundle(root, major):
    """Build a small pinned fixture with the real producer, never a substitute emitter."""
    generator_path = copy_file(root, GENERATOR)
    copy_file(root, GENERATOR.with_name('validator.go.txt'))
    producer = load_module(generator_path, 'repair_protocol_codegen')
    contracts = root / BUNDLES / ('v' + str(major))
    schemas = {'EventId': {'type': 'string', 'minLength': 1}}
    document = {'$schema': 'https://json-schema.org/draft/2020-12/schema',
                'x-schema-major': major, '$defs': schemas}
    write_json(contracts / 'schema-definitions.json', document)
    paths = {}
    for path, method, operation in (
        ('/v2/pools/{poolAddress}/trades', 'get', 'IF_TRADES'),
        ('/v2/snapshots', 'get', 'IF_TOPIC_SNAPSHOT_READ'),
        ('/v2/stream/tickets', 'post', 'IF_STREAM_TICKET_ISSUE'),
    ):
        paths[path] = {method: {
            'operationId': operation, 'x-timeout-ms': 5000,
            'responses': {'200': {'description': 'Success', 'content': {
                'application/json': {'schema': {'$ref': '#/components/schemas/EventId'}}}}},
        }}
    for filename, version in (('openapi.json', {'openapi': '3.1.0', 'paths': paths}),
                              ('asyncapi.json', {'asyncapi': '3.0.0'})):
        write_json(contracts / filename, {
            **version, 'info': {'title': 'Private codegen fixture', 'version': str(major) + '.0.0'},
            'components': {'schemas': schemas},
        })
    for name, schema in schemas.items():
        producer.validate_schema(schema, schemas, name)
    outputs = producer.Generator(document, major=major, format_output=False).emit()
    digest = lambda data: hashlib.sha256(data).hexdigest()
    manifest = {
        'generator': str(GENERATOR), 'generatorSha256': digest(generator_path.read_bytes()),
        'validatorTemplateSha256': digest(generator_path.with_name('validator.go.txt').read_bytes()),
        'schemaMajor': major, 'schemaSha256': digest((contracts / 'schema-definitions.json').read_bytes()),
        'goPackage': 'v' + str(major), 'formatOutput': False,
        'outputs': {name: digest(source.encode()) for name, source in outputs.items()},
    }
    write_json(contracts / 'codegen-manifest.json', manifest)
    return contracts


def run_generator(module, monkeypatch, *arguments):
    monkeypatch.setattr(sys, 'argv', [str(module.__file__), *arguments])
    output = io.StringIO()
    with contextlib.redirect_stdout(output):
        module.main()
    return json.loads(output.getvalue())


def test_finding4_default_and_examples_paths_share_protocol_owner(tmp_path):
    generate = load_script(tmp_path, 'generate')
    examples = load_script(tmp_path, 'examples')
    assert generate.repository_root() == tmp_path
    for major in (1, 2):
        assert generate.default_contracts(major) == tmp_path / BUNDLES / ('v' + str(major))
    assert examples.examples_paths(tmp_path) == (
        generate.default_contracts(2) / 'openapi.json', tmp_path / SDK / 'examples'
    )


@pytest.mark.parametrize('arguments', [(), ('--operations-only',)])
def test_finding4_retired_contract_tree_is_outside_allowlist(tmp_path, monkeypatch, arguments):
    generate = load_script(tmp_path, 'generate')
    retired = tmp_path / 'sdk/launchpad/protocol/contracts/v2'
    retired.mkdir(parents=True)
    with pytest.raises(ValueError, match='bundle/manifest outside contract allowlist'):
        run_generator(generate, monkeypatch, '--major', '2', '--contracts', str(retired), *arguments)


@pytest.mark.parametrize('major', (1, 2))
def test_finding4_manifest_resolution_and_refusal(tmp_path, major):
    generate = load_script(tmp_path, 'generate')
    contracts = generate.default_contracts(major)
    expected = contracts / 'codegen-manifest.json'
    with pytest.raises(ValueError) as failure:
        generate.read_manifest(contracts)
    assert str(expected) in str(failure.value)
    assert 'complete immutable v' + str(major) + ' contract bundle' in str(failure.value)
    assert all(filename in str(failure.value) for filename in BUNDLE_FILES)
    assert 'launchpad owner' in str(failure.value)
    manifest = {'schemaMajor': major}
    write_json(expected, manifest)
    assert generate.read_manifest(contracts) == manifest
    assert generate.read_manifest(contracts, expected) == manifest


@pytest.mark.parametrize('major', (1, 2))
def test_finding4_authoritative_bundle_matches_frozen_release(tmp_path, major, monkeypatch):
    generate = load_script(tmp_path, 'generate')
    contracts = copy_bundle(tmp_path, major)
    manifest = generate.read_manifest(contracts)
    generator = tmp_path / manifest['generator']
    assert generate.sha(generator) == manifest['generatorSha256']
    assert manifest['schemaMajor'] == major
    assert generate.sha(generator.with_name('validator.go.txt')) == manifest['validatorTemplateSha256']
    assert generate.sha(contracts / 'schema-definitions.json') == manifest['schemaSha256']
    definitions = json.loads((contracts / 'schema-definitions.json').read_bytes())['$defs']
    versions = []
    for filename in ('openapi.json', 'asyncapi.json'):
        document = json.loads((contracts / filename).read_bytes())
        assert generate.local_refs(document['components']['schemas']) == definitions
        assert int(document['info']['version'].split('.')[0]) == major
        versions.append(document['info']['version'])
    assert versions[0] == versions[1]
    if major == 2:
        release = run_generator(generate, monkeypatch, '--major', '2')
        assert release['generatorSha256'] == manifest['generatorSha256']
        assert release == json.loads((tmp_path / V2_RELEASE).read_bytes())
        for filename, digest in release['inputs'].items():
            assert generate.sha(contracts / filename) == digest
        for filename, digest in release['outputs'].items():
            target = tmp_path / V2_RELEASE.parent / 'src' / filename
            assert generate.sha(target) == digest
            source = 'sdk/launchpad/packages/production-contracts-v2/src/' + filename
            assert digest == manifest['outputs'][source]


def test_finding4_clean_default_v2_operations(tmp_path, monkeypatch):
    results = []
    for name in ('clean-a', 'clean-b'):
        root = tmp_path / name
        generate = load_script(root, 'generate')
        copy_bundle(root, 2)
        (root / SDK / 'src').mkdir(parents=True)
        report = run_generator(generate, monkeypatch, '--major', '2', '--operations-only')
        output = (root / SDK / 'src/operations.gen.ts').read_bytes()
        assert report['outputSha256'] == hashlib.sha256(output).hexdigest()
        assert report['openapiSha256'] == generate.sha(generate.default_contracts(2) / 'openapi.json')
        assert report['operationCount'] == 3
        assert all(name.encode() in output for name in (
            'IF_TRADES', 'IF_TOPIC_SNAPSHOT_READ', 'IF_STREAM_TICKET_ISSUE'))
        assert b'V2ContractTypes["EventId"]' in output
        results.append((report, output))
    assert results[0] == results[1]


def test_finding5_clean_digest_bound_examples(tmp_path, monkeypatch):
    results = []
    for name in ('clean-a', 'clean-b'):
        root = tmp_path / name
        examples = load_script(root, 'examples')
        generate = load_script(root, 'generate')
        copy_bundle(root, 2)
        release = run_generator(generate, monkeypatch, '--major', '2')
        report = examples.emit(root)
        assert report['openapiSha256'] == release['inputs']['openapi.json']
        assert set(report['outputs']) == {'pagination.mjs', 'snapshot-resume.mjs'}
        outputs = {}
        for filename, digest in report['outputs'].items():
            output = (root / SDK / 'examples' / filename).read_bytes()
            assert output == (ROOT / SDK / 'examples' / filename).read_bytes()
            assert hashlib.sha256(output).hexdigest() == digest
            outputs[filename] = output
        results.append((report, outputs))
    assert results[0] == results[1]


def test_finding5_unpinned_recorded_document_is_refused_before_writing(tmp_path):
    examples = load_script(tmp_path, 'examples')
    copy_bundle(tmp_path, 2)
    copy_file(tmp_path, V2_RELEASE)
    contracts, output = examples.examples_paths(tmp_path)
    recorded = ROOT / 'services/launchpad/api/internal/rest/testdata/contracts/v2/openapi.json'
    shutil.copyfile(recorded, contracts)
    with pytest.raises(ValueError, match='Pinned v2 OpenAPI digest mismatch'):
        examples.emit(tmp_path)
    assert not output.exists()


@pytest.mark.parametrize('script,arguments,major', [
    ('generate', (), 1), ('generate', ('--major', '2'), 2),
    ('generate', ('--major', '2', '--operations-only'), 2),
    ('examples', (), 2),
])
def test_missing_bundle_cli_has_one_actionable_error(tmp_path, monkeypatch, script, arguments, major):
    module = load_script(tmp_path, script)
    monkeypatch.setattr(sys, 'argv', [str(module.__file__), *arguments])
    stdout, stderr = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
        with pytest.raises(SystemExit) as failure:
            runpy.run_path(module.__file__, run_name='__main__')
    assert failure.value.code == 1
    assert stdout.getvalue() == ''
    lines = stderr.getvalue().splitlines()
    assert len(lines) == 1
    assert str(tmp_path / BUNDLES / ('v' + str(major)) / 'codegen-manifest.json') in lines[0]
    assert 'complete immutable v' + str(major) + ' contract bundle' in lines[0]
    assert 'launchpad owner' in lines[0]
    assert not (tmp_path / SDK / 'examples').exists()
    assert not (tmp_path / SDK / 'src').exists()
    assert not (tmp_path / V2_RELEASE).exists()


@pytest.mark.parametrize('kind,message', [
    ('generator', 'Pinned generator digest mismatch'),
    ('template', 'Pinned validator template mismatch'),
    ('schema', 'Pinned schema input mismatch'),
    ('major', 'versioned codegen manifest major mismatch'),
    ('transport-major', 'API/schema major mismatch'),
    ('transport-version', 'transport document versions differ'),
    ('transport-schema', 'schema drift'),
    ('output', 'Pinned generated output mismatch'),
])
def test_finding4_pinned_input_and_output_checks_remain(tmp_path, monkeypatch, kind, message):
    generate = load_script(tmp_path, 'generate')
    contracts = copy_bundle(tmp_path, 2)
    manifest = generate.read_manifest(contracts)
    if kind in ('generator', 'template', 'schema'):
        field = {'generator': 'generatorSha256', 'template': 'validatorTemplateSha256',
                 'schema': 'schemaSha256'}[kind]
        manifest[field] = '0' * 64
    elif kind == 'major':
        manifest['schemaMajor'] = 1
    elif kind.startswith('transport-'):
        path = contracts / 'asyncapi.json'
        document = json.loads(path.read_bytes())
        if kind == 'transport-major':
            document['info']['version'] = '1.0.0'
        elif kind == 'transport-version':
            document['info']['version'] = '2.1.0'
        else:
            document['components']['schemas']['EventId']['minLength'] = 2
        write_json(path, document)
    else:
        manifest['outputs'][next(iter(manifest['outputs']))] = '0' * 64
    write_json(contracts / 'codegen-manifest.json', manifest)
    with pytest.raises(ValueError, match=message):
        run_generator(generate, monkeypatch, '--major', '2')
    assert not (tmp_path / V2_RELEASE).exists()
    assert not (tmp_path / V2_RELEASE.parent / 'src').exists()


def test_finding4_manifest_cannot_escape_bundle(tmp_path, monkeypatch):
    generate = load_script(tmp_path, 'generate')
    outside = tmp_path / 'codegen-manifest.json'
    write_json(outside, {'schemaMajor': 2})
    with pytest.raises(ValueError, match='bundle/manifest outside contract allowlist'):
        run_generator(generate, monkeypatch, '--major', '2', '--manifest', str(outside))


@pytest.mark.parametrize('major', (1, 2))
def test_repository_bundle_absence_is_reported(major):
    generate = load_module(ROOT / SCRIPTS / 'generate.py', 'repair_repository_codegen')
    contracts = generate.default_contracts(major)
    assert not (contracts / 'codegen-manifest.json').exists()
    with pytest.raises(ValueError) as failure:
        generate.read_manifest(contracts)
    assert str(contracts / 'codegen-manifest.json') in str(failure.value)
    assert 'launchpad owner' in str(failure.value)
