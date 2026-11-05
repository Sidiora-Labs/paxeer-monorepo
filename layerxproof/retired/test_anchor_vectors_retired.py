import subprocess
from pathlib import Path


def test_anchor_vectors_generator_absent() -> None:
    root = Path(__file__).resolve().parents[2]
    layerxproof = root / "layerxproof"
    generator = layerxproof / "testdata" / "anchor_vectors.c"
    assert not (generator.exists() or generator.is_symlink()), (
        "the retired anchor vector generator must be absent"
    )

    retired_object = b"_".join((b"lxp", b"daemon", b"evidence"))
    references = [
        str(path.relative_to(root))
        for path in sorted(layerxproof.rglob("*"))
        if path.is_file() and retired_object in path.read_bytes()
    ]
    assert not references, f"retired daemon evidence references remain: {references}"

    result = subprocess.run(
        ["make", "-n", "build", "test"],
        cwd=root,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, (
        f"make -n build test exited {result.returncode}\n"
        f"stdout:\n{result.stdout}\nstderr:\n{result.stderr}"
    )
