import ERC20Artifact from '@openzeppelin/contracts/build/contracts/ERC20.json';
import { queryOptions, useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import type { GetLogsParameters, PublicClient } from 'viem';
import { isAddress, getAbiItem } from 'viem';

import type { EssentialDappsChainConfig } from 'types/client/marketplace';

import { ALLOWANCES } from 'stubs/essentialDapps/revoke';

import createPublicClient from '../lib/createPublicClient';
import getLogs from '../lib/getLogs';
import { ApprovalScanError, checkScanCancellation } from './approvalScan';
import type { ApprovalScanIssue } from './approvalScan';
import useSearchErc20Allowances from './useSearchErc20Allowances';
import useSearchNftAllowances from './useSearchNftAllowances';

export function approvalsQueryOptions(
  chain: EssentialDappsChainConfig | undefined,
  userAddress: string,
  publicClient: PublicClient | undefined,
  searchErc20Allowances: ReturnType<typeof useSearchErc20Allowances>,
  searchNftAllowances: ReturnType<typeof useSearchNftAllowances>,
) {
  return queryOptions({
    queryKey: [ 'revoke:approvals', chain?.id, userAddress ],
    queryFn: async({ signal }) => {
      checkScanCancellation(signal);
      if (!publicClient) throw new Error('Public client not found');
      const latestBlockNumber = await publicClient.getBlockNumber();
      checkScanCancellation(signal);
      const filter = {
        event: getAbiItem({ abi: ERC20Artifact.abi, name: 'Approval' }),
        args: { owner: userAddress },
      } as unknown as GetLogsParameters;
      const approvalEvents = await getLogs(publicClient, filter, BigInt(0), latestBlockNumber, signal);
      const [ erc20, nft ] = await Promise.allSettled([
        searchErc20Allowances(chain, userAddress, approvalEvents, publicClient, signal),
        searchNftAllowances(chain, userAddress, approvalEvents, publicClient, latestBlockNumber, signal),
      ]);
      checkScanCancellation(signal);
      const issues: Array<ApprovalScanIssue> = [];
      for (const scan of [ erc20, nft ]) {
        if (scan.status === 'rejected') {
          checkScanCancellation(signal, scan.reason);
          issues.push({ kind: 'scan', cause: scan.reason });
        }
      }
      if (erc20.status === 'fulfilled') issues.push(...erc20.value.issues);
      const allowances = [
        ...(erc20.status === 'fulfilled' ? erc20.value.allowances : []),
        ...(nft.status === 'fulfilled' ? nft.value : []),
      ].sort((a, b) => b.timestamp - a.timestamp);
      if (issues.length) throw new ApprovalScanError(allowances, issues);
      return allowances;
    },
    enabled: Boolean(userAddress) && isAddress(userAddress),
    placeholderData: ALLOWANCES,
  });
}

export default function useApprovalsQuery(chain: EssentialDappsChainConfig | undefined, userAddress: string) {
  const searchErc20Allowances = useSearchErc20Allowances();
  const searchNftAllowances = useSearchNftAllowances();
  const publicClient = useMemo(() => createPublicClient(chain?.id), [ chain?.id ]);
  const query = useQuery(approvalsQueryOptions(chain, userAddress, publicClient, searchErc20Allowances, searchNftAllowances));
  const incompleteScan = query.error instanceof ApprovalScanError ? query.error : undefined;

  let coverage: 'pending' | 'incomplete' | 'complete' = 'complete';
  if (query.isPlaceholderData || query.isPending) coverage = 'pending';
  else if (query.isError) coverage = 'incomplete';

  return {
    ...query,
    coverage,
    partialAllowances: incompleteScan?.partialAllowances,
    scanIssues: incompleteScan?.issues,
    retry: query.refetch,
  };
}
