from pathlib import Path


fixtures = tuple(
    f"receipt-programs-{case}-v4.json"
    for case in ("executed", "principal", "mutated-leg")
) + ("generate_" + "executed_program_v4_fixture.py",)


def test_kernel_fixture_group_3_absent():
    conformance = Path(__file__).resolve().parent
    platform = conformance.parent
    retired = tuple(conformance / "fixtures" / name for name in fixtures)
    retired += (
        platform / "conformance" / ("terminal-" + "v4.test.py"),
        platform / "go" / ("terminal_" + "v4_test.go"),
        platform
        / "jvm/src/test/java/com/sidiora/layerx/sdk"
        / ("Terminal" + "V4Test.java"),
    )
    remaining = [
        str(path.relative_to(platform))
        for path in retired
        if path.exists() or path.is_symlink()
    ]
    assert not remaining, "Retired kernel files still exist:\n" + "\n".join(remaining)

    references = []
    for path in sorted(conformance.rglob("*")):
        if not path.is_file() or {"__pycache__", ".pytest_cache"}.intersection(
            path.parts
        ):
            continue
        if path.parent == conformance / "fixtures" and path.match("generate_*.py"):
            continue
        source = path.read_bytes()
        for name in fixtures:
            if name.encode("utf-8") in source:
                references.append(f"{path.relative_to(platform)}: {name}")
        for deleted_test in retired[4:]:
            if deleted_test.name.encode("utf-8") in source:
                references.append(
                    f"{path.relative_to(platform)}: {deleted_test.name}"
                )
    assert not references, "Retired kernel file references remain:\n" + "\n".join(
        references
    )
