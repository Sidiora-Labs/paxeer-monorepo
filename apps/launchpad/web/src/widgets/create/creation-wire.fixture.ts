import { createHash } from 'node:crypto';
import {
  createServer,
  type IncomingMessage,
  type ServerResponse,
} from 'node:http';
import { once } from 'node:events';
import {
  createPublicClient,
  createWalletClient,
  decodeFunctionData,
  encodeAbiParameters,
  encodeEventTopics,
  encodeFunctionData,
  encodeFunctionResult,
  http,
  keccak256,
  parseAbi,
  stringToHex,
  toHex,
  zeroAddress,
  type Address,
  type Hex,
  type PublicClient,
  type WalletClient,
} from 'viem';
import type { RuntimeAuthority } from '@/core/runtime-authority';
import {
  runtimeCreationAdmission,
  type CreationAdmissionStage,
} from './cap-admission';
import {
  ChainCreationAdapter,
  type LaunchDeployment,
} from './creation-adapters';
import { CreationController } from './creation-controller';
import { address } from './creation-domain';
import {
  checkedAttempt,
  validateCreationDraft,
  type CreationDraftV2,
  type CreationStorage,
  type LaunchAttempt,
} from './creation-storage';

const a = (n: number): Address => `0x${n.toString(16).padStart(40, '0')}`;
const h = (n: number): Hex => `0x${n.toString(16).padStart(64, '0')}`;
const owner = a(90),
  native = a(0x1017),
  pool = a(91),
  token = a(92),
  treasury = a(93);
const chain = {
  id: 125,
  name: 'Creation wire fixture',
  nativeCurrency: { name: 'PAX', symbol: 'PAX', decimals: 18 },
  rpcUrls: { default: { http: ['http://127.0.0.1'] } },
} as const;
const abi = parseAbi([
  'function decimals() view returns (uint8)',
  'function balanceOf(address) view returns (uint256)',
  'function allowance(address,address) view returns (uint256)',
  'function approve(address,uint256) returns (bool)',
  'function transfer(address,uint256) returns (bool)',
  'function createMarket(string name,string symbol,uint8 feeStrategy,address optical) returns (address tokenAddr,address poolAddr,uint256 nftId)',
  'function buy(address pool,uint256 usdlAmountIn,uint256 minTokensOut,uint256 deadline) returns (uint256)',
  'function factory() view returns (address)',
  'function poolRegistry() view returns (address)',
  'function protocolConfig() view returns (address)',
  'function usdlAddress() view returns (address)',
  'function treasury() view returns (address)',
  'function creationFee() view returns (uint256)',
  'function getPoolMetadata(address pool) view returns ((address creator,address token,address optical,uint256 nftId,uint256 createdAt,uint256 createdBlock))',
  'function getPoolsByCreatorPage(address creator,uint256 offset,uint256 limit) view returns (address[])',
  'function isRegisteredPool(address pool) view returns (bool)',
  'function getPoolByToken(address token) view returns (address)',
  'function tokenAddress() view returns (address)',
  'function opticalAddress() view returns (address)',
  'function quoteExactInput(address pool,uint256 amountIn,bool isBuy) view returns ((uint256 amountOut,uint256 feeAmount,uint256 priceImpactBps))',
  'event MarketCreated(address indexed token,address indexed pool,address indexed creator,uint256 nftId,address optical)',
  'event Swap(bytes32 indexed poolId,address indexed sender,bool isBuy,uint256 amountIn,uint256 amountOut,uint256 fee,uint256 price,uint256 timestamp,uint256 blockNumber)',
]);
const revision: RuntimeAuthority['revision'] = {
  schemaVersion: 2,
  deploymentVersion: 'native-wire-fixture',
  manifestSha256: '1'.repeat(64),
  capabilitySha256: '2'.repeat(64),
  contractSupportSha256: '3'.repeat(64),
  serialInterfaceSha256: '4'.repeat(64),
  revisionSha256: '5'.repeat(64),
};
const code = '0x60006000' as const;
const trade = {
  revision: revision.revisionSha256,
  router: a(1),
  quoter: a(2),
  registry: a(3),
  emitter: a(4),
  usdl: a(5),
  pins: [a(1), a(2), a(3), a(4)].map((value) => ({
    address: value,
    runtimeHash: keccak256(code),
  })),
};
const deployment: LaunchDeployment = {
  revision: trade.revision,
  chainId: 125,
  router: trade.router,
  usdl: trade.usdl,
  usdlSpender: native,
  factory: a(6),
  protocolConfig: a(7),
  poolRegistry: trade.registry,
  quoter: trade.quoter,
  opticalRegistry: a(8),
  tradeDeployment: trade,
};
function exactTopics(values: ReturnType<typeof encodeEventTopics>): Hex[] {
  return values.map((value) => {
    if (typeof value !== 'string')
      throw new Error('Event topic must be concrete');
    return value;
  });
}
export function creationWireDraft(): CreationDraftV2 {
  return {
    draftId: crypto.randomUUID(),
    version: 2,
    creator: owner,
    chainId: 125,
    name: 'Native creation',
    symbol: 'NATIVE',
    description: '',
    tags: [],
    socialLinks: [],
    uploadRefs: [],
    firstBuyRaw: '2000000',
    opticalSelection: 'standard',
    updatedAt: Date.now(),
    feeStrategy: 0,
  };
}

