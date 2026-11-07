#!/usr/bin/env python3
"""Wallet cap, custody and payment qualification at current merged boundaries."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import signal
import ssl
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(Path(__file__).parent))
import merged_qualification as merged
from layerx_sdk.web_search import decode_payer_grant

STATE = None
PROCESSES = []
CA = None
CASES = []
LEDGER = '0x' + '00' * 18 + '101a'
PAY = '0x' + '00' * 18 + '101b'
XWEB = '0x' + '00' * 18 + '1019'
ADDR = '0x' + '00' * 18 + '1004'
REQUIRED = {'confirmed-empty', 'signed-native-population', 'foreign-prefix-filtered',
    'anonymous-refused', 'cross-principal-refused', 'wrong-chain-refused', 'selector-refused',
    'expired-session-refused', 'private-core-token-required', 'typed-populated',
    'decoder-bounds-and-foreign-refusals', 'inflight-session-discard', 'account-switch-empty',
    'disconnect-refused', 'ui-loading', 'ui-verified-values', 'ui-mutation-refresh',
    'ui-session-invalidated', 'ui-reload-fresh-empty', 'mutation-refresh', 'grant-revocation-refresh',
    'unavailable-not-empty', 'restart-fresh-verified-state'}


def require(value, reason):
    if not value:
        raise RuntimeError(reason)


def private(path):
    path = Path(path)
    info = path.lstat()
    require(
        path.is_file() and not path.is_symlink() and info.st_mode & 0o077 == 0,
        "protected fixture file required",
    )
    require(
        path.name != ".env" and not path.name.startswith(".env."),
        "environment files are not fixture artifacts",
    )
    return path


def local(url):
    value = urllib.parse.urlparse(url)
    require(
        value.hostname in ("localhost", "127.0.0.1", "::1")
        and value.scheme in ("http", "https"),
        "isolated loopback service required",
    )
    return url


def command(argv, env=None):
    subprocess.run(
        argv,
        env=env,
        check=True,
        timeout=90,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
    )


def start(argv, env, name):
    with (STATE / (name + ".log")).open("ab", buffering=0) as log:
        process = subprocess.Popen(
            argv, env=env, cwd=ROOT, stdout=log, stderr=log, start_new_session=True
        )
    PROCESSES.append(process)
    return process


def stop(process):
    if process.poll() is None:
        os.killpg(process.pid, signal.SIGTERM)
        try:
            process.wait(timeout=8)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait(timeout=5)


def http(url, token=None, body=None):
    local(url)
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request(
        url, data=None if body is None else json.dumps(body).encode(), headers=headers
    )
    try:
        response = urllib.request.urlopen(request, context=CA, timeout=20)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        data = response.read(4 * 1024 * 1024 + 1)
        require(len(data) <= 4 * 1024 * 1024, "bounded HTTP response required")
        return response.status, json.loads(data)


def rpc(url, token, method, params):
    status, value = http(
        url, token, {"jsonrpc": "2.0", "id": 1, "method": method, "params": params}
    )
    answers = value if isinstance(value, list) else [value]
    for answer in answers:
        if (
            isinstance(answer, dict)
            and isinstance(answer.get("error"), dict)
            and answer["error"].get("code") == -32601
        ):
            raise RuntimeError("JSON-RPC -32601 method not found: " + method)
    require(
        status == 200 and value.get("jsonrpc") == "2.0" and value.get("id") == 1,
        "real unified RPC response required",
    )
    return value


def wait_ready(process, url):
    deadline = time.monotonic() + 90
    while time.monotonic() < deadline:
        require(process.poll() is None, "real service exited before readiness")
        try:
            if http(url)[0] == 200:
                return
        except (OSError, ValueError, urllib.error.URLError):
            pass
        time.sleep(0.25)
    raise RuntimeError("real service readiness deadline")


def expect_error(value, code):
    require(value.get('error', {}).get('code') == code and 'result' not in value, 'required refusal was not observed')


def exact(value, expected):
    for kind in ('budgets', 'grants'):
        actual = {row['id']: row for row in value[kind]}
        require(len(actual) == len(value[kind]) and set(actual) == set(expected[kind]), 'caps record omission or foreign disclosure')
        for identifier, fields in expected[kind].items():
            require(all(actual[identifier].get(key) == wanted for key, wanted in fields.items()),
                    'verified cap differs from signed fixture operation')


def grant_record(raw):
    return decode_payer_grant(raw)


class WalletCaps:
    def __init__(self, client, activation_height):
        require(type(activation_height) is int and activation_height >= 0, 'ledger activation height required')
        self.client, self.node, self.activation_height = client, client.node, activation_height

    def inventory(self, surface, address, event, height):
        abi = json.loads((ROOT / 'precompiles' / surface / 'abi.json').read_text())
        row = next(row for row in abi if row.get('type') == 'event' and row['name'] == event)
        signature = event + '(' + ','.join(merged.CHAIN.Node.abi_type(item) for item in row['inputs']) + ')'
        topic = '0x' + merged.evm.keccak256(signature.encode()).hex()
        require(height - self.activation_height <= 1_000_000, 'bounded complete cap discovery required')
        identifiers, seen = [], set()
        for start in range(self.activation_height, height + 1, 10_000):
            stop = min(height, start + 9999)
            logs = self.node.call('eth_getLogs', [{'address': address, 'topics': [topic], 'fromBlock': hex(start), 'toBlock': hex(stop)}])
            require(isinstance(logs, list) and len(logs) <= 10000, 'bounded cap log discovery required')
            for log in logs:
                require(log.get('address', '').lower() == address and log.get('removed') is False
                        and len(log.get('topics', [])) == 2 and log['topics'][0] == topic
                        and re.fullmatch('0x[0-9a-f]{64}', log['topics'][1]), 'invalid cap issuance event')
                block_number = int(log['blockNumber'], 16)
                require(start <= block_number <= stop, 'cap log escaped discovery range')
                block = self.node.call('eth_getBlockByNumber', [log['blockNumber'], False])
                require(block['hash'] == log['blockHash'] and log['transactionHash'] in block['transactions'],
                        'cap issuance log lacks committed block')
                key = (log['transactionHash'], log['logIndex'])
                require(key not in seen, 'duplicate cap issuance log')
                seen.add(key)
                merged.decode_abi([merged.CHAIN.Node.abi_type(item) for item in row['inputs'] if not item['indexed']], log['data'])
                identifiers.append(log['topics'][1])
        require(len(identifiers) == len(set(identifiers)), 'duplicate cap identifier')
        return identifiers

    def snapshot(self, account):
        require(account['chain_id'] == 125 and self.node.call('eth_chainId', []) == '0x7d', 'caps connection substitution')
        block = self.node.call('eth_getBlockByNumber', ['finalized', False])
        require(isinstance(block, dict) and re.fullmatch('0x[0-9a-f]+', block.get('number', ''))
                and re.fullmatch('0x[0-9a-fA-F]{64}', block.get('stateRoot', ''))
                and re.fullmatch('0x[0-9a-fA-F]{64}', block.get('hash', '')) and int(block['hash'], 16) != 0,
                'committed caps block required; unavailable is never empty')
        height = int(block['number'], 16)
        view = lambda surface, address, method, arguments: self.client.view(surface, address, method, arguments, block['number'])
        address = bytes.fromhex(account['address'].removeprefix('0x'))
        unified = view('addr', ADDR, 'getUnifiedAccount', [address])
        require(unified[0] == address and unified[2] != bytes(32) and unified[3].hex() == account['account_id']
                and account['did'] == 'did:layerx:' + unified[2].hex(), 'caps identity substitution')
        budgets, grants = [], []
        for identifier in self.inventory('layerxpay', PAY, 'BudgetCreated', height):
            values = view('layerxpay', PAY, 'getBudget', [bytes.fromhex(identifier[2:])])[0]
            if values[0].hex() != account['account_id']:
                continue
            remaining = view('layerxpay', PAY, 'budgetRemaining', [bytes.fromhex(identifier[2:])])[0]
            budgets.append({'id': identifier[2:], 'owner': values[0].hex(), 'account': values[1].hex(),
                'asset': values[2].hex(), 'limit': str(values[4]), 'configured_limit': str(values[5]),
                'carry_cap': str(values[6]), 'spent': str(values[7]), 'carried': str(values[8]),
                'period_length': str(values[9]), 'period_start': str(values[10]), 'expiry': str(values[11]),
                'revocation_height': str(values[12]), 'closed': values[14], 'revoked': values[15],
                'source_account': None if values[17] == bytes(32) else values[17].hex(), 'remaining': str(remaining)})
        for identifier in self.inventory('layerxledger', LEDGER, 'PayerGrantIssued', height):
            values = view('layerxledger', LEDGER, 'getPayerGrant', [bytes.fromhex(identifier[2:])])
            grant = grant_record(values[0])
            require(grant['grant_id'] == identifier[2:], 'payer grant id substitution')
            if grant['from'] != account['account_id']:
                continue
            grants.append({'id': grant['grant_id'], 'owner': grant['from'], 'recipient': grant['recipient'],
                'asset': grant['asset'], 'per_draw_maximum': grant['per_draw_maximum'], 'allowance': grant['allowance'],
                'recurring': grant['recurring'], 'window_length': grant['window_length'], 'expiration': grant['expiration'],
                'revocation_height': grant['revocation_sequence'], 'drawn_total': str(values[1]),
                'drawn_this_period': str(values[2]), 'window_start': str(values[3]), 'revoked_at_height': str(values[4]),
                'revoked': values[5], 'invoice_settled': values[6]})
        fee = view('xweb', XWEB, 'fee', [])[0]
        require(self.node.call('eth_getBlockByNumber', [block['number'], False]) == block, 'caps committed block changed during traversal')
        require(all(row['owner'] == account['account_id'] for row in budgets + grants), 'foreign prefix data disclosed')
        return {'state': 'ready' if budgets or grants else 'empty', 'did': account['did'], 'account_id': account['account_id'],
            'context': {'address': account['address'], 'chain_id': 125}, 'fee': str(fee),
            'observation': {'rung': 'committed', 'block_number': str(height), 'state_root': block['stateRoot'][2:]},
            'budgets': budgets, 'grants': grants}

    def mutate(self, raw, expected_sender, target, data, refused=False):
        transaction, signature = merged.evm.decode_signed(bytes.fromhex(raw.removeprefix('0x')))
        require(transaction.chain_id == 125 and transaction.to.hex() == target.removeprefix('0x').lower()
                and transaction.data.hex() == data.removeprefix('0x').lower()
                and transaction.recover_signer(signature).lower() == expected_sender.lower(),
                'custody-signed mutation action or payer substitution')
        reference = '0x' + merged.evm.keccak256(bytes.fromhex(raw.removeprefix('0x'))).hex()
        require(self.node.call('eth_sendRawTransaction', [raw]) == reference, 'signed mutation hash substituted')
        receipt = self.node.committed(reference, expected_sender, refused=refused)
        return receipt

    def custody_refusal(self, url, token, request, status, error):
        merged.CHAIN.endpoint(url)
        code, result = http(url.rstrip('/') + '/v1/wallet/sign', token, request)
        require(code == status and result.get('error') == error and 'signedTransaction' not in result
                and 'signature' not in result and 'signed_tx' not in result, 'custody refusal changed or signature leaked')
        return result


def snapshot(url, account):
    require(CAPS is not None and CAPS.node.url == url, 'provisioned current caps client required')
    return CAPS.snapshot(account)


CAPS = None


def qualify(config, directory):
    global STATE, CA, CAPS
    STATE = directory
    revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    require(config['version'] == 2 and config['isolated'] is True and config['source_revision'] == revision,
            'source-bound merged wallet fixture v2 required')
    require(not subprocess.check_output(['git', 'status', '--porcelain', '--untracked-files=no'], cwd=ROOT)
            and config['source_tree'] == subprocess.check_output(['git', 'rev-parse', 'HEAD^{tree}'], cwd=ROOT, text=True).strip(),
            'source changed since bounded wallet build')
    merged.source_artifacts(config, revision)
    require(config['paxd']['source_revision'] == revision, 'wallet merged node seed revision mismatch')
    node = merged.OWNED.PaxdFixture(config['paxd'], config['artifacts']['paxd']['path'], directory)
    CA = ssl.create_default_context(cafile=str(private(config['ca_file'])))
    CA.load_cert_chain(str(private(config['client_certificate'])), str(private(config['client_key'])))
    require(merged.CHAIN.endpoint(config['gateway_url']).scheme == 'https', 'real wallet gateway TLS listener required')
    client = merged.Client(node.node, config['gateway_url'], context=CA)
    CAPS = WalletCaps(client, config['activation_height'])
    accounts = config['accounts']
    require(set(accounts) == {'owner', 'empty', 'foreign'} and len({row['did'] for row in accounts.values()}) == 3,
            'three distinct bound wallet principals required')
    cases, running = [], []
    try:
        node.start()
        services = config['services']
        require(sum(row['kind'] == 'attestor' for row in services) == 5
                and sum(row['kind'] == 'wallet_gateway' for row in services) == 1
                and sum(row['kind'] == 'gateway' for row in services) == 1,
                'five genuine attestors, wallet gateway and hosted gateway required')
        require(all(row['kind'] not in {'layerxd', 'sequencer', 'clock', 'chain'} for row in services), 'retired wallet process refused')
        for row in services:
            environment = dict(row['environment'])
            if row['kind'] == 'wallet_gateway':
                require(environment['ATTESTOR_QUORUM'] == '3', 'production custody threshold must remain three')
            for key, value in environment.items():
                if key.endswith('_URL') and urllib.parse.urlsplit(value).scheme in ('http', 'https'):
                    local(value)
            binary = config['artifacts'][row['artifact']]['path']
            process = start([binary, *row['arguments']], environment, row['name'])
            running.append(process)
            wait_ready(process, row['ready_url'])
        capabilities = client.capabilities()
        require(all(capabilities[name] for name in ('ledger', 'pay', 'auth')), 'required wallet precompiles unavailable')
        require(CAPS.snapshot(accounts['empty'])['state'] == 'empty', 'confirmed-empty fixture must prove no caps')
        cases.append('confirmed-empty')
        for name in ('owner-budget', 'owner-grant', 'foreign-budget', 'foreign-grant'):
            row = config['mutations'][name]
            CAPS.mutate(row['raw_transaction'], accounts[row['account']]['address'], row['target'], row['data'])
            exact(CAPS.snapshot(accounts[row['account']]), row['expected'])
        populated = CAPS.snapshot(accounts['owner'])
        require(populated['budgets'] and populated['grants'], 'real populated budgets and grants required')
        cases.extend(['signed-native-population', 'foreign-prefix-filtered'])
        refused = config['custody_refusals']
        required_refusals = {'anonymous-refused', 'cross-principal-refused', 'wrong-chain-refused',
                            'selector-refused', 'expired-session-refused', 'private-core-token-required'}
        require(len(refused) == len(required_refusals) and {row['id'] for row in refused} == required_refusals,
                'all retained custody refusal cases required')
        for row in refused:
            before = CAPS.snapshot(accounts['owner'])
            CAPS.custody_refusal(config['wallet_gateway_url'], row['token'], row['request'], row['status'], row['error'])
            require(CAPS.snapshot(accounts['owner']) == before, 'custody refusal changed caps or payment state')
            cases.append(row['id'])
        for name, marker in [('owner-budget-change', 'mutation-refresh'), ('owner-grant-revoke', 'grant-revocation-refresh')]:
            row = config['mutations'][name]
            CAPS.mutate(row['raw_transaction'], accounts['owner']['address'], row['target'], row['data'])
            fresh = CAPS.snapshot(accounts['owner'])
            exact(fresh, row['expected'])
            require(int(fresh['observation']['block_number']) > int(populated['observation']['block_number'])
                    and fresh['observation']['state_root'] != populated['observation']['state_root'],
                    'mutation must advance committed observation and state root')
            if marker == 'grant-revocation-refresh':
                require(any(grant['revoked'] for grant in fresh['grants']), 'real grant revocation required')
            cases.append(marker)
        payment = config['payment']
        before = CAPS.snapshot(accounts['owner'])
        require(before['fee'] == payment['expected_fee'], 'fee differs from current precompile quote')
        CAPS.mutate(payment['raw_transaction'], accounts['owner']['address'], payment['target'], payment['data'])
        exact(CAPS.snapshot(accounts['owner']), payment['expected'])
        # Prebuilt consumers retain all original SDK/session/UI assertions. They run
        # the current SDK and UI against these same owned node/gateway listeners.
        groups = {'sdk': {'typed-populated', 'decoder-bounds-and-foreign-refusals', 'inflight-session-discard',
                           'account-switch-empty', 'disconnect-refused'},
                  'browser': {'ui-loading', 'ui-verified-values', 'ui-mutation-refresh', 'ui-session-invalidated', 'ui-reload-fresh-empty'}}
        require(set(config['consumers']) == set(groups), 'both real wallet consumers required')
        for name, expected in groups.items():
            row = config['consumers'][name]
            require(row['source_revision'] == revision and row['command'][0] == config['artifacts'][row['artifact']]['path'],
                    'source-bound prebuilt consumer command required')
            result = subprocess.run(row['command'], cwd=ROOT, env=dict(row['environment']), capture_output=True, timeout=180)
            (directory / (name + '.log')).write_bytes(result.stdout + result.stderr)
            executed = json.loads(result.stdout.decode().splitlines()[-1])
            require(result.returncode == 0 and len(executed) == len(expected) and set(executed) == expected,
                    'real consumer omitted retained assertions')
            cases.extend(executed)
        node.stop()
        try:
            CAPS.snapshot(accounts['owner'])
        except (OSError, ValueError):
            pass
        else:
            require(False, 'unavailable caps masqueraded as empty')
        cases.append('unavailable-not-empty')
        node.start()
        recovered = CAPS.snapshot(accounts['owner'])
        exact(recovered, payment['expected'])
        require(int(recovered['observation']['block_number']) >= int(fresh['observation']['block_number']), 'restart regressed evidence')
        cases.append('restart-fresh-verified-state')
        require(len(cases) == len(REQUIRED) and set(cases) == REQUIRED and all(p.poll() is None for p in running),
                'every retained focused case must execute on owned services')
        merged.source_artifacts(config, revision)
        require(subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip() == revision
                and not subprocess.check_output(['git', 'status', '--porcelain', '--untracked-files=no'], cwd=ROOT),
                'source changed during wallet qualification')
        return cases
    finally:
        for process in reversed(running):
            stop(process)
        node.stop()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--manifest', default=os.environ.get('WALLET_CAPS_FIXTURE_BUNDLE'))
    args = parser.parse_args()
    require(args.manifest, 'protected merged wallet fixture required')
    config = merged.OWNED.private_json(args.manifest)
    parent = private(config['evidence_anchor']).parent
    directory = Path(tempfile.mkdtemp(prefix='wallet-402-caps-', dir=parent))
    directory.chmod(0o700)
    record = {'revision': config['source_revision'], 'exit_code': 1, 'cases': [], 'skipped': 0}
    try:
        record['cases'] = qualify(config, directory)
        record['exit_code'] = 0
    finally:
        (directory / 'result.json').write_text(json.dumps(record, indent=2))
        print(json.dumps({'exit_code': record['exit_code'], 'log_path': str(directory)}))
    return record['exit_code']


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (OSError, ValueError, RuntimeError, KeyError, TypeError, subprocess.SubprocessError) as error:
        print('wallet-402-caps: refused: ' + str(error), file=sys.stderr)
        sys.exit(1)
