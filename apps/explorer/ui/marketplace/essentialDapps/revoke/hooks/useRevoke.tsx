import { getPublicClient, waitForTransactionReceipt } from '@wagmi/core';
import { useCallback, useRef, useState } from 'react';
import { useAccount, useWriteContract, useSwitchChain } from 'wagmi';

import type { AllowanceType } from 'types/client/revoke';

import useApiFetch from 'lib/api/useApiFetch';
import useRewardsActivity from 'lib/hooks/useRewardsActivity';
import * as mixpanel from 'lib/mixpanel/index';
import wagmiConfig from 'lib/web3/wagmiConfig';
import { toaster } from 'toolkit/chakra/toaster';

import { getRevokeContractParameters, verifyNftRevocation } from './revokeApproval';
import { RevokeSubmission } from './revokeSubmission';
import type { RevokeSubmissionState } from './revokeSubmission';

export default function useRevoke() {
  const { address: userAddress } = useAccount();
  const { switchChainAsync } = useSwitchChain();
  const { writeContractAsync } = useWriteContract();
  const { trackTransaction } = useRewardsActivity();
  const apiFetch = useApiFetch();
  const submissions = useRef(new Map<string, RevokeSubmission>());
  const latestSubmission = useRef<RevokeSubmission>(null);
  const [ transaction, setTransaction ] = useState<RevokeSubmissionState>();

  const revoke = useCallback(async(approval: AllowanceType, chainId: number) => {
    if (!userAddress) return false;
    const key = [ userAddress, chainId, approval.address, approval.spender, approval.tokenId ?? 'operator' ].join(':');
    let submission = submissions.current.get(key);
    if (!submission) {
      submission = new RevokeSubmission({
        submit: async() => {
          await switchChainAsync({ chainId });
          return writeContractAsync({
            ...getRevokeContractParameters(approval), account: userAddress, chainId,
          });
        },
        receipt: (hash) => waitForTransactionReceipt(wagmiConfig.config, { hash, chainId }),
        verify: async(receipt) => {
          if (approval.type === 'ERC-20') return;
          const publicClient = getPublicClient(wagmiConfig.config, { chainId });
          if (!publicClient) throw new Error('Unable to verify the revoked approval on this chain.');
          await verifyNftRevocation(approval, userAddress, receipt.blockNumber, publicClient);
        },
        registerRewards: () => trackTransaction(userAddress, approval.address, String(chainId)),
        confirmRewards: (hash, token, signal) => apiFetch('rewards:user_activity_track_tx_confirm', {
          fetchParams: { method: 'POST', body: { tx_hash: hash, token }, signal },
        }),
      }, (state) => {
        setTransaction(state);
        if (state.rewards === 'failed') {
          toaster.error({ title: 'Rewards unavailable', description: 'Retry rewards without resubmitting the revoke transaction.' });
        }
      });
      submissions.current.set(key, submission);
    }
    latestSubmission.current = submission;
    const success = await submission.submit();
    if (success) {
      mixpanel.logEvent(mixpanel.EventTypes.WALLET_ACTION, {
        Action: 'Send Transaction', Address: userAddress, AppId: 'revoke',
        Source: 'Essential dapps', ChainId: String(chainId),
      });
      toaster.success({ title: 'Success', description: 'Approval revoked successfully.' });
    } else {
      toaster.error({
        title: 'Error',
        description: submission.state.error instanceof Error ? submission.state.error.message : 'Unable to confirm the revoke transaction.',
      });
    }
    return success;
  }, [ userAddress, writeContractAsync, switchChainAsync, trackTransaction, apiFetch ]);

  const retryRewards = useCallback(() => latestSubmission.current?.retryRewards(), []);
  return Object.assign(revoke, { transaction, retryRewards });
}
