import NftArtifact from '@openzeppelin/contracts/build/contracts/ERC721.json';
import { useCallback } from 'react';
import { getAbiItem, getAddress, slice } from 'viem';
import type { PublicClient, GetLogsParameters, Log } from 'viem';

import type { TokenInfo } from 'types/api/token';
import type { EssentialDappsChainConfig } from 'types/client/marketplace';
import type { AllowanceType, ContractAllowanceType } from 'types/client/revoke';

import useApiFetch from 'lib/api/useApiFetch';
import { ZERO_ADDRESS } from 'toolkit/utils/consts';

import getLogs from '../lib/getLogs';
import { ApprovalScanError, checkScanCancellation, latestPermissionEvents } from './approvalScan';
import type { ApprovalScanIssue } from './approvalScan';
import useGetBlockTimestamp from './useGetBlockTimestamp';

async function getLimitedNftAllowancesFromApprovals(
  tokenAddress: `0x${ string }`,
  ownerAddress: string,
  approvals: Array<Log>,
  publicClient: PublicClient,
) {
  const deduplicatedApprovals = latestPermissionEvents(approvals, (approval) => nftTokenId(approval).toString());

  const allowances: Array<ContractAllowanceType | undefined> = await Promise.all(
    deduplicatedApprovals.map((approval) =>
      getLimitedNftAllowanceFromApproval(tokenAddress, ownerAddress, approval, publicClient),
    ),
  );

  return allowances;
}

function nftTokenId(approval: Log): bigint {
  // Non-indexed token IDs are emitted in data by some NFT contracts.
  return BigInt(approval.topics.length === 4 ? approval.topics[3] as `0x${ string }` : approval.data);
}

async function getLimitedNftAllowanceFromApproval(
  tokenAddress: `0x${ string }`,
  ownerAddress: string,
  approval: Log,
  publicClient: PublicClient,
) {
  const tokenId = nftTokenId(approval);
  const owner = await publicClient.readContract({
    address: tokenAddress,
    abi: NftArtifact.abi,
    functionName: 'ownerOf',
    args: [ tokenId ],
  }) as `0x${ string }`;
  if (getAddress(owner) !== getAddress(ownerAddress)) return undefined;

  const spender = await publicClient.readContract({
    address: tokenAddress,
    abi: NftArtifact.abi,
    functionName: 'getApproved',
    args: [ tokenId ],
  }) as `0x${ string }`;
  if (getAddress(spender) === ZERO_ADDRESS) return undefined;

  return {
    spender,
    transactionId: approval.transactionHash,
    allowance: BigInt(-1),
    tokenId: tokenId.toString(),
    blockNumber: approval.blockNumber as bigint,
  };
}

async function getNftUnlimitedAllowancesFromApprovals(
  tokenAddress: `0x${ string }`,
  ownerAddress: string,
  approvals: Array<Log>,
  publicClient: PublicClient,
) {
  const deduplicatedApprovals = latestPermissionEvents(approvals, (approval) => approval.topics[2] as string);

  const allowances: Array<ContractAllowanceType | undefined> = await Promise.all(
    deduplicatedApprovals.map((approval) =>
      getNftUnlimitedAllowanceFromApproval(
        tokenAddress,
        ownerAddress,
        approval,
        publicClient,
      ),
    ),
  );

  return allowances;
}

async function getNftUnlimitedAllowanceFromApproval(
  tokenAddress: `0x${ string }`,
  ownerAddress: string,
  approval: Log,
  publicClient: PublicClient,
): Promise<ContractAllowanceType | undefined> {
  const spender = getAddress(slice(approval.topics[2] as `0x${ string }`, 12));

  const isApprovedForAll = await publicClient.readContract({
    address: tokenAddress,
    abi: NftArtifact.abi,
    functionName: 'isApprovedForAll',
    args: [ ownerAddress, spender ],
  });
  if (!isApprovedForAll) return undefined;

  return {
    spender,
    transactionId: approval.transactionHash,
    allowance: isApprovedForAll && BigInt(-1),
    blockNumber: approval.blockNumber as bigint,
  };
}

