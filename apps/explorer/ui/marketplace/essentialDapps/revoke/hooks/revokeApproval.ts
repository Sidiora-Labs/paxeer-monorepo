import ERC20Artifact from '@openzeppelin/contracts/build/contracts/ERC20.json';
import NftArtifact from '@openzeppelin/contracts/build/contracts/ERC721.json';
import type { Address, PublicClient } from 'viem';

import type { AllowanceType } from 'types/client/revoke';

import { ZERO_ADDRESS } from 'toolkit/utils/consts';

export function getRevokeContractParameters(approval: AllowanceType) {
  if (approval.type === 'ERC-20') {
    return {
      address: approval.address,
      abi: ERC20Artifact.abi,
      functionName: 'approve',
      args: [ approval.spender, 0 ],
    } as const;
  }

  if (approval.type === 'ERC-721' && approval.tokenId !== undefined) {
    return {
      address: approval.address,
      abi: NftArtifact.abi,
      functionName: 'approve',
      args: [ ZERO_ADDRESS, BigInt(approval.tokenId) ],
    } as const;
  }

  return {
    address: approval.address,
    abi: NftArtifact.abi,
    functionName: 'setApprovalForAll',
    args: [ approval.spender, false ],
  } as const;
}

export async function verifyNftRevocation(
  approval: AllowanceType,
  owner: Address,
  blockNumber: bigint,
  publicClient: PublicClient,
) {
  if (approval.type === 'ERC-721' && approval.tokenId !== undefined) {
    const approved = await publicClient.readContract({
      address: approval.address,
      abi: NftArtifact.abi,
      functionName: 'getApproved',
      args: [ BigInt(approval.tokenId) ],
      blockNumber,
    });

    if (approved !== ZERO_ADDRESS) {
      throw new Error('Token approval remains active after the revoke transaction.');
    }
  } else {
    const approved = await publicClient.readContract({
      address: approval.address,
      abi: NftArtifact.abi,
      functionName: 'isApprovedForAll',
      args: [ owner, approval.spender ],
      blockNumber,
    });

    if (approved !== false) {
      throw new Error('Operator approval remains active after the revoke transaction.');
    }
  }
}
