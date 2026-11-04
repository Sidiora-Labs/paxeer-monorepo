"""Regenerate the C-host transcript; this recorder is never run by the Go gate."""

import hashlib
import json
from pathlib import Path
import subprocess
import tempfile


def main():
    directory = Path(__file__).resolve().parent
    root = directory.parents[2]
    host = root / "modules/lxphost/host.go"
    source = host.read_text()
    preamble = source.split("/*", 1)[1].split("*/", 1)[0]
    preamble = "\n".join(
        line for line in preamble.splitlines()
        if not line.strip().startswith("#cgo ")
    )
    with tempfile.TemporaryDirectory(prefix="repair-m2-recorder-") as temporary:
        temporary = Path(temporary)
        (temporary / "lxphost_native.h").write_text(preamble)
        binary = temporary / "record"
        library = root / "build/liblayerx.a"
        subprocess.run([
            "cc", "-std=c17", "-O2", "-Werror=implicit-function-declaration", "-I" + str(temporary),
            "-I" + str(root / "include"),
            "-I" + str(root / "build/generated"),
            str(directory / "repair_m2.c"), str(library),
            str(root / "programs/target/release/liblayerx_programs_sandbox.a"),
            str(library), "-lssl", "-lcrypto", "-pthread", "-ldl", "-lm",
            "-o", str(binary),
        ], check=True)
        store = temporary / "store"
        store.mkdir()
        result = subprocess.run([str(binary), str(store)], check=True,
                                capture_output=True, text=True)
        transcript = json.loads(result.stdout)
        transcript["source_hashes"] = {
            str(path.relative_to(root)): hashlib.sha256(path.read_bytes()).hexdigest()
            for path in [host, directory / "repair_m2.c",
                         root / "src/state/lxp_state_root.c",
                         root / "src/state/lxp_state_proof.c",
                         root / "src/state/lxp_snapshot.c",
                         root / "src/storage/lxp_snapshot_store.c"]
        }
        (directory / "repair_m2.json").write_text(
            json.dumps(transcript, indent=2, sort_keys=True) + "\n"
        )


if __name__ == "__main__":
    main()
