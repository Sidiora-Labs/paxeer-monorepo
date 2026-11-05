#!/usr/bin/env python3
import argparse
import importlib.util
import json
import os
import re
from pathlib import Path
import ssl
import subprocess
import sys
import time

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'tools/paxeer-x'))
from candidate import SERVICE_CATALOGUE, catalogue, load_private, validate

spec = importlib.util.spec_from_file_location('human_boundary', ROOT / 'tools/qualification/paxeer-x/human-api-boundary.py')
B = importlib.util.module_from_spec(spec)
spec.loader.exec_module(B)

C = sys.modules.get('human_committed_projections')
if C is None:
    spec = importlib.util.spec_from_file_location('human_committed_projections', Path(__file__).with_name('human-canonical-projections.py'))
    C = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = C
    spec.loader.exec_module(C)


def qualify_plan(chain, plan, expected):
    C.require(plan == expected and plan['journey_kind'] == 'move'
              and re.fullmatch('[0-9a-f]{64}', plan['plan_digest']) and plan['legs'],
              'exact observed signed-intent plan required')
    capabilities = chain.capabilities()
    C.require(capabilities['pay'] and capabilities['ledger'], 'merged bank and budget capabilities required')
    requirements = plan['signing_requirements']
    C.require(len(requirements) == len(plan['legs']), 'one exact signing requirement per leg')
    fee = 0
    for index, (leg, requirement) in enumerate(zip(plan['legs'], requirements, strict=True)):
        C.require(leg['index'] == index and leg['chain_id'] == 125 and leg['value'] == '0'
                  and leg['source']['kind'] != 'paxeer-wallet' and leg['destination']['kind'] != 'paxeer-wallet'
                  and leg['mechanism'] in ('send', 'budget-defund')
                  and requirement['leg_index'] == index
                  and re.fullmatch('[0-9a-f]{64}', requirement['action_key'])
                  and re.fullmatch('[0-9a-f]{64}', requirement['signing_context']),
                  'single-chain executable intent capability required')
        raw = bytes.fromhex(leg['calldata'].removeprefix('0x'))
        if leg['mechanism'] == 'send':
            C.require(leg['to'] == '0x' + '00' * 18 + '1001'
                      and raw[:4] == C.merged.evm.encode_call('send', ['address', 'address', 'string', 'uint256'],
                                                            [bytes(20), bytes(20), '', 0])[:4],
                      'current bank send capability required')
            fields = C.merged.decode_abi(['address', 'address', 'string', 'uint256'], '0x' + raw[4:].hex())
            C.require(fields[3] == int(leg['money']['amount']) and fields[2], 'bank send amount and denomination differ')
        else:
            C.require(leg['to'] == C.PAY and raw[:4] == C.merged.evm.encode_call('budgetDefund', ['bytes32', 'uint128'],
                                                                                       [bytes(32), 0])[:4],
                      'current budget return capability required')
            fields = C.merged.decode_abi(['bytes32', 'uint128'], '0x' + raw[4:].hex())
            C.require(fields[1] == int(leg['money']['amount']), 'budget return amount differs')
        C.require(leg['fee']['currency'] == plan['total_fee']['currency']
                  and int(leg['fee']['amount']) == leg['gas_limit'] * int(leg['max_fee_per_gas']),
                  'exact disclosed EVM fee required')
        fee += int(leg['fee']['amount'])
    C.require(fee == int(plan['total_fee']['amount']), 'exact total intent fee required')
    return plan


def qualify_intent(client, foreign, chain, case):
    plan = client.call('POST', '/v1/intents/plan', case['request'])
    qualify_plan(chain, plan, case['expected_plan'])
    body = case['submission']
    C.require(body == case['expected_submission'] and body['plan_digest'] == plan['plan_digest']
              and re.fullmatch('[0-9a-f]{64}', body['signed_digest'])
              and len(body['bindings']) == len(plan['legs']), 'exact independently signed intent required')
    for binding, requirement, leg in zip(body['bindings'], plan['signing_requirements'], plan['legs'], strict=True):
        C.require(binding['leg_index'] == leg['index'] and binding['action_key'] == requirement['action_key']
                  and binding['account_sequence'] == leg['nonce'] and binding['authority'] == requirement['authority']
                  and binding['fee_limit'] == leg['fee'], 'signed intent leg binding differs')
    C.denied(lambda: foreign.call('POST', '/v1/intents/submit', body, case['idempotency_key']))
    first = client.call('POST', '/v1/intents/submit', body, case['idempotency_key'])
    C.require(first['plan_digest'] == plan['plan_digest'] and first['state'] == 'processing',
              'intent submission is only submitted progress')
    C.require(client.call('POST', '/v1/intents/submit', body, case['idempotency_key']) == first,
              'exact signed intent retry differs')
    changed = C.deepcopy(body)
    changed['signed_digest'] = ('0' if body['signed_digest'][0] != '0' else '1') + body['signed_digest'][1:]
    C.denied(lambda: client.call('POST', '/v1/intents/submit', changed, case['idempotency_key']),
             ('conflict', 'forbidden', 'invalid-request'), (400, 403, 409))
    return first


