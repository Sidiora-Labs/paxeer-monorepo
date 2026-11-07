"""Exercise routing regressions with real filesystem producers and consumers."""

import hashlib
import json
import os
from pathlib import Path
import sys
import tomllib
import unittest

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[2]))
from dispatcher.evidence import EvidenceError, _paths, source_digest
from dispatcher.graph import Graph, GraphError
from dispatcher.patches import PatchError, build_patch
from dispatcher.workspace import WorkspaceError, create_archive, create_workspace
from dispatcher.tests import test_graph as graph_tests
from dispatcher.tests import test_owned_leaf_artifacts as leaf_tests
from dispatcher.tests import test_patches as patch_tests
from dispatcher.tests import test_qualification_fixture as fixture_tests
from dispatcher.tests import test_source_policy as policy_tests


REPOSITORY = Path(__file__).resolve().parents[5]
DOCUMENTS = REPOSITORY / 'launchpad/spec/production-system'
FIXTURE = Path(__file__).resolve().parent / 'fixtures/repair_22_33_915/spec.kvx'


def _methods(case):
    return unittest.defaultTestLoader.getTestCaseNames(case)


@pytest.fixture
def transported_documents(tmp_path):
    """Explicit test peer; canonical document discovery never falls back here."""
    producer = tmp_path / 'producer'
    documents = producer / 'launchpad/spec/production-system'
    documents.mkdir(parents=True)
    spec = documents / 'spec.kvx'
    spec.write_bytes(FIXTURE.read_bytes())
    graph = Graph(spec)
    definitions = tomllib.loads(spec.read_text())
    plan = {
        'spec_sha256': hashlib.sha256(spec.read_bytes()).hexdigest(),
        'tasks': graph.tasks,
        'lanes': [
            dict(id=lane, write_paths=declaration['write_paths'], tasks=[
                dict(id=identifier, **task)
                for identifier, task in graph.tasks.items() if task.get('lane') == lane
            ])
            for lane, declaration in definitions['lane'].items()
        ],
    }
    (documents / 'execution-plan.json').write_text(json.dumps(plan))
    archive = tmp_path / 'documents.tar'
    digest = create_archive(producer, archive)
    consumer = tmp_path / 'consumer'
    assert create_workspace(archive, consumer) == digest
    received = consumer / 'launchpad/spec/production-system'
    assert (received / 'spec.kvx').read_bytes() == spec.read_bytes()
    assert Graph(received / 'spec.kvx', received / 'execution-plan.json').status() == graph.status()
    return received


def test_routing_checkout_paths_and_explicit_fixture_override():
    assert graph_tests.ROOT == REPOSITORY
    assert graph_tests.SPEC == DOCUMENTS / 'spec.kvx'
    assert graph_tests.PLAN == DOCUMENTS / 'execution-plan.json'
    assert graph_tests.GraphTests.spec_path == DOCUMENTS / 'spec.kvx'
    assert graph_tests.GraphTests.plan_path == DOCUMENTS / 'execution-plan.json'
    assert patch_tests.PatchTests.spec_path == DOCUMENTS / 'spec.kvx'
    expected_root = Path(os.environ.get('PRODUCTION_DISPATCHER_SPEC_ROOT', REPOSITORY / 'launchpad'))
    assert fixture_tests.FixtureTests.root == expected_root
    record = (Path(__file__).parent / 'repair_22_33_915.kvx').read_text()
    for finding in ('graph-test-machine-root', 'owned-leaf-tests-old-layout',
                    'patch-test-machine-root', 'fixture-test-machine-root',
                    'source-policy-tests-old-layout'):
        assert '[finding.' + finding + ']' in record
    assert 'wiring_request' in record


@pytest.mark.parametrize('method', _methods(graph_tests.GraphTests))
def test_routing_graph_bounds_dependencies_and_ownership(transported_documents, method):
    case = graph_tests.GraphTests(method)
    case.spec_path = transported_documents / 'spec.kvx'
    case.plan_path = transported_documents / 'execution-plan.json'
    case.debug()


@pytest.mark.parametrize('method', _methods(leaf_tests.OwnedLeafArtifactTests))
def test_routing_current_owned_leaves_and_all_refusals(method):
    leaf_tests.OwnedLeafArtifactTests(method).debug()


@pytest.mark.parametrize('method', ('test_canonical_fifty_lane_ownership', 'test_handoff_exception_bounded'))
def test_routing_fifty_lane_patch_and_handoff_bounds(transported_documents, method):
    case = patch_tests.PatchTests(method)
    case.spec_path = transported_documents / 'spec.kvx'
    case.debug()


def test_routing_status_normalization_and_projection_mismatch(transported_documents):
    case = fixture_tests.FixtureTests('test_live_status_variants_are_private_and_projection_consistent')
    case.root = transported_documents.parents[1]
    case.debug()


@pytest.mark.parametrize('method', _methods(policy_tests.SourcePolicyTests))
def test_routing_source_policy_all_consumers_and_secret_refusals(method):
    policy_tests.SourcePolicyTests(method).debug()


@pytest.mark.parametrize('relative', (
    'sdk/launchpad/protocol/coverage/coverage.go',
    'sdk/launchpad/shared/secrets/manager.go',
))
@pytest.mark.parametrize('kind', ('symlink', 'hardlink'))
def test_routing_moved_source_links_refused_by_every_consumer(tmp_path, relative, kind):
    baseline = tmp_path / 'baseline'
    baseline.mkdir()
    candidate = tmp_path / 'candidate'
    target = candidate / relative
    target.parent.mkdir(parents=True)
    external = tmp_path / 'external.go'
    external.write_bytes((REPOSITORY / relative).read_bytes())
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
        build_patch(baseline, candidate, [str(Path(relative).parent) + '/**'], tmp_path / 'patch')
    assert not (tmp_path / 'unsafe.tar').exists()
    assert not (tmp_path / 'patch').exists()


def test_routing_explicit_missing_documents_fail_closed(tmp_path):
    missing = tmp_path / 'missing/launchpad/spec/production-system'
    with pytest.raises(GraphError, match='invalid graph input'):
        Graph(missing / 'spec.kvx', missing / 'execution-plan.json')
    case = fixture_tests.FixtureTests('test_live_status_variants_are_private_and_projection_consistent')
    case.root = missing.parents[1]
    with pytest.raises(FileNotFoundError):
        case.debug()
    case = patch_tests.PatchTests('test_canonical_fifty_lane_ownership')
    case.spec_path = missing / 'spec.kvx'
    with pytest.raises(FileNotFoundError):
        case.debug()
