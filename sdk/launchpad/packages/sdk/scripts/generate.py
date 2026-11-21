#!/usr/bin/env python3
"""Project pinned generator with explicit versioned bundle and package outputs."""
import argparse
import copy
import hashlib
import importlib.util
import json
import pathlib
import re
import sys
import types

sys.dont_write_bytecode = True

RECORDED = pathlib.Path('services/launchpad/api/internal/rest/testdata/contracts/v2/openapi.json')
PRODUCER = pathlib.Path('tools/launchpad/production/interfaces/codegen.py')


def released_schema(package, major):
    release = json.loads((package / 'release-manifest.json').read_bytes())
    if release['apiMajor'] != major or release['streamMajor'] != major:
        raise ValueError('Released schema major mismatch')
    schema = package / 'src/schemas.gen.json'
    if sha(schema) != release['outputs']['schemas.gen.json']:
        raise ValueError('Released schema digest mismatch')
    document = json.loads(schema.read_bytes())
    inventory = (package / 'src/types.gen.ts').read_text().split('export interface ContractTypes {', 1)[1]
    names = re.findall(r'^"([^"]+)":', inventory, re.M)
    if not names or len(names) != len(set(names)):
        raise ValueError('Released definition inventory mismatch')
    document['$defs'] = {name: document['$defs'][name] for name in names}
    data = (json.dumps(document, sort_keys=True, indent=2) + '\n').encode()
    if hashlib.sha256(data).hexdigest() != release['inputs']['schema-definitions.json']:
        raise ValueError('Released schema input mismatch')
    return document


def parameter_type(schema):
    if '$ref' in schema:
        return 'V2ContractTypes[' + json.dumps(schema['$ref'].split('/')[-1]) + ']'
    if 'enum' in schema:
        return '|'.join(json.dumps(value) for value in schema['enum'])
    if 'anyOf' in schema:
        return '|'.join(parameter_type(value) for value in schema['anyOf'])
    kind = schema.get('type')
    if kind == 'array':
        return 'Array<' + parameter_type(schema['items']) + '>'
    if kind == 'object':
        return '{' + ';'.join(json.dumps(key) + ('' if key in schema.get('required', []) else '?') + ':' + parameter_type(value) for key, value in schema.get('properties', {}).items()) + '}'
    if kind in ('integer', 'number'):
        return 'number'
    if kind in ('string', 'boolean', 'null'):
        return kind
    raise ValueError('Unsupported operation parameter schema: ' + str(schema))


def current_operations(package, recorded):
    catalog = json.loads((package / 'scripts/operation-catalog.json').read_bytes())
    rows = catalog['operations']
    admitted = set()
    for path, methods in recorded['paths'].items():
        for method, operation in methods.items():
            if method not in ('get', 'post', 'put', 'patch', 'delete'):
                continue
            name = operation['operationId']
            if name in admitted:
                raise ValueError('Duplicate recorded operation: ' + name)
            admitted.add(name)
            response = operation['responses']['200']['content']['application/json']['schema']['$ref'].split('/')[-1]
            if name in rows:
                row = rows[name]
                if (row['method'], row['path'], row['response']) != (method.upper(), path, response):
                    raise ValueError('Recorded operation identity mismatch: ' + name)
            else:
                if method != 'get' or operation.get('x-authorization') != 'public':
                    raise ValueError('Explicit public read metadata required: ' + name)
                row = {'path': path, 'wireMajor': 2, 'binaryBody': False, 'binaryResponse': False,
                       'method': 'GET', 'service': 'api', 'authenticated': False, 'cookieSession': False,
                       'idempotentRead': True, 'public': True, 'timeoutMs': 1000, 'body': None,
                       'response': response, 'emptyResponse': False, 'idempotencyKey': False}
                rows[name] = row
            row['parameters'] = local_refs(operation.get('parameters', []))
    types_out = []
    for name, row in sorted(rows.items()):
        fields = []
        for location in ('path', 'query'):
            selected = [value for value in row['parameters'] if value['in'] == location]
            if selected:
                fields.append(location + ('' if any(value.get('required') for value in selected) else '?') + ':{' + ';'.join(json.dumps(value['name']) + ('' if value.get('required') else '?') + ':' + parameter_type(value['schema']) for value in selected) + '}')
        if row['binaryBody']:
            fields.append('body:Blob')
        elif row['body']:
            fields.append('body:V2ContractTypes[' + json.dumps(row['body']) + ']')
        response = 'Response' if row['binaryResponse'] else 'V2ContractTypes[' + json.dumps(row['response']) + ']' if row['response'] else 'undefined'
        types_out.append(json.dumps(name) + ':{input:{' + ';'.join(fields) + '};response:' + response + '}')
    generated = "import type { ContractTypes as V2ContractTypes } from './types-v2.gen.js';\n"
    for name in catalog['identifiers']:
        generated += 'export const ' + name + ' = ' + json.dumps(name) + ' as const;\n'
    return generated + 'export interface OperationTypes {\n' + ';\n'.join(types_out) + '\n}\nexport const operationDefinitions = ' + json.dumps(rows, sort_keys=True, indent=2) + ' as const;\n'


