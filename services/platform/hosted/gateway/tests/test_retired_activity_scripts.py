from pathlib import Path


def test_gateway_tests_ship_no_activity_script():
    tests = Path(__file__).resolve().parent
    retired_scripts = (
        "hosted-boundary.sh",
        "run-load.sh",
        "load.js",
        "lifecycle-boundary.sh",
    )
    remaining_scripts = [
        name
        for name in retired_scripts
        if (tests / name).exists() or (tests / name).is_symlink()
    ]
    assert not remaining_scripts, f"Retired gateway scripts remain: {remaining_scripts}"

    activity_route = b"/".join((b"", b"v1", b"activities"))
    references = []
    for path in sorted(tests.rglob("*")):
        relative = path.relative_to(tests)
        if (
            relative.parts[0] == "local"
            or path.suffix not in {".sh", ".js", ".json"}
            or not path.is_file()
        ):
            continue
        if activity_route in path.read_bytes():
            references.append(str(relative))

    assert not references, f"Retired activity route remains in gateway tests: {references}"
