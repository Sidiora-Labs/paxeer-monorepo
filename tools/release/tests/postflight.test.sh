#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
work=$(mktemp -d)
server_pid=
cleanup() {
  if [[ -n "$server_pid" ]]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf -- "$work"
}
trap cleanup EXIT

# Response files describe the Cosmos, CometBFT and EVM wire interfaces.
python3 - "$work" <<'PY'
import base64
import copy
import json
from pathlib import Path
import sys

work = Path(sys.argv[1])

def word(*values):
    return ''.join(f'{value:064x}' for value in values)

def addr(suffix):
    return '0x' + suffix.zfill(40)

def rpc_key(method, params):
    return json.dumps([method, params], sort_keys=True)

def result(value):
    return {'jsonrpc': '2.0', 'id': 1, 'result': value}

base = {'get': {}, 'rpc': {}}
labels = ['version', 'height', 'catching_up', 'chain_id']
base['get']['/cosmos/base/tendermint/v1beta1/node_info'] = {'application_version': {'version': 'v7.0.0'}}
base['status'] = [{'result': {'sync_info': {'latest_block_height': str(height), 'catching_up': False}}}
                  for height in (101, 102)]
for module in ('layerxledger', 'layerxauth', 'layerxchains'):
    data = b'' if module != 'layerxchains' else b'{"max_chains":1024,"min_bond":"100000000"}'
    base['get'][module] = {'result': {'response': {'code': 0, 'height': '101', 'key': 'AQ==',
                              'value': base64.b64encode(data).decode()}}}
for module in ('layerxpay', 'layerxagent'):
    base['get'][f'/paxprotocol/paxchain/{module}/params'] = {'params': {'paused': False}}
for key in ('Leaders', 'ReceiptKeys'):
    base['get'][key] = {'param': {'subspace': 'sequencer', 'key': key, 'value': '[]'}}
labels += ['module sequencer parameters', 'module layerxledger parameters', 'module layerxauth parameters',
           'module layerxchains parameters', 'module layerxpay parameters', 'module layerxagent parameters',
           'module layerxprograms fee schedule', 'module lxphost metering schedule']

# Static ABI heads and dynamic offsets follow the checked-in view/pure ABIs.
vectors = [
    ('101a', 'accountId', '3f050cbc' + word(0, 0), word(1)),
    ('101b', 'askStreamOf', '929c1012' + word(0), word(0)),
    ('101c', 'isLive', 'd4a03b3d' + word(0, 0, 0), word(0, 64, 0)),
    ('101d', 'getCurrentFeeSchedule', 'bbe2e42d', word(1, 1, 1, 1, 1, 1, 1, 1, 125)),
    ('101e', 'chainCount', 'e02e1bfd', word(0)),
    ('0901', 'resolveOffset', '454ad3a1' + word(100, 0), word(100)),
    ('0902', 'computeClearing', '8397e9f8' + word(100, 96, 128, 0, 0), word(0, 100, 0)),
    ('0903', 'aggregate', '81097a95' + word(32, 1, 100, 1, 1), word(100, 1)),
    ('0904', 'scoreFill', 'e087a6ba' + word(100, 100), word(10000)),
    ('0905', 'pending', '5eebea20' + word(0), word(32, 0)),
    ('0906', 'accrued', 'f52981f4' + word(0), word(0)),
    ('0907', 'rootCount', '26f02502' + word(0), word(0)),
    ('0908', 'hashTypedData', 'd61c3c34' + word(0, 0), word(1)),
]
# An absent grant is not live; its reason is a dynamically encoded string.
reason = b'LXP_NOT_FOUND'
vectors[2] = (*vectors[2][:3], word(0, 64, len(reason)) + reason.hex().ljust(64, '0'))
for suffix, method, calldata, output in vectors:
    base['rpc'][rpc_key('eth_call', [{'to': addr(suffix), 'data': '0x' + calldata}, 'latest'])] = result('0x' + output)
    labels.append(f'precompile {addr(suffix)} {method}')
