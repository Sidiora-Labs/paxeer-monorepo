import { ethers } from 'ethers';
import { executeSwap, SwapRevertedError } from '@/lib/swap/executor';
import { CONTRACTS, PECOR_ABI, PECOR_ROUTER_ABI, SIDIORA_ROUTER_ABI } from '@/lib/swap/sdk';
import type { SwapParams, SwapQuote } from '@/lib/swap/types';

export type SwapProgress = 'pending' | 'partial' | 'confirmed' | 'reverted' | 'failed';

export interface JournalTransaction {
  kind: 'approval' | 'swap';
  leg: number;
  nonce: number;
  to: string;
  data: string;
  value: string;
  startBlock: number;
  scannedThrough: number;
  hash: string | null;
  receipt: {
    status: number;
    blockNumber: number;
    blockHash: string;
    logs: { address: string; topics: string[]; data: string }[];
  } | null;
}

export interface SwapIntent {
  id: string;
  account: string;
  chainId: string;
  quote: SwapQuote;
  params: SwapParams;
  expectedLegs: number;
  transactions: JournalTransaction[];
  error: string;
  acknowledged: boolean;
}

interface JournalData {
  version: 1;
  intents: SwapIntent[];
}

const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value);
const isUint = (value: unknown): value is number =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
const isHex = (value: unknown): value is string =>
  typeof value === 'string' && ethers.isHexString(value);
const isHash = (value: unknown): value is string =>
  typeof value === 'string' && ethers.isHexString(value, 32);
const isAddress = (value: unknown): value is string =>
  typeof value === 'string' && ethers.isAddress(value);
const isAmount = (value: unknown): value is string =>
  typeof value === 'string' && /^\d+$/.test(value);

function validToken(value: unknown): boolean {
  return (
    isObject(value) &&
    typeof value.symbol === 'string' &&
    typeof value.name === 'string' &&
    isUint(value.decimals) &&
    (isAddress(value.address) || (value.address === '' && value.isNative === true))
  );
}

function validIntent(value: unknown, account: string, chainId: string): value is SwapIntent {
  if (
    !isObject(value) ||
    typeof value.id !== 'string' ||
    !value.id ||
    value.account !== account ||
    value.chainId !== chainId ||
    typeof value.error !== 'string' ||
    typeof value.acknowledged !== 'boolean' ||
    !isObject(value.quote) ||
    !isObject(value.params) ||
    !Array.isArray(value.transactions)
  )
    return false;
  const quote = value.quote;
  const params = value.params;
  if (
    !['pecor', 'sidiora', 'sidiora-multihop', 'pecor-v4-multihop'].includes(
      String(quote.protocol)
    ) ||
    (value.expectedLegs !== 1 &&
      !(quote.protocol === 'pecor-v4-multihop' && value.expectedLegs === 2)) ||
    !validToken(quote.tokenIn) ||
    !validToken(quote.tokenOut) ||
    !isAmount(quote.amountIn) ||
    !isAmount(quote.amountOut) ||
    typeof quote.amountOutDisplay !== 'string' ||
    !isUint(params.slippageBps) ||
    !isAddress(params.recipient) ||
    params.recipient.toLowerCase() !== account ||
    params.amountIn !== quote.amountIn ||
    !validToken(params.tokenIn) ||
    !validToken(params.tokenOut)
  )
    return false;
  return value.transactions.every((tx: unknown) => {
    if (
      !isObject(tx) ||
      !['approval', 'swap'].includes(String(tx.kind)) ||
      !isUint(tx.leg) ||
      !isUint(tx.nonce) ||
      !isAddress(tx.to) ||
      !isHex(tx.data) ||
      !isAmount(tx.value) ||
      !isUint(tx.startBlock) ||
      !isUint(tx.scannedThrough) ||
      (tx.hash !== null && !isHash(tx.hash))
    )
      return false;
    if (tx.receipt === null) return true;
    const receipt = tx.receipt;
    return (
      isObject(receipt) &&
      (receipt.status === 0 || receipt.status === 1) &&
      isUint(receipt.blockNumber) &&
      isHash(receipt.blockHash) &&
      Array.isArray(receipt.logs) &&
      receipt.logs.every(
        (log: unknown) =>
          isObject(log) &&
          isAddress(log.address) &&
          isHex(log.data) &&
          Array.isArray(log.topics) &&
          log.topics.every(isHash)
      )
    );
  });
}