function useGetNftAllowances() {
  const apiFetch = useApiFetch();
  const getBlockTimestamp = useGetBlockTimestamp();

  return useCallback(async(
    tokenAddresses: Array<`0x${ string }`>,
    searchQuery: string,
    approvals: Array<Log>,
    approvalsForAll: Array<Log>,
    publicClient: PublicClient,
    chain: EssentialDappsChainConfig | undefined,
    signal?: AbortSignal,
  ) => {
    checkScanCancellation(signal);
    const scans = await Promise.allSettled(tokenAddresses.map(async(tokenAddress): Promise<Array<AllowanceType>> => {
      checkScanCancellation(signal);
      const tokenData = await apiFetch('general:token', {
        pathParams: { hash: tokenAddress }, chain, fetchParams: { signal },
      }) as TokenInfo;
      checkScanCancellation(signal);

      const tokenApprovals = approvals.filter((approval) => getAddress(approval.address) === tokenAddress);
      const tokenApprovalsForAll = approvalsForAll.filter((approval) => getAddress(approval.address) === tokenAddress);
      const unlimitedAllowances = await getNftUnlimitedAllowancesFromApprovals(
        tokenAddress, searchQuery, tokenApprovalsForAll, publicClient,
      );
      const limitedAllowances = await getLimitedNftAllowancesFromApprovals(
        tokenAddress, searchQuery, tokenApprovals, publicClient,
      );
      checkScanCancellation(signal);
      const allAllowances = [ ...limitedAllowances, ...unlimitedAllowances ].filter(
        (allowance): allowance is ContractAllowanceType => allowance !== undefined,
      );
      return Promise.all(allAllowances.map(async(allowance): Promise<AllowanceType> => {
        const timestamp = await getBlockTimestamp(chain, allowance.blockNumber, signal);
        checkScanCancellation(signal);
        return {
          type: 'ERC-721',
          allowance: allowance.allowance === BigInt(-1) ? 'Unlimited' : allowance.allowance?.toString(),
          address: tokenAddress,
          name: tokenData.name || undefined,
          symbol: tokenData.symbol || undefined,
          tokenIcon: tokenData.icon_url || undefined,
          tokenReputation: tokenData.reputation,
          transactionId: allowance.transactionId,
          timestamp,
          spender: allowance.spender,
          tokenId: allowance.tokenId,
        };
      }));
    }));
    checkScanCancellation(signal);
    const allowances: Array<AllowanceType> = [];
    const issues: Array<ApprovalScanIssue> = [];
    scans.forEach((scan, index) => {
      if (scan.status === 'fulfilled') {
        allowances.push(...scan.value);
      } else {
        checkScanCancellation(signal, scan.reason);
        issues.push({ kind: 'scan', address: tokenAddresses[index], cause: scan.reason });
      }
    });
    if (issues.length) throw new ApprovalScanError(allowances, issues);
    return allowances;
  }, [ apiFetch, getBlockTimestamp ]);
}

export default function useSearchNftAllowances() {
  const getNftAllowances = useGetNftAllowances();

  return useCallback(async(
    chain: EssentialDappsChainConfig | undefined,
    searchQuery: string,
    approvalEvents: Array<Log>,
    publicClient: PublicClient,
    latestBlockNumber: bigint,
    signal?: AbortSignal,
  ) => {
    try {
      const transferEventsPromise = getLogs(
        publicClient,
        {
          event: getAbiItem({ abi: NftArtifact.abi, name: 'Transfer' }),
          args: { to: searchQuery },
        } as unknown as GetLogsParameters,
        BigInt(0),
        latestBlockNumber,
        signal,
      );

      const approvalForAllEventsPromise = getLogs(
        publicClient,
        {
          event: getAbiItem({ abi: NftArtifact.abi, name: 'ApprovalForAll' }),
          args: { owner: searchQuery },
        } as unknown as GetLogsParameters,
        BigInt(0),
        latestBlockNumber,
        signal,
      );

      const [ transferEvents, approvalForAllEvents ] = await Promise.all([
        transferEventsPromise,
        approvalForAllEventsPromise,
      ]);

      const nftApprovalEvents = approvalEvents.filter(
        (ev) => ev.topics.length === 4,
      );
      const nftTransferEvents = transferEvents.filter(
        (ev) => ev.topics.length === 4,
      );

      const nftEvents = [
        ...nftApprovalEvents,
        ...approvalForAllEvents,
        ...nftTransferEvents,
      ];

      const nftAddresses = Array.from(new Set(nftEvents.map((event) => getAddress(event.address))));

      return getNftAllowances(
        nftAddresses,
        searchQuery,
        nftApprovalEvents,
        approvalForAllEvents,
        publicClient,
        chain,
        signal,
      );
    } catch (error) {
      checkScanCancellation(signal, error);
      throw error;
    }
  }, [ getNftAllowances ]);
}
