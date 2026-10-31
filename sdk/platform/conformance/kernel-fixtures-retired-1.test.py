from pathlib import Path


def fixtures():
    directory = Path(__file__).resolve().with_name("fixtures")
    names = [
        f"receipt-{kind}-v{version}.json"
        for kind in ("positive", "refusals")
        for version in (1, 2, 3)
    ]
    names.extend(
        f"generate_{kind}_fixture.c" for kind in ("receipt", "program_receipt")
    )
    return tuple(directory / name for name in names)


def test_kernel_fixture_group_1_absent():
    retired = fixtures()
    present = [str(path) for path in retired if path.exists() or path.is_symlink()]
    assert not present, "Retired kernel fixtures still exist:\n" + "\n".join(present)

    conformance = Path(__file__).resolve().parent
    deferred = {
        f"receipt-programs-positive-v{version}.json" for version in (1, 2, 3)
    }
    references = []
    for path in sorted(conformance.rglob("*")):
        if path.parent == conformance / "fixtures" and (
            path.match("generate_*.py") or path.name in deferred
        ):
            continue
        if path.is_file():
            content = path.read_bytes()
            for fixture in retired:
                if fixture.name.encode("utf-8") in content:
                    references.append(f"{path.relative_to(conformance)}: {fixture.name}")
    assert not references, "Retired kernel fixture references remain:\n" + "\n".join(
        references
    )
