"""Routing repairs through the production filesystem codecs and consumers."""

import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import sys
import tarfile

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from dispatcher.evidence import EvidenceError, _paths, source_digest
from dispatcher.patches import PatchError, _reserved, build_patch, stage_patches
from dispatcher.source_policy import ignored
from dispatcher.workspace import WorkspaceError, create_archive, create_workspace


REPOSITORY = Path(__file__).resolve().parents[5]
DISPATCHER = REPOSITORY / 'tools/launchpad/production/dispatcher'
GO_INPUTS = (
    'sdk/launchpad/protocol/coverage/coverage.go',
    'sdk/launchpad/protocol/coverage/authorities.json',
    'sdk/launchpad/shared/secrets/manager.go',
    'sdk/launchpad/shared/secrets/manager_test.go',
)
LEAF_LANES = (
    ('sdk/launchpad/packages/sdk/**', (
        'sdk/launchpad/packages/sdk/package.json',
        'sdk/launchpad/packages/sdk/src/types.gen.ts',
    )),
    ('apps/launchpad/mobile/**', ('apps/launchpad/mobile/package.json',)),
)


def write(root, relative, content):
    path = root / relative
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(content)
    return path


def stage_and_compare(base, candidate, patch, stage, owned):
    expected = source_digest(candidate)
    observed = []

    def verify(tree):
        # This is the actual consumer check, executed by production staging.
        assert source_digest(tree) == expected
        for entry in patch['entries']:
            path = entry['path']
            assert (tree / path).read_bytes() == (candidate / path).read_bytes()
        observed.append(tree)
        return True

    # Reopen the producer's on-disk JSON manifest; no in-memory substitute.
    result = stage_patches(base, [Path(patch['manifest_path'])], stage,
                           verify=verify, ownership={patch['patch_digest']: owned})
    assert result['qualified'] is True
    assert observed == [stage]
    assert source_digest(stage) == expected
    return result


@pytest.mark.parametrize('lane,paths', LEAF_LANES)
def test_routing_moved_owned_leaves_round_trip(tmp_path, lane, paths):
    base = tmp_path / 'base'
    base.mkdir()
    write(base, 'anchor.txt', b'baseline\n')
    candidate = tmp_path / 'candidate'
    shutil.copytree(base, candidate)
    for relative in paths:
        write(candidate, relative, (REPOSITORY / relative).read_bytes())
    patch = build_patch(base, candidate, [lane], tmp_path / 'patch')
    manifest = json.loads(Path(patch['manifest_path']).read_bytes())
    assert manifest['owned'] == [lane]
    assert {entry['path'] for entry in manifest['entries']} == set(paths)
    stage_and_compare(base, candidate, patch, tmp_path / 'stage', [lane])
    with pytest.raises(PatchError, match='outside ownership'):
        build_patch(base, candidate, ['services/launchpad/api/**'], tmp_path / 'foreign')

    def reject_unexpected_verification(_):
        pytest.fail('Foreign ownership reached the verifier')

    with pytest.raises(PatchError, match='trusted ownership'):
        stage_patches(base, [patch], tmp_path / 'foreign-stage', verify=reject_unexpected_verification,
                      ownership={patch['patch_digest']: ['services/launchpad/api/**']})
    assert not (tmp_path / 'foreign-stage').exists()


def test_routing_all_fifty_handoffs_and_foreign_lane_refusal(tmp_path):
    base = tmp_path / 'base'
    base.mkdir()
    write(base, 'anchor.txt', b'baseline\n')
    for number in range(1, 51):
        lane = f'L{number:02d}'
        prefix = f'tools/launchpad/production/dispatcher/tests/fixtures/repair_22_33_915/handoffs/{lane}'
        candidate = tmp_path / lane
        shutil.copytree(base, candidate)
        relative = prefix + '/phase1/result.json'
        write(candidate, relative, json.dumps({'lane': lane, 'qualified': False}).encode())
        owned = [prefix + '/**']
        patch = build_patch(base, candidate, owned, tmp_path / ('patch-' + lane))
        assert [entry['path'] for entry in patch['entries']] == [relative]
        stage_and_compare(base, candidate, patch, tmp_path / ('stage-' + lane), owned)
        foreign = f'L{number % 50 + 1:02d}'
        with pytest.raises(PatchError, match='outside ownership'):
            build_patch(base, candidate,
                        [f'tools/launchpad/production/dispatcher/tests/fixtures/repair_22_33_915/handoffs/{foreign}/**'],
                        tmp_path / ('foreign-' + lane))


