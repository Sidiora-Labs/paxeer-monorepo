"""Keep the public workflow configuration inventory tied to its real readers."""

from pathlib import Path
import re

import yaml


ROOT = Path(__file__).resolve().parents[2]
PAGE = ROOT / "docs/ci/secrets-and-runners.md"
WORKFLOWS = ROOT / ".github/workflows"


def workflow_files():
    paths = sorted(
        path for path in WORKFLOWS.iterdir() if path.suffix in {".yml", ".yaml"}
    )
    assert paths, "The inventory must inspect real workflow files"
    return paths


def documented_rows(section):
    page = PAGE.read_text(encoding="utf-8")
    body = page.split(f"\n## {section}\n", 1)[1].split("\n## ", 1)[0]
    rows = {}
    for line in body.splitlines():
        if not line.startswith("| `"):
            continue
        cells = [cell.strip() for cell in line.strip("|").split("|")]
        assert len(cells) == 4, f"Inventory row needs four columns: {line}"
        name = cells[0].strip("`")
        assert name not in rows, f"Duplicate {section} entry: {name}"
        assert all(cells), f"Incomplete {section} entry: {name}"
        rows[name] = cells[1:]
    assert rows, f"The {section} inventory must not be empty"
    return rows


def assert_context_documented(context, section):
    rows = documented_rows(section)
    # Scan the source as well as YAML values so expressions in conditionals and
    # inline shell scripts are included. Literal index access is supported too.
    pattern = re.compile(
        rf"\b{context}\s*(?:\.\s*([A-Za-z_][A-Za-z0-9_]*)"
        r"|\[\s*['\"]([A-Za-z_][A-Za-z0-9_]*)['\"]\s*\])"
    )
    readers = {}
    for path in workflow_files():
        for dotted, indexed in pattern.findall(path.read_text(encoding="utf-8")):
            readers.setdefault(dotted or indexed, set()).add(path.name)
    assert readers, f"No {context} references were inspected"
    missing = sorted(readers.keys() - rows.keys())
    assert not missing, f"Undocumented workflow {context}: {', '.join(missing)}"
    for name, workflows in sorted(readers.items()):
        checks, purpose, provisioner = rows[name]
        documented_checks = set(
            re.findall(r"\]\(\.\./\.\./\.github/workflows/([^/)]+)\)", checks)
        )
        assert workflows <= documented_checks, (
            f"{context}.{name} is missing readers: "
            f"{', '.join(sorted(workflows - documented_checks))}"
        )
        assert purpose and provisioner, f"{context}.{name} needs purpose and owner"


def test_every_workflow_secret_is_documented():
    assert_context_documented("secrets", "Secrets")


def test_every_workflow_var_is_documented():
    assert_context_documented("vars", "Variables")
    page = PAGE.read_text(encoding="utf-8")
    assert "`CI_LINUX_RUNNER`" in page
    assert "vars.CI_LINUX_RUNNER || 'ubuntu-24.04'" in page


def test_actionlint_labels_are_used():
    config = yaml.load(
        (ROOT / ".github/actionlint.yaml").read_text(encoding="utf-8"),
        Loader=yaml.BaseLoader,
    )
    labels = config["self-hosted-runner"]["labels"]
    assert isinstance(labels, list) and labels, "Custom runner labels must be listed"
    assert len(labels) == len(set(labels)), "Custom runner labels must be unique"
    used = set()
    for path in workflow_files():
        workflow = yaml.load(path.read_text(encoding="utf-8"), Loader=yaml.BaseLoader)
        for job in workflow["jobs"].values():
            runs_on = job.get("runs-on")
            if isinstance(runs_on, dict):
                runs_on = runs_on.get("labels")
            if isinstance(runs_on, str):
                used.add(runs_on)
            elif isinstance(runs_on, list):
                used.update(runs_on)
    unused = sorted(set(labels) - used)
    assert not unused, f"Unused actionlint runner labels: {', '.join(unused)}"
    page = PAGE.read_text(encoding="utf-8")
    for label in labels:
        assert f"`{label}`" in page, f"Undocumented runner label: {label}"