def current_artifacts(root):
    package = root / 'sdk/launchpad/packages/sdk'
    producer = root / PRODUCER
    module_spec = importlib.util.spec_from_file_location('current_codegen', producer)
    module = importlib.util.module_from_spec(module_spec)
    module_spec.loader.exec_module(module)
    dependency = root / 'sdk/launchpad/packages/sdk-v2'
    document = released_schema(dependency, 2)
    ts_root = 'sdk/launchpad/packages/production-contracts-v2/src/'
    original = module.Generator(document, major=2, format_output=False).emit()
    original_types = original[ts_root + 'types.gen.ts']
    frozen = json.loads((dependency / 'release-manifest.json').read_bytes())
    if hashlib.sha256(original_types.encode()).hexdigest() != frozen['outputs']['types.gen.ts']:
        raise ValueError('Immutable v2 type reproduction mismatch')
    recorded = json.loads((root / RECORDED).read_bytes())
    if recorded['openapi'] != '3.1.0' or recorded['info']['version'] != 'v2':
        raise ValueError('Recorded OpenAPI v2 required')
    document['$defs'].update(local_refs(recorded['components']['schemas']))
    document['$defs']['SessionsRequest']['properties']['cursor']['maxLength'] = 2048
    notification = copy.deepcopy(document['$defs']['Notification'])
    notification['properties'].update({'summary': {'type': 'string', 'minLength': 1, 'maxLength': 512}, 'read': {'type': 'boolean'}})
    notification['required'].extend(['summary', 'read'])
    document['$defs']['NotificationListItem'] = notification
    document['$defs']['NotificationsData']['properties']['items']['items'] = {'$ref': '#/$defs/NotificationListItem'}
    for name, node in document['$defs'].items():
        module.validate_schema(node, document['$defs'], name)
    generated = module.Generator(document, major=2, format_output=False).emit()
    examples = importlib.util.spec_from_file_location('current_examples', package / 'scripts/examples.py')
    examples_module = importlib.util.module_from_spec(examples)
    examples.loader.exec_module(examples_module)
    operations = current_operations(package, recorded)
    admitted = json.loads((package / 'scripts/operation-catalog.json').read_bytes())['operations']
    return {
        'releases/0.1.0/types-v2.gen.ts': original_types,
        'src/types-v2.gen.ts': generated[ts_root + 'types.gen.ts'],
        'src/schemas-v2.gen.json': generated[ts_root + 'schemas.gen.json'],
        'src/operations.gen.ts': operations,
        **{'examples/' + name: source for name, source in examples_module.example_sources(recorded, admitted).items()},
    }


def emit_current_release(root):
    package = root / 'sdk/launchpad/packages/sdk'
    artifacts = current_artifacts(root)
    metadata = json.loads((package / 'package.json').read_bytes())
    inputs = [PRODUCER, PRODUCER.with_name('validator.go.txt'), PRODUCER.with_name('validator.compiled.go.txt'), RECORDED]
    for name in ('sdk', 'sdk-v2'):
        inputs.extend(pathlib.Path('sdk/launchpad/packages') / name / path for path in ('src/types.gen.ts', 'src/schemas.gen.json', 'release-manifest.json'))
    inputs.extend(pathlib.Path('sdk/launchpad/packages/sdk/scripts') / name for name in ('generate.py', 'examples.py', 'operation-catalog.json'))
    release = {
        'packageVersion': metadata['version'], 'apiMajor': 2, 'streamMajor': 2, 'streamSubprotocol': 'sidiora.v2',
        'generatorSha256': sha(package / 'scripts/generate.py'),
        'inputs': {str(path): sha(root / path) for path in inputs},
        'outputs': {path: hashlib.sha256(source.encode()).hexdigest() for path, source in artifacts.items()},
    }
    manifest = package / metadata['kindleCompatibility']['currentReleaseManifest']
    if manifest != package / 'releases' / (metadata['version'] + '.json'):
        raise ValueError('Versioned release manifest required')
    if manifest.exists() and json.loads(manifest.read_bytes()) != release:
        raise ValueError('Published release inputs differ; select a new package version')
    for path, source in artifacts.items():
        target = package / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(source)
    manifest.parent.mkdir(parents=True, exist_ok=True)
    manifest.write_text(json.dumps(release, sort_keys=True, indent=2) + '\n')
    return release


