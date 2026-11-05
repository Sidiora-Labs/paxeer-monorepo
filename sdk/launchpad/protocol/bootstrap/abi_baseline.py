#!/usr/bin/env python3
"""Read-only comparison of generated Go bindings and role ABIs or contract artifacts."""
import argparse
import hashlib
import json
import pathlib
import re


def digest(p):
    return hashlib.sha256(p.read_bytes()).hexdigest()


def typ(v):
    t = v['type']
    return '(' + ','.join(typ(x) for x in v.get('components', [])) + ')' + t[5:] if t.startswith('tuple') else t


def identity(v):
    return v.get('type', '') + ':' + v.get('name', '') + '(' + ','.join(typ(x) for x in v.get('inputs', [])) + ')'


def canonical(v):
    def normalize(value):
        if isinstance(value, dict):
            return {
                key: re.sub(r'\s+', '', item) if key == 'internalType' else normalize(item)
                for key, item in value.items()
            }
        if isinstance(value, list):
            return [normalize(item) for item in value]
        return value

    return json.dumps(normalize(v), sort_keys=True, separators=(',', ':'))


def binding_metadata(binding_path):
    source = binding_path.read_text()
    match = re.search(r'var (\w+)MetaData.*?ABI:\s*("(?:[^"\\]|\\.)*")', source, re.S)
    if not match:
        raise ValueError('Missing actual metadata in ' + str(binding_path))
    return match[1], json.loads(json.loads(match[2]))


def compare(binding_path, abi_entries):
    _, existing = binding_metadata(binding_path)
    old = {identity(entry) for entry in existing}
    new = {identity(entry) for entry in abi_entries}
    return sorted(new - old), sorted(old - new)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--contracts', type=pathlib.Path, required=True)
    parser.add_argument(
        '--abi-root', type=pathlib.Path,
        default=pathlib.Path(__file__).resolve().parents[4] / 'precompiles/launchpad/abi',
    )
    parser.add_argument('--output', type=pathlib.Path, required=True)
    args = parser.parse_args()
    module = pathlib.Path(__file__).resolve().parents[1]
    manifest = args.contracts / 'deployments/paxeer-x-network-addresses.json'
    deployment = json.loads(manifest.read_text())
    items = []
    for binding in sorted((module / 'bindings').glob('*.go')):
        if binding.name.endswith('_test.go'):
            continue
        name, existing = binding_metadata(binding)
        abi_path = args.abi_root / (name + '.json')
        if abi_path.is_file():
            current = json.loads(abi_path.read_text())
            source = 'role-abi'
        else:
            candidates = list((args.contracts / 'artifacts/contracts').glob('**/' + name + '.sol/' + name + '.json'))
            if len(candidates) != 1:
                raise ValueError('Ambiguous or absent artifact ' + name)
            abi_path = candidates[0]
            current = json.loads(abi_path.read_text())['abi']
            source = 'artifact'
        added, removed = compare(binding, current)
        old = {identity(x): x for x in existing}
        new = {identity(x): x for x in current}
        items.append({
            'contract': name,
            'binding': str(binding),
            'binding_sha256': digest(binding),
            'source': source,
            'artifact': str(abi_path),
            'artifact_sha256': digest(abi_path),
            'embedded_abi_sha256': hashlib.sha256(canonical(existing).encode()).hexdigest(),
            'current_abi_sha256': hashlib.sha256(canonical(current).encode()).hexdigest(),
            'missing_signatures': added,
            'removed_signatures': removed,
            'changed_schemas': [k for k in sorted(new.keys() & old.keys()) if canonical(new[k]) != canonical(old[k])],
            'current_address': deployment.get(name + '_proxy', deployment.get(name)),
        })
    records = {
        k: {field: v.get(field) for field in ['address', 'status', 'txHash', 'blockNumber', 'creationHash', 'runtimeHash']}
        for k, v in deployment['_deployment']['records'].items()
    }
    out = {
        'evidence': 'Read-only source/ABI comparison; no live RPC, deployed-code validation or production qualification',
        'manifest': str(manifest),
        'manifest_sha256': digest(manifest),
        'manifest_status': deployment['_meta']['status'],
        'chain_id': int(deployment['_meta']['network']),
        'bindings': items,
        'deployment_records': records,
        'missing_binding_contracts': ['SidioraPool', 'SidioraERC20', 'OpticalRegistry', 'Treasury', 'Timelock', 'GovernanceModule', 'EmergencyPauseController'],
        'limitations': [
            'ABI provenance is captured by file hash; compiler build-input linkage and runtime matching require separate release verification.',
            'Signature identity comparison also records schema changes including indexed fields and argument names.',
            'Declared ABI event does not prove any implementation emission path.',
            'Deployment records are persisted source evidence, not fresh chain confirmation.',
        ],
    }
    args.output.write_text(json.dumps(out, indent=2) + '\n')


if __name__ == '__main__':
    main()
