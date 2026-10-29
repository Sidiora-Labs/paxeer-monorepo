from pathlib import Path
import subprocess


DIRECTORY = Path(__file__).resolve().parent


def test_beta_cluster_deploys_no_relay_archive():
    for script in ("beta-images.sh", "beta-cluster.sh"):
        subprocess.run(
            ["bash", "-n", str(DIRECTORY / script)],
            check=True,
            capture_output=True,
            text=True,
        )

    images_path = DIRECTORY / "beta-images.sh"
    images = images_path.read_text()
    assert "layerx-relay-archive" not in images

    listed = subprocess.run(
        [
            "bash",
            "--noprofile",
            "--norc",
            "-c",
            'source "$1"\nprintf \'%s\\n\' "${IMAGE_NAMES[@]}"',
            "beta-images-test",
            str(images_path),
        ],
        check=True,
        capture_output=True,
        text=True,
    ).stdout.splitlines()
    assert "layerx-node" in listed
    assert "layerx-relay-archive" not in listed

    cluster = (DIRECTORY / "beta-cluster.sh").read_text()
    for retired in (
        "relay_archive",
        "relay-archive",
        "layerx-archive-codec",
        "RELAY_NODE_SOURCE_DIR",
        "RELAY_DATA_DIR",
        "RELAY_TLS_DIR",
        "RELAY_CODEC",
        "RELAY_HOST",
        "LAYERX_BETA_RELAY_",
    ):
        assert retired not in cluster, f"Beta cluster still references {retired}"