def repository_root():
    return pathlib.Path(__file__).resolve().parents[5]


def default_contracts(major):
    return repository_root() / ('sdk/launchpad/protocol/contracts/v' + str(major))


def read_manifest(contracts, manifest_path=None):
    path = manifest_path or contracts / 'codegen-manifest.json'
    try:
        return json.loads(path.read_bytes())
    except FileNotFoundError:
        raise ValueError(
            f'Missing codegen manifest: {path}; requires the complete immutable '
            f'{contracts.name} contract bundle (codegen-manifest.json, schema-definitions.json, '
            'openapi.json, asyncapi.json and pinned generator/validator inputs) from the launchpad owner'
        ) from None


def expected_package(major):
    return repository_root() / ('sdk/launchpad/packages/sdk' if major == 1 else 'sdk/launchpad/packages/sdk-v' + str(major))


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def local_refs(value):
    if isinstance(value, dict):
        result = {}
        for key, child in value.items():
            if key == '$ref' and isinstance(child, str) and child.startswith('#/components/schemas/'):
                result[key] = '#/$defs/' + child[len('#/components/schemas/'):]
            else:
                result[key] = local_refs(child)
        return result
    if isinstance(value, list):
        return [local_refs(child) for child in value]
    return value



def emit_operations(root, contracts):
    pin = read_manifest(contracts)
    if pin.get('schemaMajor') != 2 or sha(contracts / 'schema-definitions.json') != pin.get('schemaSha256'):
        raise ValueError('Pinned v2 definition digest mismatch')
    definitions = json.loads((contracts / 'schema-definitions.json').read_bytes())
    document = json.loads((contracts / 'openapi.json').read_bytes())
    async_document = json.loads((contracts / 'asyncapi.json').read_bytes())
    if async_document.get('asyncapi') != '3.0.0' or local_refs(async_document['components']['schemas']) != definitions['$defs'] or local_refs(document['components']['schemas']) != definitions['$defs']:
        raise ValueError('Authoritative HTTP/stream definition drift')
    if document.get('openapi') != '3.1.0' or not document['info']['version'].startswith('2.'):
        raise ValueError('operation generation requires explicit OpenAPI3.1 v2')
    def ts(schema):
        if '$ref' in schema:
            return 'V2ContractTypes[' + json.dumps(schema['$ref'].split('/')[-1]) + ']'
        if 'enum' in schema:
            return '|'.join(json.dumps(v) for v in schema['enum'])
        if 'anyOf' in schema:
            return '|'.join(ts(v) for v in schema['anyOf'])
        kind = schema.get('type')
        if kind == 'array': return 'Array<' + ts(schema['items']) + '>'
        if kind == 'object':
            return '{' + ';'.join(json.dumps(k) + ('' if k in schema.get('required',[]) else '?') + ':' + ts(v) for k,v in schema.get('properties',{}).items()) + '}'
        if kind in ('integer','number'): return 'number'
        if kind in ('string','boolean','null'): return kind
        raise ValueError('Unsupported operation parameter schema: ' + str(schema))
    rows = {}; types_out = []
    for path, methods in sorted(document['paths'].items()):
        for method, operation in sorted(methods.items()):
            if method not in ('get','post','put','patch','delete'): continue
            name = operation['operationId']
            content = operation.get('requestBody',{}).get('content',{})
            body = content.get('application/json',{}).get('schema')
            binary_body = bool(content) and not body
            binary_response = any(response.get('content') and 'application/json' not in response.get('content',{}) for code,response in operation['responses'].items() if code.startswith('2'))
            successes = [response for code,response in operation['responses'].items() if code.startswith('2')]
            responses = [response.get('content',{}).get('application/json',{}).get('schema') for response in successes]
            response = next((node['$ref'].split('/')[-1] for node in responses if node), None)
            parameters = operation.get('parameters',[])
            fields = []
            for location in ('path','query'):
                selected = [v for v in parameters if v['in'] == location]
                if selected:
                    fields.append(location + ('' if any(v.get('required') for v in selected) else '?') + ':{' + ';'.join(json.dumps(v['name']) + ('' if v.get('required') else '?') + ':' + ts(v['schema']) for v in selected) + '}')
            if binary_body: fields.append('body:Blob')
            if body: fields.append('body' + ('' if operation['requestBody'].get('required') else '?') + ':' + ts(body))
            types_out.append(json.dumps(name) + ':{input:{' + ';'.join(fields) + '};response:' + ('Response' if binary_response else 'V2ContractTypes['+json.dumps(response)+']' if response else 'undefined') + '}')
            service = operation.get('x-service') or ('identity' if path.startswith('/v2/auth/') or path == '/v2/stream/tickets' else 'media' if path.startswith(('/v2/assets','/v2/tokens/','/v2/streams','/v2/provider-webhooks','/v2/voice-rooms','/v2/profiles','/v2/me/watchlist','/v2/me/following')) else 'api')
            security = operation.get('security',[])
            rows[name] = {'path':path,'wireMajor':int(path.split('/')[1][1:]),'binaryBody':binary_body,'binaryResponse':binary_response,'method':method.upper(),'service':service,'authenticated':bool(security) and not any(not v for v in security),'cookieSession':name in ('IF_LOGIN','IF_REFRESH'),'idempotentRead':method=='get' and ('read' in operation.get('x-idempotency','').lower() or name=='IF_TOPIC_SNAPSHOT_READ'),'public':not security and operation.get('x-cache-policy','').startswith('Public'),'timeoutMs':operation['x-timeout-ms'],'body':body['$ref'].split('/')[-1] if body else None,'response':response,'emptyResponse':all(not value.get('content') for value in successes),'parameters':local_refs(parameters),'idempotencyKey':'Idempotency-Key' in operation.get('x-idempotency','')}
    generated = "import type { ContractTypes as V2ContractTypes } from '@kindle/sdk-v2/types';\nexport interface OperationTypes {\n" + ';\n'.join(types_out) + '\n}\nexport const operationDefinitions = ' + json.dumps(rows,sort_keys=True,indent=2) + ' as const;\n'
    target = root / 'sdk/launchpad/packages/sdk/src/operations.gen.ts'
    target.write_text(generated)
    return {'generatorSha256':sha(pathlib.Path(__file__)), 'openapiSha256':sha(contracts/'openapi.json'),'outputSha256':sha(target),'operationCount':len(rows)}


