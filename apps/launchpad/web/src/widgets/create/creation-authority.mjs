import { createServer } from 'node:https';
import { request } from 'node:http';
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { createPublicClient, defineChain, http, parseAbi, keccak256, getAddress } from 'viem';

const [inputPath, profilePath, certPath, keyPath, root] = process.argv.slice(2);
const input = JSON.parse(readFileSync(inputPath, 'utf8')),
  fixture = input.phase2;
if (!fixture?.ownedFixture) throw new Error('Owned creation deployment required');
const manifest = fixture.authorityDeployment;
const sha = (value) => createHash('sha256').update(value).digest('hex');
const canonical = (value) =>
  JSON.stringify(value).replace(
    /[<>&\u2028\u2029]/g,
    (value) => `\\u${value.charCodeAt(0).toString(16).padStart(4, '0')}`
  );
const chain = defineChain({
  id: 125,
  name: 'Owned creation fixture',
  nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
  rpcUrls: { default: { http: [fixture.rpcUrl] } },
});
const client = createPublicClient({ chain, transport: http(fixture.rpcUrl), cacheTime: 0 });
const abi = parseAbi([
  'function decimals() view returns(uint8)',
  'function creationFee() view returns(uint256)',
  'function owner() view returns(address)',
  'function pendingOwner() view returns(address)',
  'function implementation() view returns(address)',
  'function approve(address,uint256) returns(bool)',
  'function createMarket(string,string,uint8,address) returns(address,address,uint256)',
]);
let origin, network, revision;
let headRefresh;
async function refreshOwnedHead() {
  if (!headRefresh) {
    headRefresh = (async () => {
      const block = await client.getBlock();
      if (Date.now() - Number(block.timestamp) * 1000 <= 30000) return;
      const rpc = new URL(fixture.rpcUrl);
      if (
        rpc.protocol !== 'http:' ||
        rpc.hostname !== '127.0.0.1' ||
        !(await client.request({ method: 'web3_clientVersion' })).toLowerCase().includes('anvil') ||
        !(await client.request({ method: 'anvil_getAutomine' }))
      )
        throw new Error('Fresh authority requires the owned automining Anvil chain');
      await client.request({ method: 'evm_mine', params: [Math.floor(Date.now() / 1000)] });
    })().finally(() => {
      headRefresh = undefined;
    });
  }
  await headRefresh;
}
async function attest(blockHash) {
  if ((await client.getChainId()) !== 125) throw new Error('Creation chain identity changed');
  let block = await client.getBlock(blockHash ? { blockHash } : {});
  if (blockHash && (await client.getBlock({ blockNumber: block.number })).hash !== blockHash)
    throw new Error('Requested creation head is not canonical');
  // An instant-mining fixture produces no heads while the workspace runs other
  // suites. Refresh the owned chain itself rather than relabeling an old head.
  if (Date.now() - Number(block.timestamp) * 1000 > 30000) {
    if (blockHash) throw new Error('Requested creation head is stale');
    await refreshOwnedHead();
    block = await client.getBlock();
  }
  const identities = {};
  for (const [role, pin] of Object.entries(manifest.roles)) {
    const code = await client.getCode({ address: pin.address, blockNumber: block.number });
    if (!code || code === '0x' || keccak256(code) !== pin.runtimeHash)
      throw new Error('Creation runtime differs: ' + role);
    if (pin.kind !== 'direct') {
      const actual =
        pin.kind === 'beacon'
          ? await client.readContract({
              address: pin.address,
              abi,
              functionName: 'implementation',
              blockNumber: block.number,
            })
          : getAddress(
              '0x' +
                (
                  await client.getStorageAt({
                    address: pin.address,
                    slot: '0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc',
                    blockNumber: block.number,
                  })
                ).slice(-40)
            );
      const implementation = await client.getCode({
        address: actual,
        blockNumber: block.number,
      });
      if (
        getAddress(actual) !== getAddress(pin.implementationAddress) ||
        !implementation ||
        keccak256(implementation) !== pin.implementationHash
      )
        throw new Error('Creation implementation differs: ' + role);
    }
    identities[role] = {
      role,
      address: pin.address,
      runtimeHash: pin.runtimeHash,
      verified: true,
      ...(pin.implementationAddress
        ? {
            implementationAddress: pin.implementationAddress,
            implementationHash: pin.implementationHash,
          }
        : {}),
    };
  }
  const creationFeeRaw = String(
    await client.readContract({
      address: manifest.roles.ProtocolConfig.address,
      abi,
      functionName: 'creationFee',
      blockNumber: block.number,
    })
  );
  const usdlDecimals = await client.readContract({
    address: manifest.roles.USDL.address,
    abi,
    functionName: 'decimals',
    blockNumber: block.number,
  });
  const actualOwner = await client.readContract({
    address: manifest.roles.PoolBeacon.address,
    abi,
    functionName: 'owner',
    blockNumber: block.number,
  });
  const pendingOwner = await client.readContract({
    address: manifest.roles.PoolBeacon.address,
    abi,
    functionName: 'pendingOwner',
    blockNumber: block.number,
  });
  if (
    creationFeeRaw !== '1000000' ||
    usdlDecimals !== 6 ||
    getAddress(actualOwner) !== getAddress(manifest.governance.expectedBeaconOwner) ||
    BigInt(pendingOwner) !== 0n
  )
    throw new Error('Creation activation policy differs');
  // Initial deployment mines many consecutive transactions faster than real
  // time. Await that actual head; never forge an observed block time.
  const ahead = Number(block.timestamp) * 1000 - Date.now();
  if (ahead > 120000) throw new Error('Owned creation head is too far ahead of the clock');
  if (ahead > 0) await new Promise((resolve) => setTimeout(resolve, ahead));
  return {
    manifestHash: revision.manifestSha256,
    deploymentVersion: revision.deploymentVersion,
    blockHash: block.hash,
    blockNumber: Number(block.number),
    observedAt: new Date().toISOString(),
    blockTime: new Date(Number(block.timestamp) * 1000).toISOString(),
    identities,
    actualOwner,
    pendingOwner,
    creationFeeRaw,
    settlementAddress: manifest.roles.USDL.address,
    usdlDecimals,
    eligible: true,
  };
}
async function creationPreflight(raw) {
  const input = JSON.parse(raw);
  const current = await attest(input.blockHash);
  const blockNumber = BigInt(current.blockNumber);
  const zero = '0x0000000000000000000000000000000000000000';
  if (
    input.family !== 'creation' ||
    input.expectedRevision !== revision.revisionSha256 ||
    input.blockHash !== current.blockHash ||
    input.creationPath !== 'Router' ||
    !['approval', 'creation'].includes(input.creationStage) ||
    getAddress(input.trader) !== getAddress(input.recipient) ||
    input.pool !== zero ||
    input.optical !== zero ||
    input.amountIn !== '' ||
    input.isBuy !== false ||
    input.nftId !== '' ||
    typeof input.name !== 'string' ||
    input.name.length < 1 ||
    input.name.length > 64 ||
    typeof input.symbol !== 'string' ||
    input.symbol.length < 1 ||
    input.symbol.length > 16 ||
    !Number.isInteger(input.feeStrategy) ||
    input.feeStrategy < 0 ||
    input.feeStrategy > 3
  )
    throw new Error('Creation preflight context differs from the owned deployment: ' + canonical({
      request: input, revision: revision.revisionSha256, blockHash: current.blockHash,
    }));
  const account = getAddress(input.trader);
  const approval = input.creationStage === 'approval';
  const call = approval
    ? {
        address: manifest.roles.USDL.address,
        abi,
        functionName: 'approve',
        args: [manifest.roles.Router.address, BigInt(current.creationFeeRaw)],
        account,
        blockNumber,
      }
    : {
        address: manifest.roles.Router.address,
        abi,
        functionName: 'createMarket',
        args: [input.name, input.symbol, input.feeStrategy, zero],
        account,
        blockNumber,
      };
  await client.simulateContract(call);
  const gas = await client.estimateContractGas(call);
  if ((await client.getBlock({ blockNumber })).hash !== input.blockHash || gas <= 0n)
    throw new Error('Creation simulation lost its canonical block');
  return {
    schemaVersion: 2,
    family: 'creation',
    revision,
    creationStage: input.creationStage,
    requestSha256: sha(raw),
    blockHash: input.blockHash,
    blockNumber: current.blockNumber,
    trader: input.trader,
    recipient: input.recipient,
    supported: true,
    executionPermitted: true,
    simulationMode: approval ? 'usdl_approval' : 'create_market',
    creationCost: {
      protocolFeeAsset: 'USDL',
      protocolFeeRaw: current.creationFeeRaw,
      gasAsset: 'PAX',
      gasEstimateAvailable: true,
      gasEstimatePaxRaw: String(gas * (await client.getGasPrice())),
      blockHash: input.blockHash,
      spender: manifest.roles.Router.address,
    },
  };
}

