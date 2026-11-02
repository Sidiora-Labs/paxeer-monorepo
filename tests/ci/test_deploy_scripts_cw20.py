import re
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def test_deploy_scripts_use_cw20_base():
    scripts = (
        "tests/integration/contracts/deploy_wasm_contracts.sh",
        "tools/benchmark/loadtest/contracts/deploy_ten_contracts.sh",
    )
    assert (ROOT / "contracts/wasm/cw20_base.wasm").is_file()
    for script in scripts:
        path = ROOT / script
        source = path.read_text(encoding="utf-8")
        assert "saturn.wasm" not in source, script
        assert "venus.wasm" not in source, script
        stores = re.findall(
            r'(?:store_wasm|tx wasm store)\s+("[^"]+"|[^\s)]+)', source
        )
        cw20_stores = [store for store in stores if "cw20_base.wasm" in store]
        expected = (
            '"$paxhome/contracts/wasm/cw20_base.wasm"'
            if script.startswith("tools/")
            else "contracts/wasm/cw20_base.wasm"
        )
        assert cw20_stores == [expected, expected], script
        result = subprocess.run(
            ["bash", "-n", str(path)], capture_output=True, text=True, check=False
        )
        assert result.returncode == 0, f"{script}: {result.stdout}{result.stderr}"
