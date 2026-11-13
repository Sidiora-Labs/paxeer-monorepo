import { BaseError, ContractFunctionRevertedError, parseAbi, toEventSelector, type Address, type Hex, type PublicClient, type WalletClient } from 'viem';
import { address, hash } from '../create/creation-domain';
import type { AccountScope } from '../create/creation-adapters';
import { CHAIN_LAUNCH_ABI, LAUNCHPAD_ADDRESS } from '@/core/network/contracts';
import { poolPath } from '@/core/app-foundation/navigation';
import {
  CHAINS_PRECOMPILE, SIZE_TIERS, chainCost, decodeChainLaunched,
  validateChainDraft, validateChainName, walletNetwork,
  type ChainDraft, type ChainsParams,
} from './chain-domain';

export const chainsAbi = parseAbi([
  'function createChain(string name,uint64 gasAllowance,uint64 byteAllowance,uint16 feeShareBps) payable returns (uint32 id)',
  'function fundChain(uint32 id) payable',
  'function pauseChain(uint32 id)',
  'function resumeChain(uint32 id)',
  'function retireChain(uint32 id)',
  'function governancePause(uint32 id,bool paused)',
  'function setOperator(uint32 id,address operator)',
  'function getChain(uint32 id) view returns (uint32 id,string name,address operator,uint8 status,uint64 chainId,uint64 gasAllowance,uint64 byteAllowance,uint16 feeShareBps,uint256 bond,uint256 baseFeePerGas,uint64 createdHeight)',
  'function chainByName(string name) view returns (uint32)',
  'function chainCount() view returns (uint32)',
  'function chainIdOf(uint32 id) pure returns (uint64)',
  'function params() view returns (address authority,uint256 minBond,uint256 creationFee,uint64 minGasAllowance,uint64 maxGasAllowance,uint64 totalGasAllowanceLeft,uint32 maxChains)',
]);

const chainLaunchedTopic = toEventSelector('ChainLaunched(uint32,address,address,uint64,uint256,address,uint256,uint256)');
const tokenAbi = parseAbi(['function name() view returns (string)', 'function symbol() view returns (string)']);
export type ChainLaunchStep = 'create' | 'confirm' | 'ready';
export interface ChainLaunchResult {
  id: number;
  chainId: number;
  name: string;
  tokenName: string;
  tokenSymbol: string;
  token: Address;
  pool: Address;
  tradingPath: string;
  rpcUrl: string;
  explorerUrl: string;
  walletNetwork: ReturnType<typeof walletNetwork>;
  txHash: Hex;
}

export interface ChainSubmission {
  creator: string;
  chainId: 125;
  txHash: Hex;
  phase: 'confirm' | 'reverted';
  bondRaw: string;
}

export interface ChainConfirmationOutcome {
  txHash: Hex;
  submission: ChainSubmission;
  error: string;
}

export type ChainLaunchOutcome = ChainLaunchResult | ChainConfirmationOutcome;
export type ChainStepReporter = (step: ChainLaunchStep, submission?: ChainSubmission) => void;

export function chainLaunchRequest(draft: ChainDraft, creator: string, value: bigint) {
  const tier = SIZE_TIERS[draft.tier];
  return {
    address: LAUNCHPAD_ADDRESS, abi: CHAIN_LAUNCH_ABI, functionName: 'launchChain' as const,
    args: [draft.tokenName, draft.tokenSymbol, draft.feeStrategy,
      '0x0000000000000000000000000000000000000000', draft.name,
      BigInt(tier.gas), BigInt(tier.bytes), draft.feeShareBps] as const,
    account: address(creator) as Address, value,
  } as const;
}

export class ChainLaunchAdapter {
  private pending: ChainSubmission | null = null;

  constructor(
    readonly client: PublicClient,
    private wallet: WalletClient,
    private currentScope: () => AccountScope,
  ) {}

  scope(expected?: AccountScope): AccountScope {
    const value = this.currentScope();
    if (value.chainId !== 125
      || address(value.creator) !== address(this.wallet.account?.address ?? value.creator)
      || (expected && (value.epoch !== expected.epoch || address(value.creator) !== address(expected.creator)
        || value.chainId !== expected.chainId))) {
      throw new Error('Wallet changed. Review this launch again.');
    }
    return { ...value, creator: address(value.creator) };
  }

