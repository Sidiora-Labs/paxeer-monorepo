"""Owned Vitest runner; preserve subprocess exit independently of its wrapper."""

import json
import pathlib
import subprocess
import sys

root = pathlib.Path(__file__).resolve().parents[4]
command = [
    str(root / 'apps/launchpad/web/node_modules/.bin/vitest'),
    'run',
    '--config',
    'apps/launchpad/qualification/account-ui/vitest.config.mts',
]


def main():
    result = subprocess.run(command, cwd=root)
    evidence = root / 'launchpad/evidence/production-system/L31/phase1/native-exit.json'
    evidence.write_text(json.dumps({'command': command, 'exit_code': result.returncode}) + '\n')
    return result.returncode


if __name__ == '__main__':
    sys.exit(main())
