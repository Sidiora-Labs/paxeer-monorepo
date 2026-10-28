import { ethers } from 'ethers';

export function units(amount: string, decimals: number | null | undefined): string {
  if (decimals === null || decimals === undefined) return amount;
  return ethers.formatUnits(BigInt(amount), decimals);
}
