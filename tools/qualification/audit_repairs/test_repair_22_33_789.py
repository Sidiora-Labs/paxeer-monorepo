"""Source/consumer proofs for audit 22.30.176; no compiled toolchain is needed."""

import ast
import hashlib
import importlib.util
import os
from pathlib import Path
import re
import subprocess
import types

import pytest
import yaml


ROOT = Path(__file__).resolve().parents[3]
CI = ROOT / 'scripts/ci'
SCRIPTS = (
    'no-float-scan.sh', 'public-repo-audit.sh', 'release-gate.sh',
    'secret-scan.sh', 'solidity-state-surface.sh', 'symbol-allowlist.sh',
    'xweb-forge-libs.sh', 'xweb-workflow-check.sh', 'beta-ledger-check.sh',
)


def load_module(name):
    spec = importlib.util.spec_from_file_location(name, CI / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def embedded(name, delimiter, before=None):
    text = (CI / name).read_text()
    source = text.split("<<'" + delimiter + "'\n", 1)[1].split('\n' + delimiter, 1)[0]
    if before:
        source = source.split(before, 1)[0]
    module = types.ModuleType(delimiter)
    exec(compile(source, str(CI / name) + ':' + delimiter, 'exec'), module.__dict__)
    return module, source


objects = load_module('audit_object_checks')
search = load_module('checked_rg')
solidity = load_module('solidity_state_evidence')
dependencies = load_module('xweb_dependency_identity')


def test_all_repaired_shell_scripts_parse_and_use_production_helpers():
    for name in SCRIPTS:
        result = subprocess.run(
            ['bash', '-n', str(CI / name)],
            text=True, capture_output=True, check=False,
        )
        assert result.returncode == 0, result.stderr
    for script, helper in (
        ('no-float-scan.sh', 'audit_object_checks.py" float'),
        ('symbol-allowlist.sh', 'audit_object_checks.py" symbols'),
        ('solidity-state-surface.sh', 'solidity_state_evidence.py'),
        ('xweb-forge-libs.sh', 'xweb_dependency_identity.py'),
    ):
        assert helper in (CI / script).read_text()
    for name in ('audit_object_checks', 'checked_rg', 'solidity_state_evidence',
                 'xweb_dependency_identity'):
        ast.parse((CI / (name + '.py')).read_text())


def inventory_tree(tmp_path):
    root = tmp_path / 'source'
    source = root / 'src/protocol/rule.c'
    source.parent.mkdir(parents=True)
    source.write_text('int rule(void) { return 0; }\n')
    build = root / 'build'
    return root, build


def test_finding_8_missing_empty_and_partial_object_inventories_refuse(tmp_path):
    root, build = inventory_tree(tmp_path)
    with pytest.raises(objects.InspectionError, match='directory'):
        objects.object_inventory(root, build)
    directory = build / 'obj/src/protocol'
    directory.mkdir(parents=True)
    with pytest.raises(objects.InspectionError, match='inventory differs'):
        objects.object_inventory(root, build)
    artifact = directory / 'rule.o'
    artifact.touch()
    with pytest.raises(objects.InspectionError, match='object is empty'):
        objects.object_inventory(root, build)
    # This is an inventory fixture only; no test credits it as a valid binary.
    artifact.write_bytes(b'nonempty inventory input')
    assert objects.object_inventory(root, build) == [artifact]
    (root / 'src/protocol/new_rule.c').write_text('int new_rule;\n')
    with pytest.raises(objects.InspectionError, match='inventory differs'):
        objects.object_inventory(root, build)


@pytest.mark.parametrize('tool', ['objdump', 'nm'])
@pytest.mark.parametrize('status', [1, 2, 127, -9])
def test_finding_8_failed_inspection_is_never_no_match(tool, status):
    with pytest.raises(objects.InspectionError, match='inspection failed'):
        objects.checked_output(tool, status, '')
    source = (CI / 'audit_object_checks.py').read_text()
    assert 'checked_output(tool, result.returncode, result.stdout)' in source
    assert "inspect('objdump', '-d', path)" in source
    assert "inspect('nm', '-u', path)" in source
    assert 'stderr=subprocess.PIPE' in source


def test_finding_8_source_only_and_binary_claims_are_separate():
    source = (CI / 'no-float-scan.sh').read_text()
    assert 'source-only inspection; binary instructions and imports were not inspected' in source
    assert source.index('if [ "$source_only" -eq 1 ]') < source.index('audit_object_checks.py')
    assert source.index('audit_object_checks.py') < source.index('no floating-point syntax, instructions')
    assert '|| true' not in source
    assert 'consensus source inventory is empty' in source
    makefile = (ROOT / 'Makefile').read_text()
    for path in objects.EXCLUDED_SOURCES:
        assert path in makefile.split('LIB_OBJECTS :=', 1)[0]
    assert '$(patsubst %.c,$(BUILD_DIR)/obj/%.o,$(LIB_SOURCES))' in makefile
    with pytest.raises(objects.InspectionError, match='no disassembly'):
        objects.check_float('', '')
    with pytest.raises(objects.InspectionError, match='floating-point instructions'):
        objects.check_float('  0: addsd %xmm1,%xmm0\n', '')
    with pytest.raises(objects.InspectionError, match='libm'):
        objects.check_float('  0: ret\n', '                 U sqrt\n')
    objects.check_float('  0: ret\n', '')


def test_finding_9_empty_inventory_and_nm_failure_refuse(tmp_path):
    root, build = inventory_tree(tmp_path)
    (build / 'obj/src').mkdir(parents=True)
    with pytest.raises(objects.InspectionError):
        objects.object_inventory(root, build)
    with pytest.raises(objects.InspectionError):
        objects.checked_output('nm', 2, '                 U allowed\n')
    source = (CI / 'symbol-allowlist.sh').read_text()
    assert 'audit_object_checks.py" symbols "$build_dir"' in source
    assert '&& exit 0' not in source
    assert '|| true' not in source
    assert 'memcmp[[:space:]]*' in source
    assert 'ctx_(set_)?balance|balance[[:space:]]*=' in source


@pytest.mark.parametrize('symbol', [
    'time', 'clock_gettime', 'gettimeofday', 'rand', 'random', 'getenv', 'getpid',
    'gethostname', 'stat', 'socket', 'connect', 'accept', 'bind', 'listen', 'send',
    'sendto', 'recv', 'recvfrom', 'getaddrinfo', 'curl_easy_perform',
])
def test_finding_9_all_operating_system_symbols_remain_forbidden(symbol):
    for group in objects.EXECUTION_DIRS:
        for suffix in ('', '@GLIBC_2.2.5'):
            for kind in ('U', 'w', 'v'):
                with pytest.raises(objects.InspectionError, match='operating-system'):
                    objects.check_symbols(group, ' ' + kind + ' ' + symbol + suffix + '\n')


@pytest.mark.parametrize('symbol', [
    'lxp_apply_transfer', 'lxp_apply_transfer_set', 'lxp_balance_apply_leg',
])
def test_finding_9_sqlite_and_balance_writers_remain_forbidden(symbol):
    with pytest.raises(objects.InspectionError, match='balance-apply'):
        objects.check_symbols('modules', ' U ' + symbol + '\n')
    for group in objects.CONSENSUS_DIRS:
        with pytest.raises(objects.InspectionError, match='SQLite'):
            objects.check_symbols(group, ' U sqlite3_prepare_v2\n')
    objects.check_symbols('protocol', ' U lxp_allowed\n')


@pytest.mark.parametrize('status', [2, 123, 124, 125, 126, 127, -9])
def test_findings_12_and_13_search_errors_are_distinct_from_no_match(status):
    assert search.classify(0) is True
    assert search.classify(1) is False
    assert search.scan([], ['pattern']) == (False, b'')
    with pytest.raises(search.SearchError):
        search.classify(status)


@pytest.mark.parametrize('inaccessible', ['missing', 'unreadable', 'directory'])
def test_finding_12_selected_inaccessible_input_refuses_publication(tmp_path, inaccessible):
    path = tmp_path / 'selected'
    if inaccessible == 'unreadable':
        path.write_text('selected publication content\n')
        path.chmod(0)
    elif inaccessible == 'directory':
        path.mkdir()
    selected = search.selected_files(os.fsencode(path) + b'\0')
    with pytest.raises((OSError, search.SearchError)):
        # Preflight fails inside the real scanner before any rg process is needed.
        search.scan(selected, ['pattern'])
    shell = (CI / 'secret-scan.sh').read_text()
    assert 'set -euo pipefail' in shell
    assert '|| true' not in shell
    assert shell.count('|| return $?') >= 7
    assert 'checked_rg.py" --no-match-ok --' in shell
    assert 'report pem-private-key >> "$scan_hits" || return $?' in shell
    assert '%.12s' in shell
    assert shell.index('report jwt >> "$scan_hits" || return $?') < shell.index('cat "$scan_hits"')
    transport = (CI / 'checked_rg.py').read_text()
    assert 'stderr=subprocess.PIPE' in transport
    assert "['rg', *arguments[:-1], '--', arguments[-1], *batch]" in transport
    assert 'matched |= classify(result.returncode)' in transport
    assert "print('publication scan refused: selected input or search tool unavailable'" in transport


def test_finding_13_public_audit_handles_every_search_status():
    shell = (CI / 'public-repo-audit.sh').read_text()
    function = shell.split('audit_rg() {', 1)[1].split('\n}', 1)[0]
    assert 'checked_rg.py" --gitlinks "$gitlinks" --exclude "$audit_exclude" --' in function
    assert 'xargs' not in function and '|| true' not in function
    assert 'audit_status=$?' in shell
    assert 'case $audit_status in' in shell
    assert '1) ;;' in shell
    assert '*) echo "publication refused: private-reference search incomplete"' in shell
    assert 'git ls-files --cached --others --exclude-standard -z > "$publication_files_nul"' in shell
    assert '--select-only --gitlinks "$gitlinks"' in shell
    assert search.selected_files(b'a b\0line\nname\0excluded\0', '^excluded$') == ['a b', 'line\nname']
    with pytest.raises(search.SearchError, match='NUL'):
        search.selected_files(b'partial inventory')
    assert 'bash scripts/ci/secret-scan.sh' in shell


def release_module():
    return embedded('beta-ledger-check.sh', 'PY_SOURCE', '\nparser = argparse.ArgumentParser')[0]


def test_finding_14_private_unique_runs_refuse_collisions_and_reopen_unchanged(tmp_path):
    release = release_module()
    root = tmp_path / 'checkout'
    root.mkdir()
    private = tmp_path / 'private'
    private.mkdir(mode=0o700)
    base = release.private_root(private, root)
    first, log, sidecar = release.new_run_directory(base, '1' * 32)
    assert first.stat().st_mode & 0o077 == 0
    assert root not in first.parents
    with log.open('xb') as stream:
        os.chmod(log, 0o600)
        stream.write(b'private command output\n')
    release.atomic(sidecar, {'source_revision': 'a' * 40})
    digest, _ = release.evidence_digest(str(first), base, sidecar)
    with pytest.raises(FileExistsError):
        release.new_run_directory(base, '1' * 32)
    with pytest.raises(FileExistsError):
        log.open('xb')
    release.new_run_directory(base, '2' * 32)
    # Reload the production recorder and recover the persisted evidence.
    reopened = release_module()
    assert reopened.load(sidecar, base) == {'source_revision': 'a' * 40}
    assert reopened.evidence_digest(str(first), base, sidecar)[0] == digest
    assert log.read_bytes() == b'private command output\n'
    assert sidecar.stat().st_mode & 0o077 == 0
    with pytest.raises(release.EvidenceError, match='outside source'):
        release.private_root(root, root)
    public = tmp_path / 'public'
    public.mkdir(mode=0o755)
    with pytest.raises(release.EvidenceError, match='owned and private'):
        release.private_root(public, root)


def test_finding_14_source_binding_and_only_private_references_are_emitted():
    release = release_module()
    shell = (CI / 'release-gate.sh').read_text()
    assert 'kind=directory' in shell
    assert 'exec bash "$root/scripts/ci/beta-ledger-check.sh" --source-run' in shell
    assert '--spec "$spec" --task "$task" --ledger "$ledger" --evidence-root "$directory"' in shell
    assert 'private ledger $ledger' not in shell
    assert 'spec/layerx-beta' not in shell
    backend = (CI / 'beta-ledger-check.sh').read_text()
    assert 'new_run_directory(base, identifier)' in backend
    assert "with log.open('xb') as output:" in backend
    assert 'stdout=output, stderr=subprocess.STDOUT' in backend
    assert "'revision': start['revision']" in backend
    assert "'source_evidence': str(sidecar)" in backend
    assert "value['evidence'], unused = evidence_digest(evidence, base, sidecar)" in backend
    state = {
        'state': 'complete', 'development_outcome': 'pass', 'command_exit_status': 0,
        'executed': True, 'source_start': {'revision': 'a' * 40, 'clean': True, 'inspection_ok': True},
        'source_end': {'revision': 'a' * 40, 'clean': True, 'inspection_ok': True},
        'mutation_observation': {'changed': False, 'complete': True},
    }
    assert release.reasons(state, 'a' * 40) == []
    state['source_end']['revision'] = 'b' * 40
    assert {'revision_changed', 'candidate_mismatch', 'source_changed'} <= set(release.reasons(state, 'a' * 40))


def timelock_abi():
    # Derive the current source surface; only production Forge claims ABI execution.
    source = (ROOT / 'contracts/governance/SidioraProxyTimelock.sol').read_text()
    abi = []
    for match in re.finditer(r'function\s+(\w+)\s*\(([^)]*)\)\s*([^{;]*)\{', source):
        name, parameters, modifiers = match.groups()
        if not re.search(r'\b(public|external)\b', modifiers):
            continue
        if re.search(r'\b(view|pure)\b', modifiers):
            continue
        abi.append({'type': 'function', 'name': name, 'stateMutability': 'nonpayable',
                    'inputs': [{'type': parameter.strip().split()[0]}
                               for parameter in parameters.split(',') if parameter.strip()]})
    return abi


def test_finding_10_full_abi_signatures_bind_executed_tests_with_state_assertions():
    source = (ROOT / solidity.TEST_PATH).read_text()
    assert solidity.verify_source_bindings(timelock_abi(), source) == set(solidity.BINDINGS)
    producer = (CI / 'solidity_state_evidence.py').read_text()
    assert "forge_json(['inspect', CONTRACT, 'abi', '--json'], environment)" in producer
    assert "forge_json(['test', '--json', '--match-path', TEST_PATH," in producer
    assert "'--match-contract', '^' + TEST_CONTRACT + '$'" in producer
    assert "environment['FOUNDRY_PROFILE'] = 'governance'" in producer
    assert "if result.returncode != 0:" in producer
    assert "suite.get(name + '()', {}).get('status') != 'Success'" in producer
    assert "not suite or any(test.get('status') != 'Success' for test in suite.values())" in producer
    assert solidity.CONTRACT == 'contracts/governance/SidioraProxyTimelock.sol:SidioraProxyTimelock'
    assert 'method=${method%%(*}' not in (CI / 'solidity-state-surface.sh').read_text()
    config = __import__('tomllib').loads((ROOT / 'foundry.toml').read_text())
    assert config['profile']['governance']['src'] == 'contracts/governance'
    assert config['profile']['governance']['test'] == str(Path(solidity.TEST_PATH).parent)


@pytest.mark.parametrize('mutation', ['overload', 'comment', 'string', 'no_state'])
def test_finding_10_name_only_overloads_and_unexecuted_tests_are_refused(mutation):
    abi = timelock_abi()
    source = (ROOT / solidity.TEST_PATH).read_text()
    if mutation == 'overload':
        abi[-1]['inputs'][0]['type'] = 'uint256'
    elif mutation == 'comment':
        source = source.replace('timelock.cancel(id);', '/* timelock.cancel(id); */')
    elif mutation == 'string':
        source = source.replace('timelock.cancel(id);', 'string memory mention = "timelock.cancel(id)";')
    elif mutation == 'no_state':
        source = source.replace('require(timelock.readyAt(id) == 0,', 'require(true,')
    with pytest.raises(solidity.SurfaceError):
        solidity.verify_source_bindings(abi, source)


@pytest.mark.parametrize('invalid', ['empty', 'failure', 'skipped', 'missing', 'wrong_suite'])
def test_finding_10_only_complete_successful_execution_can_supply_coverage(invalid):
    # Rejected evidence never stands in for an executed Foundry result.
    source = (ROOT / solidity.TEST_PATH).read_text()
    name = solidity.BINDINGS['cancel(bytes32)'][0]
    results = {}
    if invalid in ('failure', 'skipped'):
        results = {solidity.TEST_KEY: {'test_results': {name + '()': {'status': invalid}}}}
    elif invalid == 'missing':
        results = {solidity.TEST_KEY: {'test_results': {}}}
    elif invalid == 'wrong_suite':
        results = {'unrelated.t.sol:Other': {'test_results': {}}}
    with pytest.raises(solidity.SurfaceError):
        solidity.verify_evidence(timelock_abi(), results, source)


def test_finding_11_reviewed_digests_and_existing_repository_revision_guards():
    bootstrap = (ROOT / 'contracts/bootstrap-libs.sh').read_text()
    dependencies.validate_bootstrap(bootstrap)
    with pytest.raises(dependencies.DependencyError):
        dependencies.validate_bootstrap(bootstrap.replace('v1.9.6', 'v1.9.7'))
    workflow = yaml.safe_load((ROOT / '.github/workflows/paxeer-forge-test.yml').read_text())
    assert any('bash contracts/bootstrap-libs.sh' in str(step.get('run', ''))
               for step in workflow['jobs']['check']['steps'])
    for name, (url, _, digest) in dependencies.PINS.items():
        assert re.fullmatch('[0-9a-f]{40}', digest)
        assert dependencies.validate_identity(name, url, digest, digest, b'') == digest
        for field in ('origin', 'head', 'tag'):
            values = [url, digest, digest]
            values[['origin', 'head', 'tag'].index(field)] = 'unexpected'
            with pytest.raises(dependencies.DependencyError):
                dependencies.validate_identity(name, *values, b'')
        for status in (b' M src/Library.sol\0', b'?? injected.sol\0', b'!! ignored.sol\0'):
            with pytest.raises(dependencies.DependencyError, match='contents'):
                dependencies.validate_identity(name, url, digest, digest, status)
    producer = (CI / 'xweb_dependency_identity.py').read_text()
    assert "'--ignored=matching', '--ignore-submodules=none'" in producer
    assert 'validate_identity(name, origin, head, tag_commit, status)' in producer
    assert 'verify_tracked_bytes(directory)' in producer
    assert 'commit={digest} tracked-bytes=verified clean=true' in producer


def test_finding_11_tracked_byte_checks_detect_edits_hidden_from_git_status(tmp_path):
    path = tmp_path / 'Library.sol'
    data = b'contract ReviewedLibrary {}\n'
    path.write_bytes(data)
    path.chmod(0o644)
    digest = hashlib.sha1(b'blob ' + str(len(data)).encode() + b'\0' + data).hexdigest().encode()
    dependencies.verify_blob(path, b'100644', digest, b'0')
    path.write_bytes(b'contract ModifiedLibrary {}\n')
    with pytest.raises(dependencies.DependencyError, match='tracked bytes'):
        dependencies.verify_blob(path, b'100644', digest, b'0')
    path.unlink()
    path.symlink_to(tmp_path / 'other.sol')
    with pytest.raises(dependencies.DependencyError, match='unsupported'):
        dependencies.verify_blob(path, b'100644', digest, b'0')


def test_finding_11_uninitialized_upstream_gitlinks_cannot_hide_source(tmp_path):
    directory = tmp_path / 'forge-std'
    directory.mkdir()
    dependencies.verify_blob(directory, b'160000', b'a' * 40, b'0')
    (directory / 'unexpected.sol').write_text('contract Unexpected {}\n')
    with pytest.raises(dependencies.DependencyError, match='submodule source'):
        dependencies.verify_blob(directory, b'160000', b'a' * 40, b'0')


def workflow_module():
    return embedded('xweb-workflow-check.sh', 'PYCHECK')[0]


def test_finding_15_kept_inventory_uses_current_authority_and_actual_inputs(tmp_path):
    checker = workflow_module()
    assert checker.SPEC_PATH == 'spec/layerx-merge/spec.kvx'
    assert checker.TASK_AUTHORITY == '22.33.789'
    declared = checker.declared_paths(str(ROOT))
    assert checker.REFUSALS == []
    assert declared == set(checker.KEPT_XWEB_PATHS)
    assert len(checker.FILTERS) == len(set(checker.FILTERS))
    for path in declared:
        assert (ROOT / path).exists()
        assert not any(retired in path for retired in ('cmd/layerxd', 'custody', 'core-kernel', 'src/modules/web'))
    checker.present_or_declared(str(tmp_path), {'missing/input.c'}, 'missing/input.c', 'build input')
    assert len(checker.REFUSALS) == 1
    assert 'current build inputs must exist' in checker.REFUSALS[0]
    checker.REFUSALS.clear()
    checker.check_command_paths(str(ROOT), declared, 'sidecar', None,
                                ['cargo', 'test', '--manifest-path', 'services/interop/Cargo.toml', '-p', 'x-websearch'])
    assert checker.REFUSALS == []
    shell = (CI / 'xweb-workflow-check.sh').read_text()
    assert 'paxeer-x-web' not in shell
    assert 'DECLARED_PATHS' not in shell
    assert 'is_declared(' not in shell
    assert 'FEATURE_MAKE_TARGETS' not in shell
    assert 'cmd/layerxd/' not in shell


def test_finding_15_workflow_owner_wiring_is_recorded_without_blessing_absent_filters():
    checker = workflow_module()
    document = yaml.load((ROOT / checker.WORKFLOW_PATH).read_text(), Loader=yaml.BaseLoader)
    checker.check_filters(str(ROOT), checker.declared_paths(str(ROOT)), document['on'])
    assert any('path filters are' in refusal for refusal in checker.REFUSALS)
    record = (ROOT / 'tools/qualification/audit_repairs/repair_22_33_789.kvx').read_text()
    assert '.github/workflows/xweb-test.yml' in record
    assert 'fm-tools-ops' in record
    assert '22.33.700' in record


def test_repair_record_maps_all_findings_and_declares_limits():
    record = (ROOT / 'tools/qualification/audit_repairs/repair_22_33_789.kvx').read_text()
    for number in (8, 9, 10, 11, 12, 13, 14, 15):
        assert '[finding.' + str(number) + ']' in record
    for test in re.findall(r'^test = "([^"]+)"', record, re.M):
        assert 'def ' + test + '(' in Path(__file__).read_text()
    assert 'not run' in record
    assert 'review' in record
    assert 'hf-done' in record
