#!/usr/bin/env python3
"""Registry publication followed by direct paxd and hosted gateway qualification."""
import argparse
import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import socket
import stat
import sys
import time
from urllib.parse import urlsplit

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(Path(__file__).parent))
import merged_qualification as merged
import native_evidence as evidence

_spec = importlib.util.spec_from_file_location('bootstrap_registry_contract', Path(__file__).with_name('registry-contract.py'))
registry_contract = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(registry_contract)

STAGES = ('material', 'registry-bootstrap', 'paxd-gateway-activation', 'committed-proof')
PUBLIC = 'https://api-mainnet-beta.paxeer.network'
PUBLIC_HOSTS = tuple(f'api{n}.mainnet-beta.paxeer.network' for n in range(1, 15)) + ('api-mainnet-beta.paxeer.network',)


class Refused(Exception):
    pass


def require(condition, reason):
    if not condition:
        raise Refused(reason)


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def private_dir(path):
    info = os.lstat(path)
    require(
        stat.S_ISDIR(info.st_mode)
        and info.st_uid == os.geteuid()
        and stat.S_IMODE(info.st_mode) == 0o700,
        f"{path} is not a 0700 directory of this user",
    )


def private_file(path):
    info = os.lstat(path)
    require(
        stat.S_ISREG(info.st_mode)
        and info.st_nlink == 1
        and info.st_uid == os.geteuid()
        and stat.S_IMODE(info.st_mode) & 0o077 == 0,
        f"{path} is not a private regular file of this user",
    )


def validate_stage(directory, stage, revision, image, generation):
    require(stage in STAGES, "unknown deployment stage")
    require(
        isinstance(revision, str) and re.fullmatch(r"[0-9a-f]{40}", revision),
        "candidate revision pin absent",
    )
    require(
        isinstance(image, str) and re.fullmatch(r"\S+@sha256:[0-9a-f]{64}", image),
        "candidate image pin absent",
    )
    require(
        isinstance(generation, str) and re.fullmatch(r"[0-9a-f]{64}", generation),
        "material generation pin absent",
    )
    directory = Path(directory)
    private_dir(directory)
    previous_digest = None
    previous_finished = None
    for name in STAGES[: STAGES.index(stage) + 1]:
        path = directory / (name + ".json")
        private_file(path)
        row = json.loads(path.read_text())
        require(
            row.get("schema_version") == 1
            and row.get("stage") == name
            and row.get("outcome") == "passed",
            name + ": successful versioned stage record absent",
        )
        require(
            (row.get("revision"), row.get("image"), row.get("material_generation"))
            == (revision, image, generation),
            name + ": candidate/image/material mismatch",
        )
        require(
            row.get("previous_record_sha256") == previous_digest,
            name + ": prerequisite record changed",
        )
        require(
            isinstance(row.get("checks"), list)
            and row["checks"]
            and all(isinstance(check, str) and check for check in row["checks"])
            and len(row["checks"]) == len(set(row["checks"])),
            name + ": explicit distinct checks absent",
        )
        require(
            type(row.get("started_at")) is int
            and type(row.get("finished_at")) is int
            and 0 <= row["started_at"] <= row["finished_at"]
            and (previous_finished is None or previous_finished <= row["started_at"]),
            name + ": stage order invalid",
        )
        previous_digest = sha256(path)
        previous_finished = row["finished_at"]
    return previous_digest


AMINO_UINT64 = re.compile(r"0|[1-9][0-9]*")


def amino_uint64(value, reason):
    require(type(value) is str and AMINO_UINT64.fullmatch(value) is not None, reason)
    number = int(value)
    require(number < 1 << 64, reason)
    return number


def governed_base64_key(value):
    require(type(value) is str, "governed receipt key encoding differs")
    try:
        decoded = base64.b64decode(value, validate=True)
    except ValueError as error:
        raise Refused("governed receipt key bytes differ") from error
    require(
        len(decoded) == 32
        and decoded != bytes(32)
        and base64.b64encode(decoded).decode() == value,
        "governed receipt key bytes differ",
    )
    return decoded


def governed_receipt_keys(value):
    keys = json.loads(value)
    require(type(keys) is list and keys, "governed receipt key list is empty")
    pinned = {}
    for entry in keys:
        require(
            type(entry) is dict
            and set(entry) == {"key_id", "public_key", "first_height", "last_height"},
            "governed receipt key fields differ",
        )
        require(
            type(entry["key_id"]) is str and type(entry["public_key"]) is str,
            "governed receipt key encoding differs",
        )
        key_id = governed_base64_key(entry["key_id"])
        public_key = governed_base64_key(entry["public_key"])
        reason = "governed receipt key height range differs"
        first = amino_uint64(entry["first_height"], reason)
        last = amino_uint64(entry["last_height"], reason)
        require(first <= last, reason)
        require(key_id not in pinned, "governed receipt key id is duplicated")
        pinned[key_id] = (public_key, first, last)
    return pinned


