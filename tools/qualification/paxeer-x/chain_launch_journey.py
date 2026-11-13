#!/usr/bin/env python3
"""Run the chain launch market and revenue journey on a disposable node."""

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'services/agent/sdk/python'))
sys.path.insert(0, str(Path(__file__).resolve().parent / 'fixtures'))

from layerx_sdk import evm_transaction as transaction
from layerx_sdk.account_derivation import keccak256
import merged_runtime_fixture as fixture
import xweb_native_evm as signing

LAUNCHPAD = '0x' + '00' * 18 + '1017'
CHAINS = '0x' + '00' * 18 + '101e'
TASK = '22.40.17'
SOURCE_FILES = ('tools/qualification/paxeer-x/chain_launch_journey.py',
                'tools/qualification/paxeer-x/fixtures/merged_runtime_fixture.py',
                'tools/qualification/paxeer-x/program_market_attestation.py',
                'precompiles/launchpad/abi.json', 'modules/launchpad/keeper/market.go',
                'go.mod', 'Makefile')


def require(condition, reason):
    if not condition:
        raise RuntimeError('chain launch journey: ' + reason)


def load(name, filename):
    spec = importlib.util.spec_from_file_location(name, filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


attestation = load('chain_launch_evidence', ROOT / 'tools/qualification/paxeer-x/program_market_attestation.py')
merged = load('chain_launch_abi', ROOT / 'tools/qualification/paxeer-x/merged_qualification.py')


def digest(path):
    with Path(path).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def source_identity():
    return {name: digest(ROOT / name) for name in SOURCE_FILES}


def build_base():
    builder = os.environ.get('PAXEER_X_BUILDER_ID', '')
    require(re.fullmatch(r'[A-Za-z0-9_.-]{1,128}', builder), 'builder identity')
    base = Path('/var/tmp') / ('paxeer-x-chain-launch-' + hashlib.sha256(builder.encode()).hexdigest()[:20])
    base.mkdir(mode=0o700, exist_ok=True)
    require(base.stat().st_uid == os.geteuid() and not base.stat().st_mode & 0o077,
            'private build directory')
    return base


def begin_build():
    base = build_base()
    directory = base / secrets.token_hex(16)
    directory.mkdir(mode=0o700)
    fixture.write_json(directory / 'start.json', {
        'task': TASK, 'source_revision': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT,
                                                                 text=True).strip(),
        'source': source_identity(), 'builder': os.environ['PAXEER_X_BUILDER_ID']})
    active = base / 'active.json.tmp'
    active.write_text(json.dumps({'directory': str(directory)}) + '\n')
    active.chmod(0o600)
    active.replace(base / 'active.json')
    print('Build directory: ' + str(directory), flush=True)


def active_build():
    base = build_base()
    active = json.loads((base / 'active.json').read_text())
    directory = Path(active['directory'])
    require(directory.parent == base and re.fullmatch(r'[0-9a-f]{32}', directory.name),
            'active build directory')
    start = json.loads((directory / 'start.json').read_text())
    require(start['task'] == TASK and start['builder'] == os.environ['PAXEER_X_BUILDER_ID']
            and start['source'] == source_identity(), 'build source identity')
    return directory, start


def build():
    directory, start = active_build()
    with (directory / 'build.log').open('xb') as log:
        dependencies = subprocess.run(['go', 'mod', 'download', 'all'], cwd=ROOT,
                                      stdout=log, stderr=subprocess.STDOUT, timeout=900)
        require(dependencies.returncode == 0, 'Go module dependencies; inspect build.log')
        finished = subprocess.run(['make', '-C', str(ROOT), 'paxd-build'], stdout=log,
                                  stderr=subprocess.STDOUT, timeout=3600)
    require(finished.returncode == 0, 'paxd build; inspect build.log')
    product = ROOT / 'build/paxd'
    require(product.is_file() and os.access(product, os.X_OK), 'paxd executable')
    binary = directory / 'paxd'
    shutil.copyfile(product, binary)
    binary.chmod(0o700)
    require(start['source'] == source_identity(), 'build input changed')
    fixture.write_json(directory / 'artifact.json', {
        'task': TASK, 'source_revision': start['source_revision'], 'source': start['source'],
        'builder': start['builder'], 'paxd': {'path': str(binary), 'sha256': digest(binary)}})
    print('Artifact: ' + str(directory / 'artifact.json'), flush=True)


