"""Run the audited assertions against actual producer files and policy inputs.

The feature clients use local JSON/KVX file transport. No HTTP service, signing
toolchain, environment variable or synthetic production catalogue is involved.
"""

import importlib.util
from pathlib import Path
import tempfile


HERE = Path(__file__).resolve().parent
DIRECTORY = HERE.parent


def load_suite(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + '.py'))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


phase2 = load_suite('test_phase2')
phase4 = load_suite('test_phase4')
inventory_suite = phase4.fixture


def test_catalog_test_missing_input():
    case = phase2.Phase2Tests(
        'test_actual_v2_catalog_funding_voice_bounty_automation_stay_unavailable'
    )
    case.test_actual_v2_catalog_funding_voice_bounty_automation_stay_unavailable()


def test_closure_test_missing_input():
    case = phase4.Phase4Tests(
        'test_closure_manifest_uses_all200_lane_tasks_and_real_serial_gate_commands'
    )
    with tempfile.TemporaryDirectory() as temporary:
        case.root = Path(temporary)
        case.now = 2000000000
        case.test_closure_manifest_uses_all200_lane_tasks_and_real_serial_gate_commands()


def test_inventory_test_missing_input():
    case = inventory_suite.PolicyTests(
        'test_exact_inventory_and_reproducible_spec_materialization'
    )
    case.inventory = inventory_suite.p.strict_json(
        (DIRECTORY / 'inventory.json').read_bytes()
    )
    case.test_exact_inventory_and_reproducible_spec_materialization()


def test_recorded_inventory_reaches_real_policy_and_collector():
    inventory = phase2.policy.strict_json((DIRECTORY / 'inventory.json').read_bytes())
    assert set(phase2.policy.validate_inventory(inventory)) == phase2.policy.FEATURES
    case = phase2.Phase2Tests()
    case.setUp()
    try:
        assert case.inventory == inventory
        case.test_actual_inventory_all_55_owners_and_missing_proofs_remain_blocked()
        case.test_dependency_declaration_does_not_create_owner_execution()
    finally:
        case.doCleanups()


def test_catalog_missing_input_has_no_fallback():
    with tempfile.TemporaryDirectory() as temporary:
        try:
            phase2.read_capability_catalog(Path(temporary))
        except FileNotFoundError as error:
            assert Path(error.filename) == Path(temporary) / 'capabilities.json'
        else:
            raise AssertionError('missing authoritative catalogue was accepted')


def test_catalog_mismatched_input_rejected():
    root = DIRECTORY.parents[3]
    with tempfile.TemporaryDirectory() as temporary:
        bundle = Path(temporary)
        (bundle / 'capabilities.json').write_bytes(
            (root / 'sdk/launchpad/protocol/contracts/v1/capabilities.json').read_bytes()
        )
        (bundle / 'catalog.json').write_bytes(
            (root / 'sdk/launchpad/packages/sdk-v2/src/schemas.gen.json').read_bytes()
        )
        try:
            phase2.read_capability_catalog(bundle)
        except phase2.policy.Rejected as error:
            assert str(error) == 'selected catalog source digest mismatch'
        else:
            raise AssertionError('mismatched producer inputs were accepted')


def test_specification_uses_programme_kvx_reader():
    generator_spec = importlib.util.spec_from_file_location(
        'repair_inventory_generator', DIRECTORY / 'build_inventory.py'
    )
    generator = importlib.util.module_from_spec(generator_spec)
    generator_spec.loader.exec_module(generator)
    specification = generator.specification(generator.SPEC)
    assert specification['meta']['slug'] == 'layerx-merge'
    assert specification['decision.1803']
    assert generator.build(generator.SPEC) == inventory_suite.p.strict_json(
        (DIRECTORY / 'inventory.json').read_bytes()
    )
    references = generator.inventory_references(generator.build(generator.SPEC))
    assert references['spec'] == 'spec/layerx-merge/spec.kvx'
    assert references['references']
    assert all(item['class'] == 'retired-reference' for item in references['references'])
