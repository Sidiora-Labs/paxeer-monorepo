"""Manifest transport shared by the caps build script and admission gate."""
import hashlib
import json
import os
from pathlib import Path
import stat

ARTIFACTS = frozenset({'native', 'rust'})
VERSION = 1
PURPOSE = 'native-complete-caps-discovery'
BUILD_LOGS = ['native-build.log', 'fixture-build.log', 'client-build.log']


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def artifact(path):
    path = Path(path).resolve(strict=True)
    require(path.is_file() and path.stat().st_size > 0, 'missing artifact')
    with path.open('rb') as stream:
        digest = hashlib.file_digest(stream, 'sha256').hexdigest()
    return {'path': str(path), 'sha256': digest}


def publish(output, source, native, rust):
    value = {'version': VERSION, 'purpose': PURPOSE, **source,
             'artifacts': {'native': artifact(native), 'rust': artifact(rust)},
             'build_command': 'timeout 15m bash tools/qualification/paxeer-x/build-caps-discovery.sh',
             'build_logs': BUILD_LOGS}
    path = Path(output) / 'manifest.json'
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'w') as stream:
        json.dump(value, stream, indent=2)
        stream.write('\n')
        stream.flush()
        os.fsync(stream.fileno())
    return path


def admit(path, source):
    path = Path(path)
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd) as stream:
        info = os.fstat(stream.fileno())
        require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                and not info.st_mode & 0o077, 'private caller-owned manifest required')
        value = json.load(stream)
    require(value.get('version') == VERSION and value.get('purpose') == PURPOSE,
            'artifact contract')
    require(set(value.get('artifacts', {})) == ARTIFACTS, 'artifact set')
    require({key: value.get(key) for key in source} == source,
            'stale source-bound artifacts')
    require(value.get('build_logs') == BUILD_LOGS, 'build log inventory')
    for saved in value['artifacts'].values():
        target = Path(saved['path'])
        require(target.is_absolute() and not target.is_symlink(), 'canonical artifact path required')
        require(artifact(target) == saved, 'artifact digest mismatch')
    return value