  async assertWallet(expected: AccountScope): Promise<void> {
    const accounts = await this.wallet.getAddresses();
    if (await this.wallet.getChainId() !== 125
      || !accounts.some(value => address(value) === address(expected.creator))) {
      throw new Error('Wallet account or chain changed. Review again.');
    }
    this.scope(expected);
  }

  async assertChain(): Promise<void> {
    if (await this.client.getChainId() !== 125 || await this.wallet.getChainId() !== 125) {
      throw new Error('Switch to chain 125.');
    }
  }

  async readParams(): Promise<ChainsParams> {
    return this.client.readContract({ address: CHAINS_PRECOMPILE, abi: chainsAbi, functionName: 'params' });
  }

  async isNameAvailable(name: string): Promise<boolean> {
    if (validateChainName(name)) throw new Error('invalid_chain_name');
    return await this.client.readContract({
      address: CHAINS_PRECOMPILE, abi: chainsAbi, functionName: 'chainByName', args: [name],
    }) === 0;
  }

  async launch(draft: ChainDraft, onStep: ChainStepReporter): Promise<ChainLaunchOutcome> {
    if (this.pending) throw new Error('Retry confirmation of the submitted transaction first.');
    const reviewed = { ...draft };
    const errors = validateChainDraft(reviewed, Date.now());
    if (Object.keys(errors).length) throw new Error('Invalid chain fields: ' + Object.keys(errors).join(', '));
    const expected = this.scope();
    if (address(reviewed.creator) !== expected.creator) throw new Error('Draft creator changed.');
    await this.assertChain();
    const params = await this.readParams();
    if (!await this.isNameAvailable(reviewed.name)) throw new Error('Chain name is taken.');
    const rpcBase = this.client.chain?.rpcUrls.default.http[0];
    const explorerBase = this.client.chain?.blockExplorers?.default.url;
    if (!rpcBase || !explorerBase) throw new Error('Chain network URLs are unavailable.');
    const request = chainLaunchRequest(reviewed, expected.creator, params[1] + params[2]);
    const [gas, gasPrice] = await Promise.all([
      this.client.estimateContractGas(request), this.client.getGasPrice(),
    ]);
    const cost = chainCost(params, reviewed.tier, (gas * gasPrice).toString());
    onStep('create');
    await this.assertWallet(expected);
    const txHash = hash(await this.wallet.writeContract({
      ...request, value: BigInt(cost.totalValueRaw), chain: this.wallet.chain,
    })) as Hex;
    const submission: ChainSubmission = {
      creator: expected.creator, chainId: 125, txHash, phase: 'confirm', bondRaw: params[1].toString(),
    };
    this.pending = submission;
    try {
      onStep('confirm', submission);
    } catch (cause) {
      return { txHash, submission, error: cause instanceof Error ? cause.message : 'Submission could not be saved.' };
    }
    return this.confirm(txHash, reviewed, submission, onStep);
  }