CLIENT = r'''
import {readFile,writeFile} from 'node:fs/promises';
import {pathToFileURL} from 'node:url';
import {randomBytes,createHash} from 'node:crypto';
const [root,manifest,output,phase,previous] = process.argv.slice(2);
const fixture = JSON.parse(await readFile(manifest,'utf8'));
const {createHumanApiClient,HumanApiError} = await import(pathToFileURL(root+'/apps/web/src/api/generated/index.ts').href);
const {conformance} = await import(pathToFileURL(root+'/apps/web/src/api/generated/conformance.ts').href);
const stable = value => JSON.stringify(value,(_,item)=>typeof item==='bigint'?item.toString():item);
let tests=0;
function check(value,message){if(!value)throw new Error(message);tests++;}
const outputs = new Map(previous==='-'?[]:JSON.parse(await readFile(previous,'utf8')).outputs);
const jars = new Map(Object.entries(fixture.sessions).map(([name,row])=>[name,new Map(Object.entries(row.cookies))]));
const clients = new Map();
for(const [name,jar] of jars){clients.set(name,createHumanApiClient({baseUrl:fixture.url,
csrfToken:()=>jar.get('__Host-layerx_csrf'),trace:()=> 'trc_'+randomBytes(16).toString('hex'),
fetch:async(url,init)=>{const headers=new Headers(init.headers);headers.set('Origin',fixture.origin);
headers.set('Authorization','Bearer '+fixture.sessions[name].assertion);
headers.set('Cookie',[...jar].map(([k,v])=>k+'='+v).join('; '));
const response=await fetch(url,{...init,headers,redirect:'error'});
for(const cookie of response.headers.getSetCookie()){const pair=cookie.split(';')[0],at=pair.indexOf('=');jar.set(pair.slice(0,at),pair.slice(at+1));}
return response;}}));}
function resolve(value){if(Array.isArray(value))return value.map(resolve);if(value&&typeof value==='object'){
if(Object.keys(value).length===1&&typeof value.$result==='string'){const [id,...parts]=value.$result.split('/');let item=outputs.get(id);for(const part of parts)item=item?.[part];check(item!==undefined,'actual predecessor result');return item;}
return Object.fromEntries(Object.entries(value).map(([key,item])=>[key,resolve(item)]));}return value;}
const cases=fixture.cases[phase];check(Array.isArray(cases)&&cases.length>0&&cases.length<=32,'bounded real cases');
for(const row of cases){check(typeof row.id==='string'&&!outputs.has(row.id)&&clients.has(row.principal)&&conformance[row.operation],'real schema-owned authenticated operation');
const input={client:clients.get(row.principal),params:resolve(row.params??{}),...(row.body===undefined?{}:{body:resolve(row.body)}),...(row.idempotency_key===undefined?{}:{idempotencyKey:row.idempotency_key})};
if(row.error_codes){let error;try{await conformance[row.operation](input);}catch(caught){error=caught;}
check(error instanceof HumanApiError&&row.error_codes.includes(error.detail.code),'real owner/provider/disclosure typed refusal');outputs.set(row.id,{refused:true});continue;}
const result=await conformance[row.operation](input);outputs.set(row.id,result);
if(row.operation==='intent.plan'){
check(/^[a-f0-9]{64}$/.test(result.plan_digest)&&result.legs.length>0,'actual deterministic sealed plan');
check(result.total_fee.amount===BigInt(row.expected_fee)&&result.total_fee.currency===row.currency,'exact real fee disclosure');
check(result.legs[0].money.amount===BigInt(row.expected_amount)&&result.legs[0].money.currency===row.currency,'exact actual amount disclosure');
if(row.coverage==='native-send')check(result.legs.length===1&&result.legs[0].chain_id===125&&result.legs[0].mechanism==='send','native SEND with no custody movement leg');
if(row.coverage==='budget-return')check(result.legs.some(leg=>leg.chain_id===125&&leg.mechanism==='budget-defund'),'real merged budget return path');
}
if(row.replay_of)check(stable(result)===stable(outputs.get(row.replay_of)),'durable exact idempotent outcome after restart');
if(row.operation==='journey.get'){
check(result.state==='done','actual canonical verified final journey required');
const refs=[...result.evidence,...result.stages.flatMap(stage=>stage.evidence)];
check(refs.some(ref=>ref.class==='layerx-receipt'&&['receipt-verified','checkpoint-finalised'].includes(ref.verification)),'real verified native outcome');
for(const ref of refs.filter(ref=>ref.class==='layerx-receipt'&&['receipt-verified','checkpoint-finalised'].includes(ref.verification))){
const material=await input.client.evidenceGet(ref.evidence_id);const bytes=Buffer.from(material.bytes_base64,'base64');
const portable=JSON.parse(bytes.toString('utf8'));
check(bytes.length>0&&material.verification==='checkpoint-finalised'&&material.rung==='committed'&&portable.format==='layerx-receipt-proof-v2','actual tenant-owned committed receipt export');
const signed=Buffer.from(portable.signedReceipt.slice(2),'hex');
const committedReference=fixture.committed_journeys.some(row=>'evd_'+row.transaction_hash.slice(2)===ref.evidence_id);
check(/^evd_[a-f0-9]{64}$/.test(ref.evidence_id)&&('evd_'+createHash('sha256').update(signed).digest('hex')===ref.evidence_id||committedReference),'exact signed receipt or independently committed transaction identity');
}}
}
await writeFile(output,stable({tests,outputs:[...outputs]}),{mode:0o600});
'''


