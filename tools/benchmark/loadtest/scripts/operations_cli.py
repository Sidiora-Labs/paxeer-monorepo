"""Transport script requests to the benchmark operations executable."""

import os
import subprocess
from pathlib import Path


def executable():
    return os.environ.get(
        "PAX_LOADTEST_OPS",
        str(Path(__file__).resolve().parents[4] / "build" / "loadtest"),
    )


def run(operation, arguments):
    os.execv(executable(), [executable(), "benchmark-ops", operation, *arguments])


def request(operation, payload):
    return subprocess.run(
        [executable(), "benchmark-ops", operation],
        input=payload,
        text=True,
        capture_output=True,
        timeout=60,
        check=False,
    )
