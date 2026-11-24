"""Recorded core peer: consume the adopted native files and remain alive."""
import hashlib
import json
import os
from pathlib import Path
import signal
import sys

core = Path(os.environ['LAYERX_CORE_ENV_FILE'])
result = {
    'pid': os.getpid(),
    'generation': os.environ['LAYERX_GENERATION_NUMBER'],
    'public_key': os.environ['LAYERX_NODE_SEQUENCER_PUBLIC_KEY'],
    'rpc': os.environ['LAYERX_CORE_CHAIN_RPC_URL'],
    'funding_key': os.environ['LAYERX_CORE_FUNDING_SENDER_KEY_FILE'],
    'artifacts': {path.name: hashlib.sha256(path.read_bytes()).hexdigest()
                  for path in core.parent.iterdir()},
    'trust': hashlib.sha256(Path(os.environ['LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER'])
                            .read_bytes()).hexdigest(),
    'keys': sorted(key for key in os.environ if key.startswith('LAYERX_')),
}
output = Path(sys.argv[1])
pending = output.with_suffix('.pending')
pending.write_text(json.dumps(result))
pending.replace(output)
signal.pause()
