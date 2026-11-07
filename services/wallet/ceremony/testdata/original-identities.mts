import { readFileSync } from 'node:fs';
import { createInterface } from 'node:readline';
import { createRequire } from 'node:module';
import { join } from 'node:path';
import { TestChain } from '../../gateway/test/support/chain.ts';
import { AttestorDaemonClient, bindLayerX, bindMessage, mainAccountId, readUnifiedAccount, signBindWithAttestors } from '../../gateway/src/provision/bind.ts';
import { env } from '../../gateway/src/env.ts';
import { closePool } from '../../gateway/src/db/pool.ts';
import { RpcPool } from '../../gateway/src/rpc/pool.ts';

const require = createRequire(join(process.cwd(), 'package.json'));
const { privateKeyToAccount } = require('viem/accounts');
const chain = new TestChain(125);
await chain.start();
const rpc = new RpcPool({ urls: [chain.url], chainId: 125, timeoutMs: 10_000, healthIntervalMs: 60_000, lagThresholdBlocks: 3 });
process.stdout.write(JSON.stringify({ url: chain.url }) + '\n');
try {
  for await (const line of createInterface({ input: process.stdin })) {
    const request = JSON.parse(line);
    Object.assign(env, request.authority, { ATTESTOR_ENDPOINTS: request.config.Nodes.map((node) => node.URL), ATTESTOR_QUORUM: 3 });
    const client = new AttestorDaemonClient({
      endpoints: request.config.Nodes.map((node) => node.URL),
      cert: readFileSync(request.config.GatewayCertFile), key: readFileSync(request.config.GatewayKeyFile),
      ca: readFileSync(request.config.CAFile), quorum: 3, timeoutMs: 120_000,
    });
    try {
      const health = await client.health();
      if (health.some((node) => !node.ready)) {
        process.stderr.write(`original identity participant health: ${JSON.stringify(health)}\n`);
        throw new Error('original identity participants are not ready');
      }
      if (request.op === 'generate') {
        const generated = await client.generate(request.keyId, 'ed25519', request.owner, request.address);
        process.stdout.write(JSON.stringify(generated) + '\n');
      } else if (request.op === 'bind') {
        const account = privateKeyToAccount(`0x${request.privateKey}`);
        const message = bindMessage(125, account.address, 0);
        const signature = await signBindWithAttestors(client, request.keyId, request.publicKey, message, request.token);
        const raw = await account.signTransaction(bindLayerX({ chainId: 125, address: account.address, nonce: 0, publicKey: request.publicKey,
          signature, gas: 200_000n, maxFeePerGas: 2_000_000_000n, maxPriorityFeePerGas: 2_000_000n }));
        chain.balances.set(account.address.toLowerCase(), 10n ** 20n);
        await rpc.request('eth_sendRawTransaction', [raw]);
        const unified = await readUnifiedAccount(rpc, account.address);
        const expected = mainAccountId(`did:layerx:${request.publicKey}`);
        if (unified.didPublicKey !== request.publicKey || unified.mainAccountId !== expected) throw new Error('binding did not commit');
        process.stdout.write(JSON.stringify({ signature, publicKey: request.publicKey, did: `did:layerx:${request.publicKey}`, main: expected }) + '\n');
      } else {
        throw new Error('unknown fixture operation');
      }
    } finally {
      client.close();
    }
  }
} finally {
  await closePool();
  rpc.stop();
  await chain.stop();
}
