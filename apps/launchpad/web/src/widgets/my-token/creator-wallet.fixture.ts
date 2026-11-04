import { strict as assert } from 'node:assert';
import { createServer } from 'node:http';
import {
  decodeFunctionData,
  encodeFunctionResult,
  parseAbi,
  type Address,
  type Hex,
} from 'viem';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { RecordedProvider, recordedCall } from '../swap/swap-chain.fixture';
import recording from './creator-chain.recording.json';
import metadata from './creator-metadata.recording.json';

const creatorAbi = parseAbi([
  'function getPoolsByCreator(address creator) view returns (address[])',
]);

export async function creatorWalletFixture() {
  const coins = recording.coins;
  const wallets = [
    coins[0].Creator as Address,
    '0xFFcf8FDEE72ac11b5c542428B35EEF5769C409f0' as Address,
  ] as const;
  const pools = new Map<string, readonly Address[]>([
    [wallets[0].toLowerCase(), [coins[0].Pool as Address]],
    [wallets[1].toLowerCase(), [coins[1].Pool as Address]],
  ]);
  let accounts: readonly Address[] = [wallets[0]];
  const provider = new RecordedProvider(({ method, params = [] }) => {
    switch (method) {
      case 'eth_chainId':
        return recording.chainId;
      case 'eth_accounts':
      case 'eth_requestAccounts':
        return accounts;
      case 'eth_call': {
        const call = params[0] as { to: string; data: Hex };
        if (call.to.toLowerCase() !== CORE_DEPLOYMENT.quoter.toLowerCase())
          return recordedCall(recording.calls, params);
        const decoded = decodeFunctionData({ abi: creatorAbi, data: call.data });
        assert.equal(decoded.functionName, 'getPoolsByCreator');
        const list = pools.get(decoded.args[0].toLowerCase());
        assert.ok(list, 'Creator must exist in the in-memory registry');
        return encodeFunctionResult({
          abi: creatorAbi,
          functionName: 'getPoolsByCreator',
          result: list,
        });
      }
      default:
        throw new Error(`Unrecorded wallet RPC method: ${method}`);
    }
  });
  const requests: string[] = [];
  const errors: unknown[] = [];
  type Hold = {
    token: string;
    started: boolean;
    cancelled: boolean;
    finished: boolean;
    resumed: Promise<void>;
    release(): void;
  };
  const holds: Hold[] = [];
  function holdMetadata(token: string) {
    let release!: () => void;
    const resumed = new Promise<void>((resolve) => {
      release = resolve;
    });
    const hold: Hold = {
      token: token.toLowerCase(),
      started: false,
      cancelled: false,
      finished: false,
      resumed,
      release,
    };
    holds.push(hold);
    return hold;
  }
  const server = createServer(async (request, response) => {
    try {
      assert.equal(request.method, 'GET');
      const path = new URL(request.url!, 'http://localhost').pathname.toLowerCase();
      requests.push(path);
      const row = metadata.find((item) => item.path === path);
      assert.ok(row, `Unrecorded HTTP request: ${path}`);
      const hold = holds.find(
        (item) => !item.started && path === `/metadata/${item.token}`,
      );
      if (hold) {
        hold.started = true;
        response.once('close', () => {
          if (!response.writableFinished) hold.cancelled = true;
        });
        await hold.resumed;
        hold.finished = true;
      }
      if (response.destroyed) return;
      response.writeHead(row.status, { 'content-type': 'application/json' });
      response.end(JSON.stringify(row.body));
    } catch (error) {
      errors.push(error);
      response.writeHead(500, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ error: String(error) }));
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  assert.ok(address && typeof address !== 'string');
  return {
    coins,
    wallets,
    pools,
    provider,
    requests,
    errors,
    holdMetadata,
    origin: `http://127.0.0.1:${address.port}`,
    switchWallet(wallet: Address | null) {
      accounts = wallet ? [wallet] : [];
      provider.emit('accountsChanged', accounts);
    },
    async close() {
      for (const hold of holds) hold.release();
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
      await provider.disconnect();
    },
  };
}