def governed_receipt_key(pinned, key_id, height):
    require(key_id in pinned, "receipt key is not governed at the receipt height")
    public_key, first, last = pinned[key_id]
    require(first <= height <= last, "receipt height is outside its governed key range")
    return public_key


def public_endpoints():
    return {host: 'https://' + host + '/' for host in PUBLIC_HOSTS}


def observe_public_dns():
    return {host: sorted({row[4][0] for row in socket.getaddrinfo(host, 443, type=socket.SOCK_STREAM)})
            for host in PUBLIC_HOSTS}


def observe_public_nodes(receipt_keys):
    observations = {}
    for host, endpoint in public_endpoints().items():
        node = merged.CHAIN.Node(endpoint, receipt_keys)
        chain = node.call('eth_chainId', [])
        block = node.call('eth_getBlockByNumber', ['finalized', False])
        require(chain == '0x7d' and int(block['number'], 16) > 0 and int(block['hash'], 16) != 0,
                host + ': native root JSON-RPC unavailable')
        observations[host] = {'chain_id': chain, 'block_number': block['number'], 'block_hash': block['hash']}
    return observations


def case_public_interface(staged, **_):
    require(staged, 'runtime DNS and endpoint evidence required')
    document = staged['doc']
    dns = document['dns']
    require(set(dns) == {'before', 'after'} and all(set(dns[phase]) == set(PUBLIC_HOSTS) for phase in dns),
            'fourteen node hosts and aggregate DNS inventory required')
    for host in PUBLIC_HOSTS:
        require(isinstance(dns['before'][host], list) and dns['before'][host]
                and all(isinstance(value, str) and value for value in dns['before'][host])
                and sorted(dns['before'][host]) == sorted(dns['after'][host]), host + ': DNS changed or unrecorded')
    require(document['node_endpoints'] == public_endpoints(), 'public root JSON-RPC endpoints differ')
    observations = document['node_observations']
    require(set(observations) == set(PUBLIC_HOSTS), 'runtime native root endpoint observations required')
    for host, observation in observations.items():
        require(set(observation) == {'chain_id', 'block_number', 'block_hash'}
                and observation['chain_id'] == '0x7d'
                and isinstance(observation['block_number'], str)
                and re.fullmatch(r'0x[1-9a-f][0-9a-f]*', observation['block_number'])
                and isinstance(observation['block_hash'], str)
                and re.fullmatch(r'0x[0-9a-f]{64}', observation['block_hash'])
                and int(observation['block_hash'], 16) != 0, host + ': native endpoint observation differs')
    gateway = merged.CHAIN.endpoint(document['gateway_endpoint'])
    require(gateway.scheme == 'https' and gateway.hostname not in PUBLIC_HOSTS,
            'separately hosted root px_ gateway required')
    return 'fourteen native root JSON-RPC hosts, aggregate host and separate hosted gateway observed'


def verify_material(files):
    require(set(files) == {'request_token', 'publication_token', 'client_ca', 'client_certificate', 'client_key'},
            'complete registry identity and distinct credential roles required')
    values = {name: evidence.protected(path) for name, path in files.items()}
    require(values['request_token'].strip() and values['publication_token'].strip()
            and values['request_token'].strip() != values['publication_token'].strip(), 'distinct retained credential roles required')
    import ssl
    context = ssl.create_default_context(cadata=values['client_ca'].decode())
    context.load_cert_chain(files['client_certificate'], files['client_key'])
    return {name: hashlib.sha256(value).hexdigest() for name, value in values.items()}


