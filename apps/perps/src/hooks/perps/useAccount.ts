import { createWalletClient, custom, type Abi, type Address } from "viem";
import { getAccount as getConnectedAccount, getWalletClient, watchAccount } from "wagmi/actions";

import { paxeerChain } from "@/core/chain";
import { wagmiConfig } from "@/core/wagmi";
import { getPublicClient } from "@/core/viem/clients";
import { gasWithMargin } from "@/core/network/gas";
import { accountViewFromState, type AccountView } from "@/core/perps/accountView";
import {
  buildDepositPlan,
  buildGetAccount,
  buildWithdraw,
  buildWithdrawAll,
  type AccountState,
} from "@/core/perps/calls";
import { PERPS_SPENDER } from "@/core/network/approvals";
import { COLLATERAL_TOKENS, erc20Abi } from "@/core/perps/config";
import { SPOT_NATIVE_TOKENS } from "@/hooks/spot/useSpotApprovals";
import { readContract, waitForTx, type WaitForTxResult } from "./useContract";

const USDL: Address = COLLATERAL_TOKENS.usdl.address;

export type DepositStep = "approving" | "executing";

export interface CollateralIdentity {
  readonly address: Address;
  readonly chainId: number;
}

export class CollateralSessionChangedError extends Error {
  constructor(readonly identity: CollateralIdentity) {
    super(`Wallet session changed. Review account ${identity.address} on chain ${identity.chainId} before starting another transfer.`);
    this.name = "CollateralSessionChangedError";
  }
}

interface CollateralCall {
  address: Address;
  abi: Abi;
  functionName: string;
  args?: readonly unknown[];
}

async function withCollateralSession<T>(
  address: Address,
  chainId: number,
  run: (session: {
    read: (call: CollateralCall) => Promise<unknown>;
    write: (call: CollateralCall) => Promise<WaitForTxResult>;
    assertCurrent: () => void;
  }) => Promise<T>,
): Promise<T> {
  const identity: CollateralIdentity = { address, chainId };
  const initial = getConnectedAccount(wagmiConfig);
  const connectorUid = initial.connector?.uid;
  let changed = false;
  const matches = () => {
    const current = getConnectedAccount(wagmiConfig);
    return current.status === "connected" &&
      current.address?.toLowerCase() === address.toLowerCase() &&
      current.chainId === chainId && chainId === paxeerChain.id &&
      current.connector?.uid === connectorUid;
  };
  const assertCurrent = () => {
    if (changed || !matches()) throw new CollateralSessionChangedError(identity);
  };
  const unwatch = watchAccount(wagmiConfig, {
    onChange() { if (!matches()) changed = true; },
  });

  try {
    assertCurrent();
    const original = await getWalletClient(wagmiConfig, { account: address, chainId: paxeerChain.id });
    assertCurrent();
    if (original.account.address.toLowerCase() !== address.toLowerCase()) {
      throw new CollateralSessionChangedError(identity);
    }
    const publicClient = getPublicClient();
    const assertChain = async () => {
      assertCurrent();
      const [publicChainId, walletChainId] = await Promise.all([
        publicClient.getChainId(), original.getChainId(),
      ]);
      assertCurrent();
      if (publicChainId !== chainId || walletChainId !== chainId) {
        changed = true;
        throw new CollateralSessionChangedError(identity);
      }
    };
    const wallet = createWalletClient({
      account: original.account,
      chain: paxeerChain,
      transport: custom({
        async request(request) {
          assertCurrent();
          if (request.method === "eth_sendTransaction" || request.method === "eth_sendRawTransaction") {
            await assertChain();
            const accounts = await original.request({ method: "eth_accounts" });
            assertCurrent();
            if (!accounts.some((account) => account.toLowerCase() === address.toLowerCase())) {
              changed = true;
              throw new CollateralSessionChangedError(identity);
            }
          }
          assertCurrent();
          return original.transport.request(request);
        },
      }, { retryCount: 0 }),
    });
    await assertChain();

    return await run({
      assertCurrent,
      async read(call) {
        await assertChain();
        const result = await publicClient.readContract({ ...call, account: address });
        assertCurrent();
        return result;
      },
      async write(call) {
        await assertChain();
        const boundCall = { ...call, account: address, args: call.args ?? [] };
        await publicClient.simulateContract(boundCall);
        assertCurrent();
        const gas = gasWithMargin(await publicClient.estimateContractGas(boundCall));
        assertCurrent();
        const fees = await publicClient.estimateFeesPerGas();
        assertCurrent();
        const maxPriorityFeePerGas = fees.maxPriorityFeePerGas > 10n ? fees.maxPriorityFeePerGas : 10n;
        const maxFeePerGas = fees.maxFeePerGas > maxPriorityFeePerGas ? fees.maxFeePerGas : maxPriorityFeePerGas;
        const hash = await wallet.writeContract({
          ...boundCall, account: original.account, chain: paxeerChain,
          gas, maxFeePerGas, maxPriorityFeePerGas,
        });
        return waitForTx(hash);
      },
    });
  } catch (error) {
    if (changed || !matches()) throw new CollateralSessionChangedError(identity);
    throw error;
  } finally {
    unwatch();
  }
}

