"""Behavioral closure of audit finding container-inventory-test."""

from pathlib import Path
import re
import shlex

import pytest

from test_out_of_scope_list import (
    assert_reviewed_out_of_scope,
    parse_out_of_scope,
    test_out_of_scope_omits_consensus_test_dockerfile as check_current_inventory,
)


SCRIPT = Path(__file__).resolve().with_name("check.sh")


def inventory_assignment():
    script = SCRIPT.read_text(encoding="utf-8")
    match = re.search(r"^OUT_OF_SCOPE='\n(.*?)\n'$", script, re.M | re.S)
    assert match is not None
    return script, match


def test_container_inventory_test_runs_against_real_producer():
    check_current_inventory()


def test_container_inventory_decodes_the_real_shell_payload():
    script, assignment = inventory_assignment()
    # Decode the actual quoted shell assignment independently of the repaired
    # parser. No shell, git, Docker installation or external service is needed.
    tokens = shlex.split(assignment.group(0), comments=False, posix=True)
    assert len(tokens) == 1
    name, payload = tokens[0].split("=", 1)
    assert name == "OUT_OF_SCOPE"
    rows = payload.split("\n")
    assert any(row.startswith("#") for row in rows)
    assert "" in rows
    expected = [row for row in rows if row and not row.lstrip().startswith("#")]
    assert parse_out_of_scope(script) == expected
    assert len(expected) == 34
    assert "consensus/test/docker/Dockerfile" not in expected


def test_container_inventory_accepts_only_grammar_neutral_changes():
    script, assignment = inventory_assignment()
    payload = assignment.group(1)
    annotated = "\n\t# Inventory commentary\n" + payload + "\n  # End\n"
    changed = script[: assignment.start(1)] + annotated + script[assignment.end(1) :]
    assert parse_out_of_scope(changed) == parse_out_of_scope(script)
    assert_reviewed_out_of_scope(changed)


@pytest.mark.parametrize(
    "change",
    ["missing", "unreviewed", "duplicate", "reordered", "consensus", "retired-human"],
)
def test_container_inventory_rejects_unreviewed_scope_changes(change):
    script, assignment = inventory_assignment()
    rows = parse_out_of_scope(script)
    if change == "missing":
        rows.pop()
    elif change == "unreviewed":
        rows.append("tools/unreviewed/Dockerfile")
    elif change == "duplicate":
        rows.append(rows[0])
    elif change == "reordered":
        rows[0], rows[1] = rows[1], rows[0]
    elif change == "consensus":
        rows.append("consensus/test/docker/Dockerfile")
    elif change == "retired-human":
        rows.append("services/platform/hosted/human/Dockerfile")
    changed = (
        script[: assignment.start(1)]
        + "\n".join(rows)
        + script[assignment.end(1) :]
    )
    with pytest.raises(AssertionError):
        assert_reviewed_out_of_scope(changed)


def test_container_inventory_preserves_path_content():
    script, assignment = inventory_assignment()
    payload = assignment.group(1)
    for suffix in (" ", "#literal", "\t"):
        changed_payload = payload.replace(
            "apps/perps/Dockerfile\n", f"apps/perps/Dockerfile{suffix}\n", 1
        )
        changed = (
            script[: assignment.start(1)]
            + changed_payload
            + script[assignment.end(1) :]
        )
        assert parse_out_of_scope(changed)[0] == f"apps/perps/Dockerfile{suffix}"
        with pytest.raises(AssertionError):
            assert_reviewed_out_of_scope(changed)


def test_container_inventory_requires_an_explicit_declaration():
    script, assignment = inventory_assignment()
    changed = script[: assignment.start()] + script[assignment.end() :]
    with pytest.raises(AssertionError, match="must declare the OUT_OF_SCOPE list"):
        parse_out_of_scope(changed)