export function swapProgress(intent: SwapIntent): SwapProgress {
  const swaps = intent.transactions.filter((tx) => tx.kind === 'swap');
  if (intent.transactions.some((tx) => !tx.receipt)) return 'pending';
  if (swaps.filter((tx) => tx.receipt?.status === 1).length === intent.expectedLegs)
    return 'confirmed';
  if (swaps.some((tx) => tx.receipt?.status === 1)) return 'partial';
  if (intent.transactions.some((tx) => tx.receipt?.status === 0)) return 'reverted';
  return 'failed';
}

/** One atomic storage write retains the account/chain's entire intent history. */
export class SwapJournal {
  readonly key: string;
  readonly account: string;
  readonly chainId: string;

  constructor(
    private readonly storage: Storage,
    account: string,
    chainId: bigint | number
  ) {
    this.account = ethers.getAddress(account).toLowerCase();
    this.chainId = BigInt(chainId).toString();
    this.key = `paxeer:swap-journal:v1:${this.chainId}:${this.account}`;
  }

  private read(): JournalData {
    const raw = this.storage.getItem(this.key);
    if (raw === null) return { version: 1, intents: [] };
    const data: unknown = JSON.parse(raw);
    if (
      !isObject(data) ||
      data.version !== 1 ||
      !Array.isArray(data.intents) ||
      !data.intents.every((intent: unknown) => validIntent(intent, this.account, this.chainId))
    ) {
      throw new Error('Swap recovery journal is invalid. Preserve it before continuing.');
    }
    return data as unknown as JournalData;
  }

  current(): SwapIntent | null {
    return this.read().intents.findLast((intent) => !intent.acknowledged) ?? null;
  }

  save(intent: SwapIntent): void {
    const data = this.read();
    const index = data.intents.findIndex((entry) => entry.id === intent.id);
    if (index === -1) data.intents.push(intent);
    else data.intents[index] = intent;
    const encoded = JSON.stringify(data);
    this.storage.setItem(this.key, encoded);
    if (this.storage.getItem(this.key) !== encoded)
      throw new Error('Swap journal could not be saved.');
  }

  begin(quote: SwapQuote, params: SwapParams): SwapIntent {
    if (this.current()) throw new Error('Recover the previous swap before approving a new intent.');
    const intent: SwapIntent = {
      id: crypto.randomUUID(),
      account: this.account,
      chainId: this.chainId,
      quote: structuredClone(quote),
      params: structuredClone(params),
      expectedLegs: 1,
      transactions: [],
      error: '',
      acknowledged: false,
    };
    this.save(intent);
    return intent;
  }

  acknowledge(): void {
    const intent = this.current();
    if (!intent) return;
    if (['pending', 'partial'].includes(swapProgress(intent))) {
      throw new Error('This swap still needs recovery; its identity cannot be cleared.');
    }
    intent.acknowledged = true;
    this.save(intent);
  }
}

const pecor = new ethers.Interface(PECOR_ABI);
const sidiora = new ethers.Interface(SIDIORA_ROUTER_ABI);
const atomicRouter = new ethers.Interface(PECOR_ROUTER_ABI);

function transactionKind(to: string, data: string): JournalTransaction['kind'] {
  if (data.slice(0, 10) === ethers.id('approve(address,uint256)').slice(0, 10)) return 'approval';
  const abi =
    to === CONTRACTS.PECOR.toLowerCase()
      ? pecor
      : to === CONTRACTS.Router.toLowerCase()
        ? sidiora
        : to === CONTRACTS.PECORRouter.toLowerCase()
          ? atomicRouter
          : null;
  const name = abi?.parseTransaction({ data })?.name;
  if (
    !name ||
    ![
      'swapExactInNative',
      'swapExactInToNative',
      'swapExactIn',
      'buy',
      'sell',
      'swapTokenForToken',
      'swapMultiHop',
    ].includes(name)
  ) {
    throw new Error('Unrecognized swap transaction; no transaction was sent.');
  }
  return 'swap';
}

