"""Publish authenticated merged-chain receipts without a handover transport."""
import hashlib
import json
from pathlib import Path
import sys

from onboarding_material import write
from provision import protected_bytes, protected_json, require, strict_pairs

CHAIN_FILES = ('native-context.json', 'merged-owner-evidence.json', 'rpc-ca.pem', 'owner-registration.json')
CHAIN_ROLES = ('agent', 'security', 'authority')
LEGACY_FILES = ('trust-history', 'genesis-handover-trust.lxt', 'handover-finality.conf')


def refuse_legacy(material):
    material = Path(material)
    for role in CHAIN_ROLES:
        for name in LEGACY_FILES:
            path = material / role / name
            require(not path.exists() and not path.is_symlink(), path,
                    'retained pre-merge trust requires explicit migration; preserve original material')
    for name in ('genesis-trust-file', 'handover-finality-file'):
        path = material / 'authority-config' / name
        require(not path.exists() and not path.is_symlink(), path,
                'retained pre-merge trust requires explicit migration; preserve original material')
    for name in ('LAYERX_AGENT_SEQUENCER_TRUST_HISTORY', 'LAYERX_AGENT_GENESIS_TRUST', 'LAYERX_AGENT_HANDOVER_FINALITY'):
        path = material / 'agent-config' / name
        require(not path.exists() and not path.is_symlink(), path,
                'retained pre-merge trust configuration requires explicit migration; preserve original material')


def validate(files, expected_context, network, chain):
    from merged_owner import validate_evidence
    require(set(files) == set(CHAIN_FILES), 'chain receipts', 'complete current receipt material')
    require(0 < len(files['rpc-ca.pem']) <= 65536, 'chain receipts', 'bounded RPC CA')
    context = json.loads(files['native-context.json'], object_pairs_hook=strict_pairs)
    require(context == expected_context, 'chain receipts', 'independently pinned chain receipt context')
    registration = json.loads(files['owner-registration.json'], object_pairs_hook=strict_pairs)
    return validate_evidence(json.loads(files['merged-owner-evidence.json'], object_pairs_hook=strict_pairs),
        context, files['rpc-ca.pem'], registration, network, chain)


def verify(directory, expected_context, network, chain):
    directory = Path(directory)
    files = {name: protected_bytes(directory / name, 65536 if name == 'rpc-ca.pem' else 1048576)
             for name in CHAIN_FILES}
    return validate(files, expected_context, network, chain)


def publish(source, material, expected_context, network, chain):
    """Validate the whole retained set before writing any consumer projection."""
    source, material = Path(source), Path(material)
    refuse_legacy(material)
    files = {name: protected_bytes(source / name) for name in CHAIN_FILES}
    validate(files, expected_context, network, chain)
    from material import protected_file
    for role in CHAIN_ROLES:
        directory = material / role
        protected_file(directory, 0o700)
        for name, data in files.items():
            path = directory / name
            if path.exists() or path.is_symlink():
                require(protected_bytes(path) == data, path, 'identical retained chain receipts; reconciliation required')
    require(all(protected_bytes(source / name) == data for name, data in files.items()),
            source, 'unchanged receipt source during publication')
    for role in CHAIN_ROLES:
        for name, data in files.items():
            write(material / role / name, data)


def produce(work_dir, material, network, chain):
    from provision import funding_evidence
    work_dir = Path(work_dir)
    artifact = work_dir / 'genesis/genesis-handover-trust.lxt'
    require(not artifact.exists() and not artifact.is_symlink(), artifact,
            'retained pre-merge genesis trust requires explicit migration; preserve original material')
    funding_evidence(work_dir)
    source = work_dir / 'human-evidence-input'
    config = protected_json(source / 'owner-native.json')
    context = {key: config[key] for key in ('network_id', 'chain_id', 'rpc_url', 'confirmations')}
    context['rpc_ca_sha256'] = hashlib.sha256(protected_bytes(config['rpc_ca_file'], 65536)).hexdigest()
    publish(source, material, context, network, chain)


if __name__ == '__main__':
    produce(sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4]))
