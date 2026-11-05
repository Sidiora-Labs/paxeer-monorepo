from __future__ import annotations

import json
import sys
import unittest
from dataclasses import replace
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
sys.path.insert(0, str(ROOT / "services/agent/sdk/python"))

from layerx_sdk.generated.receipt import (  # noqa: E402
    PROGRAM_ABI_V5,
    supports_program_guest_abi,
)
from layerx_sdk.native_program_call import (  # noqa: E402
    decode_native_program_call,
    encode_native_program_call,
    native_guest_abi_for_protocol,
)

FIXTURES = ROOT / "sdk/platform/conformance/fixtures"
CALL = json.loads((FIXTURES / "native-program-call-v4.json").read_text())


class NativeAbi5Test(unittest.TestCase):
    def test_generated_policy_admits_exactly_the_fifth_version(self) -> None:
        self.assertEqual(PROGRAM_ABI_V5, 5)
        self.assertEqual([supports_program_guest_abi(abi) for abi in (0, 1, 2, 3, 4, 5, 6, 65_535)],
                         [False, True, True, True, True, True, False, False])
        self.assertEqual([native_guest_abi_for_protocol(5, protocol) for protocol in (1, 2, 3, 4)],
                         [False, False, True, False])
        self.assertEqual([native_guest_abi_for_protocol(abi, 2) for abi in (1, 2, 3, 4, 5)],
                         [True, True, False, False, False])

    def test_native_call_encodes_and_decodes_guest_abi_5(self) -> None:
        payload_v4 = bytes.fromhex(CALL["payload_hex"])
        self.assertEqual(payload_v4[32:34], b"\0\x02")
        payload_v5 = payload_v4[:32] + b"\0\x05" + payload_v4[34:]
        decoded = decode_native_program_call(payload_v5)
        self.assertEqual(decoded.guest_abi, 5)
        self.assertEqual(encode_native_program_call(decoded), payload_v5)
        from_v4 = decode_native_program_call(payload_v4)
        self.assertEqual(replace(from_v4, guest_abi=5), decoded)
        self.assertEqual(encode_native_program_call(replace(from_v4, guest_abi=5)), payload_v5)
        self.assertEqual(decoded.resources, tuple(int(value) for value in CALL["resources"]))
        for abi in (0, 6, 65_535):
            with self.assertRaisesRegex(ValueError, "^invalid native program call$"):
                decode_native_program_call(payload_v4[:32] + abi.to_bytes(2, "big") + payload_v4[34:])
            with self.assertRaisesRegex(ValueError, "^invalid native program call$"):
                encode_native_program_call(replace(from_v4, guest_abi=abi))

    def test_guest_abi_5_codec_rejects_truncation_and_trailing_bytes(self) -> None:
        native = replace(decode_native_program_call(bytes.fromhex(CALL["payload_hex"])), guest_abi=5)
        canonical = encode_native_program_call(native)
        for length in range(len(canonical)):
            with self.subTest(length=length), self.assertRaises(ValueError):
                decode_native_program_call(canonical[:length])
        with self.assertRaisesRegex(ValueError, "^invalid native program call$"):
            decode_native_program_call(canonical + b"\0")


if __name__ == "__main__":
    unittest.main()