base['rpc'][rpc_key('eth_call', [{'to': addr('101d'), 'data': '0x543c7a2f'}, 'latest'])] = result('0x' + word(1, *([1] * 9)))

reads = {
    'debug': ('debug_traceCall', [{'to': addr('0'), 'gas': '0x5208'}, 'latest', {'tracer': 'callTracer'}], {'type': 'CALL', 'output': '0x'}),
    'echo': ('echo_echo', ['postflight'], 'postflight'),
    'eth': ('eth_blockNumber', [], '0x66'),
    'net': ('net_version', [], '125'),
    'pax': ('pax_getBlockByNumber', ['latest', False], {'number': '0x66', 'transactions': []}),
    'pax2': ('pax2_getBlockByNumber', ['latest', False], {'number': '0x66', 'transactions': []}),
    'px': ('px_getNetwork', [], {'paxeer': {'chain_id': '0x7d', 'latest_block': '0x66'}, 'consensus': {'catching_up': False}}),
    'txpool': ('txpool_status', [], {'pending': '0x0', 'queued': '0x0'}),
    'web3': ('web3_clientVersion', [], 'Geth/linux-amd64/go1.24'),
}
base['rpc'][rpc_key('eth_chainId', [])] = result('0x7d')
base['rpc'][rpc_key('rpc_modules', [])] = result({name: '1.0' for name in (*reads, 'rpc')})
labels.append('namespace rpc rpc_modules')
for name, (method, params, output) in reads.items():
    base['rpc'][rpc_key(method, params)] = result(output)
    labels.append(f'namespace {name} {method}')
(work / 'expected').write_text(''.join(label + ': ok\n' for label in labels))

def save(name, fixture):
    (work / (name + '.json')).write_text(json.dumps(fixture))

def variant(name, change):
    fixture = copy.deepcopy(base)
    change(fixture)
    save(name, fixture)

save('passing', base)
variant('unprefixed_version', lambda f: f['get']['/cosmos/base/tendermint/v1beta1/node_info']['application_version'].update(version='7.0.0'))
variant('wrong_version', lambda f: f['get']['/cosmos/base/tendermint/v1beta1/node_info']['application_version'].update(version='v6.12.0'))
variant('suffix_version', lambda f: f['get']['/cosmos/base/tendermint/v1beta1/node_info']['application_version'].update(version='v7.0.0-rc1'))
variant('below_height', lambda f: f['status'][0]['result']['sync_info'].update(latest_block_height='99'))
variant('at_height', lambda f: f['status'][0]['result']['sync_info'].update(latest_block_height='100'))
variant('stalled_height', lambda f: f['status'][1]['result']['sync_info'].update(latest_block_height='101'))
variant('regressed_height', lambda f: f['status'][1]['result']['sync_info'].update(latest_block_height='100'))
variant('invalid_height', lambda f: f['status'][1]['result']['sync_info'].update(latest_block_height='0x66'))
variant('catching_up', lambda f: f['status'][1]['result']['sync_info'].update(catching_up=True))
variant('string_catching_up', lambda f: f['status'][0]['result']['sync_info'].update(catching_up='false'))
variant('wrong_chain', lambda f: f['rpc'][rpc_key('eth_chainId', [])].update(result='0x1'))
variant('invalid_chain', lambda f: f['rpc'][rpc_key('eth_chainId', [])].update(result=125))
variant('null_chain', lambda f: f['rpc'][rpc_key('eth_chainId', [])].update(result=None))
variant('rpc_error', lambda f: f['rpc'][rpc_key('eth_chainId', [])].update(error={'code': -32000, 'message': 'read\nunavailable'}))
variant('wrong_id', lambda f: f['rpc'][rpc_key('eth_chainId', [])].update(id=2))
variant('store_error', lambda f: f['get']['layerxauth']['result']['response'].update(code=1))
variant('bad_store_key', lambda f: f['get']['layerxledger']['result']['response'].update(key='Ag=='))
variant('bad_params', lambda f: f['get']['/paxprotocol/paxchain/layerxpay/params'].update(params=None))
variant('bad_sequencer', lambda f: f['get']['ReceiptKeys']['param'].update(key='Other'))
variant('http_error', lambda f: f['get'].update({'/paxprotocol/paxchain/layerxagent/params': {'http_status': 503}}))
variant('invalid_json', lambda f: f['get'].update({'/cosmos/base/tendermint/v1beta1/node_info': {'raw_body': '{'}}))
variant('status_error', lambda f: f.update(status=[{'error': {'code': -32603, 'message': 'status unavailable'}}] * 2))
for suffix, method, calldata, output in vectors:
    variant('empty_' + suffix, lambda f, suffix=suffix, calldata=calldata: f['rpc'][rpc_key('eth_call', [{'to': addr(suffix), 'data': '0x' + calldata}, 'latest'])].update(result='0x'))
