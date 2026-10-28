import re
import shlex
from pathlib import Path, PurePosixPath

from programme.kvx import load
from programme.parity import command_for
from programme.retired import load_components

DATA = Path(__file__).resolve().parents[1] / "data/refusals.kvx"
REQUIRED_REFUSALS = {
    "0x1012": (
        "LXP_MODULE_DISABLED",
        "precompiles/layerxverify/retire_gate_test.go"
        "::TestEveryLayerXVerifyMethodRevertsModuleDisabledAfterTheMerge",
    ),
    "0x1013": (
        "LXP_MODULE_DISABLED",
        "precompiles/layerxcustody/retire_test.go"
        "::TestEveryCustodyMethodRevertsModuleDisabledFromTheMergePlanHeight",
    ),
    "0x1014": (
        "LXP_MODULE_DISABLED",
        "precompiles/layerxanchor/retired_surface_test.go"
        "::TestEveryAnchorSelectorRetired",
    ),
    "merge-plan": (
        "LXP_MODULE_DISABLED",
        "precompiles/common/retire_test.go::TestRetiredAtMergePlan",
    ),
    "custody-release": (
        "ErrModuleRetired code 29 (decision.654)",
        "modules/layerxcustody/keeper/release_test.go"
        "::TestReleaseRetiredIsRefusedBeforeTheMergePlanHeight",
    ),
    "exchange-intents": (
        "LXP_EXCHANGE_RETIRED code 14",
        "modules/layerxexchange/keeper/keeper_test.go"
        "::TestRecordIntentRefusedAfterRetirement",
    ),
}


def test_every_refusal_has_one_exact_command() -> None:
    refusals = load(DATA)
    assert refusals.pop("meta") == {"format_version": 1}
    assert refusals
    for surface, (expect, selector) in REQUIRED_REFUSALS.items():
        section = f"refusal.{surface}"
        assert section in refusals, section
        assert refusals[section]["expect"] == expect, section
        assert refusals[section]["test"] == selector, section

    for section, row in refusals.items():
        assert section.startswith("refusal.") and section != "refusal.", section
        assert set(row) == {"expect", "test", "command"}, section
        for key, value in row.items():
            assert isinstance(value, str) and value.strip(), (section, key)

        selector = row["test"]
        assert row["command"] == command_for({"selectors": [selector]}), section
        path, name = selector.split("::")
        assert path.endswith("_test.go"), section
        assert re.fullmatch(r"Test[A-Za-z0-9_]+", name), section
        args = shlex.split(row["command"])
        assert args == [
            "go",
            "test",
            "-count=1",
            f"./{PurePosixPath(path).parent}/",
            "-run",
            f"^({name})$",
        ], section
        pattern = args[-1]
        assert re.search(pattern, name), section
        for other in (f"{name}Extra", f"Other{name}", name[:-1], f"{name}/case"):
            assert re.search(pattern, other) is None, (section, other)


def test_every_retired_precompile_has_a_refusal() -> None:
    refusals = load(DATA)
    components = load_components()
    addressed = {
        name: component
        for name, component in components.items()
        if "address" in component
    }
    assert addressed
    for name, component in addressed.items():
        address = component["address"]
        assert isinstance(address, str), name
        assert re.fullmatch(r"0x[0-9a-fA-F]{4}", address), name
        section = f"refusal.{address}"
        assert section in refusals, (name, address)
        row = refusals[section]
        assert isinstance(row["expect"], str) and row["expect"].strip(), name
        assert isinstance(row["test"], str) and row["test"].strip(), name
        assert row["command"] == command_for({"selectors": [row["test"]]}), name
