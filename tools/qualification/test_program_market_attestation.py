import importlib.util
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parent / "paxeer-x" / "program_market_attestation.py"
SPEC = importlib.util.spec_from_file_location("program_market_attestation", SCRIPT)
driver = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(driver)


class MarketAttestationDriverTest(unittest.TestCase):
    def test_default_cppflags_is_include(self):
        self.assertEqual(driver.parse_arguments([]).cppflags, "-Iinclude")

    def test_cppflags_stays_overridable(self):
        arguments = driver.parse_arguments(["--cppflags", "-Ifoo -Ibar"])
        self.assertEqual(arguments.cppflags, "-Ifoo -Ibar")
        command = driver.native_command(
            arguments, Path("libnative.a"), Path("libruntime.a"), Path("native")
        )
        self.assertEqual(
            command[:4],
            ["cc", "-Ifoo", "-Ibar", "tests/programs/test_market_attestation.c"],
        )
        self.assertNotIn("-Iinclude", command)

        arguments = driver.parse_arguments(["--cppflags", ""])
        self.assertEqual(arguments.cppflags, "")
        command = driver.native_command(
            arguments, Path("libnative.a"), Path("libruntime.a"), Path("native")
        )
        self.assertFalse(any(flag.startswith("-I") for flag in command))

    def test_native_command_carries_cppflags(self):
        arguments = driver.parse_arguments([])
        native_library = Path("libnative.a")
        runtime_library = Path("libruntime.a")
        output = Path("native")
        command = driver.native_command(
            arguments, native_library, runtime_library, output
        )
        self.assertEqual(
            command[:3],
            ["cc", "-Iinclude", "tests/programs/test_market_attestation.c"],
        )
        self.assertEqual(
            command[3:-6],
            [str(native_library), str(runtime_library), str(native_library)],
        )
        self.assertEqual(
            command[-6:], ["-lcrypto", "-pthread", "-ldl", "-lm", "-o", str(output)]
        )


if __name__ == "__main__":
    unittest.main()
