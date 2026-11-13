"""Offline source qualification for the launchpad discovery failure guard."""

import ast
import re
import shlex
import tomllib
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[3]
LAUNCHPAD = ROOT / 'services/launchpad'
WORKSPACE_TEST = LAUNCHPAD / 'security/supplychain/test_workspace_gates.py'
FIXTURE = WORKSPACE_TEST.parent / 'fixtures/package_discovery'
RECORD = Path(__file__).with_name('repair_22_33_1016.kvx')


def make_parts():
    source = (LAUNCHPAD / 'Makefile').read_text()
    logical = source.replace('\\\n', ' ')
    macros = dict(re.findall(r'^define (\w+)\n(.*?)\nendef$', logical, re.M | re.S))
    targets = {}
    current = None
    for line in logical.splitlines():
        if line.startswith('\t') and current is not None:
            targets[current].append(re.sub(r'[ \t]+', ' ', line.strip().removeprefix('@')))
        elif match := re.match(r'^([\w-]+):(?:\s|$)', line):
            current = match[1]
            targets[current] = []
        else:
            current = None
    return logical, macros, targets


def test_discovery_captures_status_before_inspecting_output():
    source, macros, _ = make_parts()
    guard = macros['has_pkgs']
    assert guard == (
        'pkgs=$$(cd $(1) && $(GO) list ./...) || { status=$$?; '
        'echo ">> package discovery failed for $(1)" >&2; exit $$status; }'
    )
    # The assignment is the left side of ||: its failure is observed even
    # when Bash errexit is suppressed inside a conditional command context.
    discovery, failure = guard.split(' || ', 1)
    assert discovery == 'pkgs=$$(cd $(1) && $(GO) list ./...)'
    assert failure.index('status=$$?') < failure.index('echo ') < failure.index('exit $$status')
    assert '2>/dev/null' not in guard
    assert '|| true' not in guard
    assert 'SHELL := /usr/bin/env bash' in source
    assert '.SHELLFLAGS := -euo pipefail -c' in source


@pytest.mark.parametrize(('target', 'command'), [
    ('build', '$(GO) build $(GOFLAGS) ./...'),
    ('vet', '$(GO) vet ./...'),
    ('lint', 'golangci-lint run'),
    ('test', 'python3 $(TEST_RUNNER) --module "$$m" --go "$(GO)" --flags="$(GOFLAGS)"'),
    ('race', 'python3 $(TEST_RUNNER) --module "$$m" --go "$(GO)" --flags="$(strip $(GOFLAGS) -race)"'),
    ('cover', 'python3 $(TEST_RUNNER) --module "$$m" --go "$(GO)" --flags="$(GOFLAGS)" --profile "$(CURDIR)/.cover/$$name.out"'),
])
def test_each_workspace_gate_requires_successful_discovery(target, command):
    _, macros, targets = make_parts()
    loops = [recipe for recipe in targets[target] if 'for m in $(MODULES)' in recipe]
    assert len(loops) == 1
    loop = loops[0]
    prefix = 'for m in $(MODULES); do $(call has_pkgs,$$m); if [ -z "$$pkgs" ]; then '
    if target == 'cover':
        assert loop.startswith('run_id=$$(python3 $(COVER_INVENTORY) begin .cover .cover/run.json --modules "$(MODULES)"); for m in $(MODULES); do ')
        assert '$(call has_pkgs,$$m); if [ -z "$$pkgs" ]; then python3 $(COVER_INVENTORY) record .cover .cover/run.json --run-id "$$run_id" --module "$$m" --empty no-packages; continue; fi;' in loop
    else:
        assert loop.startswith(prefix)
    assert loop.count('$(call has_pkgs,$$m)') == 1
    if target != 'cover':
        assert 'echo ">> skip $$m (no packages)"; continue; fi;' in loop
    assert loop.index('continue; fi;') < loop.index(command)
    if target in {'test', 'race'}:
        assert f'{command} || exit 1;' in loop
    elif target == 'cover':
        assert f'{command};' in loop
    else:
        assert f'( cd $$m && {command} ) || exit 1;' in loop
    if target == 'cover':
        assert loop.endswith('python3 $(COVER_INVENTORY) finish .cover .cover/run.json --run-id "$$run_id"; printf \'%s\\n\' "$$run_id" > .cover/run-id')
    else:
        assert loop.endswith('done')
    expanded = loop.replace('$(call has_pkgs,$$m)', macros['has_pkgs'].replace('$(1)', '$$m'))
    assert expanded.index('exit $$status; }') < expanded.index('if [ -z "$$pkgs" ]')
    assert expanded.index('if [ -z "$$pkgs" ]') < expanded.index(command)
    if target in {'test', 'race', 'cover'}:
        source, _, _ = make_parts()
        assert 'TEST_RUNNER := $(abspath ../../tests/ci/data/launchpad_go_tests.py)' in source
        runner = (ROOT / 'tests/ci/data/launchpad_go_tests.py').read_text()
        assert 'command = [go, "test", *flags]' in runner
        assert 'subprocess.run([*command, package], cwd=module)' in runner
        assert 'status = result.returncode' in runner
        assert 'if status:\n            return status' in runner