class ABI:
    def __init__(self):
        self.methods = {}
        self.events = {}
        for surface in ('launchpad', 'layerxchains'):
            rows = json.loads((ROOT / 'precompiles' / surface / 'abi.json').read_text())
            for row in rows:
                if row['type'] == 'function':
                    self.methods[surface, row['name']] = row
                elif row['type'] == 'event':
                    signature = row['name'] + '(' + ','.join(item['type'] for item in row['inputs']) + ')'
                    self.events[surface, row['name']] = (row, '0x' + keccak256(signature.encode()).hex())

    def call_data(self, surface, name, *values):
        row = self.methods[surface, name]
        args = [item['type'] for item in row['inputs']]
        converted = [bytes.fromhex(value[2:]) if kind == 'address' and isinstance(value, str) else value
                     for kind, value in zip(args, values, strict=True)]
        return '0x' + transaction.encode_call(name, args, converted).hex()

    def read(self, rpc, surface, address, name, *values):
        row = self.methods[surface, name]
        result = rpc.request('eth_call', [{'to': address, 'data': self.call_data(surface, name, *values)}, 'latest'])
        decoded = merged.decode_abi([item['type'] for item in row['outputs']], result)
        return {item['name'] or str(index): ('0x' + value.hex() if isinstance(value, bytes) else value)
                for index, (item, value) in enumerate(zip(row['outputs'], decoded, strict=True))}

    def event(self, receipt, surface, name, address=LAUNCHPAD):
        row, topic = self.events[surface, name]
        matches = [entry for entry in receipt.get('logs', [])
                   if entry.get('address', '').lower() == address.lower()
                   and entry.get('topics', [None])[0] == topic]
        require(len(matches) == 1, f'{name} event count')
        log = matches[0]
        indexed = [item for item in row['inputs'] if item['indexed']]
        plain = [item for item in row['inputs'] if not item['indexed']]
        require(len(log['topics']) == len(indexed) + 1, f'{name} indexed fields')
        fields = {}
        for item, word in zip(indexed, log['topics'][1:], strict=True):
            raw = bytes.fromhex(word[2:])
            fields[item['name']] = '0x' + raw[-20:].hex() if item['type'] == 'address' else int.from_bytes(raw, 'big')
        decoded = merged.decode_abi([item['type'] for item in plain], log['data'])
        for item, value in zip(plain, decoded, strict=True):
            fields[item['name']] = '0x' + value.hex() if isinstance(value, bytes) else value
        return fields


