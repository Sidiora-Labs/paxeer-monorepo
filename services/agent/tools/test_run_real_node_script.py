from pathlib import Path
import subprocess

import pytest


def test_script_names_no_custody_proof():
    script = Path(__file__).with_name("run-real-node-tests.sh").read_text(
        encoding="utf-8"
    )

    assert "layerx-custody-proof" not in script
    assert "custody-proof-build" not in script
    assert "sign-credit" not in script
    assert "LAYERX_CUSTODY_PROOF_BIN" not in script
    assert "export LAYERX_TEST_NATIVE_BIN_DIR=$LAYERX_TEST_NATIVE_BUILD_DIR/bin" in script
    assert "export PAXD=$repo_root/build/paxd" in script


@pytest.mark.parametrize("target", ["agent-test", "agent-test-sanitize"])
def test_real_node_make_dry_run_names_no_custody_proof(target):
    repo_root = Path(__file__).resolve().parents[3]
    result = subprocess.run(
        ["make", "--no-print-directory", "-n", target],
        cwd=repo_root,
        capture_output=True,
        text=True,
        check=True,
    )

    assert "run-real-node-tests.sh prepare" in result.stdout
    assert "custody-proof" not in result.stdout
    assert "LAYERX_CUSTODY_PROOF_BIN" not in result.stdout
    assert "custody-proof" not in result.stderr
