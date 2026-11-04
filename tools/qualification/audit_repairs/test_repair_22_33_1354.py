"""Qualify guarded production shell commands without Docker, curl or jq.

The declared hf-build performs bash -n. These tests inspect the real script's
transport, validation and failure paths; they do not claim live localnet proof.
"""

from pathlib import Path
import json
import re
import shlex
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "sdk/contrib/localnet_liveness.sh"
RECORD = Path(__file__).with_name("repair_22_33_1354.kvx")


def source():
    return SCRIPT.read_text(encoding="utf-8")


def logical_source():
    return re.sub(r"\\\n[ \t]*", " ", source())


def test_liveness_request_deadline():
    script = logical_source()
    request = re.search(r"if status_response=\$\((curl [^\n]+)\); then", script)
    assert request, "Capture curl's exit status before accepting its body"
    assert shlex.split(request[1]) == [
        "curl", "--fail", "--silent", "--show-error",
        "--connect-timeout", "2", "--max-time", "5",
        "--max-filesize", "1048576", "--proto", "=http,https",
        "--write-out", r"\n%{http_code}", "--", "$NODEADDR:26657/status",
    ]
    assert len(re.findall(r"\bcurl\b", script)) == 1
    assert 'http_code=${status_response##*$\'\\n\'}' in script
    assert 'status_body=${status_response%$\'\\n\'*}' in script
    assert 'if [[ "$http_code" == 200 ]] && curr_block=' in script
    assert "Status request failed or timed out; counting failed observation." in script
    assert "Invalid HTTP/JSON status response; counting failed observation." in script


def test_liveness_http_json_validation():
    script = source()
    pipeline = re.search(
        r'''curr_block=\$\(printf '%s' "\$status_body" \| jq -ers '(.*?)'\); then''',
        script,
        re.S,
    )
    assert pipeline, "JSON parser failure must guard the height consumer"
    assert " ".join(pipeline[1].split()) == (
        'if length == 1 then .[0] else error("expected one status response") end '
        '| if type == "object" and .error == null then .result.sync_info.latest_block_height '
        'else error("invalid status response") end '
        '| if type == "string" and test("^(0|[1-9][0-9]{0,17})\\\\z") then . '
        'else error("invalid block height") end'
    )
    consumer = script[pipeline.end():script.index("  # Emulate network chaos")]
    assert re.match(
        r'\s*echo "Number of Blocks: \$\{curr_block\}"\n'
        r'      if \(\( curr_block > NUMBLOCKS \)\); then\n'
        r'        echo "Number of blocks reached\. Success!"\n'
        r'        exit 0\n      fi\n    else\n',
        consumer,
    )
    assert script.count("exit 0") == 1, "Only a validated height may succeed"
    # Exercise the literal production height grammar, including arithmetic input.
    literal = re.search(r'test\("([^"\n]+)"\)', pipeline[1])[1]
    grammar = json.loads('"' + literal + '"')
    assert grammar.endswith(r"\z")
    # jq/Oniguruma's absolute end anchor is equivalent to fullmatch here.
    grammar = grammar.removesuffix(r"\z")
    for height in ("0", "1", "999999999999999999"):
        assert re.fullmatch(grammar, height)
    for height in ("", "null", "01", "-1", "1.0", "1e2", "1\n", "1 2",
                   "1000000000000000000", "a[$(id)]"):
        assert re.fullmatch(grammar, height) is None


def test_liveness_bounded_failure_path():
    script = logical_source()
    # Bound Docker too: a stalled daemon must not defeat the request budget.
    restart = re.search(r"if ! (timeout [^\n]+) >/dev/null; then", script)
    assert restart
    assert shlex.split(restart[1]) == [
        "timeout", "--kill-after=1", "5", "docker", "restart", "--", "$rand_container",
    ]
    assert script.count("while ") == 1
    assert "while (( CNT < ITER )); do\n" in script
    assert "if (( CNT % 10 == 0 )); then" in script
    assert script.endswith(
        '  CNT=$((CNT + 1))\n'
        '  if (( CNT < ITER )); then\n'
        '    sleep "$SLEEP" || exit 1\n'
        '  fi\ndone\necho "Timeout reached. Failure!"\nexit 1\n'
    )
    assert script.count("CNT=$((CNT + 1))") == 1
    # No failure can bypass the counter, reset it, retry, or leave a child behind.
    commands = [line for line in script.splitlines() if not line.lstrip().startswith("#")]
    assert not re.search(r"\b(continue|break|exec|wait)\b", "\n".join(commands))
    assert not any(re.search(r"(?<!&)&\s*$", line) for line in commands)
    assert script.index("CNT=$((CNT + 1))") > script.index("Status request failed")
    assert script.count("CNT=") == 2


