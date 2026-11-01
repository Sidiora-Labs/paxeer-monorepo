import re
from pathlib import Path

from programme.activation import PLAN_PATH, constant_sites
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]


def test_node_merge_constant_is_derived() -> None:
    plan = load(PLAN_PATH)["plan.v7.0"]
    assert plan["declaring_constant"] == "pcommon.MergeUpgrade"
    assert plan["constant_site"] == "precompiles/common/precompiles.go:30"
    node = (ROOT / "node/upgrades.go").read_text(encoding="utf-8")
    assert re.search(
        r"^const MergeUpgrade = pcommon\.MergeUpgrade\s*$", node, re.MULTILINE
    )
    setup = (ROOT / "precompiles/setup.go").read_text(encoding="utf-8")
    assert re.search(
        r"^const MergeUpgrade = pcommon\.MergeUpgrade\s*$", setup, re.MULTILINE
    )
    sites = constant_sites(ROOT)
    assert not any(path.startswith("node/") for path in sites)
    assert "precompiles/setup.go" not in sites
    assert sites == [
        "modules/layerxanchor/types/retire.go",
        "modules/layerxcustody/types/retire.go",
        "modules/xweb/keeper/activation.go",
        "precompiles/common/precompiles.go",
    ]