for namespace, (method, params, output) in reads.items():
    variant('denied_' + namespace, lambda f, method=method, params=params: f['rpc'][rpc_key(method, params)].update(error={'code': -32601, 'message': 'method denied'}))
variant('missing_namespace', lambda f: f['rpc'][rpc_key('rpc_modules', [])]['result'].pop('pax2'))
variant('extra_namespace', lambda f: f['rpc'][rpc_key('rpc_modules', [])]['result'].update(test='1.0'))
(work / 'active').write_text('passing')
PY

python3 - "$work" <<'PY' >"$work/server.out" 2>"$work/server.err" &
import http.server
import json
from pathlib import Path
import sys
import urllib.parse

work = Path(sys.argv[1])
counts = {}

class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def respond(self, body, status=200):
        if 'http_status' in body:
            status = body['http_status']
        data = body.get('raw_body', json.dumps(body)).encode()
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def dispatch(self, verb):
        case = (work / 'active').read_text().strip()
        fixture = json.loads((work / (case + '.json')).read_text())
        path = urllib.parse.urlsplit(self.path)
        query = urllib.parse.parse_qs(path.query)
        record = {'case': case, 'verb': verb, 'path': path.path, 'query': query}
        body = None
        if verb == 'GET':
            if path.path == '/status' and not query:
                index = counts.get(case, 0)
                counts[case] = index + 1
                if index < len(fixture['status']):
                    body = fixture['status'][index]
            elif path.path == '/abci_query' and set(query) == {'path', 'data', 'prove'}:
                store = json.loads(query['path'][0])
                for module in ('layerxledger', 'layerxauth', 'layerxchains'):
                    if store == f'/store/{module}/key' and query['data'] == ['0x01'] and query['prove'] == ['false']:
                        body = fixture['get'][module]
            elif path.path == '/cosmos/params/v1beta1/params' and query.get('subspace') == ['sequencer'] and set(query) == {'subspace', 'key'}:
                body = fixture['get'].get(query['key'][0])
            elif not query:
                body = fixture['get'].get(path.path)
        elif verb == 'POST' and path.path == '/' and not query:
            request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
            record['request'] = request
            if request.get('jsonrpc') == '2.0' and type(request.get('id')) is int and request['id'] == 1 and set(request) == {'jsonrpc', 'id', 'method', 'params'}:
                key = json.dumps([request['method'], request['params']], sort_keys=True)
                body = fixture['rpc'].get(key)
        record['matched'] = body is not None
        with (work / 'requests.jsonl').open('a') as log:
            log.write(json.dumps(record) + '\n')
        self.respond(body if body is not None else {'error': {'code': -32601, 'message': 'unknown read'}}, 200 if body is not None else 404)

    def do_GET(self):
        self.dispatch('GET')

    def do_POST(self):
        self.dispatch('POST')

server = http.server.HTTPServer(('127.0.0.1', 0), Handler)
(work / 'port').write_text(str(server.server_port))
server.serve_forever()
PY
server_pid=$!
for ((wait_count = 0; wait_count < 100; wait_count++)); do
  [[ ! -s "$work/port" ]] || break
  kill -0 "$server_pid" 2>/dev/null || { cat "$work/server.err" >&2; exit 1; }
  sleep 0.05
