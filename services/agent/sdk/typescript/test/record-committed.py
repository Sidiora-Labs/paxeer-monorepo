"""Convert current producer evidence into the in-scope committed recording."""
import hashlib
import importlib.util
import json
from pathlib import Path

root = Path(__file__).resolve().parents[5]
converter_path = root / "sdk/platform/conformance/fixtures/refresh_receipt_vectors.py"
module_spec = importlib.util.spec_from_file_location("receipt_converter", converter_path)
converter = importlib.util.module_from_spec(module_spec)
module_spec.loader.exec_module(converter)
recording = json.loads((root / "sdk/platform/conformance/fixtures/vector-v4-committed.json").read_text())
source_bytes = (root / recording["provenance"]["source"]).read_bytes()
source = json.loads(source_bytes)
recording["provenance"]["sha256"] = hashlib.sha256(source_bytes).hexdigest()
recording["positive"] = converter._committed(source)
recording["refusals"] = [{
    "id": "insufficient-power",
    "nextCommit": converter._commit(source["settlement"]["commit_at_most_two_thirds"]),
    "expectedCheck": "committed-next-commit",
}]
(Path(__file__).parent / "fixtures/repair-22-33-24/committed.json").write_text(json.dumps(recording, indent=2) + "\n")
