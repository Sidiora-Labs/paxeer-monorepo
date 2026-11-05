import pathlib
import shutil
import unittest

from authoritative_inputs import OWNER, RECORDED, ROOT, load_module, operation_catalog


def test_examples_paths_use_monorepo_layout(tmp_path):
    package = ROOT / "sdk/launchpad/packages/sdk"
    module = load_module(package / "scripts/examples.py", "sdk_examples")
    recorded_operations = operation_catalog()
    copied = tmp_path / RECORDED
    copied.parent.mkdir(parents=True)
    shutil.copyfile(ROOT / RECORDED, copied)
    assert copied.read_bytes() == (ROOT / RECORDED).read_bytes()
    assert operation_catalog(copied) == recorded_operations

    for selected_root in (ROOT, tmp_path):
        contracts, output = module.examples_paths(selected_root)
        assert contracts == selected_root / OWNER / "v2/openapi.json"
        assert output == selected_root / "sdk/launchpad/packages/sdk/examples"
        with unittest.TestCase().assertRaisesRegex(ValueError, "Missing codegen manifest:.*complete immutable"):
            module.emit(selected_root)
    assert not (tmp_path / "sdk/launchpad/packages/sdk/examples").exists()
    assert module.examples_paths(ROOT)[1] == package / "examples"
    pagination = (package / "examples/pagination.mjs").read_text()
    snapshot = (package / "examples/snapshot-resume.mjs").read_text()
    assert "api.call('IF_TRADES'" in pagination
    assert "snapshotPath: '/v2/snapshots'" in snapshot
    assert "SnapshotStreamV2" in snapshot
    assert recorded_operations["IF_TRADES"] == ("GET", "/v2/pools/{poolAddress}/trades")
    assert recorded_operations["IF_TOPIC_SNAPSHOT_READ"] == ("GET", "/v2/snapshots")