@pytest.mark.parametrize(
    ("argument", "valid", "invalid"),
    [
        ("ITER", ("1", "999999"), ("0", "01", "1000000", "-1", "1.5", "")),
        ("SLEEP", ("0", "1", "999999"), ("00", "1000000", "-1", "nan", "1e2", "")),
        ("NUMBLOCKS", ("0", "1", "999999999999999999"),
         ("00", "1000000000000000000", "-1", "1.5", "null", "")),
    ],
)
def test_liveness_numeric_arguments(argument, valid, invalid):
    script = source()
    guard = re.search(r"if \[\[ ! .*?\]\]; then\n(.*?)\nfi", script, re.S)
    assert guard
    grammar = re.search(r'! "\$' + argument + r'" =~ (\S+)', guard[0])[1]
    for value in valid:
        assert re.fullmatch(grammar, value)
    for value in (*invalid, "1\n", "a[$(id)]", "2+3", " 1"):
        assert re.fullmatch(grammar, value) is None
    assert 'echo "Configuration error:' in guard[1]
    assert guard[1].endswith("  exit 2")
    assert guard.end() < script.index("docker ps")
    assert script.startswith(
        '#!/bin/bash\n\nif [[ $# -lt 4 || $# -gt 5 ]]; then\n'
        '  echo "Usage: $0 ITER SLEEP NUMBLOCKS NODEADDR [CONTAINER_NAME_FILTER]" >&2\n'
        '  exit 2\nfi\n'
    )
    assert re.search(
        r'if \[\[ -z "\$NODEADDR" \|\| -z "\$CONTAINER_FILTER" \]\]; then\n'
        r'  echo "Configuration error: [^\n]+ >&2\n  exit 2\nfi', script
    )


def test_liveness_empty_container_set():
    script = logical_source()
    discovery = re.search(r"if ! container_output=\$\((timeout [^\n]+)\); then\n(.*?)\nfi", script, re.S)
    assert discovery
    assert shlex.split(discovery[1]) == [
        "timeout", "--kill-after=1", "5", "docker", "ps", "-q",
        "--filter", "name=$CONTAINER_FILTER", "--format={{.Names}}",
    ]
    assert "Docker target discovery failed or timed out" in discovery[2]
    assert discovery[2].endswith("  exit 2")
    empty = re.search(r'if \[\[ -z "\$container_output" \]\]; then\n(.*?)\nfi', script, re.S)
    assert empty
    assert "no running containers match '$CONTAINER_FILTER'" in empty[1]
    assert "make -f chain.mk docker-cluster-start" in empty[1]
    assert "pass CONTAINER_NAME_FILTER as argument 5" in empty[1]
    assert empty[1].endswith("  exit 2")
    array = script.index('mapfile -t docker_containers <<< "$container_output"')
    polling = script.index("while (( CNT < ITER )); do")
    selection = script.index('rand_container=${docker_containers[RANDOM % ${#docker_containers[@]}]}')
    assert discovery.end() < empty.start() < empty.end() < array < polling < selection
    assert script.count("docker_containers=") == 0, "Do not reintroduce word-split discovery"
    assert script.count("mapfile ") == 1
    assert script.count("RANDOM") == 1


def test_liveness_maintained_container_names():
    script = source()
    default = re.search(r"^CONTAINER_FILTER=\$\{5-'([^']+)'\}$", script, re.M)
    assert default, "An explicit fifth argument selects a different localnet"
    compose = (ROOT / "docker/docker-compose.yml").read_text(encoding="utf-8")
    containers = re.findall(r"^    container_name: ([^\n]+)$", compose, re.M)
    assert containers
    nodes = [name for name in containers if name.startswith("pax-node-")]
    assert len(nodes) == 4
    for name in nodes:
        assert re.search(default[1], name), "Discover real maintained node containers"
    assert re.search(default[1], "localnet-simd-1"), "Retain inherited simd discovery"
    for name in ("pax-x-websearch", "pax-node-0-unrelated", "unrelated"):
        assert re.search(default[1], name) is None
    chain = (ROOT / "chain.mk").read_text(encoding="utf-8")
    assert re.search(r"^docker-cluster-start: [^\n]+\n", chain, re.M)
    assert "docker compose up $$DETACH_FLAG" in chain


def test_liveness_repair_record():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    task = record["repair"]["22"]["33"]["1354"]
    assert task["task"] == "22.33.1354"
    assert task["audit"] == "22.30.74"
    assert task["base_revision"] == "cfe99ca0f6e389c490375eed57c49af7d14b30b8"
    assert task["finding_ids"] == ["liveness-request-deadline", "liveness-empty-container-set"]
    assert task["build_cmd"] == "bash -n sdk/contrib/localnet_liveness.sh"
    assert task["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1354.py"
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same revision" in task["verified_revision_source"]
    assert "not run" in record["baseline"]["unchanged_verify"]
    assert "No pre-change failed test" in record["baseline"]["unchanged_verify"]
    assert "not executed" in task["review_only"]
    assert task["wiring_requests"] == []
    for finding_id in task["finding_ids"]:
        finding = record["finding"][finding_id]
        assert finding["file"] == "sdk/contrib/localnet_liveness.sh"
        assert finding["tests"]
        for name in finding["tests"]:
            assert callable(globals()[name]) and name.startswith("test_")
        assert finding["source_proof"]