// A committed in-memory peer implements the USDL ledger and JSON-RPC boundary.
// The component under repair is always the production adapter/controller; all
// requests and receipt calldata pass through real viem encoders and decoders.
class QuoteLedger {
  balances = new Map<string, bigint>([
    [owner, 20_000_000n],
    [treasury, 0n],
  ]);
  allowances = new Map<string, bigint>();
  credits = 0n;
  allowance(account: string, spender: string) {
    return this.allowances.get(`${address(account)}:${address(spender)}`) ?? 0n;
  }
  approve(account: string, spender: string, amount: bigint) {
    this.allowances.set(`${address(account)}:${address(spender)}`, amount);
  }
  pull(account: string, amount: bigint, commit: boolean) {
    const allowed = this.allowance(account, native),
      balance = this.balances.get(address(account)) ?? 0n;
    if (allowed < amount || balance < amount)
      throw new Error(
        'USDL transferFrom: insufficient native spender allowance or balance',
      );
    if (commit) {
      this.approve(account, native, allowed - amount);
      this.balances.set(address(account), balance - amount);
      this.credits += amount;
    }
  }
}
interface WireTransaction {
  from: Address;
  to: Address;
  data?: Hex;
  input?: Hex;
  value?: Hex;
}
type WireRecord = Record<string, unknown>;
class CreationWirePeer {
  interruptBuyReply = false;
  readonly ledger = new QuoteLedger();
  readonly transactions = new Map<Hex, WireRecord>();
  readonly receipts = new Map<Hex, WireRecord>();
  readonly stages: {
    stage: CreationAdmissionStage;
    height: bigint;
    allowance: bigint;
  }[] = [];
  readonly allowanceReads: string[] = [];
  height = 1n;
  registered = false;
  allowed = true;
  stale = false;
  fee = 1_000_000n;
  readonly blocks = new Map<bigint, WireRecord>();
  constructor() {
    this.blocks.set(1n, this.block(1n));
  }
  block(number: bigint): WireRecord {
    return {
      number: toHex(number),
      hash: h(Number(number)),
      parentHash: h(Number(number - 1n)),
      timestamp: toHex(Math.floor(Date.now() / 1000)),
      nonce: '0x0000000000000000',
      sha3Uncles: h(0),
      logsBloom: `0x${'0'.repeat(512)}`,
      transactionsRoot: h(10),
      stateRoot: h(11),
      receiptsRoot: h(12),
      miner: a(100),
      difficulty: '0x0',
      totalDifficulty: '0x0',
      extraData: '0x',
      size: '0x100',
      gasLimit: '0x1c9c380',
      gasUsed: '0x186a0',
      baseFeePerGas: '0x1',
      transactions: [],
      uncles: [],
    };
  }
  execute(tx: WireTransaction, commit = false): Hex {
    const input = tx.data ?? tx.input;
    if (!input) throw new Error('Missing transaction input');
    const decoded = decodeFunctionData({ abi, data: input });
    const args = decoded.args ?? [];
    let result: unknown;
    switch (decoded.functionName) {
      case 'decimals':
        result = 6;
        break;
      case 'balanceOf':
        result = this.ledger.balances.get(address(String(args[0]))) ?? 0n;
        break;
      case 'allowance':
        this.allowanceReads.push(address(String(args[1])));
        result = this.ledger.allowance(String(args[0]), String(args[1]));
        break;
      case 'approve':
        if (address(tx.to) !== deployment.usdl)
          throw new Error('Wrong USDL destination');
        if (commit)
          this.ledger.approve(
            tx.from,
            String(args[0]),
            BigInt(String(args[1])),
          );
        result = true;
        break;
      case 'transfer': {
        const amount = BigInt(String(args[1])),
          balance = this.ledger.balances.get(address(tx.from)) ?? 0n;
        if (balance < amount) throw new Error('Insufficient USDL balance');
        if (commit) {
          this.ledger.balances.set(address(tx.from), balance - amount);
          this.ledger.balances.set(
            address(String(args[0])),
            (this.ledger.balances.get(address(String(args[0]))) ?? 0n) + amount,
          );
        }
        result = true;
        break;
      }
      case 'factory':
        result = deployment.factory;
        break;
      case 'poolRegistry':
        result = deployment.poolRegistry;
        break;
      case 'protocolConfig':
        result = deployment.protocolConfig;
        break;
      case 'usdlAddress':
        result = deployment.usdl;
        break;
      case 'treasury':
        result = treasury;
        break;
      case 'creationFee':
        result = this.fee;
        break;
      case 'createMarket':
        if (address(tx.to) !== deployment.router)
          throw new Error('Wrong Router destination');
        this.ledger.pull(tx.from, this.fee, commit);
        if (commit) this.registered = true;
        result = [token, pool, 1n];
        break;
      case 'buy': {
        if (
          !this.registered ||
          address(String(args[0])) !== pool ||
          address(tx.to) !== deployment.router
        )
          throw new Error('Unknown purchase route');
        const amount = BigInt(String(args[1]));
        if (
          BigInt(String(args[2])) > amount * 10n ||
          BigInt(String(args[3])) <= BigInt(Math.floor(Date.now() / 1000))
        )
          throw new Error('Unsafe purchase');
        this.ledger.pull(tx.from, amount, commit);
        result = amount * 10n;
        break;
      }
      case 'isRegisteredPool':
        result = this.registered && address(String(args[0])) === pool;
        break;
      case 'getPoolByToken':
        if (!this.registered || address(String(args[0])) !== token)
          throw new Error('Unregistered token');
        result = pool;
        break;
      case 'getPoolsByCreatorPage':
        result =
          this.registered && address(String(args[0])) === owner ? [pool] : [];
        break;
      case 'getPoolMetadata':
        if (!this.registered || address(String(args[0])) !== pool)
          throw new Error('Unregistered pool');
        result = {
          creator: owner,
          token,
          optical: zeroAddress,
          nftId: 1n,
          createdAt: 1n,
          createdBlock: 1n,
        };
        break;
      case 'tokenAddress':
        result = token;
        break;
      case 'opticalAddress':
        result = zeroAddress;
        break;
      case 'quoteExactInput':
        if (!this.registered) throw new Error('Unregistered pool');
        result = {
          amountOut: BigInt(String(args[1])) * 10n,
          feeAmount: 0n,
          priceImpactBps: 1n,
        };
        break;
      default:
        throw new Error('Unsupported ledger operation');
    }
    return encodeFunctionResult({
      abi,
      functionName: decoded.functionName,
      result,
    } as Parameters<typeof encodeFunctionResult>[0]);
  }
  send(tx: WireTransaction): Hex {
    this.execute(tx, true);
    this.height++;
    const block = this.block(this.height);
    this.blocks.set(this.height, block);
    const input = tx.data ?? tx.input!,
      hash = keccak256(
        stringToHex(JSON.stringify([tx, this.height.toString()])),
      );
    const index = toHex(this.transactions.size);
    const record = {
      hash,
      from: tx.from,
      to: tx.to,
      input,
      value: '0x0',
      chainId: '0x7d',
      nonce: index,
      gas: '0x186a0',
      gasPrice: '0x1',
      type: '0x0',
      v: '0x11d',
      r: h(1),
      s: h(1),
      blockHash: block.hash,
      blockNumber: block.number,
      transactionIndex: '0x0',
    };
    this.transactions.set(hash, record);
    const decoded = decodeFunctionData({ abi, data: input });
    const logs: WireRecord[] = [];
    const log = (topics: readonly Hex[], data: Hex) =>
      logs.push({
        address:
          decoded.functionName === 'buy' ? trade.emitter : deployment.factory,
        topics,
        data,
        blockNumber: block.number,
        blockHash: block.hash,
        transactionHash: hash,
        transactionIndex: '0x0',
        logIndex: '0x0',
        removed: false,
      });
    if (decoded.functionName === 'createMarket')
      log(
        exactTopics(
          encodeEventTopics({
            abi,
            eventName: 'MarketCreated',
            args: { token, pool, creator: owner },
          }),
        ),
        encodeAbiParameters(
          [{ type: 'uint256' }, { type: 'address' }],
          [1n, zeroAddress],
        ),
      );
    if (decoded.functionName === 'buy') {
      const amount = decoded.args[1];
      log(
        exactTopics(
          encodeEventTopics({
            abi,
            eventName: 'Swap',
            args: {
              poolId: `0x${pool.slice(2).padStart(64, '0')}`,
              sender: tx.from,
            },
          }),
        ),
        encodeAbiParameters(
          [
            { type: 'bool' },
            ...Array.from({ length: 6 }, () => ({ type: 'uint256' }) as const),
          ],
          [
            true,
            amount,
            amount * 10n,
            0n,
            1n,
            BigInt(Math.floor(Date.now() / 1000)),
            this.height,
          ],
        ),
      );
    }
    this.receipts.set(hash, {
      transactionHash: hash,
      transactionIndex: '0x0',
      blockHash: block.hash,
      blockNumber: block.number,
      from: tx.from,
      to: tx.to,
      cumulativeGasUsed: '0x186a0',
      gasUsed: '0x186a0',
      contractAddress: null,
      logs,
      logsBloom: `0x${'0'.repeat(512)}`,
      status: '0x1',
      effectiveGasPrice: '0x1',
      type: '0x0',
    });
    return hash;
  }
  rpc(method: string, params: unknown[]): unknown {
    switch (method) {
      case 'eth_chainId':
        return '0x7d';
      case 'eth_accounts':
        return [owner];
      case 'eth_blockNumber':
        return toHex(this.height);
      case 'eth_getBlockByNumber':
        return this.blocks.get(
          params[0] === 'latest' ? this.height : BigInt(String(params[0])),
        );
      case 'eth_getBlockByHash':
        return [...this.blocks.values()].find(
          (block) => block.hash === params[0],
        );
      case 'eth_getCode':
        return code;
      case 'eth_getBalance':
        return '0xde0b6b3a7640000';
      case 'eth_gasPrice':
        return '0x1';
      case 'eth_call':
        return this.execute(params[0] as WireTransaction);
      case 'eth_estimateGas':
        this.execute(params[0] as WireTransaction);
        return '0x186a0';
      case 'eth_sendTransaction':
        return this.send(params[0] as WireTransaction);
      case 'eth_getTransactionReceipt':
        return this.receipts.get(params[0] as Hex) ?? null;
      case 'eth_getTransactionByHash':
        return this.transactions.get(params[0] as Hex) ?? null;
      default:
        throw new Error(`Unexpected JSON-RPC method: ${method}`);
    }
  }
  async serve(req: IncomingMessage, res: ServerResponse) {
    const chunks: Buffer[] = [];
    for await (const chunk of req) chunks.push(Buffer.from(chunk));
    const text = Buffer.concat(chunks).toString();
    res.setHeader('Content-Type', 'application/json');
    if (req.url === '/rpc') {
      const request = JSON.parse(text) as {
        id: number;
        method: string;
        params: unknown[];
      };
      try {
        const result = this.rpc(request.method, request.params);
        if (
          this.interruptBuyReply &&
          request.method === 'eth_sendTransaction' &&
          decodeFunctionData({
            abi,
            data: (request.params[0] as WireTransaction).data!,
          }).functionName === 'buy'
        ) {
          this.interruptBuyReply = false;
          res.destroy();
          return;
        }
        res.end(JSON.stringify({ jsonrpc: '2.0', id: request.id, result }));
      } catch (error) {
        res.end(
          JSON.stringify({
            jsonrpc: '2.0',
            id: request.id,
            error: {
              code: -32000,
              message: error instanceof Error ? error.message : String(error),
            },
          }),
        );
      }
      return;
    }
    if (req.url === '/v2/control/caps/pool_creation') {
      res.end(
        JSON.stringify({
          data: {
            capability: 'pool_creation',
            allowed: this.allowed,
            stale: this.stale,
            reason: this.allowed ? 'available' : 'operator_disabled',
            revision: '1',
            observedAt: new Date().toISOString(),
          },
        }),
      );
      return;
    }
    if (req.url === '/v2/preflight/create') {
      const request = JSON.parse(text) as Record<string, unknown>;
      const stage = request.creationStage as CreationAdmissionStage;
      if (
        request.expectedRevision !== revision.revisionSha256 ||
        request.family !== 'creation' ||
        request.trader !== owner ||
        request.recipient !== owner ||
        request.blockHash !== h(Number(this.height)) ||
        !['creation', 'approval'].includes(stage)
      )
        throw new Error('Unbound fixture preflight request');
      this.stages.push({
        stage,
        height: this.height,
        allowance: this.ledger.allowance(owner, native),
      });
      const data =
        stage === 'approval'
          ? encodeFunctionData({
              abi,
              functionName: 'approve',
              args: [native, this.fee],
            })
          : encodeFunctionData({
              abi,
              functionName: 'createMarket',
              args: [
                String(request.name),
                String(request.symbol),
                Number(request.feeStrategy),
                zeroAddress,
              ],
            });
      let supported = true;
      try {
        this.execute({
          from: owner,
          to: (stage === 'approval'
            ? deployment.usdl
            : deployment.router) as Address,
          data,
        });
      } catch {
        supported = false;
      }
      res.end(
        JSON.stringify({
          schemaVersion: 2,
          revision,
          family: 'creation',
          creationStage: stage,
          blockHash: request.blockHash,
          blockNumber: Number(this.height),
          trader: owner,
          recipient: owner,
          supported,
          executionPermitted: supported && this.allowed,
          simulationMode:
            stage === 'approval' ? 'usdl_approval' : 'create_market',
          requestSha256: createHash('sha256').update(text).digest('hex'),
          creationCost: {
            protocolFeeAsset: 'USDL',
            protocolFeeRaw: this.fee.toString(),
            gasAsset: 'PAX',
            gasEstimateAvailable: supported,
            blockHash: request.blockHash,
            spender: native,
          },
          ...(!supported ? { reasonCode: 'gas_estimate_unavailable' } : {}),
        }),
      );
      return;
    }
    res.statusCode = 404;
    res.end('{}');
  }
}

