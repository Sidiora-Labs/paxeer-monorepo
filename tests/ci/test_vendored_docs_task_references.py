"""Keep internal task references out of the vendored documentation page."""

import re
from pathlib import Path


def test_documentation_page_names_no_task_ids():
    page = (
        Path(__file__).resolve().parents[2] / "docs/hyperfide/DOCUMENTATION.md"
    ).read_text(encoding="utf-8")

    task_ids = re.findall(r"\b\d+(?:\.\d+)+\b", page)
    spec_paths = re.findall(r"\b(?:[\w.-]+/)+[\w.-]+\.kvx\b", page)
    assert not task_ids, f"Internal task identifiers in product docs: {task_ids}"
    assert not spec_paths, f"Internal spec paths in product docs: {spec_paths}"
    assert (
        "In distributed authority mode, `docs packet`, `docs inspect`, "
        "`docs check` and `docs close` route to the control service with explicit "
        "operation identities; the local lifecycle sequence below applies in "
        "local authority mode. See the [distributed documentation API]"
        "(hyperfide/operations.md#distributed-documentation-api) and "
        "[Authority mode](integrate.md#authority-mode)."
    ) in page
