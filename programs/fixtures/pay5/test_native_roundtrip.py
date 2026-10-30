import os
from pathlib import Path
import subprocess
import sys


def test_native_roundtrip_replays_without_history_feed(tmp_path):
    script = Path(__file__).with_name("run_native_roundtrip.py").resolve()
    result = subprocess.run(
        [sys.executable, str(script)],
        cwd=script.parents[3],
        env=os.environ | {"PAY5_QUAL_DIR": str(tmp_path)},
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr
