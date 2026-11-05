import re
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
INTEGRATION_SCRIPT = "tests/integration/contracts/deploy_wasm_contracts.sh"
LOADTEST_SCRIPT = "tools/benchmark/loadtest/contracts/deploy_ten_contracts.sh"
LOADTEST_IMPLEMENTATION = "tools/benchmark/loadtest/operations/deployment.go"


def test_deploy_scripts_use_cw20_base():
    assert (ROOT / "contracts/wasm/cw20_base.wasm").is_file()

    source = (ROOT / INTEGRATION_SCRIPT).read_text(encoding="utf-8")
    assert "saturn.wasm" not in source, INTEGRATION_SCRIPT
    assert "venus.wasm" not in source, INTEGRATION_SCRIPT
    stores = re.findall(r'(?:store_wasm|tx wasm store)\s+("[^"]+"|[^\s)]+)', source)
    cw20_stores = [store for store in stores if "cw20_base.wasm" in store]
    assert cw20_stores == ["contracts/wasm/cw20_base.wasm"] * 2, INTEGRATION_SCRIPT

    source = (ROOT / LOADTEST_IMPLEMENTATION).read_text(encoding="utf-8")
    for retired in ("jupiter", "mars", "saturn", "venus"):
        assert retired not in source, LOADTEST_IMPLEMENTATION
    stores = re.findall(r'tx\("code_id",\s*"wasm",\s*"store",\s*([^\n]+)\)', source)
    expected = 'filepath.Join(options.Root, "contracts", "wasm", "cw20_base.wasm")'
    assert stores == [expected, expected], LOADTEST_IMPLEMENTATION

    for script in (INTEGRATION_SCRIPT, LOADTEST_SCRIPT):
        result = subprocess.run(
            ["bash", "-n", str(ROOT / script)],
            capture_output=True,
            text=True,
            check=False,
        )
        assert result.returncode == 0, f"{script}: {result.stdout}{result.stderr}"