function matches(
  tx: ethers.TransactionResponse,
  saved: JournalTransaction,
  journal: SwapJournal
): boolean {
  return (
    tx.from.toLowerCase() === journal.account &&
    tx.chainId.toString() === journal.chainId &&
    tx.nonce === saved.nonce &&
    tx.to?.toLowerCase() === saved.to &&
    tx.data.toLowerCase() === saved.data.toLowerCase() &&
    tx.value.toString() === saved.value
  );
}

function completed(tx: JournalTransaction, receipt: ethers.TransactionReceipt): void {
  if (receipt.status !== 0 && receipt.status !== 1)
    throw new Error('Swap receipt has no execution status.');
  tx.receipt = {
    status: receipt.status,
    blockNumber: receipt.blockNumber,
    blockHash: receipt.blockHash,
    logs: receipt.logs.map((log) => ({
      address: log.address,
      topics: [...log.topics],
      data: log.data,
    })),
  };
}

/** The executor still constructs all calls; this boundary journals and observes every send. */
export class JournaledSwapExecution {
  private readonly abort = new AbortController();

  constructor(
    readonly journal: SwapJournal,
    private readonly changed: (intent: SwapIntent | null) => void,
    private readonly isCurrent: () => boolean,
    private readonly receiptTimeoutMs = 60_000
  ) {}

  close(): void {
    this.abort.abort();
  }

  private check(): void {
    if (this.abort.signal.aborted || !this.isCurrent())
      throw new Error('Swap observation stopped. Recover the original intent.');
  }

  private publish(intent: SwapIntent): void {
    this.journal.save(intent);
    if (!this.abort.signal.aborted && this.isCurrent()) this.changed(structuredClone(intent));
  }

  private async bound(signer: ethers.Signer): Promise<ethers.Provider> {
    this.check();
    if (!signer.provider) throw new Error('Swap recovery requires a chain provider.');
    const [account, network] = await Promise.all([
      signer.getAddress(),
      signer.provider.getNetwork(),
    ]);
    this.check();
    if (
      account.toLowerCase() !== this.journal.account ||
      network.chainId.toString() !== this.journal.chainId
    ) {
      throw new Error(
        'Swap account or chain changed. Reopen recovery for the original account and chain.'
      );
    }
    return signer.provider;
  }

  private async locked<T>(action: () => Promise<T>): Promise<T> {
    if (!globalThis.navigator?.locks)
      throw new Error('Swap execution requires browser transaction locking.');
    return navigator.locks.request(this.journal.key, { ifAvailable: true }, async (lock) => {
      if (!lock) throw new Error('This swap is already being observed in another window.');
      this.check();
      return action();
    });
  }

  private async observe(
    provider: ethers.Provider,
    intent: SwapIntent,
    tx: JournalTransaction
  ): Promise<void> {
    if (!tx.hash) {
      // A signer can lose its response after accepting a write. Reconcile the reserved nonce,
      // including pending transactions; never infer "not sent" from an absent hash.
      const latest = await provider.getBlockNumber();
      const tags: (number | string)[] = ['pending'];
      for (
        let block = Math.max(tx.startBlock, tx.scannedThrough + 1);
        block <= Math.min(latest, tx.scannedThrough + 128);
        block++
      )
        tags.push(block);
      for (const tag of tags) {
        this.check();
        const block = await provider.getBlock(tag, true);
        if (!block) throw new Error('Swap nonce reconciliation could not read a block.');
        for (const hash of block.transactions) {
          const candidate = await provider.getTransaction(hash);
          if (
            !candidate ||
            candidate.from.toLowerCase() !== this.journal.account ||
            candidate.nonce !== tx.nonce
          )
            continue;
          if (!matches(candidate, tx, this.journal))
            throw new Error(
              'The original swap nonce was used by a different transaction. Manual reconciliation is required.'
            );
          tx.hash = candidate.hash;
          this.publish(intent);
          break;
        }
        if (typeof tag === 'number') tx.scannedThrough = tag;
        if (tx.hash) break;
      }
      this.publish(intent);
    }
    if (!tx.hash) return;
    const transaction = await provider.getTransaction(tx.hash);
    if (transaction && !matches(transaction, tx, this.journal))
      throw new Error('Recovered transaction does not match the original swap identity.');
    const receipt = await provider.getTransactionReceipt(tx.hash);
    this.check();
    if (receipt) {
      if (!transaction)
        throw new Error('Swap receipt is visible without its transaction identity.');
      completed(tx, receipt);
      this.publish(intent);
    } else if (tx.receipt) {
      tx.receipt = null;
      this.publish(intent);
    }
  }

