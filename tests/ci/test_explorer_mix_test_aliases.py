import pathlib
import re
import shlex

import yaml


ROOT = pathlib.Path(__file__).resolve().parents[2]
BACKEND = ROOT / "services/explorer/backend"
BUILDER = ROOT / "services/explorer/deploy/tools/mix-in-builder.sh"


def test_every_umbrella_app_test_alias_skips_start():
    manifests = sorted((BACKEND / "apps").glob("*/mix.exs"))
    assert manifests, "No umbrella app manifests found"

    for manifest in manifests:
        source = re.sub(r"(?m)^\s*#.*$", "", manifest.read_text(encoding="utf-8"))
        project = re.search(r"def project do\n(.*?)^  end", source, re.S | re.M)
        assert project, f"{manifest}: missing project definition"
        aliases = re.search(r"\baliases:\s*(\[|\w+)", project[1])
        assert aliases, f"{manifest}: project does not register aliases"
        if aliases[1] == "[":
            alias_source = project[1][aliases.end() :]
        else:
            definition = re.search(
                rf"defp {aliases[1]}(?:\([^\n]*\))? do\n(.*?)^  end",
                source,
                re.S | re.M,
            )
            assert definition, f"{manifest}: missing registered aliases definition"
            alias_source = definition[1]

        test_alias = re.search(
            r'\btest:\s*(\[(?:[^\]"\n]|\n|"[^"\n]*")*\]|"[^"\n]*")',
            alias_source,
        )
        assert test_alias, f"{manifest}: missing test alias"
        commands = [
            shlex.split(command)
            for command in re.findall(r'"([^"]*)"', test_alias[1])
        ]
        test_commands = [
            index for index, command in enumerate(commands) if command[0] == "test"
        ]
        assert test_commands, f"{manifest}: alias does not run tests"
        for index in test_commands:
            creates_first = any(
                command[0] == "ecto.create" for command in commands[:index]
            )
            assert "--no-start" in commands[index] or creates_first, (
                f"{manifest}: test alias starts applications before creating the database"
            )


def test_mix_in_builder_creates_database_before_test():
    source = BUILDER.read_text(encoding="utf-8")
    container_script = re.search(
        r"cat <<'CONTAINER_SCRIPT_END'\n(.*?)\nCONTAINER_SCRIPT_END", source, re.S
    )
    assert container_script, "Missing builder container script"
    script = container_script[1]
    assert script.splitlines()[0] == "set -e", (
        "Database setup failures must stop the run"
    )
    setup = re.search(
        r"""if \[ "\$\{1:-\}" = 'test' \]; then\n(.*?)\nfi""", script, re.S
    )
    assert setup, "Database setup must run for the test command"
    commands = [
        shlex.split(line) for line in setup[1].splitlines() if line.strip()
    ]
    assert commands == [["mix", "ecto.create", "--quiet"], ["mix", "ecto.migrate"]]
    assert script[setup.end() :].strip() == 'exec mix "$@"', (
        "Database creation and migration must finish before forwarding all test arguments"
    )
    assert 'sh -c "$CONTAINER_SCRIPT" mix-in-builder "$@"' in source

    workflow = yaml.safe_load(
        (ROOT / ".github/workflows/explorer-images.yml").read_text(encoding="utf-8")
    )
    steps = workflow["jobs"]["explorer-backend-test"]["steps"]
    test_steps = [
        step
        for step in steps
        if step.get("run") == "services/explorer/deploy/tools/mix-in-builder.sh test"
    ]
    assert len(test_steps) == 1, "CI must run the umbrella suite through the builder"
    assert test_steps[0]["env"]["MIX_ENV"] == "test"