const server = createServer(
  { cert: readFileSync(certPath), key: readFileSync(keyPath), minVersion: 'TLSv1.3' },
  async (req, res) => {
    try {
      if (req.method === 'POST' && req.url === '/rpc') {
        const upstream = request(
          fixture.rpcUrl,
          { method: 'POST', headers: { 'Content-Type': 'application/json' }, timeout: 15000 },
          (reply) => {
            res.writeHead(reply.statusCode, reply.headers);
            reply.pipe(res);
          }
        );
        upstream.on('error', (error) => res.destroy(error));
        upstream.on('timeout', () => upstream.destroy());
        req.pipe(upstream);
        return;
      }
      if (req.method === 'GET' && req.url === '/v2/control/caps/pool_creation') {
        const activation = await attest();
        // This Router has no pause interface. Eligibility comes from the
        // actual code pins, governance and creation policy attested above.
        const allowed = activation.eligible;
        res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
        res.end(
          canonical({
            data: {
              capability: 'pool_creation',
              allowed,
              stale: false,
              reason: allowed ? 'available' : 'activation_ineligible',
              revision: String(activation.blockNumber),
              observedAt: activation.observedAt,
            },
          })
        );
        return;
      }
      if (req.method === 'POST' && req.url === '/v2/preflight/create') {
        let raw = '';
        for await (const chunk of req) {
          raw += chunk;
          if (Buffer.byteLength(raw) > 65536) throw new Error('Bounded creation request required');
        }
        const proof = await creationPreflight(raw);
        res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
        res.end(canonical(proof));
        return;
      }
      if (req.method !== 'GET' || req.url !== '/v2/network/authority') {
        res.writeHead(404).end();
        return;
      }
      const activation = await attest();
      res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
      res.end(
        canonical({
          schemaVersion: 2,
          serialExport: {
            schemaVersion: 2,
            revision,
            network,
            networkSha256: sha(canonical(network)),
          },
          attestation: activation,
          committedHead: {
            number: String(activation.blockNumber),
            hash: activation.blockHash,
          },
        })
      );
    } catch (error) {
      console.error(error);
      if (!res.headersSent) res.writeHead(503);
      res.end();
    }
  }
);
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
origin = `https://127.0.0.1:${server.address().port}`;
revision = {
  schemaVersion: 2,
  deploymentVersion: manifest.deploymentVersion,
  manifestSha256: sha(canonical(manifest)),
  capabilitySha256: sha(
    readFileSync(resolve(root, 'sdk/launchpad/shared/config/capabilities/frozen.json'))
  ),
  contractSupportSha256: sha(
    readFileSync(resolve(root, 'sdk/launchpad/shared/config/capabilities/contract-support.json'))
  ),
  serialInterfaceSha256: sha(
    readFileSync(resolve(root, 'sdk/launchpad/shared/interfaces/v2/types.gen.go'))
  ),
  revisionSha256: '',
};
revision.revisionSha256 = sha(canonical(revision));
network = {
  schemaVersion: 1,
  deploymentVersion: revision.deploymentVersion,
  manifestHash: revision.manifestSha256,
  sourceManifestSha256: manifest.sourceManifestSha256,
  chainId: 125,
  publicRPC: origin + '/rpc',
  explorerBase: origin,
  native: manifest.native,
  settlement: manifest.settlement,
  addresses: Object.fromEntries(
    Object.entries(manifest.roles)
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
      .map(([name, value]) => [name, value.address])
  ),
  creationFeeRaw: manifest.creationFeeRaw,
};
await attest();
writeFileSync(
  profilePath,
  canonical({
    schemaVersion: 1,
    profile: 'PRIVATE_FIXTURE',
    chainId: 125,
    applicationOrigin: origin,
    sourceManifestSha256: manifest.sourceManifestSha256,
    revision,
    networkSha256: sha(canonical(network)),
    publicRPC: network.publicRPC,
    explorerBase: network.explorerBase,
    expectedBeaconOwner: manifest.governance.expectedBeaconOwner,
    roles: manifest.roles,
    sourceArtifacts: {
      chainSha256: sha(readFileSync(inputPath)),
      compilerArtifactsSha256: sha(canonical(input.consumedArtifacts)),
      reviewedManifestSha256: revision.manifestSha256,
    },
  }) + '\n',
  { mode: 0o600, flag: 'wx' }
);
process.on('SIGTERM', () => {
  server.closeAllConnections();
  server.close(() => process.exit(0));
});