  async recover(signer: ethers.Signer): Promise<void> {
    await this.locked(async () => {
      const provider = await this.bound(signer);
      const intent = this.journal.current();
      if (!intent) {
        this.changed(null);
        return;
      }
      try {
        for (const tx of intent.transactions) await this.observe(provider, intent, tx);
        if (swapProgress(intent) === 'confirmed') intent.error = '';
        this.publish(intent);
      } catch (error) {
        intent.error = error instanceof Error ? error.message : 'Swap recovery failed';
        this.publish(intent);
        throw error;
      }
    });
  }

  async execute(signer: ethers.Signer, quote: SwapQuote, params: SwapParams): Promise<void> {
    await this.locked(async () => {
      const provider = await this.bound(signer);
      if (params.recipient.toLowerCase() !== this.journal.account)
        throw new Error('Swap recipient must be the connected account.');
      const intent = this.journal.begin(quote, params);
      this.changed(intent);
      const boundary = new Proxy(signer, {
        get: (target, property) => {
          if (property === 'sendTransaction')
            return async (request: ethers.TransactionRequest) => {
              await this.bound(signer);
              const to = (await ethers.resolveAddress(request.to!, provider)).toLowerCase();
              const data = ethers.hexlify(request.data ?? '0x');
              const nonce =
                request.nonce ??
                (await provider.getTransactionCount(this.journal.account, 'pending'));
              const startBlock = await provider.getBlockNumber();
              const tx: JournalTransaction = {
                kind: transactionKind(to, data),
                leg: intent.transactions.filter((entry) => entry.kind === 'swap').length,
                nonce,
                to,
                data,
                value: ethers.getBigInt(request.value ?? 0).toString(),
                startBlock,
                scannedThrough: Math.max(0, startBlock - 1),
                hash: null,
                receipt: null,
              };
              this.check();
              intent.transactions.push(tx);
              this.publish(intent);
              let response: ethers.TransactionResponse;
              try {
                response = await signer.sendTransaction({
                  ...request,
                  to,
                  data,
                  nonce,
                  from: this.journal.account,
                  chainId: BigInt(this.journal.chainId),
                });
              } catch (error) {
                if (isObject(error) && isHash(error.hash)) {
                  tx.hash = error.hash;
                  this.publish(intent);
                }
                throw error;
              }
              tx.hash = response.hash;
              this.publish(intent); // Must precede every receipt wait, including SDK-internal waits.
              if (!matches(response, tx, this.journal))
                throw new Error(
                  'Signer returned a different swap identity. Manual reconciliation is required.'
                );
              const expires = Date.now() + this.receiptTimeoutMs;
              while (!tx.receipt) {
                this.check();
                await this.observe(provider, intent, tx);
                if (tx.receipt) break;
                if (Date.now() >= expires)
                  throw new Error(
                    'Swap receipt timeout: waiting timed out. Check the original transaction status.'
                  );
                await new Promise<void>((resolve) => {
                  const done = () => {
                    clearTimeout(timer);
                    this.abort.signal.removeEventListener('abort', done);
                    resolve();
                  };
                  const timer = setTimeout(done, 500);
                  this.abort.signal.addEventListener('abort', done, { once: true });
                  if (this.abort.signal.aborted) done();
                });
              }
              this.check();
              if (tx.receipt.status === 0) {
                const receipt = await provider.getTransactionReceipt(response.hash);
                if (!receipt) throw new Error('Reverted swap receipt disappeared.');
                throw new SwapRevertedError(receipt);
              }
              return response;
            };
          const value: unknown = Reflect.get(target, property, target);
          return typeof value === 'function' ? value.bind(target) : value;
        },
      });
      try {
        await executeSwap(boundary, quote, params);
        this.check();
        intent.error = '';
        this.publish(intent);
      } catch (error) {
        intent.error = error instanceof Error ? error.message : 'Swap execution interrupted';
        this.publish(intent);
        throw error;
      }
    });
  }
}