  async confirm(txHash: Hex, draft: ChainDraft, submission: ChainSubmission,
    onStep: ChainStepReporter = () => {}): Promise<ChainLaunchOutcome> {
    const reviewed = { ...draft };
    const pending = { ...submission, phase: 'confirm' as const };
    this.pending = pending;
    try {
      const expected = this.scope();
      if (hash(txHash) !== hash(submission.txHash) || submission.phase !== 'confirm'
        || address(submission.creator) !== expected.creator || submission.chainId !== expected.chainId
        || address(reviewed.creator) !== expected.creator || reviewed.chainId !== expected.chainId) {
        throw new Error('Submitted transaction belongs to a different account or chain.');
      }
      if (await this.client.getChainId() !== submission.chainId) throw new Error('Switch to chain 125.');
      this.scope(expected);
      const receipt = await this.client.waitForTransactionReceipt({ hash: txHash, timeout: 30000 });
      this.scope(expected);
      if (hash(receipt.transactionHash) !== hash(txHash)) throw new Error('Chain launch receipt failed.');
      if (receipt.status === 'reverted') {
        this.pending = null;
        return { txHash, submission: { ...pending, phase: 'reverted' }, error: 'Chain launch receipt failed: transaction reverted. Review again before launching.' };
      }
      if (receipt.status !== 'success') throw new Error('Chain launch receipt failed.');
      const tier = SIZE_TIERS[reviewed.tier];
      const rpcBase = this.client.chain?.rpcUrls.default.http[0];
      const explorerBase = this.client.chain?.blockExplorers?.default.url;
      if (!rpcBase || !explorerBase) throw new Error('Chain network URLs are unavailable.');
      const logs = receipt.logs.filter(log => address(log.address) === address(LAUNCHPAD_ADDRESS)
        && log.topics[0] === chainLaunchedTopic);
      if (logs.length !== 1) throw new Error('Expected one ChainLaunched receipt event.');
      const created = decodeChainLaunched(logs[0]);
      if (created.creator !== expected.creator || created.blockNumber !== receipt.blockNumber) {
        throw new Error('ChainLaunched event does not match the reviewed launch.');
      }
      for (let attempt = 0; attempt < 30; attempt++) {
        this.scope(expected);
        const launched = await this.client.readContract({
          address: LAUNCHPAD_ADDRESS, abi: CHAIN_LAUNCH_ABI, functionName: 'launchedChain',
          args: [created.id], blockNumber: receipt.blockNumber,
        });
        const record = await this.client.readContract({
          address: CHAINS_PRECOMPILE, abi: chainsAbi, functionName: 'getChain', args: [created.id],
          blockNumber: receipt.blockNumber,
        }).catch((error: unknown) => {
          const reverted = error instanceof BaseError
            ? error.walk(cause => cause instanceof ContractFunctionRevertedError) : undefined;
          if (reverted instanceof ContractFunctionRevertedError && reverted.reason?.startsWith('LXP_UNKNOWN_FIELD:')) {
            return null;
          }
          throw error;
        });
        this.scope(expected);
        if (record !== null) {
          const [tokenName, tokenSymbol] = await Promise.all([
            this.client.readContract({ address: created.token as Address, abi: tokenAbi, functionName: 'name', blockNumber: receipt.blockNumber }),
            this.client.readContract({ address: created.token as Address, abi: tokenAbi, functionName: 'symbol', blockNumber: receipt.blockNumber }),
          ]);
          this.scope(expected);
          if (launched[0] !== created.id || launched[1] !== BigInt(created.chainId)
            || address(launched[2]) !== created.pool || address(launched[3]) !== created.token
            || launched[4] !== created.nftId || address(launched[5]) !== created.creator
            || launched[6] !== created.blockNumber
            || record[0] !== created.id || record[1] !== reviewed.name || address(record[2]) !== created.creator
            || record[4] !== BigInt(created.chainId) || record[5] !== BigInt(tier.gas)
            || record[6] !== BigInt(tier.bytes) || record[7] !== reviewed.feeShareBps
            || record[8] !== BigInt(submission.bondRaw)
            || tokenName !== reviewed.tokenName || tokenSymbol !== reviewed.tokenSymbol) {
            throw new Error('Chain launch readback does not match the reviewed launch.');
          }
          const network = walletNetwork({ id: created.id, name: reviewed.name, chainId: created.chainId }, rpcBase, explorerBase);
          this.pending = null;
          onStep('ready');
          return {
            id: record[0], chainId: Number(record[4]), name: record[1],
            tokenName: reviewed.tokenName, tokenSymbol: reviewed.tokenSymbol,
            token: created.token, pool: created.pool, tradingPath: poolPath(125, created.pool),
            rpcUrl: network.rpcUrls[0], explorerUrl: network.blockExplorerUrls[0],
            walletNetwork: network, txHash,
          };
        }
        if (attempt < 29) await new Promise(resolve => setTimeout(resolve, 1000));
      }
      throw new Error('Chain registry record is unavailable after confirmation.');
    } catch (cause) {
      return { txHash, submission: pending, error: cause instanceof Error ? cause.message : 'Confirmation is unavailable. Retry confirmation.' };
    }
  }
}
