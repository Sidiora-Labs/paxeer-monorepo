import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys


SCRIPT = Path(__file__).with_name('abi_baseline.py').resolve()
ROOT = SCRIPT.parents[4]
CONTRACTS = ROOT / 'contracts/launchpad'
ABI_ROOT = ROOT / 'precompiles/launchpad/abi'
BINDINGS = SCRIPT.parents[1] / 'bindings'
spec = importlib.util.spec_from_file_location('abi_baseline', SCRIPT)
abi_baseline = importlib.util.module_from_spec(spec)
spec.loader.exec_module(abi_baseline)


def run_baseline(output, *options):
    subprocess.run(
        [sys.executable, str(SCRIPT), '--contracts', str(CONTRACTS),
         '--output', str(output), *map(str, options)],
        cwd=output.parent, check=True, capture_output=True, text=True,
    )
    return json.loads(output.read_text())


def test_role_binding_compared_with_precompile_abi(tmp_path):
    report = run_baseline(tmp_path / 'baseline.json')
    bindings = {item['contract']: item for item in report['bindings']}
    roles = ('FeeAccumulator', 'FeesRouter', 'PoolRegistry', 'ProtocolConfig',
             'Quoter', 'Router', 'SidioraFactory', 'SidioraNFT')
    for name in roles:
        role_path = ABI_ROOT / (name + '.json')
        entries = json.loads(role_path.read_text())
        binding = BINDINGS / (name.lower() + '.go')
        assert abi_baseline.compare(binding, entries) == ([], [])
        item = bindings[name]
        assert item['source'] == 'role-abi'
        assert item['artifact'] == str(role_path)
        assert item['artifact_sha256'] == hashlib.sha256(role_path.read_bytes()).hexdigest()
        assert item['missing_signatures'] == []
        assert item['removed_signatures'] == []
        assert item['changed_schemas'] == []
        assert item['embedded_abi_sha256'] == item['current_abi_sha256']

    pool_entries = json.loads((ABI_ROOT / 'PoolRegistry.json').read_text())
    _, embedded_entries = abi_baseline.binding_metadata(BINDINGS / 'poolregistry.go')
    metadata = next(entry for entry in pool_entries if entry.get('name') == 'getPoolMetadata')
    embedded_metadata = next(entry for entry in embedded_entries if entry.get('name') == 'getPoolMetadata')
    assert metadata['outputs'][0]['internalType'] == 'struct IPoolRegistry.PoolMetadata'
    assert embedded_metadata['outputs'][0]['internalType'] == 'structIPoolRegistry.PoolMetadata'
    assert abi_baseline.canonical(metadata) == abi_baseline.canonical(embedded_metadata)
    assert metadata['outputs'][0]['internalType'] == 'struct IPoolRegistry.PoolMetadata'

    router_path = ABI_ROOT / 'Router.json'
    entries = json.loads(router_path.read_text())
    event_artifact = CONTRACTS / 'artifacts/contracts/data/EventEmitter.sol/EventEmitter.json'
    event_entries = json.loads(event_artifact.read_text())['abi']
    identities = {abi_baseline.identity(entry) for entry in entries}
    added_entry = next(entry for entry in event_entries if abi_baseline.identity(entry) not in identities)
    removed_identity = abi_baseline.identity(entries[0])
    assert sum(abi_baseline.identity(entry) == removed_identity for entry in entries) == 1
    added, removed = abi_baseline.compare(BINDINGS / 'router.go', entries[1:] + [added_entry])
    assert added == [abi_baseline.identity(added_entry)]
    assert removed == [removed_identity]


def test_non_role_binding_uses_artifact(tmp_path):
    custom_root = tmp_path / 'role-abis'
    custom_root.mkdir()
    (custom_root / 'Router.json').symlink_to(ABI_ROOT / 'Router.json')
    report = run_baseline(tmp_path / 'baseline.json', '--abi-root', custom_root)
    bindings = {item['contract']: item for item in report['bindings']}
    artifact = CONTRACTS / 'artifacts/contracts/data/EventEmitter.sol/EventEmitter.json'
    entries = json.loads(artifact.read_text())['abi']
    assert not (ABI_ROOT / 'EventEmitter.json').exists()
    item = bindings['EventEmitter']
    assert item['source'] == 'artifact'
    assert item['artifact'] == str(artifact)
    assert item['artifact_sha256'] == hashlib.sha256(artifact.read_bytes()).hexdigest()
    assert item['current_abi_sha256'] == hashlib.sha256(abi_baseline.canonical(entries).encode()).hexdigest()
    added, removed = abi_baseline.compare(BINDINGS / 'eventemitter.go', entries)
    assert item['missing_signatures'] == added
    assert item['removed_signatures'] == removed
    router = bindings['Router']
    assert router['source'] == 'role-abi'
    assert router['artifact'] == str(custom_root / 'Router.json')
    assert router['missing_signatures'] == []
    assert router['removed_signatures'] == []
