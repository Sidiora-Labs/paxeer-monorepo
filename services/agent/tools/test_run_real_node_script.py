from pathlib import Path


def test_script_names_no_custody_proof():
    script = Path(__file__).with_name("run-real-node-tests.sh").read_text(
        encoding="utf-8"
    )

    assert "layerx-custody-proof" not in script
    assert "LAYERX_CUSTODY_PROOF_BIN" not in script
    assert "export LAYERX_TEST_NATIVE_BIN_DIR=$LAYERX_TEST_NATIVE_BUILD_DIR/bin" in script
    assert "export PAXD=$repo_root/build/paxd" in script
