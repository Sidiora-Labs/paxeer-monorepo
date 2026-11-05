from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
BOUNDARY = ROOT / "services/agent/tests/boundary"


def test_boundary_node_fixtures_absent() -> None:
    assert BOUNDARY.is_dir(), f"boundary harness directory is missing: {BOUNDARY}"
    for name in (
        "layerxd_lni.c",
        "preparation_state.c",
        "production-lni-cases.kvx",
        "production-lni.env.example",
    ):
        fixture = BOUNDARY / "node" / name
        assert not fixture.exists() and not fixture.is_symlink(), (
            f"retired LNI node fixture remains: {fixture.relative_to(ROOT)}"
        )

    sources = sorted(
        path
        for path in BOUNDARY.rglob("*")
        if "target" not in path.relative_to(BOUNDARY).parts and path.is_file()
    )
    assert BOUNDARY / "cases.rs" in sources, "boundary cases source is missing"
    for source in sources:
        contents = source.read_bytes()
        for name in (b"lxp_daemon.h", b"lxp_guarantor.h", b"layerxd_lni"):
            assert name not in contents, (
                f"boundary source {source.relative_to(ROOT)} names retired {name.decode()}"
            )
