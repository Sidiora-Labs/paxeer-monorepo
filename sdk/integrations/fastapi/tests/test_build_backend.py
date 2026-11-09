from __future__ import annotations

from email.parser import BytesParser
from pathlib import Path
import runpy
import tomllib
import zipfile

from packaging.requirements import Requirement


def test_wheel_metadata_declares_test_extra(tmp_path: Path) -> None:
    root = Path(__file__).resolve().parents[1]
    backend = runpy.run_path(str(root / "build_backend.py"))
    project = tomllib.loads((root / "pyproject.toml").read_text(encoding="utf-8"))["project"]
    declared_test = {
        str(Requirement(value)) for value in project["optional-dependencies"]["test"]
    }
    requirements_file = {
        str(Requirement(line.strip()))
        for line in (root / "tests/requirements.txt").read_text(encoding="utf-8").splitlines()
        if line.strip()
    }
    assert declared_test == requirements_file
    assert declared_test == {"pytest", "fastapi>=0.110", "cryptography>=41", "httpx>=0.27"}
    declared_runtime = {str(Requirement(value)) for value in project["dependencies"]}

    wheels = []
    for directory in (tmp_path / "first", tmp_path / "second"):
        wheel_name = backend["build_wheel"](str(directory))
        wheel_path = directory / wheel_name
        wheels.append(wheel_path.read_bytes())
        with zipfile.ZipFile(wheel_path) as archive:
            metadata_paths = [
                name for name in archive.namelist() if name.endswith(".dist-info/METADATA")
            ]
            assert len(metadata_paths) == 1
            metadata = BytesParser().parsebytes(archive.read(metadata_paths[0]))

        assert "test" in metadata.get_all("Provides-Extra", [])
        requirements = [Requirement(value) for value in metadata.get_all("Requires-Dist", [])]
        runtime = [requirement for requirement in requirements if requirement.marker is None]
        conditional = [requirement for requirement in requirements if requirement.marker is not None]
        assert {str(requirement) for requirement in runtime} == declared_runtime
        assert len(runtime) == len(declared_runtime)
        assert {
            str(requirement).split(";", 1)[0].strip() for requirement in conditional
        } == declared_test
        assert len(conditional) == len(declared_test)
        for requirement in conditional:
            assert requirement.marker.evaluate({"extra": "test"})
            assert not requirement.marker.evaluate({"extra": ""})
            assert not requirement.marker.evaluate({"extra": "other"})

    assert wheels[0] == wheels[1]
