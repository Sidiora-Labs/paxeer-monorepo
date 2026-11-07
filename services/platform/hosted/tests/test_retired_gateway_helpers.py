from pathlib import Path


def test_gateway_local_harness_ships_no_kernel_helper():
    hosted = Path(__file__).resolve().parents[1]
    local = hosted / "gateway" / "tests" / "local"
    assert local.is_dir(), "Gateway local harness must exist"
    assert (local / "registry_image.py").is_file(), "Registry image helper must remain"

    retired_names = tuple(
        stem + ".py"
        for stem in ("checkpoint_settlement", "custody_chain", "registry_runtime")
    )
    for name in retired_names:
        helper = local / name
        assert not helper.exists(), f"Retired kernel helper remains: {name}"
        assert not helper.is_symlink(), f"Retired kernel helper symlink remains: {name}"

    references = []
    for path in sorted(hosted.rglob("*")):
        if not path.is_file():
            continue
        contents = path.read_bytes()
        for name in retired_names:
            if name == path.name or name.encode("ascii") in contents:
                references.append(f"{path.relative_to(hosted)}: {name}")

    assert not references, f"Retired kernel helper references remain: {references}"
