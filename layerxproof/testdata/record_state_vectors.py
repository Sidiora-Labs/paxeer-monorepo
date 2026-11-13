"""Refresh state verdicts with the current production Rust witness verifier."""

import json
from pathlib import Path
import subprocess
import tempfile


def main():
    directory = Path(__file__).resolve().parent
    root = directory.parents[1]
    fixture = directory / "vectors.json"
    document = json.loads(fixture.read_text())
    with tempfile.TemporaryDirectory(prefix="state-vector-recorder-") as temporary:
        temporary = Path(temporary)
        manifest = temporary / "Cargo.toml"
        manifest.write_text(
            '[package]\nname = "state-vector-recorder"\nversion = "0.1.0"\n'
            'edition = "2021"\n[workspace]\n[dependencies]\n'
            'sha2 = "=0.10.9"\nserde_json = "=1.0.151"\n'
            'layerx-types = { path = '
            + json.dumps(str(root / "services/agent/crates/layerx-types"))
            + ' }\n[[bin]]\nname = "record"\npath = '
            + json.dumps(str(directory / "record_state_vectors.rs")) + '\n'
        )
        result = subprocess.run(
            ["cargo", "run", "--quiet", "--manifest-path", str(manifest),
             "--target-dir", str(temporary / "target")],
            input=json.dumps(document["state"]), capture_output=True, text=True,
        )
        if result.returncode:
            raise RuntimeError(result.stderr)
        recorded = json.loads(result.stdout)
        # Keep the original field order and all non-state sections byte stable.
        previous = {case["name"]: case for case in document["state"]}
        document["state"] = [
            dict(previous.get(case["name"], {}), **case) for case in recorded
        ]
        document["generator"] = (
            "layerxproof/testdata/record_state_vectors.py (state); "
            "services/agent/crates/layerx-client/examples/go_verifier_vectors.rs "
            "(historical non-state sections)"
        )
        fixture.write_text(json.dumps(document, indent=2) + "\n")


if __name__ == "__main__":
    main()
