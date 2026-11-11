import hashlib
import json
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / "daemon/committed-fixture/vectors/vector-v4-committed.json"
COPY = ROOT / "services/interop/specs/conformance/committed/vector-v4-committed.json"


def test_copy_provenance_digest_matches_source():
    copied = json.loads(COPY.read_bytes())
    assert copied["provenance"]["source"] == SOURCE.relative_to(ROOT).as_posix()
    assert copied["provenance"]["sourceSha256"] == hashlib.sha256(
        SOURCE.read_bytes()
    ).hexdigest()


def test_copy_signed_receipt_matches_source():
    copied = json.loads(COPY.read_bytes())
    recorded = json.loads(SOURCE.read_bytes())
    assert copied["cases"][0]["id"] == "committed-payExact"
    assert copied["cases"][0]["signedReceipt"] == recorded["positive"]["receipt"]
