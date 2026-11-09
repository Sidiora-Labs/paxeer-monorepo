from collections import defaultdict
from pathlib import Path
import re
import shlex

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOWS = ROOT / ".github/workflows"
TOOLCHAIN_ACTION = "./.github/actions/release-toolchain"
# Native archives are prerequisites of independent Go checks, rather than a lane.
DUPLICATE_TARGET_ALLOWLIST = {
    ("Makefile", "paxd-native-libs"),
    # Like paxd-native-libs, this is a dependency install that the cross-arch
    # build and eth blocktests workflows both run before their own lanes.
    ("chain.mk", "install"),
}
LANE_OWNERS = {
    "agent-test-sanitize": "agent.yml",
    "agent-fuzz-long": "agent.yml",
    "programs-js-install": "programs-conformance.yml",
    "programs-test": "programs-conformance.yml",
}


def _load(path):
    return yaml.load(path.read_text(encoding="utf-8"), Loader=yaml.BaseLoader)


def _workflows():
    paths = sorted([*WORKFLOWS.glob("*.yml"), *WORKFLOWS.glob("*.yaml")])
    assert paths, "No workflows found"
    return {path.name: _load(path) for path in paths}


def _events(workflow):
    triggers = workflow["on"]
    return {triggers} if isinstance(triggers, str) else set(triggers)


def _release_only(job):
    condition = job.get("if", "").removeprefix("${{").removesuffix("}}")
    alternatives = condition.split("||")
    return bool(condition) and all(
        "github.event_name == 'workflow_dispatch'" in alternative
        or re.search(r"startsWith\(github\.ref,\s*'refs/tags/", alternative)
        for alternative in alternatives
    )


def _make_targets(script):
    if not re.search(r"\bmake\s", script):
        return
    script = re.sub(r"\$\{\{.*?\}\}", "DYNAMIC_EXPRESSION", script)
    lexer = shlex.shlex(
        script.replace("\\\n", " "), posix=True, punctuation_chars=";&|()<>\n"
    )
    lexer.whitespace = " \t\r"
    lexer.whitespace_split = True
    words = list(lexer)
    for index, word in enumerate(words):
        if "\n" in word and re.search(r"\bmake\s", word):
            yield from _make_targets(word)
        if word != "make":
            continue
        makefile = "Makefile"
        goals = []
        position = index + 1
        while position < len(words):
            argument = words[position]
            position += 1
            if all(character in ";&|()<>\n" for character in argument):
                break
            if (
                argument.isdecimal()
                and position < len(words)
                and words[position].startswith((">", "<"))
            ):
                break
            if argument in {"-f", "--file", "--makefile"}:
                makefile = words[position]
                position += 1
            elif argument in {"-C", "--directory"}:
                makefile = str(Path(words[position]) / makefile)
                position += 1
            elif argument.startswith(("--file=", "--makefile=")):
                makefile = argument.partition("=")[2]
            elif argument.startswith("-f"):
                makefile = argument[2:]
            elif argument in {"-j", "--jobs", "-l", "--load-average"}:
                if position < len(words) and words[position].isdecimal():
                    position += 1
                continue
            elif argument.startswith("-") or "=" in argument:
                continue
            elif "$" not in argument and argument != "DYNAMIC_EXPRESSION":
                goals.append(argument)
        for goal in goals:
            yield makefile, goal


def test_release_workflows_share_toolchain_action():
    action_path = ROOT / TOOLCHAIN_ACTION.removeprefix("./") / "action.yml"
    action = _load(action_path)
    assert action["runs"]["using"] == "composite"
    steps = action["runs"]["steps"]
    assert len(steps) == 1
    assert steps[0]["shell"] == "bash"
    script = steps[0]["run"]
    assert script.splitlines()[0] == "set -euo pipefail"
    for required in (
        ". tools/release/toolchain.env",
        ': "${CC_VERSION:?tools/release/toolchain.env must pin CC_VERSION}"',
        'cc_package="gcc-${CC_VERSION%%.*}"',
        'apt-cache madison "$cc_package"',
        'index(candidate, version "-") == 1 { print $3; exit }',
        'if [[ -z "$cc_package_version" ]]; then',
        "sudo apt-get install -y --no-install-recommends "
        "build-essential libssl-dev libsqlite3-dev",
        '"$cc_package=$cc_package_version" "g++-${CC_VERSION%%.*}=$cc_package_version"',
        'export PATH="$compiler_bin:$PATH"',
        'echo "$compiler_bin" >> "$GITHUB_PATH"',
        "have_cc=$(cc -dumpfullversion)",
        'if [[ "$have_cc" != "$CC_VERSION" ]]; then',
    ):
        assert required in script, required
    assert script.count("exit 1") == 2
    for name, compiler in (
        ("cc", "$cc_package"),
        ("gcc", "$cc_package"),
        ("c++", "g++-${CC_VERSION%%.*}"),
        ("g++", "g++-${CC_VERSION%%.*}"),
    ):
        assert f'ln -sf "/usr/bin/{compiler}" "$compiler_bin/{name}"' in script

    for filename, job_name, build in (
        (
            "paxeer-release-check.yml",
            "release-check",
            "tools/release/reproducible-check.sh",
        ),
        ("paxeer-release-publish.yml", "releaser", "tools/release/build-paxd.sh"),
    ):
        workflow = _load(WORKFLOWS / filename)
        steps = workflow["jobs"][job_name]["steps"]
        calls = [step for step in steps if step.get("uses") == TOOLCHAIN_ACTION]
        assert len(calls) == 1, filename
        assert "if" not in calls[0]
        assert "continue-on-error" not in calls[0]
        checkout = next(
            index
            for index, step in enumerate(steps)
            if step.get("uses", "").startswith("actions/checkout@")
        )
        build_index = next(
            index for index, step in enumerate(steps) if step.get("run") == build
        )
        assert checkout < steps.index(calls[0]) < build_index
        assert not any("apt-cache madison" in step.get("run", "") for step in steps)
        assert not any("ln -sf" in step.get("run", "") for step in steps)
        if filename == "paxeer-release-check.yml":
            paths = workflow["on"]["pull_request"]["paths"]
            assert ".github/actions/release-toolchain/**" in paths