def test_discovery_regression_uses_a_listed_workspace_module():
    source, _, _ = make_parts()
    modules = shlex.split(re.search(r'^MODULES := (.+)$', source, re.M)[1])
    workspace = (LAUNCHPAD / 'go.work').read_text()
    use = re.search(r'^use \(\n(.*?)^\)', workspace, re.M | re.S)[1]
    entries = {entry.removeprefix('./') for entry in shlex.split(use)}
    excluded = {
        '../../deploy/launchpad/production/recovery/phase2/broker',
        '../../tools/launchpad/production/load/eventdriver',
    }
    assert len(modules) == len(set(modules))
    assert set(modules) == entries - excluded
    assert 'discovery' in modules
    assert (FIXTURE / 'go.mod.fixture').read_text() == (
        'module example.invalid/discovery\n\ngo 1.20\n'
    )
    packages = [
        re.fullmatch(r'package (\w+)\n', path.read_text())[1]
        for path in sorted(FIXTURE.glob('*.go.fixture'))
    ]
    assert packages == ['discovery', 'conflicting']
    assert len(set(packages)) == 2


def test_workspace_regression_drives_real_discovery_and_make():
    tree = ast.parse(WORKSPACE_TEST.read_text())
    functions = {node.name: node for node in tree.body if isinstance(node, ast.FunctionDef)}
    failure = functions['test_package_discovery_error_fails_workspace_build']
    empty = functions['test_successful_empty_discovery_still_skips_workspace_build']
    for node, expected_commands in (
        (failure, [['go', 'list', './...'], ['make', 'build', 'MODULES=discovery']]),
        (empty, [['make', 'build', 'MODULES=discovery']]),
    ):
        calls = [
            call for call in ast.walk(node)
            if isinstance(call, ast.Call) and isinstance(call.func, ast.Attribute)
            and isinstance(call.func.value, ast.Name)
            and call.func.value.id == 'subprocess' and call.func.attr == 'run'
        ]
        assert [ast.literal_eval(call.args[0]) for call in calls] == expected_commands
        for call in calls:
            keywords = {arg.arg: arg.value for arg in call.keywords}
            assert ast.literal_eval(keywords['check']) is False
            assert ast.literal_eval(keywords['capture_output']) is True
            assert ast.literal_eval(keywords['text']) is True
            assert isinstance(keywords['env'], ast.Name) and keywords['env'].id == 'env'
        body = ast.unparse(node)
        assert "shutil.copyfile(launchpad / 'Makefile', tmp_path / 'Makefile')" in body or (
            "shutil.copyfile(root / 'services/launchpad/Makefile', tmp_path / 'Makefile')" in body
        )
        assert "GOWORK='off'" in body and "GOTOOLCHAIN='local'" in body
        assert not any(isinstance(child, ast.Call) and isinstance(child.func, ast.Attribute)
                       and child.func.attr in {'skip', 'xfail', 'patch'} for child in ast.walk(node))
    checks = [ast.unparse(node.test) for node in ast.walk(failure) if isinstance(node, ast.Assert)]
    assert checks == [
        'discovery.returncode != 0', "discovery.stdout == ''",
        "'found packages' in discovery.stderr", 'result.returncode != 0',
        "'found packages' in result.stderr",
        "'package discovery failed for discovery' in result.stderr",
        "'>> skip discovery' not in result.stdout", "'>> build discovery' not in result.stdout",
    ]
    checks = [ast.unparse(node.test) for node in ast.walk(empty) if isinstance(node, ast.Assert)]
    assert checks == [
        'result.returncode == 0', "'>> skip discovery (no packages)' in result.stdout",
        "'>> build discovery' not in result.stdout",
    ]
    failure_body = ast.unparse(failure)
    assert 'for source in fixture.iterdir():' in failure_body
    assert "shutil.copyfile(source, module / source.name.removesuffix('.fixture'))" in failure_body
    assert "shutil.copyfile(fixture, module / 'go.mod')" in ast.unparse(empty)


def test_repair_record_maps_finding_to_executing_proofs():
    record = tomllib.loads(RECORD.read_text())
    assert set(record['finding']) == {'make-package-discovery'}
    finding = record['finding']['make-package-discovery']
    assert finding['file'] == 'services/launchpad/Makefile'
    assert finding['class'] == 'runtime-risk'
    assert finding['severity'] == 'medium'
    functions = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith('test_')
    }
    assert set(finding['tests']) == functions - {'test_repair_record_maps_finding_to_executing_proofs'}
    assert record['task']['wiring_requests'] == []
    assert record['task']['source_revision'] == '0ba7cfa54c7f07b37201b1f12fcc2f1dd7ed0ed7'
    assert record['task']['verify_cmd'] == (
        'python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1016.py'
    )
    assert record['task']['evidence_ledger'] == '/out/run/gates.jsonl'
    assert 'not executed' in record['task']['review_only']