/** getAccount(account) read directly at 0x1015. */
export async function getAccount(account: Address): Promise<AccountView> {
  const state = (await readContract(buildGetAccount(account))) as AccountState;
  return accountViewFromState(state);
}

/** USDL in the wallet, base units. */
export async function getWalletBalance(account: Address): Promise<bigint> {
  return (await readContract({
    address: USDL,
    abi: erc20Abi,
    functionName: "balanceOf",
    args: [account],
  })) as bigint;
}

/** The USDL allowance of `account` to 0x1015, base units. */
export async function getAllowance(account: Address): Promise<bigint> {
  return (await readContract({
    address: USDL,
    abi: erc20Abi,
    functionName: "allowance",
    args: [account, PERPS_SPENDER],
  })) as bigint;
}

/**
 * Deposit `amount` USDL base units into the account at 0x1015, approving exactly
 * `amount` first when the allowance is short. The first deposit opens the account.
 */
export async function deposit(
  account: Address,
  amount: bigint,
  onStep?: (step: DepositStep) => void,
  chainId: number = paxeerChain.id,
): Promise<WaitForTxResult> {
  return withCollateralSession(account, chainId, async (session) => {
    const plan = await buildDepositPlan(amount, async () => (await session.read({
      address: USDL, abi: erc20Abi, functionName: "allowance", args: [account, PERPS_SPENDER],
    })) as bigint, SPOT_NATIVE_TOKENS);
    let receipt: WaitForTxResult | undefined;
    for (const call of plan) {
      session.assertCurrent();
      onStep?.(call.functionName === "approve" ? "approving" : "executing");
      receipt = await session.write(call.functionName === "approve" ? { ...call, abi: erc20Abi } : call);
    }
    if (!receipt) throw new Error("deposit plan sent no transaction");
    return receipt;
  });
}

/** Withdraw `amount` USDL base units from the account at 0x1015. */
export async function withdraw(amount: bigint, account: Address, chainId: number = paxeerChain.id): Promise<WaitForTxResult> {
  return withCollateralSession(account, chainId, (session) => session.write(buildWithdraw(amount)));
}

/** Withdraw getAccount(account).withdrawable, never the idle balance. */
export async function withdrawAll(account: Address, chainId: number = paxeerChain.id): Promise<WaitForTxResult> {
  return withCollateralSession(account, chainId, async (session) => {
    const state = (await session.read(buildGetAccount(account))) as AccountState;
    return session.write(buildWithdrawAll(accountViewFromState(state)));
  });
}

export function useAccount() {
  return { deposit, withdraw, withdrawAll, getAccount, getWalletBalance, getAllowance };
}
