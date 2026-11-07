from pathlib import Path

from provision import fields, h32, protected_bytes, protected_json, require, uint, write_json


def owner_config(work_dir, network, chain=None):
    """Validate the owner transport and funding inputs before retaining them."""
    import ssl
    from urllib.parse import urlsplit
    from merged_owner import evm
    root = Path(work_dir) / 'human-evidence-input'
    path = root / 'owner-evm.json'
    config = protected_json(path)
    fields(config, 'rpc_url rpc_ca_file network_id chain_id confirmations owner_seed_file evm_key_file asset_id funding_key_file funding_source funding_amount',
           path, 'merged owner configuration')
    for name, bits in [('network_id', 32), ('chain_id', 64), ('confirmations', 32), ('funding_amount', 128)]:
        uint(config[name], bits, path, name, 1)
    require(config['network_id'] == network and (chain is None or config['chain_id'] == chain),
            path, 'merged owner network and chain binding')
    require(type(config['rpc_url']) is str, path, 'HTTPS RPC URL')
    url = urlsplit(config['rpc_url'])
    require(url.scheme == 'https' and url.hostname and url.username is None and url.password is None
            and not url.query and not url.fragment, path, 'authenticated HTTPS JSON-RPC')
    ssl.create_default_context(cadata=protected_bytes(config['rpc_ca_file'], 65536).decode('ascii'))
    for name in ('asset_id', 'funding_source'):
        h32(config[name], path, name)
    require(len(protected_bytes(config['owner_seed_file'], 32)) == 32, path, 'owner Ed25519 seed')
    for name in ('evm_key_file', 'funding_key_file'):
        protected_bytes(config[name], 128)
        evm.read_key(config[name])
    for name in ('custody-credit.bin', 'owner-custody.json', 'custody.profile'):
        require(not (root / name).exists() and not (root / name).is_symlink(), root / name,
                'retained pre-merge input requires explicit migration; preserve original custody')
    return config


def configure(work_dir, network, sequencer, image, stateful, chain=None):
    """Retain the explicit merged RPC configuration; no retired daemon sidecar."""
    root = Path(work_dir) / 'human-evidence-input'
    from owner_kms import retain_json
    config = owner_config(work_dir, network, chain)
    retain_json(root / 'owner-native.json', config)
    return stateful


if __name__ == '__main__':
    import json
    import sys

    source = Path(sys.argv[5])
    value = configure(sys.argv[1], int(sys.argv[2]), sys.argv[3], sys.argv[4], json.loads(source.read_text()))
    write_json(Path(sys.argv[6]), value)
