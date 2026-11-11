import json
import unittest
from pathlib import Path

from layerx_sdk.mirror import decode_mirror_process_result, MirrorVerificationError


class MirrorProcessTests(unittest.TestCase):
    def test_process_exit_and_requested_batch_bind_success(self):
        transcript = json.loads((Path(__file__).resolve().parents[4]
            / "sdk/platform/go/testdata/repair141/mirror-transcript.json").read_text())
        exchange = transcript["exchanges"][0]
        response = exchange["response"]
        output = json.dumps(response).encode()
        height = transcript["coordinate"]["Height"]
        verified = decode_mirror_process_result(0, output, height, receipt=True)
        self.assertEqual(verified.committed_height, height)
        self.assertEqual(verified.committed_block_hash, bytes(transcript["coordinate"]["BlockHash"]))
        self.assertEqual(verified.level, "Committed")
        self.assertEqual(verified.evidence_digest.hex(), response["verification"]["evidenceDigest"])
        for status in [1, -15]:
            with self.assertRaises(MirrorVerificationError) as caught:
                decode_mirror_process_result(status, output, height)
            self.assertEqual(caught.exception.code, "unavailable")
        with self.assertRaises(MirrorVerificationError) as caught:
            decode_mirror_process_result(0, output, height + 1)
        self.assertEqual(caught.exception.code, "malformed")
        for field, value in (("level", "receipt-verified"), ("evidenceDigest", None)):
            changed = {**response, "verification": {**response["verification"], field: value}}
            with self.subTest(field=field), self.assertRaises(MirrorVerificationError) as caught:
                decode_mirror_process_result(0, json.dumps(changed).encode(), height, receipt=True)
            self.assertEqual(caught.exception.code, "malformed")


if __name__ == "__main__":
    unittest.main()
