import { getPublicClient, waitForTransactionReceipt } from '@wagmi/core';
import { useCallback } from 'react';
import { useAccount, useWriteContract, useSwitchChain } from 'wagmi';

import type { AllowanceType } from 'types/client/revoke';

import useRewardsActivity from 'lib/hooks/useRewardsActivity';
import * as mixpanel from 'lib/mixpanel/index';
import wagmiConfig from 'lib/web3/wagmiConfig';
import { toaster } from 'toolkit/chakra/toaster';

import { getRevokeContractParameters, verifyNftRevocation } from './revokeApproval';

export default function useRevoke() {
  const { address: userAddress } = useAccount();
  const { switchChainAsync } = useSwitchChain();
  const { writeContractAsync } = useWriteContract();
  const { trackTransaction, trackTransactionConfirm } = useRewardsActivity();

  return useCallback(async(approval: AllowanceType, chainId: number) => {
    try {
      if (!userAddress) return;

      await switchChainAsync({ chainId });

      const activityResponse = await trackTransaction(userAddress, approval.address, String(chainId));

      const hash = await writeContractAsync({
        ...getRevokeContractParameters(approval),
        account: userAddress,
        chainId,
      });

      mixpanel.logEvent(mixpanel.EventTypes.WALLET_ACTION, {
        Action: 'Send Transaction',
        Address: userAddress,
        AppId: 'revoke',
        Source: 'Essential dapps',
        ChainId: String(chainId),
      });

      if (activityResponse?.token) {
        await trackTransactionConfirm(hash, activityResponse.token);
      }

      const receipt = await waitForTransactionReceipt(wagmiConfig.config, { hash, chainId });

      if (receipt.status === 'reverted') {
        throw new Error('Failed to revoke approval.');
      }

      if (approval.type !== 'ERC-20') {
        const publicClient = getPublicClient(wagmiConfig.config, { chainId });
        if (!publicClient) {
          throw new Error('Unable to verify the revoked approval on this chain.');
        }

        await verifyNftRevocation(approval, userAddress, receipt.blockNumber, publicClient);
      }

      toaster.success({
        title: 'Success',
        description: 'Approval revoked successfully.',
      });

      return true;

    } catch (error) {
      toaster.error({
        title: 'Error',
        description: (error as Error)?.message || 'Something went wrong. Try again later.',
      });

      return false;
    }
  }, [
    userAddress,
    writeContractAsync,
    switchChainAsync,
    trackTransaction,
    trackTransactionConfirm,
  ]);
}
