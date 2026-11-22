"""Behavioral closure of audit finding container-inventory-test."""

from pathlib import Path
import re
import shlex
import subprocess

import pytest

from test_out_of_scope_list import (
    assert_reviewed_out_of_scope,
    parse_out_of_scope,
    test_out_of_scope_omits_consensus_test_dockerfile as check_current_inventory,
)


SCRIPT = Path(__file__).resolve().with_name("check.sh")
ROOT = SCRIPT.parents[2]


def inventory_assignment():
    script = SCRIPT.read_text(encoding="utf-8")
    match = re.search(r"^OUT_OF_SCOPE='\n(.*?)\n'$", script, re.M | re.S)
    assert match is not None
    return script, match


def assignment_rows(script: str, name: str) -> list[str]:
    match = re.search(
        rf"^{name}='\n(.*?)\n'$", script, re.MULTILINE | re.DOTALL
    )
    assert match is not None, f"check.sh must declare the {name} list"
    return [
        row
        for row in match.group(1).splitlines()
        if row and not row.lstrip().startswith("#")
    ]


def service_rows(script: str) -> list[tuple[str, str, str, str]]:
    rows = []
    for row in assignment_rows(script, "SERVICES"):
        fields = row.split("|")
        assert len(fields) == 4, row
        rows.append(tuple(fields))
    return rows


def shell_copy_inputs(recipe: Path, context: Path) -> subprocess.CompletedProcess[str]:
    function = re.search(
        r"^copy_inputs\(\) \{\n.*?^\}",
        SCRIPT.read_text(encoding="utf-8"),
        re.MULTILINE | re.DOTALL,
    )
    assert function is not None, "check.sh must declare copy_inputs"
    return subprocess.run(
        [
            "bash",
            "-c",
            function.group(0) + '\ncopy_inputs "$1" "$2"',
            "copy-inputs-test",
            str(recipe),
            str(context),
        ],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=False,
    )


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
            "deploy/launchpad/Dockerfile.native\n",
            f"deploy/launchpad/Dockerfile.native{suffix}\n",
            1,
        )
        changed = (
            script[: assignment.start(1)]
            + changed_payload
            + script[assignment.end(1) :]
        )
        assert (
            parse_out_of_scope(changed)[0]
            == f"deploy/launchpad/Dockerfile.native{suffix}"
        )
        with pytest.raises(AssertionError):
            assert_reviewed_out_of_scope(changed)


def test_container_inventory_requires_an_explicit_declaration():
    script, assignment = inventory_assignment()
    changed = script[: assignment.start()] + script[assignment.end() :]
    with pytest.raises(AssertionError, match="must declare the OUT_OF_SCOPE list"):
        parse_out_of_scope(changed)


def test_container_inventory_tracks_an_ignore_file_for_each_docker_recipe():
    script = SCRIPT.read_text(encoding="utf-8")
    for _service, recipe, _old, _context in service_rows(script):
        if not recipe.startswith("docker/"):
            continue
        ignore = ROOT / f"{recipe}.dockerignore"
        assert ignore.is_file(), f"missing ignore file for {recipe}"
        tracked = subprocess.run(
            ["git", "ls-files", "--error-unmatch", str(ignore.relative_to(ROOT))],
            cwd=ROOT,
            capture_output=True,
            text=True,
            check=False,
        )
        assert tracked.returncode == 0, f"untracked ignore file: {ignore}"


def test_container_inventory_lists_every_tracked_recipe_outside_docker():
    script = SCRIPT.read_text(encoding="utf-8")
    out_of_scope = set(assignment_rows(script, "OUT_OF_SCOPE"))
    tracked = subprocess.run(
        ["git", "ls-files"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.splitlines()
    recipes = {
        path
        for path in tracked
        if re.search(r"(^|/)(Dockerfile[^/]*|dockerfile|[^/]+\.Dockerfile)$", path)
        and not path.endswith(".dockerignore")
    }
    assert {path for path in recipes if not path.startswith("docker/")} <= out_of_scope


def test_container_inventory_checks_explorer_recipes_from_their_declared_contexts():
    script = SCRIPT.read_text(encoding="utf-8")
    rows = {recipe: context for _service, recipe, _old, context in service_rows(script)}
    root_recipe = ROOT / "docker/explorer-backend/Dockerfile"
    old_ui_recipe = ROOT / "docker/explorer-backend/oldUI.Dockerfile"
    assert rows[str(root_recipe.relative_to(ROOT))] == "."
    assert rows[str(old_ui_recipe.relative_to(ROOT))] == "services/explorer/backend"
    assert shell_copy_inputs(root_recipe, ROOT).returncode == 0
    assert shell_copy_inputs(old_ui_recipe, ROOT / "services/explorer/backend").returncode == 0
    assert shell_copy_inputs(root_recipe, ROOT / "services/explorer/backend").returncode != 0
    assert shell_copy_inputs(old_ui_recipe, ROOT).returncode != 0