class NativeBootstrap:
    def __init__(self, directory, revision, image, generation, discovery, gateway, case, material_files):
        self.directory = Path(directory)
        private_dir(self.directory)
        require(re.fullmatch('[0-9a-f]{40}', revision), 'candidate revision pin')
        require(re.fullmatch(r'\S+@sha256:[0-9a-f]{64}', image), 'candidate image pin')
        require(re.fullmatch('[0-9a-f]{64}', generation), 'material generation pin')
        require(discovery.node.url != gateway.gateway_url and discovery.node.url == gateway.node.url,
                'native node and hosted gateway must be separate and share one native chain')
        self.revision, self.image, self.generation = revision, image, generation
        self.discovery, self.gateway, self.case, self.material_files = discovery, gateway, case, material_files

    def prerequisite(self, stage):
        require(stage in STAGES, 'unknown native bootstrap stage')
        index = STAGES.index(stage)
        if index:
            try:
                return validate_stage(self.directory, STAGES[index - 1], self.revision, self.image, self.generation)
            except (OSError, ValueError, KeyError, Refused) as error:
                raise Refused(f'fail {stage} missing={STAGES[index - 1]} producer=stage:{STAGES[index - 1]}') from error
        return None

    def stage(self, stage, source=None, publication_key=None, public_evidence=None):
        previous = self.prerequisite(stage)
        started = time.time_ns()
        # Revalidate secrets on every transition; no failed activation can
        # replace a credential, publish a success record or use stale material.
        material = verify_material(self.material_files)
        require(hashlib.sha256(evidence.canonical(material)).hexdigest() == self.generation,
                'material generation differs from retained identities')
        if stage == 'material':
            checks = ['private-identity', 'distinct-credential-roles', 'tls-identity']
        elif stage == 'registry-bootstrap':
            require(source is not None and publication_key, 'explicit source publication intent required')
            route = '/v1/programs/registry/' + self.case['program_id'] + '/source'
            status, _ = self.discovery.registry.request('POST', route, evidence.canonical(source), publication_key, authenticated=False)
            require(status in (401, 403), 'unauthenticated registry publication admitted')
            status, _ = self.discovery.registry.publish(self.case['program_id'], source, publication_key)
            require(status == 200, 'authenticated registry publication failed')
            observed = self.discovery.read(self.case)
            provenance = observed['registry']['versions'][-1]['source']
            require(provenance['status'] == 'verified' and provenance['source_digest'] == source['source_digest']
                    and re.fullmatch('[0-9a-f]{64}', provenance['environment_digest']), 'source publication provenance')
            checks = ['publication-authentication', 'native-discovery', 'committed-publication', 'source-provenance']
        elif stage == 'paxd-gateway-activation':
            self.discovery.read(self.case)
            capabilities = self.gateway.capabilities()
            require(capabilities['programs'], 'gateway Programs unavailable')
            checks = ['native-registry-consumer', 'hosted-px-capabilities', 'committed-capability-block']
        else:
            self.discovery.read(self.case)
            case_public_interface({'doc': public_evidence})
            checks = ['committed-receipt', 'current-native-program', 'public-dns', 'root-endpoints', 'separate-px-gateway']
        path = self.directory / (stage + '.json')
        if path.exists():
            # A restart rechecks the producers before accepting its immutable record.
            validate_stage(self.directory, stage, self.revision, self.image, self.generation)
            row = json.loads(evidence.protected(path))
            require(row['checks'] == checks, 'resumed stage changed checks')
            return row
        row = dict(schema_version=1, stage=stage, outcome='passed', revision=self.revision,
                   image=self.image, material_generation=self.generation, previous_record_sha256=previous,
                   checks=checks, started_at=started, finished_at=time.time_ns())
        evidence.retain(path, evidence.canonical(row))
        validate_stage(self.directory, stage, self.revision, self.image, self.generation)
        return row


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--inputs', required=True)
    parser.add_argument('--check-stage', choices=STAGES)
    args = parser.parse_args()
    try:
        config = json.loads(evidence.protected(args.inputs))
        node = merged.CHAIN.Node(config['node_url'], config['receipt_keys'], config.get('node_headers'))
        registry = registry_contract.registry_client(config['registry'])
        gateway = merged.Client(node, config['gateway_url'], config.get('gateway_headers'))
        discovery = registry_contract.NativeRegistryDiscovery(node, registry)
        bootstrap = NativeBootstrap(config['stage_directory'], config['revision'], config['image'], config['material_generation'],
                                    discovery, gateway, config['case'], config['material_files'])
        if args.check_stage:
            validate_stage(bootstrap.directory, args.check_stage, bootstrap.revision, bootstrap.image, bootstrap.generation)
            return 0
        require(config['gateway_url'].rstrip('/') == config['public_gateway_url'].rstrip('/'),
                'runtime proof must query the declared public hosted gateway')
        before = observe_public_dns()
        for stage in STAGES:
            document = dict(dns={'before': before, 'after': observe_public_dns()}, node_endpoints=public_endpoints(),
                            gateway_endpoint=config['public_gateway_url'], node_observations=observe_public_nodes(node.receipt_keys)) if stage == STAGES[-1] else None
            bootstrap.stage(stage, config['source'], config['publication_key'], document)
        print('PAXEER_X_GATE tests=4 skipped=0')
        return 0
    except (OSError, ValueError, KeyError, RuntimeError, Refused) as error:
        print('registry native bootstrap refused: ' + str(error), file=sys.stderr)
        return 1


if __name__ == '__main__':
    sys.exit(main())
