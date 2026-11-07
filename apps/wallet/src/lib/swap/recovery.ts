import { ethers } from 'ethers';
import { TOKENS, CONTRACTS, PECOR_ABI, SIDIORA_ROUTER_ABI } from './sdk';
import { toCentralSwapAddress } from './constants';
import type { SwapParams, SwapQuote } from './types';
import { assertQuoteNetwork, getSwapNetwork, type QuoteNetwork } from './network';
import { swapStorage, writeSwapRecord, type SwapStorage } from './storage';

export interface SwapOperation {
  version: 1;
  wallet: string;
  network: QuoteNetwork;
  quote: SwapQuote;
  params: SwapParams;
  minimumOut: string;
  expiresAt: number;
  state: 'prepared' | 'submitting' | 'submitted' | 'confirmed' | 'reverted' | 'bridge' | 'retained';
  hash?: string;
  firstLegHash?: string;
  receivedUsdl?: string;
  resumeAttempts: number;
  lastAttemptHash?: string;
}

const transfer = new ethers.Interface([
  'event Transfer(address indexed from, address indexed to, uint256 value)',
]);

function bridgeReceived(receipt: ethers.TransactionReceipt, wallet: string): bigint {
  let received = 0n;
  for (const log of receipt.logs) {
    if (log.address.toLowerCase() !== TOKENS.USDL.address.toLowerCase()) continue;
    const event = transfer.parseLog(log);
    if (!event || event.name !== 'Transfer') continue;
    if (event.args.to.toLowerCase() === wallet.toLowerCase()) received += event.args.value;
    if (event.args.from.toLowerCase() === wallet.toLowerCase()) received -= event.args.value;
  }
  if (received <= 0n) throw new Error('First leg did not deliver USDL to the wallet');
  return received;
}

export class SwapRecovery {
  private readonly key: string;
  constructor(
    readonly wallet: string,
    readonly network: QuoteNetwork,
    private readonly storage: SwapStorage = swapStorage()
  ) {
    if (!ethers.isAddress(wallet)) throw new Error('Invalid recovery wallet');
    this.key = `pax:swap:operation:v1:${network.chainId}:${encodeURIComponent(network.endpoint)}:${wallet.toLowerCase()}`;
  }

  read(): SwapOperation | null {
    const raw = this.storage.getItem(this.key);
    if (raw === null) return null;
    const record: SwapOperation = JSON.parse(raw);
    if (
      record.version !== 1 ||
      record.wallet.toLowerCase() !== this.wallet.toLowerCase() ||
      record.network.chainId !== this.network.chainId ||
      record.network.endpoint !== this.network.endpoint ||
      ![
        'prepared',
        'submitting',
        'submitted',
        'confirmed',
        'reverted',
        'bridge',
        'retained',
      ].includes(record.state) ||
      !/^\d+$/.test(record.minimumOut) ||
      BigInt(record.minimumOut) <= 0n ||
      !Number.isSafeInteger(record.expiresAt) ||
      !Number.isInteger(record.resumeAttempts) ||
      record.resumeAttempts < 0 ||
      record.resumeAttempts > 1 ||
      (record.hash !== undefined && !/^0x[0-9a-fA-F]{64}$/.test(record.hash)) ||
      (record.firstLegHash !== undefined && !/^0x[0-9a-fA-F]{64}$/.test(record.firstLegHash)) ||
      (record.receivedUsdl !== undefined && !/^\d+$/.test(record.receivedUsdl))
    ) {
      throw new Error('Invalid persisted swap operation');
    }
    return record;
  }

  private save(record: SwapOperation): SwapOperation {
    writeSwapRecord(this.storage, this.key, record);
    return record;
  }

