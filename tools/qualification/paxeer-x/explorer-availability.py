#!/usr/bin/env python3
import json
import os
from pathlib import Path
import subprocess
import sys

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[3]


def main():
    evidence = Path(os.environ['PAXEER_X_EVIDENCE_DIR'])
    if not evidence.is_dir() or evidence.stat().st_mode & 0o077:
        raise RuntimeError('availability gate requires owner-only evidence directory')
    outcomes = []
    browser_log = evidence / 'explorer-availability-browser.log'
    with browser_log.open('wb') as output:
        result = subprocess.run(['npm', '--prefix', str(ROOT / 'apps/web'), 'run', 'typecheck'],
                                cwd=ROOT, stdout=output, stderr=subprocess.STDOUT, timeout=240)
    browser_log.chmod(0o600)
    outcomes.append({'gate': 'actual_browser_typecheck', 'exit_code': result.returncode, 'log_path': str(browser_log)})
    print(json.dumps({'outcomes': outcomes, 'skipped': 0}, sort_keys=True))
    return 0 if all(outcome['exit_code'] == 0 for outcome in outcomes) else 1


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (KeyError, OSError, RuntimeError, ValueError, subprocess.TimeoutExpired) as error:
        print(str(error), file=sys.stderr)
        sys.exit(3)