export class MemoryCreationStore
  implements Pick<CreationStorage, keyof CreationStorage>
{
  constructor(
    private creator: string,
    readonly database = {
      rows: new Map<string, LaunchAttempt>(),
      saved: undefined as CreationDraftV2 | undefined,
      locked: false,
      failPurchaseHashWrites: 0,
      failPurchaseConfirmationWrites: 0,
      writes: [] as LaunchAttempt[],
    },
  ) {}
  saveDraft(value: CreationDraftV2) {
    if (
      address(value.creator) !== this.creator ||
      Object.keys(validateCreationDraft(value)).length
    )
      throw new Error('Invalid draft');
    this.database.saved = structuredClone(value);
  }
  restoreDraft() {
    return this.database.saved ? structuredClone(this.database.saved) : null;
  }
  async history() {
    return [...this.database.rows.values()].map((value) =>
      structuredClone(checkedAttempt(value, this.creator)),
    );
  }
  async put(value: LaunchAttempt) {
    checkedAttempt(value, this.creator);
    if (
      value.purchaseTxHash &&
      value.purchaseState === 'submitted' &&
      this.database.failPurchaseHashWrites > 0
    ) {
      this.database.failPurchaseHashWrites--;
      throw new Error('Purchase hash storage interrupted');
    }
    if (
      value.purchaseState === 'confirmed' &&
      this.database.failPurchaseConfirmationWrites > 0
    ) {
      this.database.failPurchaseConfirmationWrites--;
      throw new Error('Purchase reconciliation storage interrupted');
    }
    const prior = this.database.rows.get(value.intentId);
    if (prior?.purchaseTxHash && prior.purchaseTxHash !== value.purchaseTxHash)
      throw new Error('Purchase hash cannot change');
    this.database.rows.set(value.intentId, structuredClone(value));
    this.database.writes.push(structuredClone(value));
  }
  async admit(value: LaunchAttempt) {
    checkedAttempt(value, this.creator, true);
    if (
      [...this.database.rows.values()].some(
        (prior) =>
          prior.intentId === value.intentId ||
          (prior.draftId === value.draftId && !prior.deliberateNew),
      )
    )
      throw new Error('Recover existing intent');
    await this.put(value);
  }
  async releaseUnsent(value: LaunchAttempt) {
    const prior = this.database.rows.get(value.intentId);
    if (!prior || prior.state !== 'signing_locked' || prior.createTxHash)
      throw new Error('Submitted intent');
    this.database.rows.delete(value.intentId);
  }
  async exclusively<T>(action: () => Promise<T>) {
    if (this.database.locked) throw new Error('Concurrent launch');
    this.database.locked = true;
    try {
      return await action();
    } finally {
      this.database.locked = false;
    }
  }
}