FORBIDDEN = (
    'sdk/launchpad/shared/interfaces/v1/schema.json',
    'sdk/launchpad/shared/interfaces/v2/contract.go',
    'sdk/launchpad/packages/production-contracts/src/validate.ts',
    'sdk/launchpad/packages/production-contracts/src/types.gen.ts',
    'sdk/launchpad/protocol/contracts/v1/chain/abi-catalog.json',
    'tools/launchpad/production/dispatcher/tests/fixtures/repair_22_33_915/spec.kvx',
    'tools/launchpad/production/dispatcher/tests/fixtures/repair_22_33_915/execution-plan.json',
    'tools/launchpad/production/dispatcher/tests/fixtures/repair_22_33_915/handoffs/L00/result.json',
    'tools/launchpad/production/dispatcher/tests/fixtures/repair_22_33_915/handoffs/L51/result.json',
    'tools/launchpad/production/dispatcher/tests/fixtures/repair_22_33_915/handoffs/L1/result.json',
    'sdk/launchpad/packages/sdk/nested/package.json',
    'apps/launchpad/mobile/nested/package.json',
    'sdk/launchpad/packages/sdk-v2/package.json',
    'sdk/launchpad/packages/sdk/src/other.gen.ts',
    'sdk/launchpad/packages/sdk/types.gen.ts',
    'apps/launchpad/mobile/src/types.gen.ts',
    'sdk/launchpad/packages/sdk/src/schema.gen.go',
    'sdk/launchpad/packages/sdk/pnpm-lock.yaml',
    'apps/launchpad/mobile/package-lock.json',
    'sdk/launchpad/packages/sdk/migrations/001.sql',
    'sdk/launchpad/shared/go.mod',
    'go.work',
    'client/packages/sdk/package.json',
    'client/packages/sdk/src/types.gen.ts',
    'client/apps/mobile/package.json',
    'spec/production-system/handoffs/L01/result.json',
)


@pytest.mark.parametrize('relative', FORBIDDEN)
def test_routing_reserved_paths_refused_by_real_patch_producer(tmp_path, relative):
    base = tmp_path / 'base'
    candidate = tmp_path / 'candidate'
    base.mkdir()
    candidate.mkdir()
    write(candidate, relative, b'reserved input\n')
    assert _reserved(relative)
    with pytest.raises(PatchError):
        build_patch(base, candidate, [str(Path(relative).parent) + '/**'], tmp_path / 'patch')
    assert not (tmp_path / 'patch').exists()


def test_routing_contract_roots_and_unsafe_ownership_remain_reserved(tmp_path):
    for root in ('sdk/launchpad/shared/interfaces',
                 'sdk/launchpad/packages/production-contracts', 'launchpad/spec'):
        assert _reserved(root)
        assert _reserved(root + '/**')
    base = tmp_path / 'base'
    candidate = tmp_path / 'candidate'
    base.mkdir()
    candidate.mkdir()
    write(candidate, 'apps/launchpad/mobile/package.json', b'{}')
    for pattern in ('**', '*', '/apps/launchpad/mobile/**',
                    'apps/launchpad/mobile/../mobile/**', 'apps//launchpad/mobile/**',
                    'apps\\launchpad\\mobile\\**',
                    'tools/launchpad/production/dispatcher/tests/fixtures/repair_22_33_915/handoffs/**'):
        with pytest.raises(PatchError):
            build_patch(base, candidate, [pattern], tmp_path / 'unsafe')


def test_source_policy_moved_go_digest_archive_and_stage(tmp_path):
    base = tmp_path / 'base'
    base.mkdir()
    for relative in GO_INPUTS:
        write(base, relative, (REPOSITORY / relative).read_bytes())
    before = source_digest(base)
    baseline_archive = tmp_path / 'baseline.tar'
    archive_sha = create_archive(base, baseline_archive)
    candidate = tmp_path / 'candidate'
    assert create_workspace(baseline_archive, candidate) == archive_sha
    assert source_digest(candidate) == before
    for relative in (GO_INPUTS[0], GO_INPUTS[2]):
        previous = source_digest(candidate)
        with (candidate / relative).open('ab') as stream:
            stream.write(b'\n// Routing identity regression input.\n')
        assert source_digest(candidate) != previous
    declared = _paths(list(GO_INPUTS), candidate)
    assert declared == {
        str(candidate / relative): hashlib.sha256((candidate / relative).read_bytes()).hexdigest()
        for relative in GO_INPUTS
    }
    bound = source_digest(candidate)
    excluded = (
        'sdk/launchpad/protocol/coverage/coverage.out',
        'sdk/launchpad/protocol/coverage/coverage/index.html',
        'sdk/launchpad/protocol/coverage/dist/generated.go',
        'sdk/launchpad/protocol/coverage/node_modules/module/a.js',
        'sdk/launchpad/protocol/coverage/secrets/key.go',
        'sdk/launchpad/protocol/coverage/.env',
        'sdk/launchpad/shared/secrets/credentials.json',
        'sdk/launchpad/shared/secrets/private.pem',
        'sdk/launchpad/shared/secrets/.env.local',
        'sdk/launchpad/shared/secrets/nested/credentials.go',
        'other/sdk/launchpad/protocol/coverage/not-source.go',
        'other/sdk/launchpad/shared/secrets/not-source.go',
    )
    for relative in excluded:
        write(candidate, relative, b'excluded material\n')
        assert ignored(relative)
        with pytest.raises(EvidenceError, match='source policy'):
            _paths([relative], candidate)
    # Ignored state must be filtered before lstat/read, including dangling links.
    for relative in ('sdk/launchpad/shared/secrets/key.txt',
                     'sdk/launchpad/protocol/coverage/.env.secret'):
        (candidate / relative).symlink_to(tmp_path / 'unread-credential')
    assert source_digest(candidate) == bound
    archive = tmp_path / 'candidate.tar'
    digest = create_archive(candidate, archive)
    assert digest == hashlib.sha256(archive.read_bytes()).hexdigest()
    with tarfile.open(archive, 'r:') as transport:
        members = {member.name: member for member in transport if member.isfile()}
        assert set(members) == set(GO_INPUTS)
        for relative in GO_INPUTS:
            assert transport.extractfile(members[relative]).read() == (candidate / relative).read_bytes()
    extracted = tmp_path / 'extracted'
    assert create_workspace(archive, extracted) == digest
    assert source_digest(extracted) == bound
    owned = ['sdk/launchpad/protocol/coverage/**', 'sdk/launchpad/shared/secrets/**']
    patch = build_patch(base, candidate, owned, tmp_path / 'patch')
    assert {entry['path'] for entry in patch['entries']} == {GO_INPUTS[0], GO_INPUTS[2]}
    stage_and_compare(base, candidate, patch, tmp_path / 'stage', owned)
    for tree in (extracted, tmp_path / 'stage'):
        assert {path.relative_to(tree).as_posix() for path in tree.rglob('*') if path.is_file()} == set(GO_INPUTS)
    assert source_digest(base) == before


