import re

from retired_tests import REPO_ROOT, assert_paths_absent, build_and_run_c_suite


def assert_no_kernel_references(pattern, *, exclude=()):
    references = []
    excluded = [REPO_ROOT / path for path in exclude]
    sources = sorted(
        path
        for path in (REPO_ROOT / "tests").rglob("*")
        if path.is_file()
        and path.suffix in {".c", ".h"}
        and not any(path == tree or tree in path.parents for tree in excluded)
    )
    assert sources, "No kernel test sources found"
    for path in sources:
        for number, line in enumerate(
            path.read_text(encoding="utf-8").splitlines(), start=1
        ):
            if re.search(pattern, line):
                references.append(f"{path.relative_to(REPO_ROOT)}:{number}: {line}")
    assert not references, "Retired kernel references:\n" + "\n".join(references)


def test_no_receive_or_grant_store_caller():
    assert_paths_absent(["tests/test_gateway_receive.c"])
    registry = (REPO_ROOT / "tests/modules/test_asset_registry.c").read_text(
        encoding="utf-8"
    )
    assert not re.search(r"\blxp_grant_store\w*\b", registry)
    assert not re.search(
        r"\b(?:test_payer_grant_read|lx_asset_payer_grant_read)\b", registry
    )


def test_qual_replay_table_matches_header():
    header = (REPO_ROOT / "include/layerx/lxp_qualification.h").read_text(
        encoding="utf-8"
    )
    replay = (REPO_ROOT / "tests/qualification/lxp_qual_replay.c").read_text(
        encoding="utf-8"
    )
    count = re.search(r"\bLXP_QUAL_ACTIVITY_TYPE_COUNT\s*=\s*(\d+)\b", header)
    assert count is not None, "Qualification activity count is missing"
    table = re.search(
        r"\bactivity_types\s*\[\s*\]"
        r"\s*=\s*\{([^}]+)\}\s*;",
        replay,
    )
    assert table is not None, "Qualification activity table is missing"
    entries = table.group(1).split(",")
    if not entries[-1].strip():
        entries.pop()
    values = []
    for entry in entries:
        value = re.fullmatch(r"\s*UINT32_C\(\s*(0x[0-9a-fA-F]+)\s*\)\s*", entry)
        assert value is not None, f"Invalid qualification activity entry: {entry!r}"
        values.append(int(value.group(1), 16))
    assert len(values) == int(count.group(1)), (
        f"Replay has {len(values)} entries; header declares {count.group(1)}"
    )
    assert values == sorted(set(values)), "Activity types must be unique and sorted"
    assert 0x0003000B not in values


def test_no_handover_genesis_case():
    kernel = (REPO_ROOT / "tests/protocol/lxp_test_kernel.c").read_text(
        encoding="utf-8"
    )
    assert not re.search(r"\blxp_handover\w*\b", kernel)
    assert "web_handover_checks" not in kernel
    build_and_run_c_suite("build/tests/lxp_test_kernel")


def test_no_bridge_credit_apply_caller():
    assert_no_kernel_references(r"\blxp_ledger_apply_bridge_credit\b")