def test_no_make_target_duplicated_across_automatic_workflows():
    workflows = _workflows()
    owners = defaultdict(set)
    lanes = defaultdict(list)
    for filename, workflow in workflows.items():
        if not (_events(workflow) - {"workflow_dispatch", "workflow_call"}):
            continue
        for job_name, job in workflow["jobs"].items():
            for step in job.get("steps", []):
                script = step.get("run", "")
                targets = list(_make_targets(script))
                # The native-node wrapper selects this exact Make target.
                wrapper_path = "services/agent/tools/run-real-node-tests.sh"
                if f"sh {wrapper_path} sanitizers" in script:
                    wrapper = (ROOT / wrapper_path).read_text(encoding="utf-8")
                    assert (
                        "sanitizers) test_target=agent-test-sanitize ;;" in wrapper
                    )
                    assert 'make "$test_target"' in wrapper
                    targets.append(("Makefile", "agent-test-sanitize"))
                for makefile, target in targets:
                    if makefile == "Makefile" and target in LANE_OWNERS:
                        lanes[target].append((filename, job_name))
                    # Tag/dispatch release gates are separate from automatic
                    # branch checks; lane ownership above also covers those gates.
                    if not _release_only(job):
                        owners[makefile, target].add(filename)

    duplicates = {
        f"{makefile}:{target}": sorted(files)
        for (makefile, target), files in owners.items()
        if len(files) > 1 and (makefile, target) not in DUPLICATE_TARGET_ALLOWLIST
    }
    assert not duplicates, duplicates
    assert len(owners["Makefile", "paxd-native-libs"]) > 1
    assert owners["chain.mk", "install"] == {
        "paxeer-cross-arch-build.yml",
        "paxeer-eth-blocktests.yml",
    }
    for target, owner in LANE_OWNERS.items():
        assert len(lanes[target]) == 1, (target, lanes[target])
        assert lanes[target][0][0] == owner, (target, lanes[target])

    platform = workflows["platform.yml"]
    gates = {
        "programs-acceptance": ("programs-conformance.yml", "scope", "acceptance"),
        "agent-sanitizers": ("agent.yml", "lane", "sanitizers"),
        "agent-fuzz-corpus": ("agent.yml", "lane", "fuzz-corpus"),
    }
    for gate, (filename, input_name, value) in gates.items():
        caller = platform["jobs"][gate]
        assert caller["uses"] == f"./.github/workflows/{filename}"
        assert "steps" not in caller
        assert caller["with"] == {input_name: value}
        assert caller["if"] == platform["jobs"]["release-pipeline"]["if"]
        assert "continue-on-error" not in caller
        callee = workflows[filename]
        assert callee["on"]["workflow_call"]["inputs"][input_name]["type"] == "string"
        assert input_name in callee["concurrency"]["group"]
        target = {
            "programs-acceptance": "programs-test",
            "agent-sanitizers": "agent-test-sanitize",
            "agent-fuzz-corpus": "agent-fuzz-long",
        }[gate]
        job = callee["jobs"][lanes[target][0][1]]
        if input_name == "lane":
            assert job["if"] == f"inputs.lane == '' || inputs.lane == '{value}'"
            for other_name, other_job in callee["jobs"].items():
                if other_name != lanes[target][0][1]:
                    assert f"inputs.lane == '{value}'" not in other_job["if"]
        else:
            assert job["if"] == (
                "github.event_name != 'workflow_dispatch' || "
                "inputs.scope != 'determinism'"
            )
            for other_name, other_job in callee["jobs"].items():
                if other_name != lanes[target][0][1]:
                    assert "inputs.scope != 'acceptance'" in other_job["if"]
        assert "continue-on-error" not in job
        steps = job["steps"]
        producers = [
            step
            for step in steps
            if f"$RUNNER_TEMP/{gate}.log" in step.get("run", "")
        ]
        assert len(producers) == 1, gate
        assert producers[0]["shell"] == "bash"
        assert producers[0]["run"].splitlines()[0] == "set -euo pipefail"
        uploads = [
            step
            for step in steps
            if step.get("with", {}).get("name") == f"release-gate-{gate}"
        ]
        assert len(uploads) == 1, gate
        assert uploads[0]["if"] == "${{ always() }}"
        assert uploads[0]["with"]["path"] == f"${{{{ runner.temp }}}}/{gate}.log"
        assert uploads[0]["with"]["if-no-files-found"] == "error"
        assert steps.index(producers[0]) < steps.index(uploads[0])
    assert set(gates) <= set(platform["jobs"]["release-pipeline"]["needs"])
