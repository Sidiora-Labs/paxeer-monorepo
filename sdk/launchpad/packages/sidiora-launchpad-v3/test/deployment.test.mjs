import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createPublicClient, defineChain, http, zeroAddress } from 'viem';
import {
  createLaunchpadClient, deploymentFromDocument, deploymentFromManifest,
  factoryAbi, protocolConfigAbi, quoterAbi, routerAbi, validateDeployment,
} from '../dist/index.js';

const repositoryRoot = new URL('../../../../../', import.meta.url);
const document = JSON.parse(readFileSync(
  new URL('sdk/launchpad/shared/config/deployment/current.json', repositoryRoot), 'utf8',
));
const roleKeys = [
  ['factory', 'SidioraFactory'], ['router', 'Router'], ['quoter', 'Quoter'],
  ['protocolConfig', 'ProtocolConfig'], ['usdl', 'USDL'],
];

function deploymentDocumentPreservesChainAndRoleAddresses() {
  const input = structuredClone(document);
  const deployment = deploymentFromDocument(input);
  assert.equal(deployment.chainId, document.chainId);
  for (const [key, role] of roleKeys) {
    assert.equal(deployment[key], document.roles[role].address, key);
    input.roles[role].address = zeroAddress;
    assert.equal(deployment[key], document.roles[role].address, `${key} is a snapshot`);
  }
  assert.ok(Object.isFrozen(deployment));
}

function deploymentReadersRejectWrongChainsAndInvalidAddresses() {
  const deployment = deploymentFromDocument(document);
  for (const chainId of [1, '125', undefined]) {
    assert.throws(() => deploymentFromDocument({ ...document, chainId }), /chain 125/);
    assert.throws(() => validateDeployment({ ...deployment, chainId }), /chain 125/);
  }
  for (const [key, role] of roleKeys) {
    for (const address of [undefined, null, '', 'invalid', zeroAddress]) {
      const input = structuredClone(document);
      input.roles[role].address = address;
      assert.throws(() => deploymentFromDocument(input), new RegExp(`Invalid deployment address: ${key}`));
      assert.throws(() => validateDeployment({ ...deployment, [key]: address }), new RegExp(`Invalid deployment address: ${key}`));
    }
  }
  const copy = { ...deployment };
  const validated = validateDeployment(copy);
  copy.router = zeroAddress;
  assert.equal(validated.router, document.roles.Router.address);
  assert.ok(Object.isFrozen(validated));
}

function legacyManifestResolvesTheCurrentDeployment() {
  const manifest = JSON.parse(readFileSync(
    new URL('contracts/launchpad/deployments/paxeer-x-network-addresses.json', repositoryRoot), 'utf8',
  ));
  assert.deepEqual(deploymentFromManifest(manifest), deploymentFromDocument(document));
  assert.ok(Object.isFrozen(deploymentFromManifest(manifest)));
  assert.throws(() => deploymentFromManifest({ ...manifest, _meta: { ...manifest._meta, network: '1' } }), /chain 125/);
  for (const [key, role] of roleKeys) {
    const input = structuredClone(manifest);
    if (key === 'usdl') input._meta.usdl = zeroAddress;
    else input[`${role}_proxy`] = zeroAddress;
    assert.throws(() => deploymentFromManifest(input), new RegExp(`Invalid deployment address: ${key}`));
  }
}

function launchpadClientUsesDocumentAddressesAndRoleAbis() {
  const chain = defineChain({
    id: document.chainId, name: 'Paxeer X Network',
    nativeCurrency: { name: 'PAX', ...document.native },
    rpcUrls: { default: { http: ['https://api5.mainnet-beta.paxeer.network'] } },
  });
  const client = createPublicClient({ chain, transport: http() });
  const deployment = deploymentFromDocument(document);
  const launchpad = createLaunchpadClient(client, deployment);
  assert.deepEqual(launchpad.deployment, deployment);
  assert.ok(Object.isFrozen(launchpad.deployment));
  for (const [key, abi] of [
    ['factory', factoryAbi], ['router', routerAbi], ['quoter', quoterAbi],
    ['protocolConfig', protocolConfigAbi],
  ]) {
    assert.equal(launchpad[key].address, deployment[key]);
    assert.deepEqual(launchpad[key].abi, abi);
  }
  const otherChain = createPublicClient({ chain: { ...chain, id: 1 }, transport: http() });
  assert.throws(() => createLaunchpadClient(otherChain, deployment), /Public client must specify chain 125/);
  const noChain = createPublicClient({ transport: http(chain.rpcUrls.default.http[0]) });
  assert.throws(() => createLaunchpadClient(noChain, deployment), /Public client must specify chain 125/);
}

test(deploymentDocumentPreservesChainAndRoleAddresses);
test(deploymentReadersRejectWrongChainsAndInvalidAddresses);
test(legacyManifestResolvesTheCurrentDeployment);
test(launchpadClientUsesDocumentAddressesAndRoleAbis);
