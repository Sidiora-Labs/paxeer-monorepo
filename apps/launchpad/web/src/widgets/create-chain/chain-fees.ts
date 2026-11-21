import { erc20Abi, parseAbi, type Address, type Hex, type PublicClient, type WalletClient } from 'viem';
import { LAUNCHPAD_ADDRESS, PROTOCOL_CONFIG_ADDRESS } from '@/core/network/contracts';
import { address, hash, raw } from '../create/creation-domain';
import type { AccountScope } from '../create/creation-adapters';

const configAbi = parseAbi([
  'function creationFee() view returns (uint256)',
  'function usdlAddress() view returns (address)',
]);

export interface ChainFeeFunding {
  token: Address;
  fee: bigint;
  balance: bigint;
  allowance: bigint;
  decimals: number;
}

export interface ChainFeeApproval {
  creator: string;
  chainId: 125;
  token: Address;
  txHash: Hex;
  feeRaw: string;
}

export class ChainFeeAdapter {
  constructor(
    private client: PublicClient,
    private wallet: WalletClient,
    private currentScope: () => AccountScope,
    private config: Address = PROTOCOL_CONFIG_ADDRESS,
  ) {}

  private scope(expected: AccountScope) {
    const current = this.currentScope();
    if (current.chainId !== 125 || expected.chainId !== 125 || current.epoch !== expected.epoch
      || address(current.creator) !== address(expected.creator)) {
      throw new Error('Wallet changed. Review the market fee again.');
    }
  }

  async read(expected: AccountScope): Promise<ChainFeeFunding> {
    this.scope(expected);
    const token = address(await this.client.readContract({
      address: this.config, abi: configAbi, functionName: 'usdlAddress',
    })) as Address;
    const [fee, balance, allowance, decimals] = await Promise.all([
      this.client.readContract({ address: this.config, abi: configAbi, functionName: 'creationFee' }),
      this.client.readContract({ address: token, abi: erc20Abi, functionName: 'balanceOf', args: [address(expected.creator) as Address] }),
      this.client.readContract({ address: token, abi: erc20Abi, functionName: 'allowance', args: [address(expected.creator) as Address, LAUNCHPAD_ADDRESS] }),
      this.client.readContract({ address: token, abi: erc20Abi, functionName: 'decimals' }),
    ]);
    this.scope(expected);
    if (decimals !== 6) throw new Error('The USDL market fee configuration changed.');
    return { token, fee, balance, allowance, decimals };
  }

  async approve(expected: AccountScope, reviewedFee: bigint,
    onSubmitted: (approval: ChainFeeApproval) => void): Promise<ChainFeeApproval> {
    const funding = await this.read(expected);
    if (funding.fee !== reviewedFee) throw new Error('The market fee changed. Review it again.');
    if (funding.balance < funding.fee) throw new Error('Add USDL to cover the market creation fee.');
    if (funding.allowance >= funding.fee) throw new Error('The market fee is already approved. Refresh the launch details.');
    const accounts = await this.wallet.getAddresses();
    if (await this.client.getChainId() !== 125 || await this.wallet.getChainId() !== 125
      || !accounts.some(account => address(account) === address(expected.creator))) {
      throw new Error('Wallet account or chain changed. Review the market fee again.');
    }
    this.scope(expected);
    const txHash = hash(await this.wallet.writeContract({
      address: funding.token, abi: erc20Abi, functionName: 'approve', args: [LAUNCHPAD_ADDRESS, funding.fee],
      account: address(expected.creator) as Address, chain: this.wallet.chain,
    })) as Hex;
    const approval: ChainFeeApproval = {
      creator: address(expected.creator), chainId: 125, token: funding.token, txHash, feeRaw: funding.fee.toString(),
    };
    onSubmitted(approval);
    return approval;
  }

  async confirm(approval: ChainFeeApproval, expected: AccountScope): Promise<'success' | 'reverted'> {
    this.scope(expected);
    if (approval.chainId !== 125 || address(approval.creator) !== address(expected.creator)) throw new Error('Market fee approval belongs to another account.');
    raw(approval.feeRaw);
    if (await this.client.getChainId() !== 125) throw new Error('Switch to Paxeer to confirm the market fee.');
    const receipt = await this.client.waitForTransactionReceipt({ hash: hash(approval.txHash) as Hex, timeout: 30000 });
    this.scope(expected);
    if (hash(receipt.transactionHash) !== hash(approval.txHash)) throw new Error('Market fee approval receipt does not match.');
    if (receipt.status === 'reverted') return 'reverted';
    const funding = await this.read(expected);
    if (address(approval.token) !== address(funding.token)) throw new Error('The USDL market fee configuration changed.');
    if (funding.allowance < raw(approval.feeRaw)) throw new Error('Market fee approval is not available yet. Retry confirmation.');
    return 'success';
  }
}
