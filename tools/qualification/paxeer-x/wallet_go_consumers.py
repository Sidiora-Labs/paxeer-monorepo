#!/usr/bin/env python3
"""Build the exact Rust test consumers used by the wallet Go server suite."""
import argparse
import json
import os
from pathlib import Path
import subprocess


def build_consumer(root, output, package, target):
    command = ['cargo', 'test', '--locked', '--manifest-path', 'services/human/Cargo.toml',
               '-p', package, '--test', target, '--no-run', '--message-format=json']
    log = output / (target + '.jsonl')
    with log.open('w') as stream:
        subprocess.run(command, cwd=root, stdout=stream, check=True)
    executables = []
    for line in log.read_text().splitlines():
        row = json.loads(line)
        if (row.get('reason') == 'compiler-artifact' and row.get('target', {}).get('name') == target
                and row['target'].get('kind') == ['test'] and row.get('profile', {}).get('test') is True
                and row.get('executable')):
            executables.append(Path(row['executable']).resolve())
    if len(executables) != 1 or not os.access(executables[0], os.X_OK):
        raise RuntimeError('one compiler-emitted executable required for ' + target)
    return executables[0]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[3]
    args.output.mkdir(parents=True, exist_ok=True)
    consumers = (
        ('PAXEER_X_NATIVE_CUSTODY_RUST_TEST', 'layerx-human-kms', 'attestor_native_profile'),
        ('PAXEER_X_NATIVE_SEND_RUST_TEST', 'layerx-human-service', 'native_send_purpose'),
    )
    paths = {name: build_consumer(root, args.output, package, target)
             for name, package, target in consumers}
    with Path(os.environ['GITHUB_ENV']).open('a') as env:
        for name, path in paths.items():
            env.write(f'{name}={path}\n')


if __name__ == '__main__':
    main()