  begin(quote: SwapQuote, params: SwapParams): SwapOperation {
    assertQuoteNetwork(this.network);
    if (
      !Number.isInteger(params.slippageBps) ||
      params.slippageBps < 0 ||
      params.slippageBps >= 10000
    ) {
      throw new Error('Invalid recovery slippage');
    }
    const previous = this.read();
    if (
      previous &&
      previous.state !== 'confirmed' &&
      previous.state !== 'reverted' &&
      previous.state !== 'retained'
    ) {
      throw new Error('Reconcile the existing swap before starting another');
    }
    const minimumOut = (
      (BigInt(quote.amountOut) * BigInt(10000 - params.slippageBps)) /
      10000n
    ).toString();
    if (BigInt(minimumOut) <= 0n) throw new Error('Swap requires a positive confirmed minimum');
    const expiry = params.deadline ?? Math.floor(this.network.expiresAt / 1000);
    if (!Number.isSafeInteger(expiry) || expiry <= Math.floor(Date.now() / 1000))
      throw new Error('Swap confirmation expired');
    return this.save({
      version: 1,
      wallet: this.wallet,
      network: { ...this.network },
      quote: JSON.parse(JSON.stringify(quote)),
      params: { ...params, deadline: expiry },
      minimumOut,
      expiresAt: Math.min(expiry * 1000, this.network.expiresAt),
      state: 'prepared',
      resumeAttempts: 0,
    });
  }

  submitting(): void {
    const record = this.requireRecord();
    if (record.state !== 'prepared' && record.state !== 'bridge')
      throw new Error('Swap is already submitted');
    if (record.state === 'bridge' && record.resumeAttempts >= 1)
      throw new Error('Bridge resume attempt is spent; retain bridge assets');
    this.save({
      ...record,
      state: 'submitting',
      resumeAttempts: record.state === 'bridge' ? record.resumeAttempts + 1 : record.resumeAttempts,
    });
  }

  submitted(hash: string): void {
    if (!/^0x[0-9a-fA-F]{64}$/.test(hash)) throw new Error('Invalid submitted swap hash');
    this.save({ ...this.requireRecord(), state: 'submitted', hash });
  }

  private requireRecord(): SwapOperation {
    const record = this.read();
    if (!record) throw new Error('No swap operation to recover');
    return record;
  }

  private async provider(): Promise<ethers.Provider> {
    const selected = getSwapNetwork();
    if (
      selected.chainId !== this.network.chainId ||
      selected.endpoint !== this.network.endpoint ||
      (await selected.provider.getNetwork()).chainId !== BigInt(this.network.chainId)
    ) {
      throw new Error('Recovery requires the original network');
    }
    return selected.provider;
  }

  async reconcile(): Promise<SwapOperation> {
    const provider = await this.provider();
    const record = this.requireRecord();
    if (record.hash) {
      const receipt = await provider.getTransactionReceipt(record.hash);
      if (!receipt) return record;
      if (receipt.from.toLowerCase() !== this.wallet.toLowerCase())
        throw new Error('Receipt wallet mismatch');
      if (receipt.status === 0 && record.firstLegHash) {
        return this.save({
          ...record,
          state: 'bridge',
          lastAttemptHash: record.hash,
          hash: undefined,
        });
      }
      return this.save({ ...record, state: receipt.status === 1 ? 'confirmed' : 'reverted' });
    }
    if (record.firstLegHash) {
      if (record.state === 'submitting') return record;
      const receipt = await provider.getTransactionReceipt(record.firstLegHash);
      if (!receipt) return record;
      if (receipt.status !== 1) return this.save({ ...record, state: 'reverted' });
      if (receipt.from.toLowerCase() !== this.wallet.toLowerCase())
        throw new Error('Receipt wallet mismatch');
      const receivedUsdl = bridgeReceived(receipt, this.wallet).toString();
      if (record.receivedUsdl !== undefined && record.receivedUsdl !== receivedUsdl)
        throw new Error('Bridge receipt amount changed');
      return this.save({
        ...record,
        state: record.state === 'retained' ? 'retained' : 'bridge',
        receivedUsdl,
      });
    }
    return record;
  }

  // For a previously confirmed sequential route. New mixed routes remain atomic.
  async recordBridgeLeg(firstLegHash: string): Promise<SwapOperation> {
    const record = this.requireRecord();
    if (record.state !== 'prepared') throw new Error('Bridge operation already recorded');
    if (!/^0x[0-9a-fA-F]{64}$/.test(firstLegHash)) throw new Error('Invalid first-leg hash');
    this.save({ ...record, firstLegHash });
    return this.reconcile();
  }

