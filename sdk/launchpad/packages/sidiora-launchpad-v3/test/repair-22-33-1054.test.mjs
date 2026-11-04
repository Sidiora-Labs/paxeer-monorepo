import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync, realpathSync } from 'node:fs';
import { createServer } from 'node:http';
import { registerHooks } from 'node:module';
import { fileURLToPath } from 'node:url';
import {
  createPublicClient, decodeFunctionData, defineChain, encodeFunctionData,
  encodeFunctionResult, getContract, http,
} from 'viem';
import * as abis from '../src/abis.ts';
import { abiIdentities, canonicalAbiEntries } from './abi-conformance.test.mjs';
import {
  assertExportedArtifactAbi, assertProvenanceDigest, assertProvenanceLicense,
  containedProvenanceFile,
} from './provenance.test.mjs';

const repositoryRoot = realpathSync(fileURLToPath(new URL('../../../../../', import.meta.url)));
const roles = [
  ['factoryAbi', 'SidioraFactory', 'core'],
  ['routerAbi', 'Router', 'periphery'],
  ['quoterAbi', 'Quoter', 'periphery'],
  ['protocolConfigAbi', 'ProtocolConfig', 'protocol'],
  ['erc20Abi', 'SidioraERC20', 'core'],
];

function readRoleAbi(role) {
  const path = containedProvenanceFile(repositoryRoot, `precompiles/launchpad/abi/${role}.json`, `${role}: artifact`);
  return JSON.parse(readFileSync(path, 'utf8'));
}

function finding29RejectsWireLayoutDrift() {
  const tupleSides = new Set();
  for (const [name] of roles) {
    const original = abis[name];
    const changes = [
      ['outputs', abi => {
        const output = abi.find(entry => entry.outputs?.length).outputs[0];
        output.type = output.type === 'bytes32' ? 'uint256' : 'bytes32';
      }],
      ['indexed', abi => {
        const input = abi.find(entry => entry.type === 'event' && entry.inputs.length).inputs[0];
        input.indexed = !input.indexed;
      }],
      ['anonymous', abi => {
        const event = abi.find(entry => entry.type === 'event');
        event.anonymous = !event.anonymous;
      }],
      ['stateMutability', abi => { abi.find(entry => entry.stateMutability === 'view').stateMutability = 'pure'; }],
    ];
    for (const [field, change] of changes) {
      const altered = structuredClone(original);
      change(altered);
      assert.deepEqual(abiIdentities(altered), abiIdentities(original), `${name}: ${field} preserves selectors`);
      assert.notDeepEqual(canonicalAbiEntries(altered), canonicalAbiEntries(original), `${name}: reject ${field} drift`);
    }
    assert.deepEqual(canonicalAbiEntries([...original].reverse()), canonicalAbiEntries(original));
    assert.notDeepEqual(canonicalAbiEntries([...original, original[0]]), canonicalAbiEntries(original));
    for (const side of ['inputs', 'outputs']) {
      for (const [index, entry] of original.entries()) {
        for (const [parameterIndex, parameter] of (entry[side] ?? []).entries()) {
          if (!parameter.components) continue;
          tupleSides.add(side);
          const altered = structuredClone(original);
          altered[index][side][parameterIndex].components[0].type = 'bytes32';
          assert.notDeepEqual(canonicalAbiEntries(altered), canonicalAbiEntries(original), `${name}: reject ${side} tuple component drift`);
          const renamed = structuredClone(original);
          renamed[index][side][parameterIndex].components[0].name += 'Changed';
          assert.notDeepEqual(canonicalAbiEntries(renamed), canonicalAbiEntries(original), `${name}: reject ${side} tuple decoding name drift`);
        }
      }
    }
  }
  assert.deepEqual(tupleSides, new Set(['inputs', 'outputs']), 'exercise both request and response tuple layouts');
}

function finding28RejectsSourceAndExportDrift() {
  for (const [name, role, directory] of roles) {
    const sourcePath = `contracts/launchpad/contracts/${directory}/${role}.sol`;
    const path = containedProvenanceFile(repositoryRoot, sourcePath, `${name}: source`);
    const source = readFileSync(path);
    const digest = createHash('sha256').update(source).digest('hex');
    const license = source.toString('utf8').split(/\r?\n/)[0];
    assertProvenanceDigest(source, digest, name);
    assertProvenanceLicense(source.toString('utf8'), license, name);
    assert.throws(() => assertProvenanceDigest(Buffer.concat([source, Buffer.from('\n')]), digest, name), /SHA-256 must match/);
    assert.throws(() => assertProvenanceLicense(source.toString('utf8'), `${license}-Changed`, name), /license marker must match/);
    assert.throws(() => assertProvenanceLicense(source.toString('utf8'), '', name), /SPDX marker/);
    for (const invalidPath of ['', '/outside.sol', 'C:\\outside.sol', '../outside.sol', 'missing.sol', 'contracts']) {
      assert.throws(() => containedProvenanceFile(repositoryRoot, invalidPath, `${name}: source`));
    }
    const authoritativeAbi = readRoleAbi(role);
    assertExportedArtifactAbi({ export: name }, authoritativeAbi);
    const changed = structuredClone(authoritativeAbi);
    const output = changed.find(entry => entry.outputs?.length).outputs[0];
    output.type = output.type === 'bytes32' ? 'uint256' : 'bytes32';
    assert.throws(() => assertExportedArtifactAbi({ export: name }, changed), /exported ABI must match/);
    assert.throws(() => assertExportedArtifactAbi({ export: `${name}Missing` }, authoritativeAbi), /must name an exported ABI/);
    assert.throws(() => assertExportedArtifactAbi({ export: name }, { abi: authoritativeAbi }), /artifact must be an ABI array/);
  }
}

