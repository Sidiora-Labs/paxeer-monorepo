"""Use the shared hosted tooling implementation from shell and Python producers."""
import fcntl
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[4]
DIRECTORY = Path(__file__).resolve().parent


def tool_path():
    binary = ROOT / "build/bin/hosted-beta-tool"
    binary.parent.mkdir(parents=True, exist_ok=True)
    with (binary.parent / ".hosted-beta-tool.lock").open("a") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX)
        sources = [DIRECTORY / "beta_runtime.go", DIRECTORY / "beta_topology.go",
                   DIRECTORY / "beta_readiness.go",
                   DIRECTORY / "cmd/hosted-beta-tool/main.go", ROOT / "go.mod", ROOT / "go.sum"]
        if not binary.exists() or any(source.stat().st_mtime_ns > binary.stat().st_mtime_ns
                                      for source in sources):
            subprocess.run(["go", "build", "-o", str(binary),
                            "./services/platform/hosted/tests/cmd/hosted-beta-tool"],
                           cwd=ROOT, check=True)
    return str(binary)


def run(arguments, data=None):
    return subprocess.run([tool_path(), *map(str, arguments)], input=data,
                          check=True, capture_output=True).stdout


if __name__ == "__main__":
    import sys
    os.execv(tool_path(), [tool_path(), *sys.argv[1:]])
