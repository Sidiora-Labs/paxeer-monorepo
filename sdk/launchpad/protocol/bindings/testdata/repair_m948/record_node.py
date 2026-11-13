"""Implementation-time capture; never invoked by the offline verification gate.

Builds paxd with Go, uses the committed single-node sequencer startup procedure
and genesis export, funds public fixture keys 1..3, and records the real bindings.
All disposable node state and keys stay outside the checkout.
"""
from pathlib import Path
import json
import os
import shlex
import subprocess
import tempfile
import tomllib
import urllib.request

here = Path(__file__).resolve().parent
root = here.parents[5]
with tempfile.TemporaryDirectory(prefix='repair-m948-node-') as scratch:
    scratch = Path(scratch)
    binary = scratch / 'paxd'
    subprocess.run(['go', 'build', '-o', str(binary), './daemon/paxd'], cwd=root, check=True)
    source = (root / 'tools/qualification/localnode/v6120-node.sh').read_text()
    source = source[:source.index('printf -v MINT_DATA')]
    source = source.replace('ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)', 'ROOT=' + shlex.quote(str(root)))
    source = source.replace('set_toml "$APP" evm ws_enabled false', 'set_toml "$APP" evm ws_enabled true\n    set_toml "$APP" evm ws_port 18546')
    extra = '''for address in 7E5F4552091A69125d5DfCb7b8C2659029395Bdf 2B5AD5c4795c026514f8317c7a215E218DcCD6cF 6813Eb9362372EEF6200f3b1dbC3f819671cBA69; do
    account=$("$PAXD" debug addr "$address" 2>/dev/null | sed -n 's/^Bech32 Acc: //p')
    "$PAXD" add-genesis-account "$account" 1000000000000000uhpx --home "$NODE_HOME" >> "$NODE_HOME/init.log" 2>&1
done
'''
    source = source.replace('# Sum arbitrary-sized bank amounts exactly', extra + '# Sum arbitrary-sized bank amounts exactly')
    env_file = scratch / 'node.env'
    source += 'printf "NODE_HOME=%s\\nEVM=%s\\n" "$NODE_HOME" "$EVM" > ' + shlex.quote(str(env_file)) + '\nSUCCESS=1\n'
    startup = scratch / 'start.sh'
    startup.write_text(source)
    env = dict(os.environ, PAXD=str(binary))
    subprocess.run(['bash', str(startup), '--sequencer'], cwd=root, env=env, check=True)
    node_env = dict(line.split('=', 1) for line in env_file.read_text().splitlines())
    node = Path(node_env['NODE_HOME'])
    evm = node_env['EVM']
    try:
        config = tomllib.loads((node / 'config/config.toml').read_text())
        deployer = json.loads((node / 'deployer.json').read_text())['evm_address']
        request = urllib.request.Request(evm, data=json.dumps({'jsonrpc':'2.0','id':1,'method':'eth_getTransactionCount','params':[deployer,'latest']}).encode(), headers={'Content-Type':'application/json'})
        with urllib.request.urlopen(request) as response:
            nonce = int(json.load(response)['result'], 16)
        payload = '40c10f19' + '7e5f4552091a69125d5dfcb7b8c2659029395bdf'.rjust(64, '0') + f'{10**12:064x}'
        subprocess.run([str(binary), 'tx', 'evm', 'call-contract', '0x85FcD13735F4309833A503EE804ea32395851479', payload, '--from', 'deployer', '--home', str(node), '--keyring-backend', 'test', '--chain-id', 'hyperpax_125-1', '--node', config['rpc']['laddr'], '--evm-rpc', evm, '--nonce', str(nonce), '--gas-fee-cap', '1000000000000', '--gas-limit', '7000000', '-y'], check=True)
        (here / 'producer-revision.txt').write_text(subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True))
        subprocess.run(['python3', str(here / 'record_native.py')], cwd=root, check=True)
        subprocess.run(['python3', str(here / 'record_retained.py'), 'ws://127.0.0.1:18546', 'NativeExecutorNode'], cwd=root, check=True)
        execution_path = here / 'native-execution.json'
        execution = json.loads(execution_path.read_text())
        execution['NodeRPC'] = json.loads((here / 'native-node-rpc.json').read_text())
        execution_path.write_text(json.dumps(execution, indent=2) + '\n')
    finally:
        subprocess.run(['bash', str(startup), '--stop', str(node)], env=env, check=True)
