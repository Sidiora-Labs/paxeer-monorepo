"""Derive the offline test vector from this crate's fresh paxd recording."""

from pathlib import Path
import json
import runpy


def main():
    root = Path(__file__).resolve().parents[6]
    crate = Path(__file__).resolve().parents[2]
    recording = crate / "tests/fixtures/committed/vector-v4-committed.json"
    converter = runpy.run_path(
        str(
            root
            / "services/interop/crates/layerx-committed-proof/scripts/record_committed_vectors.py"
        )
    )["record_committed_vectors"]
    vector = converter(recording.read_bytes(), recording.relative_to(root).as_posix())
    recording.with_name("vector-v4-conformance.json").write_text(
        json.dumps(vector, indent=2) + "\n"
    )


if __name__ == "__main__":
    main()
