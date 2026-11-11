#!/usr/bin/env python3
"""Admission checks for capability manifests and committed chain evidence."""
from pathlib import Path
import sys

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(Path(__file__).parent))
import caps_discovery_contract as contract

sys.path.insert(0, str(ROOT / 'services/agent/sdk/python'))
from layerx_sdk.committed import CommitTrust


def require(value, reason):
    if not value:
        raise RuntimeError(reason)


def admit_manifest(path, source):
    return contract.admit(path, source)


def admit_chain_evidence(value, trust, root, minimum_height=1):
    require(type(minimum_height) is int and minimum_height > 0, 'positive committed height required')
    require(set(trust) == {'chain_id', 'height', 'validators'}, 'independently pinned chain trust required')
    require(value.get('chain_id') == trust['chain_id'], 'wrong discovery network')
    pinned = CommitTrust(trust['chain_id'], trust['height'], tuple(trust['validators']))
    verified = pinned.verify(bytes.fromhex(value['transaction']), minimum_height, value['evidence'])
    require(verified.app_hash.hex() == root, 'discovery root is not the chain-committed application root')
    return verified
