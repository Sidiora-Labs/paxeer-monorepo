import ast
import configparser
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
MANIFEST = ROOT / "deploy/projects/wallet/wallet-gateway.kvx"
SOURCE = ROOT / "services/wallet/gateway/src/env.ts"
INPUT_NAME = "_".join(
    (MANIFEST.parent.name, MANIFEST.stem, "AGENT_JWT_SECRET")
).replace("-", "_").upper()


def definition():
    parser = configparser.ConfigParser(interpolation=None)
    parser.optionxform = str
    parser.read_string(MANIFEST.read_text())
    return {
        section: {key: ast.literal_eval(value) for key, value in parser[section].items()}
        for section in parser.sections()
    }


def test_definition_sets_the_agent_token_secret():
    variables = definition()["vars.wallet-gateway"]
    assert variables["AGENT_JWT_SECRET"] == f"env:{INPUT_NAME}"


def test_agent_token_secret_is_a_generated_input():
    inputs = definition()["inputs.wallet-gateway"]
    assert inputs[INPUT_NAME] == "generated"
    section = MANIFEST.read_text().split("[inputs.wallet-gateway]", 1)[1]
    section = section.split("\n[", 1)[0]
    lines = section.splitlines()
    matches = [
        index for index, line in enumerate(lines)
        if re.match(rf"^\s*{re.escape(INPUT_NAME)}\s*=", line)
    ]
    assert len(matches) == 1
    assert matches[0] > 0
    comment = lines[matches[0] - 1]
    assert comment.startswith("# ") and " as " in comment


def test_schema_still_reads_the_agent_token_secret():
    assert re.search(r"(?m)^    AGENT_JWT_SECRET\s*:", SOURCE.read_text())
