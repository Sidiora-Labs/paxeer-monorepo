import type { Address } from "viem";
import { erc20Abi } from "@paxeer-network/paxeer-backend/core/perps/config";
import {
  buildGetBalance,
  buildGetLockedBalance,
} from "@paxeer-network/paxeer-backend/core/perps/calls";
import { getConnectedAddress } from "@paxeer-network/paxeer-backend/core/viem/clients";
import { readContract, writeContract, waitForTx } from "./useContract";
import {
  deposit as depositAccount,
  withdraw as withdrawAccount,
  getWalletBalance as getAccountWalletBalance,
} from "./useAccount";

/** Native accounts live at the connected wallet address; no venue vault is created. */
export async function getVaultAddress(user: Address): Promise<Address> {
  return user;
}

export async function createVault() {
  throw new Error(
    "Native accounts are opened by the first collateral deposit.",
  );
}

export async function getBalance(
  account: Address,
  token: Address,
): Promise<bigint> {
  return (await readContract(buildGetBalance(account, token))) as bigint;
}

export async function getLockedBalance(
  account: Address,
  token: Address,
): Promise<bigint> {
  return (await readContract(buildGetLockedBalance(account, token))) as bigint;
}

export async function deposit(
  account: Address,
  token: Address,
  amount: bigint,
) {
  const connected = getConnectedAddress();
  if (!connected || connected.toLowerCase() !== account.toLowerCase())
    throw new Error("Connect the requested wallet first.");
  return depositAccount(account, amount);
}

export async function withdraw(
  account: Address,
  _token: Address,
  amount: bigint,
) {
  const connected = getConnectedAddress();
  if (!connected || connected.toLowerCase() !== account.toLowerCase())
    throw new Error("Connect the requested wallet first.");
  return withdrawAccount(amount, account);
}

export async function approveToken(
  token: Address,
  spender: Address,
  amount: bigint,
) {
  return waitForTx(
    await writeContract({
      address: token,
      abi: erc20Abi,
      functionName: "approve",
      args: [spender, amount],
    }),
  );
}

export async function getAllowance(
  token: Address,
  owner: Address,
  spender: Address,
): Promise<bigint> {
  return (await readContract({
    address: token,
    abi: erc20Abi,
    functionName: "allowance",
    args: [owner, spender],
  })) as bigint;
}

export async function getWalletBalance(
  token: Address,
  user: Address,
): Promise<bigint> {
  if (token.toLowerCase() === "0x0000000000000000000000000000000000000000")
    return getAccountWalletBalance(user);
  return (await readContract({
    address: token,
    abi: erc20Abi,
    functionName: "balanceOf",
    args: [user],
  })) as bigint;
}

export function useVault() {
  return {
    getVaultAddress,
    createVault,
    getBalance,
    getLockedBalance,
    deposit,
    withdraw,
    approveToken,
    getAllowance,
    getWalletBalance,
  };
}
