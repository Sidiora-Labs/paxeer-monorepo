from pathlib import Path


def test_paxeer_deployment_ships_no_anchor_genesis():
    hosted = Path(__file__).resolve().parents[1]
    deployment = hosted / "paxeer"
    beta_cluster = hosted / "tests" / "beta-cluster.sh"
    assert deployment.is_dir(), "Paxeer deployment must exist"
    assert beta_cluster.is_file(), "Beta cluster script must exist"

    for name in ("anchor-genesis.py", "anchor-guarantors.sh"):
        retired = deployment / name
        assert not retired.exists(), f"Retired Paxeer script still exists: {retired}"
        assert not retired.is_symlink(), f"Retired Paxeer script is a symlink: {retired}"

    retired_names = (
        b"anchor-genesis",
        b"anchor-guarantors.sh",
        b"LAYERX_PAXEER_ANCHOR_GENESIS",
    )
    references = []
    for path in [*sorted(deployment.rglob("*")), beta_cluster]:
        if not path.is_file():
            continue
        contents = path.read_bytes()
        for name in retired_names:
            if name in contents:
                references.append(f"{path.relative_to(hosted)}: {name.decode('ascii')}")

    assert not references, f"Retired anchor genesis references remain: {references}"