  async actions(): Promise<readonly ('resume' | 'retain-bridge-assets')[]> {
    const record = await this.reconcile();
    if (record.state !== 'bridge') return [];
    return record.expiresAt > Date.now() && record.resumeAttempts === 0
      ? ['resume', 'retain-bridge-assets']
      : ['retain-bridge-assets'];
  }

  async retainBridgeAssets(): Promise<SwapOperation> {
    const record = await this.reconcile();
    if (record.state !== 'bridge') throw new Error('No settled bridge assets to retain');
    return this.save({ ...record, state: 'retained' });
  }

  async resume(signer: ethers.Signer): Promise<ethers.TransactionReceipt> {
    const record = await this.reconcile();
    if (record.state !== 'bridge' || !record.receivedUsdl)
      throw new Error('No settled bridge leg to resume');
    if (record.expiresAt <= Date.now())
      throw new Error('Bridge confirmation expired; retain bridge assets');
    if (record.resumeAttempts >= 1)
      throw new Error('Bridge resume attempt is spent; retain bridge assets');
    if (
      (await signer.getAddress()).toLowerCase() !== this.wallet.toLowerCase() ||
      !signer.provider ||
      (await signer.provider.getNetwork()).chainId !== BigInt(this.network.chainId)
    )
      throw new Error('Recovery signer mismatch');
    const provider = await this.provider();
    if (signer.provider !== provider) throw new Error('Recovery signer endpoint mismatch');
    const token = new ethers.Contract(
      TOKENS.USDL.address,
      [
        'function balanceOf(address) view returns (uint256)',
        'function allowance(address,address) view returns (uint256)',
        'function approve(address,uint256) returns (bool)',
      ],
      signer
    );
    const amount = BigInt(record.receivedUsdl);
    if ((await token.balanceOf(this.wallet)) < amount)
      throw new Error('Settled bridge balance is no longer available');
    const { resolveTokenCapability } = await import('./network');
    const capability = await resolveTokenCapability(record.quote.tokenOut);
    if (capability.state !== 'launchpad' && capability.state !== 'pecor')
      throw new Error('Bridge continuation requires a supported output token');
    const spender = capability.state === 'launchpad' ? CONTRACTS.Router : CONTRACTS.PECORVault;
    if ((await token.allowance(this.wallet, spender)) < amount) {
      const approved = await (await token.approve(spender, amount)).wait();
      if (approved?.status !== 1) throw new Error('Bridge approval failed');
    }
    if (signer.provider !== (await this.provider()))
      throw new Error('Recovery signer endpoint mismatch');
    if (record.expiresAt <= Date.now())
      throw new Error('Bridge confirmation expired during approval');
    const expiry = BigInt(Math.floor(record.expiresAt / 1000));
    const minimum = BigInt(record.minimumOut);
    let tx: ethers.TransactionResponse;
    this.submitting();
    if (capability.state === 'launchpad') {
      const router = new ethers.Contract(CONTRACTS.Router, SIDIORA_ROUTER_ABI, signer);
      tx = await router.buy(capability.poolAddress, amount, minimum, expiry);
    } else {
      const pecor = new ethers.Contract(CONTRACTS.PECOR, PECOR_ABI, signer);
      tx = record.quote.tokenOut.isNative
        ? await pecor.swapExactInToNative(TOKENS.USDL.address, amount, minimum, expiry)
        : await pecor.swapExactIn(
            TOKENS.USDL.address,
            toCentralSwapAddress(record.quote.tokenOut),
            amount,
            minimum,
            expiry
          );
    }
    this.submitted(tx.hash);
    const receipt = await tx.wait();
    if (!receipt) throw new Error('Bridge continuation receipt is pending');
    await this.reconcile();
    if (receipt.status !== 1) throw new Error('Bridge continuation reverted');
    return receipt;
  }
}
