"""Record through the committed daemon fixture without editing its source tree."""
import json
from pathlib import Path
import subprocess
import tempfile


def main():
    support = Path(__file__).resolve().parent
    root = next(p for p in support.parents if (p / "go.mod").is_file())
    with tempfile.TemporaryDirectory(prefix="recovery-22-33-47-") as directory:
        temporary = Path(directory)
        node = temporary / "paxd"
        subprocess.run(["make", "-j2", "build/liblayerx.a"], cwd=root, check=True)
        subprocess.run(["cargo", "build", "--locked", "--manifest-path",
                        "programs/Cargo.toml", "--package", "layerx-programs-sandbox",
                        "--features", "host-ffi", "--lib"], cwd=root, check=True)
        subprocess.run(["go", "build", "-o", str(node), "./daemon/paxd"],
                       cwd=root, check=True)
        overlay = temporary / "overlay.json"
        overlay.write_text(json.dumps({"Replace": {
            str(root / "daemon/committed-fixture/recovery_22_33_47.go"):
                str(support / "recovery.go")}}))
        subprocess.run(["go", "run", "-overlay", str(overlay),
                        "./daemon/committed-fixture", "record", "--paxd", str(node),
                        "--out", str(support), "--vector", "recovery-22-33-47"],
                       cwd=root, check=True)


if __name__ == "__main__":
    main()
