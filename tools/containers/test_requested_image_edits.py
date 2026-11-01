from pathlib import Path
import re
import subprocess
import tomllib


ROOT = Path(__file__).resolve().parents[2]


def test_images_copy_no_guarantor_script():
    image_files = sorted(
        path
        for path in (ROOT / "docker").rglob("*")
        if path.is_file()
        and (
            path.name == "Dockerfile"
            or path.name.startswith("Dockerfile.")
            or path.name.endswith(".Dockerfile")
            or path.name == "init.sh"
        )
    )
    assert image_files, "Dockerfiles and init scripts must exist"
    for path in image_files:
        source = path.read_text(encoding="utf-8")
        for script in ("guarantor.sh", "checkpoint-authority.py"):
            assert script not in source, f"{path.relative_to(ROOT)} names {script}"
    result = subprocess.run(
        ["bash", "-n", str(ROOT / "docker/kernel/init.sh")],
        capture_output=True,
        text=True,
    )
    assert result.returncode == 0, result.stderr


def test_kernel_image_copies_no_relay_archive():
    source = (ROOT / "docker/kernel/Dockerfile").read_text(encoding="utf-8")
    assert "/opt/layerx/relay_archive" not in source


def test_images_build_no_explorer_index():
    for name in ("human-service", "kernel"):
        path = ROOT / "docker" / name / "Dockerfile"
        source = path.read_text(encoding="utf-8")
        assert "layerx-explorer-index" not in source, str(path.relative_to(ROOT))


def test_rollout_has_no_explorer_index_unit():
    source = (ROOT / "deploy/rollout.kvx").read_text(encoding="utf-8")
    assert "layerx-explorer-index.service" not in source
    assert not re.search(r"^\s*\[service\.explorer-index\]\s*$", source, re.MULTILINE)
    assert not re.search(
        r'^\s*box_role\s*=\s*"explorer-index"', source, re.MULTILINE
    )
    services = tomllib.loads(source)["service"]
    assert "explorer-index" not in services
    for name, service in services.items():
        for value in service.values():
            if isinstance(value, str):
                assert "layerx-explorer-index" not in value, name
