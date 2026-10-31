import importlib
import io
import re
import sys
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
SDK_TESTS = ROOT / "tests/agent/sdk/python"
SEQUENCER_FIXTURE = "sequencer-receipt-v1.json"


def _sdk_sources():
    paths = sorted(SDK_TESTS.rglob("*.py"))
    assert paths, "Python SDK test sources are missing"
    return {path: path.read_text(encoding="utf-8") for path in paths}


def _assert_no_references(sources, pattern):
    matches = [
        f"{path.relative_to(ROOT)}:{number}: {line.strip()}"
        for path, source in sources.items()
        for number, line in enumerate(source.splitlines(), 1)
        if re.search(pattern, line)
    ]
    assert not matches, "Retired receipt fixture readers:\n" + "\n".join(matches)


def _run_sdk_tests(*names):
    original_path = sys.path[:]
    try:
        sys.path[:0] = [str(ROOT / "services/agent/sdk/python"), str(SDK_TESTS)]
        suite = unittest.TestSuite()
        for module_name, test_name in names:
            module = importlib.import_module(module_name)
            suite.addTests(unittest.defaultTestLoader.loadTestsFromName(test_name, module))
        output = io.StringIO()
        result = unittest.TextTestRunner(stream=output, verbosity=2).run(suite)
        assert result.testsRun > 0, output.getvalue()
        assert result.wasSuccessful(), output.getvalue()
        assert not result.skipped, output.getvalue()
        assert not result.expectedFailures, output.getvalue()
    finally:
        sys.path[:] = original_path


def test_sdk_tests_read_sequencer_receipt_vectors():
    sources = _sdk_sources()
    _assert_no_references(
        sources, r"receipt-(?:positive|refusals)-v[23]\.json"
    )
    for name in ("test_receipt_fixture.py", "test_program_lifecycle.py"):
        assert SEQUENCER_FIXTURE in sources[SDK_TESTS / name]
    assert "_load_fixture" in sources[SDK_TESTS / "test_supply_receipt.py"]
    assert (ROOT / "sdk/platform/conformance/fixtures" / SEQUENCER_FIXTURE).is_file()
    _run_sdk_tests(
        ("test_receipt_fixture", "ReceiptFixtureTest"),
        ("test_supply_receipt", "SupplyReceiptTests"),
    )


def test_sdk_tests_read_no_program_receipt_v2_v3():
    _assert_no_references(_sdk_sources(), r"receipt-programs[^\s\"']*-v[23]\.json")
    source = (SDK_TESTS / "test_program_executed_v3.py").read_text(encoding="utf-8")
    assert "tests/fixtures/programs/emulator-response-code" in source
    assert (ROOT / "tests/fixtures/programs/emulator-response-code/execution.json").is_file()


def test_nothing_reads_executed_v4_fixture():
    paths = (
        ROOT / "scripts/programs/generate-abi-vectors.py",
        SDK_TESTS / "test_native_abi5.py",
    )
    _assert_no_references(
        {path: path.read_text(encoding="utf-8") for path in paths},
        r"receipt-programs-executed-v4\.json",
    )
    script = (ROOT / "scripts/paxeer-x/verify-sdk-program-python.sh").read_text(
        encoding="utf-8"
    )
    assert "terminal-v4.test.py" not in script
    native = ROOT / "tests/fixtures/programs/emulator-response-code/receipt.hex"
    canonical = bytes.fromhex(native.read_text(encoding="utf-8"))
    assert len(canonical[-490:-69]) == 421
    assert canonical[-490:-486] == b"PRG4"
    assert canonical[-69:-64] == bytes.fromhex("0100000040")