def main():
    argparse.ArgumentParser(description='Qualify human attestor intents using protected manifests.').parse_args()
    B.require(len(sys.argv) == 1, 'no arguments accepted')
    candidate_path = os.environ.get('PAXEER_X_CANDIDATE_MANIFEST')
    fixture_path = os.environ.get('PAXEER_X_HUMAN_ATTESTOR_INTENTS_MANIFEST')
    B.require(candidate_path and fixture_path, 'genuine protected candidate and attestor signing fixture required')
    candidate = load_private(candidate_path)
    validate(candidate, catalogue(ROOT / SERVICE_CATALOGUE))
    revision, source = B.identity()
    B.require(candidate['source']['revision'] == revision and not candidate['source']['dirty'], 'genuine clean candidate source')
    fixture = B.load(fixture_path)
    B.require(fixture['schema'] == 'layerx-human-attestor-intents.v1', 'closed real-process fixture profile')
    directory = B.private(fixture['evidence_directory'], True)
    B.require(not any(directory.iterdir()), 'fresh private evidence directory')
    state_root = B.private(fixture['disposable_state_directory'], True)
    B.require(state_root != directory and not any(state_root.iterdir()), 'fresh disposable qualification authority state')
    artifacts = {name: B.executable(row, revision, source) for name, row in fixture['artifacts'].items()}
    roles = ('service', 'components', 'agent', 'paxd', 'gateway', 'attestor')
    B.require(all(role in artifacts for role in roles), 'actual gateway Human core wallet attestor production artifacts')
    node = Path(fixture['node'])
    B.require(node.is_absolute() and node.is_file() and os.access(node, os.X_OK) and B.digest(node) == fixture['node_sha256'], 'pinned genuine Node runtime')
    ca = B.private(fixture['ca_pem'])
    context = ssl.create_default_context(cafile=str(ca))
    url = B.endpoint(fixture['url'])
    B.require(fixture['principal'] != fixture['other_principal'], 'distinct authentic owner sessions')
    B.require(11 <= len(fixture['processes']) <= 32, 'bounded actual process inventory')
    by_role = {row['artifact']: row for row in fixture['processes']}
    B.require(all(role in by_role for role in roles), 'all actual process roles required')
    B.require(sum(row['artifact'] == 'attestor' for row in fixture['processes']) == 5, 'actual five-node signing quorum')
    components = by_role['components']['environment']
    B.require(len(json.loads(components['LAYERX_HUMAN_ATTESTOR_NODES'])) == 5 and components['LAYERX_HUMAN_PROTOCOL_VERSION'] == '3', 'actual production AttestorKms selected')
    for row in fixture['processes']:
        B.require(row['state_directory'].startswith(str(state_root) + '/'), 'qualification process state isolated from funded live services')
        for endpoint in row['qualification_endpoints']:
            B.endpoint(endpoint)
    required = {'native-send', 'budget-return', 'wrong-principal', 'changed-disclosure', 'missing-wallet', 'provider-refusal', 'restart-replay', 'verified-native-outcome'}
    coverage = {row['coverage'] for phase in ('live', 'restart') for row in fixture['cases'][phase]}
    B.require(required <= coverage, 'all actual positive refusal restart and verified outcome cases required without skips')
    for phase in ('live', 'restart'):
        for row in fixture['cases'][phase]:
            if row['coverage'] in ('native-send', 'budget-return'):
                B.require(row['operation'] == 'intent.plan' and not row.get('error_codes'), 'real positive planning coverage')
            elif row['coverage'] in ('wrong-principal', 'changed-disclosure', 'missing-wallet', 'provider-refusal'):
                B.require(row['operation'] in ('intent.plan', 'intent.submit') and isinstance(row.get('error_codes'), list) and row['error_codes'], 'actual refusal case required')
            elif row['coverage'] == 'restart-replay':
                B.require(phase == 'restart' and row['operation'] == 'intent.submit' and row.get('replay_of'), 'actual same submission restart replay')
            elif row['coverage'] == 'verified-native-outcome':
                B.require(row['operation'] == 'journey.get' and not row.get('error_codes'), 'actual canonical verified native outcome')

    processes = B.Processes(directory, artifacts)
    def ready():
        until = time.monotonic() + min(90, B.remaining())
        while time.monotonic() < until:
            B.require(all(child.poll() is None for child, _, _ in processes.children.values()), 'actual production process exited')
            try:
                status, result = B.request(url, context, 'GET', '/readyz')
                if status == 200 and result.get('result', {}).get('ready') is True:
                    return
            except (OSError, B.http.client.HTTPException):
                pass
            time.sleep(0.1)
        raise RuntimeError('actual unified endpoint readiness deadline')
    try:
        for row in fixture['processes']:
            processes.start(row)
        ready()
        owner, foreign, chain = C.current_clients(fixture)
        initial = qualify_intent(owner, foreign, chain, fixture['current_intent'])
        runner = directory / 'attestor-intents.mts'
        runner.write_text(CLIENT)
        runner.chmod(0o600)
        previous = '-'
        for phase in ('live', 'restart'):
            if phase == 'restart':
                stopped = [processes.stop(by_role[name]['name']) for name in ('service', 'components')]
                for row in reversed(stopped):
                    (directory / (row['name'] + '.log')).rename(directory / (row['name'] + '-before-restart.log'))
                    processes.start(row)
                ready()
                C.require(qualify_intent(owner, foreign, chain, fixture['current_intent']) == initial, 'signed intent survives restart')
                C.qualify_projections(owner, foreign, chain, fixture)
            output = directory / (phase + '-result.json')
            with (directory / (phase + '.log')).open('xb') as log:
                os.chmod(log.name, 0o600)
                result = subprocess.run([str(node), '--experimental-strip-types', str(runner), str(ROOT), fixture_path, str(output), phase, previous], cwd=ROOT, env=dict(os.environ, NODE_EXTRA_CA_CERTS=str(ca), NODE_TLS_REJECT_UNAUTHORIZED='1', NODE_OPTIONS=''), stdout=log, stderr=log, timeout=min(600, B.remaining()))
            B.require(result.returncode == 0, 'actual gateway-to-attestor production case failed')
            result = B.load(output)
            B.require(type(result['tests']) is int and result['tests'] >= 4, 'real assertions required without skips')
            B.COUNT += result['tests']
            previous = str(output)
        print('human-attestor-intents: PASS tests=' + str(B.COUNT) + ' revision=' + revision)
    finally:
        processes.close()


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print('human-attestor-intents: FAIL: ' + str(error), file=sys.stderr)
        raise SystemExit(1)