done
[[ -s "$work/port" ]] || { echo 'loopback server did not start' >&2; exit 1; }
rpc_url="http://127.0.0.1:$(cat "$work/port")"

run_case() {
  printf '%s' "$1" > "$work/active"
  code=0
  bash "$root/tools/release/postflight.sh" --upgrade-height 100 \
    --comet-rpc "$rpc_url" --rest-rpc "$rpc_url" --evm-rpc "$rpc_url" \
    --interval 0.001 --timeout 2 > "$work/out" 2> "$work/err" || code=$?
  [[ ! -s "$work/err" ]] || { cat "$work/err" >&2; return 1; }
}

passing_reads() {
  local scenario
  for scenario in passing unprefixed_version; do
    run_case "$scenario" || return 1
    [[ "$code" == 0 ]] || { cat "$work/out" >&2; return 1; }
    diff -u "$work/expected" "$work/out" || return 1
  done
  python3 - "$work" <<'PY' || return 1
import json
from pathlib import Path
import sys
work = Path(sys.argv[1])
records = [json.loads(line) for line in (work / 'requests.jsonl').read_text().splitlines()]
for case in ('passing', 'unprefixed_version'):
    requests = [r for r in records if r['case'] == case]
    fixture = json.loads((work / (case + '.json')).read_text())
    assert all(r['matched'] for r in requests), requests
    assert sum(r['path'] == '/status' for r in requests) == 2
    actual = {json.dumps([r['request']['method'], r['request']['params']], sort_keys=True)
              for r in requests if r['verb'] == 'POST'}
    assert actual == set(fixture['rpc']), (actual, fixture['rpc'])
    assert len(requests) == 36, len(requests)
PY
}

assert_failure() {
  local scenario=$1 line=$2
  run_case "$scenario" || return 1
  [[ "$code" == 1 ]] || { echo "$scenario: exit $code, want 1" >&2; cat "$work/out" >&2; return 1; }
  grep -qxF -- "$line" "$work/out" || { echo "$scenario: missing '$line'" >&2; cat "$work/out" >&2; return 1; }
  [[ $(wc -l < "$work/out") == 35 ]] || { cat "$work/out" >&2; return 1; }
}

version_checks() {
  assert_failure wrong_version 'version: expected 7.0.0, got v6.12.0' || return 1
  assert_failure suffix_version 'version: expected 7.0.0, got v7.0.0-rc1'
}

height_checks() {
  assert_failure below_height 'height: heights 99, 102 must be above upgrade height 100' || return 1
  assert_failure at_height 'height: heights 100, 102 must be above upgrade height 100' || return 1
  assert_failure stalled_height 'height: height did not rise (101 -> 101)' || return 1
  assert_failure regressed_height 'height: heights 101, 100 must be above upgrade height 100' || return 1
  assert_failure invalid_height 'height: invalid latest height'
}

catch_up_checks() {
  assert_failure catching_up 'catching_up: node is catching up or flag is invalid' || return 1
  assert_failure string_catching_up 'catching_up: node is catching up or flag is invalid'
}

chain_checks() {
  assert_failure wrong_chain 'chain_id: expected 125 (0x7d), got 0x1' || return 1
  assert_failure invalid_chain 'chain_id: invalid chain identifier' || return 1
  assert_failure null_chain 'chain_id: missing JSON-RPC result' || return 1
  assert_failure rpc_error 'chain_id: RPC error -32000: read unavailable' || return 1
  assert_failure wrong_id 'chain_id: invalid JSON-RPC envelope'
}

module_checks() {
  assert_failure store_error 'module layerxauth parameters: parameter store query failed' || return 1
  assert_failure bad_store_key 'module layerxledger parameters: parameter store key differs' || return 1
  assert_failure bad_params 'module layerxpay parameters: invalid module parameters' || return 1
  assert_failure bad_sequencer 'module sequencer parameters: sequencer parameter key differs' || return 1
  assert_failure http_error 'module layerxagent parameters: HTTP 503'
}

