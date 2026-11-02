import { BaseError, ContractFunctionRevertedError, parseAbi, toEventSelector, type Address, type Hex, type PublicClient, type WalletClient } from 'viem';
import { address, hash } from '../create/creation-domain';
import type { AccountScope } from '../create/creation-adapters';
import {
  CHAINS_PRECOMPILE, SIZE_TIERS, chainCost, decodeChainCreated,
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

const chainCreatedTopic = toEventSelector('ChainCreated(uint32,string,address,uint64)');
export type ChainLaunchStep = 'create' | 'confirm' | 'ready';
export interface ChainLaunchResult {
  id: number;
  chainId: number;
  name: string;
  rpcUrl: string;
  explorerUrl: string;
  walletNetwork: ReturnType<typeof walletNetwork>;
  txHash: Hex;
}

export class ChainLaunchAdapter {
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

  async launch(draft: ChainDraft, onStep: (step: ChainLaunchStep) => void): Promise<ChainLaunchResult> {
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
    const tier = SIZE_TIERS[reviewed.tier];
    const request = {
      address: CHAINS_PRECOMPILE, abi: chainsAbi, functionName: 'createChain' as const,
      args: [reviewed.name, BigInt(tier.gas), BigInt(tier.bytes), reviewed.feeShareBps] as const,
      account: expected.creator as Address,
      value: params[1] + params[2],
    } as const;
    const [gas, gasPrice] = await Promise.all([
      this.client.estimateContractGas(request), this.client.getGasPrice(),
    ]);
    const cost = chainCost(params, reviewed.tier, (gas * gasPrice).toString());
    onStep('create');
    await this.assertWallet(expected);
    const txHash = hash(await this.wallet.writeContract({
      ...request, value: BigInt(cost.totalValueRaw), chain: this.wallet.chain,
    })) as Hex;
    onStep('confirm');
    const receipt = await this.client.waitForTransactionReceipt({ hash: txHash, timeout: 30000 });
    if (receipt.status !== 'success' || hash(receipt.transactionHash) !== txHash) {
      throw new Error('Chain creation receipt failed.');
    }
    const logs = receipt.logs.filter(log => address(log.address) === CHAINS_PRECOMPILE
      && log.topics[0] === chainCreatedTopic);
    if (logs.length !== 1) throw new Error('Expected one ChainCreated receipt event.');
    const created = decodeChainCreated(logs[0]);
    if (created.name !== reviewed.name || created.operator !== expected.creator) {
      throw new Error('ChainCreated event does not match the reviewed draft.');
    }
    for (let attempt = 0; attempt < 30; attempt++) {
      this.scope(expected);
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
        if (record[0] !== created.id || record[1] !== created.name || address(record[2]) !== created.operator
          || record[4] !== BigInt(created.chainId) || record[5] !== BigInt(tier.gas)
          || record[6] !== BigInt(tier.bytes) || record[7] !== reviewed.feeShareBps
          || record[8] !== params[1]) {
          throw new Error('Chain registry record does not match the creation receipt.');
        }
        const network = walletNetwork(created, rpcBase, explorerBase);
        onStep('ready');
        return {
          id: record[0], chainId: Number(record[4]), name: record[1],
          rpcUrl: network.rpcUrls[0], explorerUrl: network.blockExplorerUrls[0],
          walletNetwork: network, txHash,
        };
      }
      if (attempt < 29) await new Promise(resolve => setTimeout(resolve, 1000));
    }
    throw new Error('Chain registry record is unavailable after confirmation.');
  }
}
