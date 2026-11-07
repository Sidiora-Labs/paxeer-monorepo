"""Keep the complete qualification directory collectable at the current tree."""

import importlib.util
from pathlib import Path
import re
import subprocess
import sys


ROOT = Path(__file__).resolve().parents[2]


def test_qualification_modules_collect():
    result = subprocess.run(
        [sys.executable, "-m", "pytest", "--collect-only", "-q", "tools/qualification"],
        cwd=ROOT,
        capture_output=True,
        text=True,
    )
    output = result.stdout + result.stderr
    assert result.returncode == 0, output
    assert re.search(r"\b[1-9][0-9]* tests? collected\b", output), output
    assert "ERROR collecting" not in output, output


def test_gateway_env_items_are_read_by_name():
    path = ROOT / "tools/qualification/audit_repairs/test_repair_22_33_1395.py"
    spec = importlib.util.spec_from_file_location("gateway_receipt_key_audit", path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)

    entries = module.CONTAINER["env"]
    assert len(module.ENV) == len(entries)
    assert set(module.ENV) == {item["name"] for item in entries}
    for item in entries:
        if "value" in item:
            assert module.ENV[item["name"]] == item["value"]
        else:
            assert module.ENV[item["name"]] in item["valueFrom"].values()
    for name, key in (
        ("LAYERX_GATEWAY_FAUCET_URL", "faucet-url"),
        ("LAYERX_GATEWAY_REDIS_URL", "redis-url"),
    ):
        assert module.ENV[name] == {
            "name": "layerx-gateway-upstreams",
            "key": key,
            "optional": False,
        }
