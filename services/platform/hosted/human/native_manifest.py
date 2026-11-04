from pathlib import Path

from provision import protected_json, require, write_json


def configure(work_dir, network, sequencer, image, stateful):
    """Retain the explicit merged RPC configuration; no retired daemon sidecar."""
    root = Path(work_dir) / 'human-evidence-input'
    config = protected_json(root / 'owner-evm.json')
    require(config.get('network_id') == network, root, 'merged owner network binding')
    write_json(root / 'owner-native.json', config)
    return stateful


if __name__ == '__main__':
    import json
    import sys

    source = Path(sys.argv[5])
    value = configure(sys.argv[1], int(sys.argv[2]), sys.argv[3], sys.argv[4], json.loads(source.read_text()))
    write_json(Path(sys.argv[6]), value)
