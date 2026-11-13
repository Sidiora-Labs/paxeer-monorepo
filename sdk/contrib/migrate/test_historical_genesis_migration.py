"""Regression proof for historical-migration-validation through the JSON CLI."""

from copy import deepcopy
import json
from pathlib import Path
import sys
from types import ModuleType, SimpleNamespace

import pytest


MIGRATION_PATH = Path(__file__).with_name('v0.33.x-to-v0.34.0.py')
FIXTURE_PATH = Path(__file__).with_name('fixtures') / 'v0.33-genesis.json'


@pytest.fixture(params=[0, 1, 2], ids=['normal', 'optimized', 'fully-optimized'])
def migration(request, monkeypatch):
    monkeypatch.syspath_prepend(str(MIGRATION_PATH.parent))
    module = ModuleType('historical_migration')
    module.__file__ = str(MIGRATION_PATH)
    module.code = compile(
        MIGRATION_PATH.read_text(), str(MIGRATION_PATH), 'exec',
        optimize=request.param,
    )
    exec(module.code, module.__dict__)
    return module


@pytest.fixture
def genesis():
    return json.loads(FIXTURE_PATH.read_text())


def run_cli(migration, genesis, tmp_path, monkeypatch):
    exported = tmp_path / 'genesis.json'
    exported.write_text(json.dumps(genesis))
    monkeypatch.setattr(sys, 'argv', [
        str(MIGRATION_PATH), str(exported),
        '--chain-id', '  cosmoshub-migrated  ',
        '--start-time', '2019-02-11T12:00:00Z',
    ])
    exec(migration.code, {'__name__': '__main__', '__file__': str(MIGRATION_PATH)})


@pytest.mark.parametrize('block_key', ['block_size', 'block'])
@pytest.mark.parametrize('proposal_count', [0, 1, 2])
def test_supported_genesis_json_preserves_proposal_facts(
    migration, genesis, tmp_path, monkeypatch, capsys, block_key, proposal_count,
):
    if block_key == 'block':
        genesis['consensus_params']['block'] = (
            genesis['consensus_params'].pop('block_size')
        )
    proposals = genesis['app_state']['gov']['proposals'][:proposal_count]
    genesis['app_state']['gov']['proposals'] = proposals
    original = deepcopy(genesis)
    expected = deepcopy(genesis)
    block = expected['consensus_params'].pop(block_key)
    block.update(time_iota_ms='1000', max_gas='2000000', max_bytes='200000')
    expected['consensus_params']['block'] = block
    expected['app_state']['crisis'] = {
        'constant_fee': {'amount': '1333000000', 'denom': 'uatom'},
    }
    expected['app_state']['mint']['params']['blocks_per_year'] = '4855015'
    expected['app_state']['bank']['send_enabled'] = True
    expected['app_state']['distr']['withdraw_addr_enabled'] = True
    expected['chain_id'] = 'cosmoshub-migrated'
    expected['genesis_time'] = '2019-02-11T12:00:00Z'
    expected['app_state']['gov']['proposals'] = [
        {
            'proposal_content': {
                'type': 'gov/TextProposal',
                'value': {
                    'title': proposal['value']['title'],
                    'description': proposal['value']['description'],
                },
            },
            **{
                key: value for key, value in proposal['value'].items()
                if key not in {'title', 'description'}
            },
        }
        for proposal in original['app_state']['gov']['proposals']
    ]

    run_cli(migration, genesis, tmp_path, monkeypatch)

    captured = capsys.readouterr()
    assert captured.err == ''
    assert json.loads(captured.out) == expected
    assert json.loads((tmp_path / 'genesis.json').read_text()) == original


REJECTION_FIXTURES = [
    pytest.param({'type': 'gov/ParameterChangeProposal'}, 'invalid proposal type',
                 id='parameter-change'),
    pytest.param({'type': 'gov/CommunityPoolSpendProposal'}, 'invalid proposal type',
                 id='community-pool-spend'),
    pytest.param({'type': ''}, 'invalid proposal type', id='empty-type'),
    pytest.param({'type': None}, 'invalid proposal type', id='null-type'),
    pytest.param({'unexpected': {'keep': ['all', 'data']}}, 'only type and value',
                 id='extra-field'),
    pytest.param({'proposal_content': {'keep': 'content'}}, 'only type and value',
                 id='content-collision'),
    pytest.param({'proposal_id': '999'}, 'only type and value', id='id-collision'),
]


@pytest.mark.parametrize('patch, message', REJECTION_FIXTURES)
@pytest.mark.parametrize('position', [0, 1], ids=['first-proposal', 'later-proposal'])
def test_rejects_unsupported_proposals_before_any_mutation(
    migration, genesis, patch, message, position,
):
    gov_data = genesis['app_state']['gov']
    gov_data['proposals'][position].update(deepcopy(patch))
    original = deepcopy(genesis)

    with pytest.raises(ValueError, match=message):
        migration.migrate_gov_data(gov_data)
    assert genesis == original

    args = SimpleNamespace(chain_id='cosmoshub-migrated', start_time='new-time')
    with pytest.raises(ValueError, match=message):
        migration.process_raw_genesis(genesis, args)
    assert genesis == original


@pytest.mark.parametrize('patch, message', REJECTION_FIXTURES)
@pytest.mark.parametrize('position', [0, 1], ids=['first-proposal', 'later-proposal'])
def test_json_cli_rejects_unsupported_proposals_without_output(
    migration, genesis, tmp_path, monkeypatch, capsys, patch, message, position,
):
    genesis['app_state']['gov']['proposals'][position].update(deepcopy(patch))
    original = deepcopy(genesis)

    with pytest.raises(ValueError, match=message) as rejected:
        run_cli(migration, genesis, tmp_path, monkeypatch)

    if 'type' in patch:
        assert repr(patch['type']) in str(rejected.value)
    captured = capsys.readouterr()
    assert captured.out == ''
    assert captured.err == ''
    assert json.loads((tmp_path / 'genesis.json').read_text()) == original


def test_supported_governance_migrates_in_place(migration, genesis):
    gov_data = genesis['app_state']['gov']
    proposal_list = gov_data['proposals']
    proposals = list(proposal_list)
    values = [deepcopy(proposal['value']) for proposal in proposals]

    migration.migrate_gov_data(gov_data)

    assert gov_data['proposals'] is proposal_list
    for index, proposal in enumerate(proposal_list):
        assert proposal is proposals[index]
        assert 'type' not in proposal
        assert 'value' not in proposal
        content = proposal['proposal_content']
        assert content['type'] == 'gov/TextProposal'
        assert {'title': content['value']['title'],
                'description': content['value']['description'],
                **{key: value for key, value in proposal.items()
                   if key != 'proposal_content'}} == values[index]
