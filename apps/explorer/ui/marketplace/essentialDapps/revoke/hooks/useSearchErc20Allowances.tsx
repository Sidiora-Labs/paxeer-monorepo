import ERC20Artifact from '@openzeppelin/contracts/build/contracts/ERC20.json';
import { uniq } from 'es-toolkit';
import { useCallback } from 'react';
import { getAddress, formatUnits, slice } from 'viem';
import type { PublicClient, Log } from 'viem';

import type { AddressTokenBalancesResponse } from 'types/api/address';
import type { TokenInfo } from 'types/api/token';
import type { EssentialDappsChainConfig } from 'types/client/marketplace';
import type { AllowanceType, ContractAllowanceType } from 'types/client/revoke';

import useApiFetch from 'lib/api/useApiFetch';

import type { ApprovalScanResult } from './approvalScan';
import { checkScanCancellation, latestPermissionEvents } from './approvalScan';
import useGetBlockTimestamp from './useGetBlockTimestamp';

function validDecimals(value: TokenInfo['decimals']): number | undefined {
  if (value === null || value === undefined || value === '') return undefined;
  const decimals = Number(value);
  return Number.isInteger(decimals) && decimals >= 0 && decimals <= 255 ? decimals : undefined;
}

function formatAllowance(allowance: bigint, decimals: number | undefined, totalSupply: bigint | undefined): string | undefined {
  if (totalSupply !== undefined && allowance > totalSupply) return 'Unlimited';
  return decimals === undefined ? undefined : formatUnits(allowance, decimals);
}

async function getERC20AllowanceFromApproval(
  tokenAddress: `0x${ string }`,
  ownerAddress: string,
  approval: Log,
  publicClient: PublicClient,
): Promise<ContractAllowanceType> {
  const spender = getAddress(slice(approval.topics[2] as `0x${ string }`, 12));
  const allowance = await publicClient.readContract({
    address: tokenAddress,
    abi: ERC20Artifact.abi,
    functionName: 'allowance',
    args: [ ownerAddress, spender ],
  });

  return {
    transactionId: approval.transactionHash,
    spender,
    allowance: allowance as bigint,
    blockNumber: approval.blockNumber as bigint,
  };
}

export default function useSearchErc20Allowances() {
  const apiFetch = useApiFetch();
  const getBlockTimestamp = useGetBlockTimestamp();

  return useCallback(async(
    chain: EssentialDappsChainConfig | undefined,
    searchQuery: string,
    approvalEvents: Array<Log>,
    publicClient: PublicClient,
    signal?: AbortSignal,
  ): Promise<ApprovalScanResult> => {
    checkScanCancellation(signal);
    const events = latestPermissionEvents(approvalEvents.filter((event) => event.topics.length === 3),
      (event) => event.topics[2] as string);
    const tokenAddresses = uniq(events.map((event) => getAddress(event.address)));
    const result: ApprovalScanResult = { allowances: [], issues: [] };
    if (!tokenAddresses.length) return result;

    let balances: Record<string, bigint> = {};
    try {
      const response = await apiFetch('general:address_token_balances', {
        pathParams: { hash: searchQuery }, chain, fetchParams: { signal },
      }) as AddressTokenBalancesResponse;
      balances = Object.fromEntries(response.map((entry) => [ getAddress(entry.token.address_hash), BigInt(entry.value) ]));
    } catch (cause) {
      checkScanCancellation(signal, cause);
      result.issues.push({ kind: 'balance', cause });
    }

    await Promise.all(tokenAddresses.map(async(tokenAddress) => {
      checkScanCancellation(signal);
      let tokenData: Pick<AllowanceType, 'symbol' | 'decimals' | 'totalSupply' | 'name' | 'tokenIcon' | 'price' | 'tokenReputation'> = {
        tokenReputation: null,
      };
      try {
        const data = await apiFetch('general:token', {
          pathParams: { hash: tokenAddress }, chain, fetchParams: { signal },
        }) as TokenInfo;
        tokenData = {
          symbol: data.symbol || undefined,
          decimals: validDecimals(data.decimals),
          totalSupply: data.total_supply ? BigInt(data.total_supply) : undefined,
          name: data.name || undefined,
          tokenIcon: data.icon_url || undefined,
          price: data.exchange_rate || undefined,
          tokenReputation: data.reputation,
        };
      } catch (cause) {
        checkScanCancellation(signal, cause);
        result.issues.push({ kind: 'metadata', address: tokenAddress, cause });
      }

      const approvals = events.filter((event) => getAddress(event.address) === tokenAddress);
      await Promise.all(approvals.map(async(approval) => {
        checkScanCancellation(signal);
        let allowance: ContractAllowanceType;
        try {
          allowance = await getERC20AllowanceFromApproval(tokenAddress, searchQuery, approval, publicClient);
        } catch (cause) {
          checkScanCancellation(signal, cause);
          result.issues.push({
            kind: 'permission', address: tokenAddress,
            spender: getAddress(slice(approval.topics[2] as `0x${ string }`, 12)), cause,
          });
          return;
        }
        checkScanCancellation(signal);
        if (allowance.allowance === BigInt(0)) return;

        let timestamp = 0;
        try {
          timestamp = await getBlockTimestamp(chain, allowance.blockNumber, signal);
        } catch (cause) {
          checkScanCancellation(signal, cause);
          result.issues.push({ kind: 'timestamp', address: tokenAddress, cause });
        }
        checkScanCancellation(signal);
        const balance = balances[tokenAddress];
        let valueAtRiskUsd;
        if (allowance.allowance !== undefined && balance !== undefined && tokenData.price && tokenData.decimals !== undefined) {
          const valueAtRisk = allowance.allowance > balance ? balance : allowance.allowance;
          valueAtRiskUsd = Number((parseFloat(formatUnits(valueAtRisk, tokenData.decimals)) * parseFloat(tokenData.price)).toFixed(2));
        }
        result.allowances.push({
          ...tokenData,
          type: 'ERC-20', address: tokenAddress,
          transactionId: allowance.transactionId, spender: allowance.spender, timestamp,
          allowance: allowance.allowance === undefined ? undefined : formatAllowance(allowance.allowance, tokenData.decimals, tokenData.totalSupply),
          valueAtRiskUsd,
        });
      }));
    }));
    checkScanCancellation(signal);
    return result;
  }, [ apiFetch, getBlockTimestamp ]);
}
