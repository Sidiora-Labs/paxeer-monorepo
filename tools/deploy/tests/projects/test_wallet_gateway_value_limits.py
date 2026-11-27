import ast
import configparser
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
MANIFEST = ROOT / "deploy/projects/wallet/wallet-gateway.kvx"
SOURCE = ROOT / "services/wallet/gateway/src/env.ts"
VALUE_LIMITS = ("POLICY_MAX_TX_VALUE_WEI", "POLICY_MAX_DAILY_VALUE_WEI")
INPUT_NAMES = {
    name: "_".join((MANIFEST.parent.name, MANIFEST.stem, name))
    .replace("-", "_")
    .upper()
    for name in VALUE_LIMITS
}


def definition():
    parser = configparser.ConfigParser(interpolation=None)
    parser.optionxform = str
    parser.read_string(MANIFEST.read_text())
    return {
        section: {key: ast.literal_eval(value) for key, value in parser[section].items()}
        for section in parser.sections()
    }


def test_definition_sets_both_value_limits():
    variables = definition()["vars.wallet-gateway"]
    for name in VALUE_LIMITS:
        assert variables[name] == f"env:{INPUT_NAMES[name]}"


def test_value_limits_are_operator_inputs():
    inputs = definition()["inputs.wallet-gateway"]
    section = MANIFEST.read_text().split("[inputs.wallet-gateway]", 1)[1]
    section = section.split("\n[", 1)[0]
    lines = section.splitlines()
    for input_name in INPUT_NAMES.values():
        assert inputs[input_name] == "operator"
        matches = [
            index for index, line in enumerate(lines)
            if re.match(rf"^\s*{re.escape(input_name)}\s*=", line)
        ]
        assert len(matches) == 1
        assert matches[0] > 0
        comment = lines[matches[0] - 1]
        assert comment.startswith("# ") and " as " in comment
        assert re.search(r"\bwei\b", comment)


def test_schema_still_reads_both_value_limits():
    text = SOURCE.read_text()
    for name in VALUE_LIMITS:
        assert re.search(rf"(?m)^    {re.escape(name)}\s*:", text)
