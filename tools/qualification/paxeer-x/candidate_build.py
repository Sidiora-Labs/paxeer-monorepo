"""Complete candidate source and private build-sidecar transport for qualifiers."""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import stat
import subprocess

ROOT = Path(__file__).resolve().parents[3]
_spec = importlib.util.spec_from_file_location('shared_candidate_source', ROOT / 'tools/paxeer-x/candidate.py')
CANDIDATE = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(CANDIDATE)
SCHEMA = 'paxeer-x.candidate-build.v1'


def require(value, message):
    if not value:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode()


def file_digest(path):
    path = Path(path)
    return sha(os.fsencode(os.readlink(path))) if path.is_symlink() else sha(path.read_bytes())


def source_digest(source):
    return sha(canonical(source))


def verify_files(root, source):
    root = Path(root).resolve(strict=True)
    require(source['candidate']['dirty'] is False, 'actual candidate source is dirty')
    require(source['files'], 'complete candidate source inventory absent')
    for name, row in source['files'].items():
        path = root / name
        require(not Path(name).is_absolute() and '..' not in Path(name).parts,
                'source path escapes checkout')
        require(path.exists() or path.is_symlink(), 'candidate source missing: ' + name)
        require(stat.S_IFMT(path.lstat().st_mode) == row['type']
                and bool(path.lstat().st_mode & 0o111) == row['executable']
                and file_digest(path) == row['sha256'], 'candidate source changed: ' + name)
    return source


def capture_files(root, candidate, names):
    root = Path(root).resolve(strict=True)
    require(candidate['dirty'] is False, 'actual candidate source is dirty')
    files = {}
    for name in sorted(names):
        path = root / name
        info = path.lstat()
        require(stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode), 'uninspectable candidate source: ' + name)
        files[name] = {'sha256': file_digest(path), 'type': stat.S_IFMT(info.st_mode),
                       'executable': bool(info.st_mode & 0o111)}
    source = {'candidate': candidate, 'files': files}
    return verify_files(root, source)


def source_identity(root=ROOT):
    root = Path(root).resolve(strict=True)
    # HEAD is sufficient for a task candidate; release integration remains the
    # candidate validator's independent mainline requirement.
    before = CANDIDATE.source_identity(root, 'HEAD')
    require(before['dirty'] is False, 'actual candidate source is dirty')
    raw = subprocess.check_output(['git', '-C', str(root), 'ls-files', '--recurse-submodules', '-z'], timeout=60)
    source = capture_files(root, before, [os.fsdecode(n) for n in raw.split(b'\0') if n])
    verify_files(root, source)
    require(CANDIDATE.source_identity(root, 'HEAD') == before, 'checkout changed during source inspection')
    return source


def private_json(path):
    path = Path(path)
    require(path.is_absolute() and path.resolve(strict=True) == path and not path.is_symlink(), 'canonical private sidecar required')
    info = path.stat()
    require(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid() and info.st_mode & 0o077 == 0
            and info.st_nlink == 1 and info.st_size <= 64 * 1024 * 1024, 'private bounded sidecar required')
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, 'duplicate sidecar field')
            result[key] = value
        return result
    return json.loads(path.read_bytes(), object_pairs_hook=pairs)


def publish(path, source_start, source_end, argv, exit_code, log, artifacts):
    require(source_start == source_end and source_start['candidate']['dirty'] is False,
            'build source changed or dirty')
    require(exit_code == 0 and isinstance(argv, list) and argv and all(isinstance(a, str) for a in argv),
            'successful declared build required')
    log = Path(log).resolve(strict=True)
    require(artifacts, 'build artifact inventory absent')
    value = {'schema': SCHEMA, 'source_start': source_start, 'source_end': source_end,
             'argv': argv, 'exit': exit_code, 'log': {'path': str(log), 'sha256': file_digest(log)},
             'artifacts': {str(Path(p).resolve(strict=True)): file_digest(p) for p in artifacts}}
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, 'wb') as stream:
        stream.write(canonical(value) + b'\n')
        stream.flush()
        os.fsync(stream.fileno())
    return {'path': str(Path(path)), 'sha256': file_digest(path)}


def admit(row, revision, expected_digest, root=ROOT):
    reference = row['build_sidecar']
    value = private_json(reference['path'])
    require(file_digest(reference['path']) == reference['sha256'], 'build sidecar changed')
    source = value['source_start']
    require(value['schema'] == SCHEMA and source == value['source_end']
            and source['candidate']['revision'] == revision
            and source_digest(source) == expected_digest, 'build sidecar source identity mismatch')
    verify_files(root, source)
    require(value['exit'] == 0 and isinstance(value['argv'], list) and value['argv']
            and all(isinstance(a, str) and a for a in value['argv']), 'successful build command absent')
    log = Path(value['log']['path'])
    require(log.is_absolute() and log.resolve(strict=True) == log and log.is_file()
            and not log.is_symlink() and log.stat().st_uid == os.geteuid()
            and log.stat().st_mode & 0o077 == 0
            and file_digest(log) == value['log']['sha256'], 'private build log changed')
    path = Path(row['path'])
    require(path.is_absolute() and path.resolve(strict=True) == path and path.is_file() and not path.is_symlink()
            and value['artifacts'].get(str(path)) == row['sha256'] == file_digest(path),
            'artifact is absent from source-bound build sidecar')
    require(row['source_revision'] == revision and row['source_digest'] == expected_digest
            and type(row['build_exit']) is int and row['build_exit'] == 0, 'artifact source binding mismatch')
    return path