export async function creationWireFixture() {
  const peer = new CreationWirePeer();
  const server = createServer((req, res) => {
    void peer.serve(req, res).catch((error) => {
      res.statusCode = 500;
      res.end(JSON.stringify({ error: String(error) }));
    });
  });
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const socket = server.address();
  if (!socket || typeof socket === 'string')
    throw new Error('No loopback address');
  const origin = `http://127.0.0.1:${socket.port}`;
  const transport = http(`${origin}/rpc`, { retryCount: 0 });
  const client = createPublicClient({
      chain,
      transport,
      cacheTime: 0,
      pollingInterval: 10,
    }),
    wallet = createWalletClient({ account: owner, chain, transport });
  const originalFetch = globalThis.fetch;
  // Node supplies URL resolution here; every request goes through real HTTP.
  globalThis.fetch = (input, init) =>
    originalFetch(
      typeof input === 'string' && input.startsWith('/')
        ? new URL(input, origin)
        : input,
      init,
    );
  const authority: Pick<
    RuntimeAuthority,
    'revision' | 'expiresAt' | 'network'
  > = {
    revision,
    expiresAt: Date.now() + 60000,
    network: {
      usdlRuntimeHash: keccak256(code),
      network: {
        schemaVersion: 1,
        deploymentVersion: revision.deploymentVersion,
        manifestHash: revision.manifestSha256,
        sourceManifestSha256: revision.manifestSha256,
        chainId: 125,
        publicRPC: `${origin}/rpc`,
        explorerBase: origin,
        native: { symbol: 'PAX', decimals: 18 },
        settlement: { symbol: 'USDL', decimals: 6 },
        creationFeeRaw: '1000000',
        addresses: {
          Router: deployment.router,
          SidioraFactory: deployment.factory,
          USDL: deployment.usdl,
          ProtocolConfig: deployment.protocolConfig,
          PoolRegistry: deployment.poolRegistry,
          LaunchpadPrecompile: native,
        },
      },
      async committedHead(signal) {
        signal.throwIfAborted();
        const block = await client.getBlock();
        if (!block.hash || block.number === null)
          throw new Error('Missing committed block');
        return { number: block.number, hash: block.hash };
      },
    },
  };
  const scope = { creator: owner, chainId: 125, epoch: 1 };
  const adapter = new ChainCreationAdapter(
    client as PublicClient,
    wallet as WalletClient,
    deployment,
    () => scope,
    runtimeCreationAdmission(authority, deployment),
  );
  const memoryStorage = new MemoryCreationStore(owner);
  const storage: Pick<CreationStorage, keyof CreationStorage> = memoryStorage;
  const controller = new CreationController(
    adapter,
    storage as CreationStorage,
    async (message) => wallet.signMessage({ account: owner, message }),
  );
  return {
    peer,
    client,
    wallet,
    authority,
    scope,
    adapter,
    storage,
    memoryStorage,
    controller,
    async close() {
      controller.dispose();
      globalThis.fetch = originalFetch;
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) =>
        server.close((error) => (error ? reject(error) : resolve())),
      );
    },
  };
}

export {
  a,
  h,
  owner,
  native,
  pool,
  token,
  deployment,
  abi,
  chain,
  revision,
  trade,
};
