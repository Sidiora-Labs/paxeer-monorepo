"""Reviewed immutable xweb Forge library identities and byte checks."""

import hashlib
from pathlib import Path
import re
import subprocess
import sys


PINS = {
    'forge-std': ('https://github.com/foundry-rs/forge-std', 'v1.9.6',
                  '3b20d60d14b343ee4f908cb8079495c07f5e8981'),
    'openzeppelin-contracts': ('https://github.com/OpenZeppelin/openzeppelin-contracts', 'v5.3.0',
                              'e4f70216d759d8e6a64144a9e1f7bbeed78e7079'),
}


class DependencyError(ValueError):
    pass


def validate_identity(name, origin, head, tag_commit, status):
    url, tag, digest = PINS[name]
    if origin != url or head != digest or tag_commit != digest:
        raise DependencyError('dependency repository, revision or reviewed digest differs')
    if status:
        raise DependencyError('dependency has modified, untracked or ignored contents')
    return digest


def validate_bootstrap(source):
    for prefix, name in [('FORGE_STD', 'forge-std'), ('OPENZEPPELIN', 'openzeppelin-contracts')]:
        url, tag, _ = PINS[name]
        for suffix, wanted in [('URL', url), ('TAG', tag)]:
            matches = re.findall(r'^' + prefix + '_' + suffix + r"='([^']+)'$", source, re.M)
            if matches != [wanted]:
                raise DependencyError('bootstrap dependency pin differs from reviewed source')


def git(directory, *arguments):
    result = subprocess.run(['git', '-C', str(directory), *arguments],
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
    if result.returncode != 0:
        raise DependencyError('dependency Git inspection failed')
    return result.stdout


def verify_blob(path, mode, expected, stage):
    # The reviewed OZ tag has three test-only gitlinks. Bootstrap leaves them
    # uninitialized; accept only empty directories, never extra source there.
    if mode == b'160000' and stage == b'0':
        if path.is_symlink() or not path.is_dir() or any(path.iterdir()):
            raise DependencyError('dependency contains unreviewed submodule source')
        return
    if stage != b'0' or mode not in (b'100644', b'100755') or path.is_symlink():
        raise DependencyError('dependency has an unsupported tracked input')
    data = path.read_bytes()
    actual = hashlib.sha1(b'blob ' + str(len(data)).encode() + b'\0' + data).hexdigest().encode()
    if actual != expected or bool(path.stat().st_mode & 0o111) != (mode == b'100755'):
        raise DependencyError('dependency tracked bytes differ from the index')


def verify_tracked_bytes(directory):
    rows = git(directory, 'ls-files', '--stage', '-z').split(b'\0')
    if not any(rows):
        raise DependencyError('dependency tracked inventory is empty')
    for row in rows:
        if not row:
            continue
        metadata, name = row.split(b'\t', 1)
        mode, expected, stage = metadata.split()
        path = directory / name.decode('utf-8')
        verify_blob(path, mode, expected, stage)


def main():
    root = Path(__file__).resolve().parents[2]
    try:
        validate_bootstrap((root / 'contracts/bootstrap-libs.sh').read_text())
        workflow = (root / '.github/workflows/paxeer-forge-test.yml').read_text()
        if 'bash contracts/bootstrap-libs.sh' not in workflow:
            raise DependencyError('Forge workflow does not use the reviewed bootstrap')
        for name, (url, tag, digest) in PINS.items():
            directory = root / 'contracts/lib' / name
            if not directory.exists():
                directory.parent.mkdir(parents=True, exist_ok=True)
                result = subprocess.run(['git', 'clone', '--quiet', '--depth', '1', '--branch',
                                         tag, url, str(directory)], check=False)
                if result.returncode != 0:
                    raise DependencyError('dependency clone failed')
            if directory.is_symlink() or not (directory / '.git').is_dir():
                raise DependencyError('dependency is not an independent clone')
            origin = git(directory, 'config', '--get', 'remote.origin.url').decode().strip()
            head = git(directory, 'rev-parse', 'HEAD').decode().strip()
            tag_commit = git(directory, 'rev-parse', '--verify', 'refs/tags/' + tag + '^{commit}').decode().strip()
            status = git(directory, 'status', '--porcelain=v1', '--untracked-files=all',
                         '--ignored=matching', '--ignore-submodules=none')
            validate_identity(name, origin, head, tag_commit, status)
            # Compare bytes as well as status: assume-unchanged/skip-worktree cannot hide edits.
            verify_tracked_bytes(directory)
            print(f'xweb-forge-libs: {name} {tag} commit={digest} tracked-bytes=verified clean=true')
    except (OSError, ValueError, KeyError) as error:
        print('xweb-forge-libs: refused: ' + str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