@pytest.mark.parametrize('kind', ('symlink', 'hardlink'))
@pytest.mark.parametrize('relative', (GO_INPUTS[0], GO_INPUTS[2]))
def test_source_policy_moved_source_links_refused_by_all_consumers(tmp_path, kind, relative):
    base = tmp_path / 'base'
    candidate = tmp_path / 'candidate'
    base.mkdir()
    candidate.mkdir()
    target = write(candidate, relative, (REPOSITORY / relative).read_bytes())
    external = write(tmp_path, 'external.go', target.read_bytes())
    target.unlink()
    if kind == 'symlink':
        target.symlink_to(external)
    else:
        os.link(external, target)
    with pytest.raises(EvidenceError):
        source_digest(candidate)
    with pytest.raises(EvidenceError):
        _paths([relative], candidate)
    with pytest.raises(WorkspaceError):
        create_archive(candidate, tmp_path / 'unsafe.tar')
    with pytest.raises(PatchError):
        build_patch(base, candidate, [str(Path(relative).parent) + '/**'], tmp_path / 'patch')
    assert not (tmp_path / 'unsafe.tar').exists()
    assert not (tmp_path / 'patch').exists()


def test_routing_documented_real_inputs_follow_archive_and_declared_input_policy(tmp_path):
    readme = (DISPATCHER / 'README.md').read_text()
    assert 'tools/production/dispatcher' not in readme
    commands = re.findall(r'(?:python3|python3 -m unittest) (tools/[^\s]+\.py)', readme)
    assert commands
    for command in commands:
        assert (REPOSITORY / command).is_file()
        assert command.startswith('tools/launchpad/production/dispatcher/')
    assert '--spec "$LAUNCHPAD_SPEC_FILE"' in readme
    assert '--plan "$LAUNCHPAD_PLAN_FILE"' in readme
    assert 'Set the shell variables `LAUNCHPAD_SPEC_FILE` and `LAUNCHPAD_PLAN_FILE`' in readme
    assert 'does not qualify a product lane' in readme
    assert 'Local dispatcher fixtures do not exercise network consensus' in readme
    assert '500,000 DAU' in readme
    blocks = [json.loads(block) for block in re.findall(r'```json\n(.*?)\n```', readme, re.S)]
    inputs = blocks[1]
    assert inputs == [GO_INPUTS[2], GO_INPUTS[3]]
    source = tmp_path / 'source'
    source.mkdir()
    for relative in inputs:
        write(source, relative, (REPOSITORY / relative).read_bytes())
    archive = tmp_path / 'documented-inputs.tar'
    create_archive(source, archive)
    workspace = tmp_path / 'workspace'
    create_workspace(archive, workspace)
    assert source_digest(source) == source_digest(workspace)
    assert set(_paths(inputs, workspace)) == {str(workspace / relative) for relative in inputs}
    assert (REPOSITORY / 'sdk/launchpad/shared/go.mod').is_file()
    record = (DISPATCHER / 'repair_22_33_914.kvx').read_text()
    for finding in ('dispatcher-readme-moved-commands', 'dispatcher-reserved-paths-before-move',
                    'source-policy-drops-moved-code'):
        assert f'[finding.{finding}]' in record
