from pathlib import Path


def fixtures() -> tuple[str, ...]:
    return tuple(
        f"receipt-programs-{kind}.json"
        for kind in (
            "positive-v1",
            "positive-v2",
            "positive-v3",
            "refusals-v2",
            "executed-v3",
        )
    ) + tuple(
        f"generate_{kind}.py"
        for kind in ("program_receipt_refusals", "executed_program_fixture")
    )


def test_kernel_fixture_group_2_absent() -> None:
    conformance = Path(__file__).resolve().parent
    fixture_dir = conformance / "fixtures"
    retired = fixtures()
    assert fixture_dir.is_dir()
    remaining = [
        name
        for name in retired
        if (fixture_dir / name).exists() or (fixture_dir / name).is_symlink()
    ]
    assert not remaining, f"Retired kernel fixture files still exist: {remaining}"

    references = []
    for path in sorted(conformance.rglob("*")):
        relative = path.relative_to(conformance)
        assert path.name not in retired, (
            f"Retired kernel fixture still exists: {relative}"
        )
        if path.parent == fixture_dir and (
            path.match("generate_*.py")
            or path.name == "generate_program_receipt_fixture.c"
        ):
            continue
        if path.is_file():
            contents = path.read_bytes()
            for name in retired:
                if name.encode("utf-8") in contents:
                    references.append(f"{relative}: {name}")
    assert not references, "Retired kernel fixture references remain:\n" + "\n".join(
        references
    )