def main():
    root = repository_root()
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--major', type=int, default=1)
    parser.add_argument('--contracts', type=pathlib.Path)
    parser.add_argument('--manifest', type=pathlib.Path)
    parser.add_argument('--package', type=pathlib.Path)
    parser.add_argument('--generator', type=pathlib.Path)
    parser.add_argument('--reproduce-v1', action='store_true')
    parser.add_argument('--operations-only', action='store_true')
    parser.add_argument('--current-release', action='store_true')
    args = parser.parse_args()
    if args.current_release:
        if args.major != 1 or any((args.contracts, args.manifest, args.package, args.generator, args.reproduce_v1, args.operations_only)):
            raise ValueError('--current-release selects the package release inputs')
        print(json.dumps(emit_current_release(root), sort_keys=True))
        return
    if args.major < 1:
        raise ValueError('positive schema major required')
    contracts = (args.contracts or default_contracts(args.major)).resolve()
    manifest_path = (args.manifest or contracts / 'codegen-manifest.json').resolve()
    if not contracts.is_relative_to((root / 'sdk/launchpad/protocol/contracts').resolve()) or not manifest_path.is_relative_to(contracts):
        raise ValueError('bundle/manifest outside contract allowlist')
    if args.operations_only:
        if args.major != 2: raise ValueError('--operations-only requires --major2')
        print(json.dumps(emit_operations(root, contracts), sort_keys=True)); return
    package = (args.package or expected_package(args.major)).resolve()
    if package != expected_package(args.major):
        raise ValueError('explicit versioned SDK package required; frozen SDK must not be overwritten')
    manifest = read_manifest(contracts, manifest_path)
    generator = (args.generator or root / manifest['generator']).resolve()
    legacy_source = None
    if args.reproduce_v1:
        if args.major != 1:
            raise ValueError('--reproduce-v1 requires major1')
        current_spec = importlib.util.spec_from_file_location('versioned_codegen', generator)
        current_module = importlib.util.module_from_spec(current_spec)
        current_spec.loader.exec_module(current_module)
        legacy_source = current_module.frozen_v1_generator()
    actual_generator_sha = hashlib.sha256(legacy_source).hexdigest() if legacy_source is not None else sha(generator)
    if actual_generator_sha != manifest['generatorSha256']:
        raise ValueError('Pinned generator digest mismatch; use explicit --reproduce-v1 or supply the immutable generator')
    template = generator.with_name('validator.go.txt' if args.major == 1 else 'validator.compiled.go.txt')
    if sha(template) != manifest['validatorTemplateSha256']:
        raise ValueError('Pinned validator template mismatch')
    if args.major != 1 and manifest.get('schemaMajor') != args.major:
        raise ValueError('versioned codegen manifest major mismatch')
    schema_path = contracts / 'schema-definitions.json'
    if sha(schema_path) != manifest['schemaSha256']:
        raise ValueError('Pinned schema input mismatch')
    document = json.loads(schema_path.read_bytes())
    transport_versions = []
    for filename in ('openapi.json', 'asyncapi.json'):
        source = json.loads((contracts / filename).read_bytes())
        transport_versions.append(source['info']['version'])
        if int(source['info']['version'].split('.')[0]) != args.major:
            raise ValueError(filename + ': API/schema major mismatch')
        if local_refs(source['components']['schemas']) != document['$defs']:
            raise ValueError(filename + ': schema drift')
    if len(set(transport_versions)) != 1:
        raise ValueError('transport document versions differ')
    if legacy_source is not None:
        module = types.ModuleType('pinned_codegen')
        module.__file__ = str(generator)
        exec(compile(legacy_source, str(generator), 'exec'), module.__dict__)
    else:
        module_spec = importlib.util.spec_from_file_location('pinned_codegen', generator)
        module = importlib.util.module_from_spec(module_spec)
        module_spec.loader.exec_module(module)
    for name, node in document['$defs'].items():
        module.validate_schema(node, document['$defs'], name)
    if args.major == 1:
        outputs = module.Generator(document).emit()
    else:
        go_outs = {str(pathlib.PurePosixPath(name).parent) for name in manifest['outputs'] if name.endswith('.go')}
        ts_outs = {str(pathlib.PurePosixPath(name).parent) for name in manifest['outputs'] if name.endswith(('.ts', '.json'))}
        if len(go_outs) != 1 or len(ts_outs) != 1:
            raise ValueError('manifest must declare exactly one Go and TypeScript generated root')
        outputs = module.Generator(document, major=args.major, go_package=manifest['goPackage'], go_out=next(iter(go_outs)), ts_out=next(iter(ts_outs)), format_output=manifest['formatOutput']).emit()
    if set(outputs) != set(manifest['outputs']):
        raise ValueError('Pinned generated output inventory mismatch')
    for name, content in outputs.items():
        if hashlib.sha256(content.encode()).hexdigest() != manifest['outputs'][name]:
            raise ValueError('Pinned generated output mismatch: ' + name)
    ts_root = 'sdk/launchpad/packages/production-contracts/src' if args.major == 1 else next(str(pathlib.PurePosixPath(name).parent) for name in outputs if name.endswith('types.gen.ts'))
    package.joinpath('src').mkdir(parents=True, exist_ok=True)
    for filename in ('types.gen.ts', 'schemas.gen.json'):
        (package / 'src' / filename).write_text(outputs[ts_root + '/' + filename])
    release = {'apiMajor': args.major, 'streamMajor': args.major, 'streamSubprotocol': 'sidiora.v' + str(args.major), 'generatorSha256': manifest['generatorSha256'], 'inputs': {name: sha(contracts / name) for name in ('openapi.json', 'asyncapi.json', 'schema-definitions.json')}, 'outputs': {name: sha(package / 'src' / name) for name in ('types.gen.ts', 'schemas.gen.json')}}
    (package / 'release-manifest.json').write_text(json.dumps(release, sort_keys=True, indent=2) + '\n')
    print(json.dumps(release, sort_keys=True))


if __name__ == '__main__':
    try:
        main()
    except (ValueError, OSError, KeyError) as error:
        print('sdk-codegen: ' + str(error), file=sys.stderr)
        raise SystemExit(1)