class Journey:
    def __init__(self, runtime, first, second, directory):
        self.runtime, self.first, self.second = runtime, first, second
        self.rpc = runtime.rpc
        self.abi = ABI()
        self.directory = directory
        self.result = {'task': TASK, 'transactions': {}, 'reads': {}, 'events': {}}

    def send(self, label, rpc, secret, chain_id, to, data='0x', value=0, gas=1_000_000,
             accepted=True):
        sender = signing.checksum(signing.address_of(secret))
        nonce = int(rpc.request('eth_getTransactionCount', [sender, 'pending']), 16)
        price = int(rpc.request('eth_gasPrice', []), 16)
        encoded = transaction.Type2Transaction(chain_id, nonce, 0, max(price * 2, 10**12), gas,
                                               bytes.fromhex(to[2:]), value, bytes.fromhex(data[2:]))
        raw = signing.sign_transaction(secret, encoded)
        reference = rpc.request('eth_sendRawTransaction', ['0x' + raw.hex()])
        require(reference == '0x' + keccak256(raw).hex(), f'{label} transaction hash')
        observed = None

        def mined():
            nonlocal observed
            observed = rpc.request('eth_getTransactionReceipt', [reference])
            return observed is not None

        self.runtime.wait(mined, 180)
        require(observed['transactionHash'] == reference and observed['status'] == ('0x1' if accepted else '0x0'),
                f'{label} receipt status')
        self.result['transactions'][label] = reference
        return observed

    def deploy_quote(self):
        source = (ROOT / 'modules/launchpad/keeper/market.go').read_text()
        match = re.search(r'var sidioraERC20Creation = common\.FromHex\("(0x[0-9a-f]+)"\)', source)
        require(match is not None, 'USDL creation code')
        owner = signing.checksum(signing.address_of(self.first))
        nonce = int(self.rpc.request('eth_getTransactionCount', [owner, 'pending']), 16)
        creator = load('chain_launch_creator', ROOT / 'services/platform/hosted/paxeer/evm.py')
        expected = creator.checksum(creator.keccak(creator.rlp([creator.unhex(owner, 20), nonce]))[12:])
        code = bytes.fromhex(match.group(1)[2:])
        constructor = transaction.encode_call('constructor', ['string', 'string', 'uint256', 'address'],
                                              ['USD Ledger', 'USDL', 1_000_000_000_000_000,
                                               bytes.fromhex(owner[2:])])[4:]
        price = int(self.rpc.request('eth_gasPrice', []), 16)
        raw = creator.sign_transaction(self.first, 125, nonce, max(price * 2, 10**12), 2_000_000,
                                       b'', 0, code + constructor)
        reference = self.rpc.request('eth_sendRawTransaction', ['0x' + raw.hex()])
        require(reference == '0x' + keccak256(raw).hex(), 'USDL deployment hash')
        receipt = None

        def mined():
            nonlocal receipt
            receipt = self.rpc.request('eth_getTransactionReceipt', [reference])
            return receipt is not None

        self.runtime.wait(mined, 180)
        require(receipt['status'] == '0x1' and receipt['contractAddress'].lower() == expected.lower(),
                'USDL deployment receipt')
        require(self.rpc.request('eth_getCode', [expected, 'latest']) != '0x', 'USDL deployed code')
        self.result['transactions']['quote_deploy'] = reference
        self.result['reads']['quote_token'] = expected
        return expected

    def run(self, runtime, quote):
        self.runtime = runtime
        owner = signing.checksum(signing.address_of(self.first))
        holder = signing.checksum(signing.address_of(self.second))
        require(self.rpc.request('eth_chainId', []) == '0x7d', 'root chain identity')
        require(quote.lower() == self.abi.read(self.rpc, 'launchpad', LAUNCHPAD, 'usdlAddress')['0'].lower(),
                'launchpad quote token')
        approval = self.send('quote_approve', self.rpc, self.first, 125, quote,
                             signing.calldata('approve(address,uint256)', LAUNCHPAD, 2**256 - 1))
        require(approval['status'] == '0x1', 'USDL approval')
        launch_value = 110_000_000 * 10**12
        launch = self.send('launch', self.rpc, self.first, 125, LAUNCHPAD,
                           self.abi.call_data('launchpad', 'launchChain', 'Chain Token', 'CHN', 0,
                                              '0x' + '00' * 20, 'chain-launch-journey', 2_000_000, 65_536, 5_000),
                           value=launch_value, gas=6_000_000)
        event = self.abi.event(launch, 'launchpad', 'ChainLaunched')
        self.abi.event(launch, 'launchpad', 'MarketCreated')
        chain = event['chain']
        require(chain > 0 and event['creator'].lower() == owner.lower(), 'launched chain event')
        binding = self.abi.read(self.rpc, 'launchpad', LAUNCHPAD, 'launchedChain', chain)
        domain = self.abi.read(self.rpc, 'layerxchains', CHAINS, 'getChain', chain)
        home = self.abi.read(self.rpc, 'launchpad', LAUNCHPAD, 'homeChainOf', binding['pool'])
        require(binding['registryNumber'] == chain and binding['chainId'] == event['chainId']
                and binding['pool'].lower() == event['pool'].lower()
                and binding['token'].lower() == event['token'].lower()
                and binding['nftId'] == event['nftId'] and binding['creator'].lower() == owner.lower(),
                'launchedChain binding')
        require(domain['id'] == chain and domain['chainId'] == event['chainId']
                and domain['status'] == 1 and domain['bond'] > 0 and home['chainId'] == event['chainId'],
                'registry and home chain')
        self.result['events']['launch'] = event
        self.result['reads'].update(launched_chain=binding, registry_chain=domain, home_chain=home)
        scoped = self.rpc.for_chain(chain)
        require(scoped.request('eth_chainId', []) == hex(event['chainId']), 'scoped chain identity')
        block = self.rpc.request('eth_getBlockByNumber', ['latest', False])
        deadline = int(block['timestamp'], 16) + 7_200
        buy_data = self.abi.call_data('launchpad', 'buy', binding['pool'], 1_000_000_000, 1, deadline)
        buy = self.send('buy', scoped, self.first, event['chainId'], LAUNCHPAD, buy_data)
        self.abi.event(buy, 'launchpad', 'Swap')
        token_balance = int(self.rpc.request('eth_call', [{'to': binding['token'],
            'data': signing.calldata('balanceOf(address)', owner)}, 'latest']), 16)
        require(token_balance > 1, 'buyer token balance')
        wrong = self.send('wrong_chain_buy', self.rpc, self.first, 125, LAUNCHPAD, buy_data, accepted=False)
        require(not wrong.get('logs'), 'wrong-chain buy emitted no logs')
        refused = self.rpc.exchange('eth_call', [{'to': LAUNCHPAD, 'from': owner, 'data': buy_data}, 'latest'])
        selector = '0x' + keccak256(b'WrongChain()')[:4].hex()
        require('error' in refused and selector.lower() in json.dumps(refused['error']).lower(),
                'wrong-chain refusal selector')
        self.result['reads']['wrong_chain_refusal'] = refused['error']
        transfer = self.send('holder_transfer', scoped, self.first, event['chainId'], binding['token'],
                             signing.calldata('transfer(address,uint256)', holder, token_balance // 2))
        transfer_topic = '0x' + keccak256(b'Transfer(address,address,uint256)').hex()
        token_logs = [entry for entry in transfer['logs']
                      if entry['address'].lower() == binding['token'].lower()
                      and entry['topics'][0] == transfer_topic]
        require(len(token_logs) == 1 and int(token_logs[0]['data'], 16) == token_balance // 2,
                'holder transfer log')
        second_balance = int(self.rpc.request('eth_call', [{'to': binding['token'],
            'data': signing.calldata('balanceOf(address)', holder)}, 'latest']), 16)
        require(second_balance == token_balance // 2, 'second holder token balance')
        operator = domain['operator']
        starting_revenue = int(self.rpc.request('eth_getBalance', [operator, 'latest']), 16)
        fee_blocks = set()
        for index in range(6):
            fee_receipt = self.send(f'chain_fee_{index + 1}', scoped, self.first, event['chainId'], holder,
                                    value=1, gas=21_000)
            require(int(fee_receipt['gasUsed'], 16) > 0 and not fee_receipt['logs'],
                    'chain fee transfer receipt and logs')
            fee_blocks.add(fee_receipt['blockNumber'])
        revenue = int(self.rpc.request('eth_getBalance', [operator, 'latest']), 16)
        require(len(fee_blocks) >= 3 and revenue > starting_revenue, 'operator base-fee share across blocks')
        self.result['reads']['operator_revenue'] = {'before': starting_revenue, 'after': revenue,
                                                    'blocks': sorted(fee_blocks)}
        launched_at = int(self.rpc.request('eth_getBlockByNumber', [launch['blockNumber'], False])['timestamp'], 16)
        while True:
            now = int(self.rpc.request('eth_getBlockByNumber', ['latest', False])['timestamp'], 16)
            if now >= launched_at + 3_600:
                break
            require(runtime.process.poll() is None, 'node remains live during epoch interval')
            time.sleep(min(30, launched_at + 3_600 - now))
        opened = self.send('open_epoch', self.rpc, self.first, 125, LAUNCHPAD,
                           self.abi.call_data('launchpad', 'openChainRevenueEpoch', chain))
        opened_event = self.abi.event(opened, 'launchpad', 'ChainRevenueEpochOpened')
        epoch = opened_event['epoch']
        row = self.abi.read(self.rpc, 'launchpad', LAUNCHPAD, 'chainRevenueEpoch', chain, epoch)
        require(epoch == 1 and row['epochAmount'] > 0 and row['epochAmount'] == opened_event['epochAmount']
                and row['claimedTotal'] == 0 and row['snapshotBlock'] == opened_event['snapshotBlock'],
                'opened revenue epoch')
        self.result['events']['opened_epoch'] = opened_event
        self.result['reads']['epoch_before_claims'] = row
        amounts = []
        for label, secret, address in (('first_claim', self.first, owner),
                                       ('second_claim', self.second, holder)):
            due = self.abi.read(self.rpc, 'launchpad', LAUNCHPAD,
                                'chainRevenueClaimable', chain, epoch, address)['amount']
            before = int(self.rpc.request('eth_getBalance', [address, 'latest']), 16)
            receipt = self.send(label, self.rpc, secret, 125, LAUNCHPAD,
                                self.abi.call_data('launchpad', 'claimChainRevenue', chain, epoch))
            claim = self.abi.event(receipt, 'launchpad', 'ChainRevenueClaimed')
            after = int(self.rpc.request('eth_getBalance', [address, 'latest']), 16)
            require(claim['holder'].lower() == address.lower() and claim['amount'] == due and due > 0,
                    f'{label} amount')
            require(after + int(receipt['gasUsed'], 16) * int(receipt['effectiveGasPrice'], 16) - before == due,
                    f'{label} payout')
            amounts.append(due)
            self.result['events'][label] = claim
        claimed = self.abi.read(self.rpc, 'launchpad', LAUNCHPAD, 'chainRevenueEpoch', chain, epoch)
        require(sum(amounts) == row['epochAmount'] and claimed['claimedTotal'] == row['epochAmount'],
                'two claims settle the epoch')
        self.result['reads']['epoch_after_claims'] = claimed
        for label, method, status in (('pause', 'pauseLaunchedChain', 2),
                                      ('resume', 'resumeLaunchedChain', 1)):
            receipt = self.send(label, self.rpc, self.first, 125, LAUNCHPAD,
                                self.abi.call_data('launchpad', method, chain))
            require(not receipt['logs'], f'{label} log count')
            observed = self.abi.read(self.rpc, 'layerxchains', CHAINS, 'getChain', chain)
            require(observed['status'] == status, f'{label} chain status')
            self.result['reads'][label] = observed
        before = int(self.rpc.request('eth_getBalance', [owner, 'latest']), 16)
        retired = self.send('retire', self.rpc, self.first, 125, LAUNCHPAD,
                            self.abi.call_data('launchpad', 'retireLaunchedChain', chain))
        retired_event = self.abi.event(retired, 'launchpad', 'LaunchedChainRetired')
        after = int(self.rpc.request('eth_getBalance', [owner, 'latest']), 16)
        final = self.abi.read(self.rpc, 'layerxchains', CHAINS, 'getChain', chain)
        require(final['status'] == 4 and final['bond'] == 0
                and retired_event['chain'] == chain and retired_event['holder'].lower() == owner.lower()
                and retired_event['bond'] == domain['bond'], 'retired chain bond event')
        require(after + int(retired['gasUsed'], 16) * int(retired['effectiveGasPrice'], 16) - before == domain['bond'],
                'fee-right holder bond payment')
        self.result['events']['retirement'] = retired_event
        self.result['reads']['retired_chain'] = final
        self.result['reads']['claims_total'] = sum(amounts)


def qualify():
    evidence = attestation.private_evidence_directory('chain-launch')
    result_path = evidence / 'result.json'
    log = evidence / 'native.log'
    log.write_text('chain launch journey started\n')
    runtime = None
    journey = None
    try:
        artifact_dir, start = active_build()
        artifact = json.loads((artifact_dir / 'artifact.json').read_text())
        require(artifact['task'] == TASK and artifact['source'] == source_identity()
                and artifact['builder'] == os.environ['PAXEER_X_BUILDER_ID'], 'qualified artifact source')
        binary = Path(artifact['paxd']['path'])
        require(binary.is_file() and digest(binary) == artifact['paxd']['sha256'], 'qualified node executable')
        first, second = (secrets.randbelow(signing.N - 1) + 1 for _ in range(2))
        first_address = signing.checksum(signing.address_of(first))
        second_address = signing.checksum(signing.address_of(second))
        quote = signing.checksum(keccak256(bytes([0xd6, 0x94]) + bytes.fromhex(first_address[2:]) + b'\x80')[12:])
        runtime = fixture.prepare_launch_runtime(evidence / 'fixture', binary, start['source_revision'],
                                                 (first_address, second_address), first, quote)
        journey = Journey(runtime, first, second, evidence)
        runtime.generate()
        runtime.start(sequencer=False)
        runtime.committed_readiness()
        runtime.cleanup()
        runtime.start()
        runtime.readiness()
        deployed = journey.deploy_quote()
        require(deployed.lower() == quote.lower(), 'configured USDL deployment')
        journey.run(runtime, quote)
        journey.result['status'] = 'passed'
        fixture.write_json(result_path, journey.result)
        with log.open('a') as output:
            output.write('chain launch journey passed\n')
        print('PAXEER_X_GATE tests=1 skipped=0', flush=True)
        print('Evidence: ' + str(result_path), flush=True)
        return 0
    except Exception as error:
        result = journey.result if journey is not None else {'task': TASK, 'transactions': {}, 'reads': {}, 'events': {}}
        result['status'] = 'failed'
        result['failure'] = str(error)
        fixture.write_json(result_path, result)
        with log.open('a') as output:
            output.write(str(error) + '\n')
        raise
    finally:
        if runtime is not None:
            runtime.cleanup()


def main():
    parser = argparse.ArgumentParser(allow_abbrev=False)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument('--begin-build', action='store_true')
    mode.add_argument('--build', action='store_true')
    args = parser.parse_args()
    os.umask(0o077)
    if args.begin_build:
        begin_build()
        return 0
    if args.build:
        build()
        return 0
    return qualify()


if __name__ == '__main__':
    try:
        sys.exit(main())
    except Exception as failure:
        print(str(failure), file=sys.stderr)
        sys.exit(1)