precompile_checks() {
  local suffix method
  while read -r suffix method; do
    assert_failure "empty_$suffix" "precompile 0x000000000000000000000000000000000000$suffix $method: empty or invalid ABI result" || return 1
  done <<'VECTORS'
101a accountId
101b askStreamOf
101c isLive
101d getCurrentFeeSchedule
101e chainCount
0901 resolveOffset
0902 computeClearing
0903 aggregate
0904 scoreFill
0905 pending
0906 accrued
0907 rootCount
0908 hashTypedData
VECTORS
}

namespace_checks() {
  local namespace method
  while read -r namespace method; do
    assert_failure "denied_$namespace" "namespace $namespace $method: RPC error -32601: method denied" || return 1
  done <<'METHODS'
debug debug_traceCall
echo echo_echo
eth eth_blockNumber
net net_version
pax pax_getBlockByNumber
pax2 pax2_getBlockByNumber
px px_getNetwork
txpool txpool_status
web3 web3_clientVersion
METHODS
  assert_failure missing_namespace 'namespace pax2 pax2_getBlockByNumber: namespace is not served' || return 1
  run_case extra_namespace || return 1
  [[ "$code" == 1 ]] || return 1
  grep -qxF 'namespace test: no read method declared for served namespace' "$work/out"
}

response_checks() {
  assert_failure invalid_json 'version: invalid JSON response' || return 1
  assert_failure status_error 'height: RPC error -32603: status unavailable'
}

option_checks() {
  local option value
  code=0
  bash "$root/tools/release/postflight.sh" --help > "$work/out" 2> "$work/err" || code=$?
  [[ "$code" == 0 && ! -s "$work/err" ]] || return 1
  grep -qF 'usage: postflight.sh' "$work/out" || return 1
  for value in 'http://127.0.0.1:26657' 'http://127.0.0.1:8545' 'http://127.0.0.1:1317'; do
    grep -qF "$value" "$work/out" || return 1
  done
  while read -r option value; do
    code=0
    bash "$root/tools/release/postflight.sh" --upgrade-height 100 "$option" "$value" > "$work/out" 2> "$work/err" || code=$?
    [[ "$code" == 1 && -s "$work/err" ]] || return 1
    grep -qF 'postflight:' "$work/err" || return 1
  done <<'OPTIONS'
--upgrade-height -1
--upgrade-height 0
--upgrade-height 1.5
--interval 0
--interval nan
--timeout inf
--timeout -1
--evm-rpc ftp://127.0.0.1
--unknown value
OPTIONS
  code=0
  bash "$root/tools/release/postflight.sh" > "$work/out" 2> "$work/err" || code=$?
  [[ "$code" == 1 ]] || return 1
  grep -qF -- '--upgrade-height' "$work/err"
}

read_requests_only() {
  python3 - "$work/requests.jsonl" <<'PY'
import json
import sys
records = [json.loads(line) for line in open(sys.argv[1])]
assert records
for record in records:
    assert record['matched'], record
    if record['verb'] == 'POST':
        assert record['request']['method'] in {
            'eth_call', 'eth_chainId', 'eth_blockNumber', 'rpc_modules', 'debug_traceCall',
            'echo_echo', 'net_version', 'pax_getBlockByNumber', 'pax2_getBlockByNumber',
            'px_getNetwork', 'txpool_status', 'web3_clientVersion',
        }, record
    else:
        assert record['verb'] == 'GET', record
PY
}

passed=0
failed=0
for test in passing_reads version_checks height_checks catch_up_checks chain_checks module_checks precompile_checks namespace_checks response_checks option_checks read_requests_only; do
  if "$test"; then
    echo "ok $test"
    passed=$((passed + 1))
  else
    echo "FAIL $test"
    failed=$((failed + 1))
  fi
done
echo "postflight.test: $passed passed, $failed failed"
[[ "$failed" == 0 ]]
