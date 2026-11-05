from hashlib import sha256
from pathlib import Path
import shutil
import subprocess
import tomllib


def test_receipt_outputs_match_pipeline_lock(tmp_path):
    root = Path(__file__).resolve().parents[5]
    lock = tomllib.loads((root / "sdk/platform/pipeline.kvx").read_text())
    for relative in (
        "sdk/platform/generators/receipt.kvx",
        "services/agent/schema/agent-api/programs.kvx",
        "programs/sdk/rust/src/abi_policy.rs",
    ):
        destination = tmp_path / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(root / relative, destination)
    generated = subprocess.run(
        [
            "cargo",
            "run",
            "--quiet",
            "--locked",
            "--manifest-path",
            str(root / "sdk/platform/generators/Cargo.toml"),
            "--bin",
            "layerx-platform-sdkgen",
            "--",
            "--write-program-contracts",
            str(tmp_path),
        ],
        cwd=root,
        capture_output=True,
        text=True,
        check=False,
    )
    assert generated.returncode == 0, generated.stdout + generated.stderr
    outputs = (
        (
            "agent-rust",
            "services/agent/crates/layerx-sdk/src",
            ("receipt_generated.rs",),
        ),
        (
            "agent-typescript",
            "services/agent/sdk/typescript/src/generated",
            ("receipt.ts",),
        ),
        (
            "agent-python",
            "services/agent/sdk/python/layerx_sdk/generated",
            ("receipt.py", "receipt.pyi"),
        ),
    )
    for name, directory, filenames in outputs:
        assert lock["output"][name]["root"] == directory
        for filename in filenames:
            path = root / directory / filename
            actual = path.read_bytes()
            assert actual == (tmp_path / directory / filename).read_bytes(), (
                f"{path.relative_to(root)} differs from generator output"
            )
            expected = lock["files"][name][filename]
            assert sha256(actual).hexdigest() == expected, (
                f"{path.relative_to(root)} differs from the resealed pipeline lock; "
                "regenerate with layerx-platform-sdkgen --write-program-contracts"
            )