async function finding29And28LaunchpadWalkthroughOverHttp(t) {
  const sourceUrl = new URL('../src/index.ts', import.meta.url).href;
  const hooks = registerHooks({
    resolve(specifier, context, nextResolve) {
      if (context.parentURL === sourceUrl && specifier === './abis.js') {
        return nextResolve('./abis.ts', context);
      }
      return nextResolve(specifier, context);
    },
  });
  t.after(() => hooks.deregister());
  const { createLaunchpadClient } = await import(sourceUrl);
  const address = digit => `0x${digit.repeat(40)}`;
  const deployment = {
    chainId: 125, factory: address('1'), router: address('2'), quoter: address('3'),
    protocolConfig: address('4'), usdl: address('5'),
  };
  const pool = address('6');
  const cases = [
    { role: 'SidioraFactory', to: deployment.factory, functionName: 'getNonce', args: [pool], result: 12n },
    { role: 'Router', to: deployment.router, functionName: 'usdlAddress', args: [], result: deployment.usdl },
    { role: 'Quoter', to: deployment.quoter, functionName: 'quoteExactInput', args: [pool, 1000n, true], result: { amountOut: 970n, feeAmount: 30n, priceImpactBps: 12n } },
    { role: 'ProtocolConfig', to: deployment.protocolConfig, functionName: 'creationFee', args: [], result: 9n },
    { role: 'SidioraERC20', to: deployment.usdl, functionName: 'balanceOf', args: [pool], result: 88n },
  ];
  const exchanges = [];
  const serverErrors = [];
  const server = createServer(async (request, response) => {
    try {
      assert.equal(request.method, 'POST');
      assert.equal(request.url, '/');
      assert.match(request.headers['content-type'], /^application\/json/);
      let body = '';
      for await (const chunk of request) body += chunk;
      const rpc = JSON.parse(body);
      assert.equal(rpc.jsonrpc, '2.0');
      assert.equal(typeof rpc.id, 'number');
      assert.equal(rpc.method, 'eth_call');
      assert.equal(rpc.params.length, 2);
      assert.equal(rpc.params[1], 'latest');
      const operation = cases[exchanges.length];
      assert.ok(operation, 'no unexpected RPC call');
      const abi = readRoleAbi(operation.role);
      assert.deepEqual(rpc.params[0], {
        to: operation.to,
        data: encodeFunctionData({ abi, functionName: operation.functionName, args: operation.args }),
      });
      const decoded = decodeFunctionData({ abi, data: rpc.params[0].data });
      assert.equal(decoded.functionName, operation.functionName);
      assert.deepEqual(decoded.args ?? [], operation.args);
      const result = encodeFunctionResult({ abi, functionName: operation.functionName, result: operation.result });
      const reply = { jsonrpc: '2.0', id: rpc.id, result };
      exchanges.push({ request: rpc, response: reply });
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(JSON.stringify(reply));
    } catch (error) {
      serverErrors.push(error);
      response.writeHead(500);
      response.end(JSON.stringify({ error: error.message }));
    }
  });
  t.after(() => new Promise((resolve, reject) => {
    server.close(error => error ? reject(error) : resolve());
    server.closeAllConnections();
  }));
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const endpoint = `http://127.0.0.1:${server.address().port}`;
  const chain = defineChain({ id: 125, name: 'Paxeer', nativeCurrency: { name: 'Paxeer', symbol: 'PAX', decimals: 18 }, rpcUrls: { default: { http: [endpoint] } } });
  const publicClient = createPublicClient({ chain, transport: http(endpoint, { retryCount: 0 }) });
  const launchpad = createLaunchpadClient(publicClient, deployment);
  assert.deepEqual(launchpad.deployment, deployment);
  assert.equal(await launchpad.factory.read.getNonce([pool]), cases[0].result);
  assert.equal(await launchpad.router.read.usdlAddress(), cases[1].result);
  assert.deepEqual(await launchpad.quoter.read.quoteExactInput([pool, 1000n, true]), cases[2].result);
  assert.equal(await launchpad.protocolConfig.read.creationFee(), cases[3].result);
  const token = getContract({ address: deployment.usdl, abi: abis.erc20Abi, client: publicClient });
  assert.equal(await token.read.balanceOf([pool]), cases[4].result);
  assert.deepEqual(serverErrors, []);
  assert.equal(exchanges.length, cases.length);
  for (const exchange of exchanges) {
    assert.deepEqual(Object.keys(exchange.response).sort(), ['id', 'jsonrpc', 'result']);
    assert.equal(exchange.response.id, exchange.request.id);
    assert.match(exchange.response.result, /^0x(?:[a-f0-9]{64})+$/);
  }
}

test(finding29RejectsWireLayoutDrift);
test(finding28RejectsSourceAndExportDrift);
test(finding29And28LaunchpadWalkthroughOverHttp);
