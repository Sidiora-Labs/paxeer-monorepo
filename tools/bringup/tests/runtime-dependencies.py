#!/usr/bin/env python3
"""Record the real producer import graph for toolchain-free source validation."""
import hashlib
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[3]
OUTPUT = ROOT / 'tools/bringup/runtime-dependencies.json'
PRODUCERS = ('daemon/paxd', 'services/wallet/attestor/cmd/attestor',
             'tools/bringup/cmd/bringup-inspect')


def packages(raw):
    decoder = json.JSONDecoder()
    offset = 0
    while offset < len(raw):
        while offset < len(raw) and raw[offset].isspace():
            offset += 1
        if offset == len(raw):
            break
        package, offset = decoder.raw_decode(raw, offset)
        yield package


def graph():
    records = {}
    manifests = {}
    commands = ((ROOT, ('-tags=netgo,ledger', './daemon/paxd')),
                (ROOT, ('./tools/bringup/cmd/bringup-inspect',)),
                (ROOT / 'services/wallet/attestor', ('./cmd/attestor',)))
    for directory, targets in commands:
        raw = subprocess.run(['go', 'list', '-deps', '-json', *targets],
                             cwd=directory, check=True, capture_output=True,
                             text=True, env=dict(os.environ, CGO_ENABLED="0") if directory != ROOT else None).stdout
        listed = list(packages(raw))
        local = {p['ImportPath']: Path(p['Dir']).relative_to(ROOT).as_posix()
                 for p in listed if Path(p.get('Dir', '/')).is_relative_to(ROOT)}
        for package in listed:
            path = Path(package.get('Dir', '/'))
            if not path.is_relative_to(ROOT):
                continue
            relative = path.relative_to(ROOT).as_posix()
            files = {}
            for name in sorted(package.get('GoFiles', []) + package.get('CgoFiles', [])):
                files[name] = hashlib.sha256((path / name).read_bytes()).hexdigest()
            # Native and embed inputs are covered by the bound package directory.
            inputs = sorted(set(name for field in ('EmbedFiles', 'CFiles', 'CXXFiles',
                            'HFiles', 'SFiles', 'SysoFiles') for name in package.get(field, [])))
            record = dict(path=relative, import_path=package['ImportPath'],
                          imports=sorted(package.get('Imports', [])),
                          dependencies=sorted(local[name] for name in package.get('Imports', []) if name in local),
                          files=files, inputs=inputs)
            if relative in records:
                previous = records[relative]
                if previous['import_path'] != record['import_path']:
                    raise ValueError('producer module identities disagree: ' + relative)
                for field in ('imports', 'dependencies', 'inputs'):
                    record[field] = sorted(set(previous[field]) | set(record[field]))
                record['files'] = dict(previous['files'], **record['files'])
            records[relative] = record
            module = package.get('Module', {})
            module = module.get('Replace', module)
            if module.get('GoMod'):
                manifest = Path(module['GoMod'])
                if manifest.is_relative_to(ROOT):
                    manifests[manifest.relative_to(ROOT).as_posix()] = hashlib.sha256(manifest.read_bytes()).hexdigest()
    if not all(producer in records for producer in PRODUCERS):
        raise ValueError('producer graph is incomplete')
    return dict(version=1, producers=list(PRODUCERS), manifests=manifests,
                packages=[records[key] for key in sorted(records)])


if __name__ == '__main__':
    OUTPUT.write_text(json.dumps(graph(), indent=2, sort_keys=True) + '\n')
