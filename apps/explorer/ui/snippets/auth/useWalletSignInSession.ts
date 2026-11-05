import { useQueryClient } from '@tanstack/react-query';
import React from 'react';

import type * as rewards from '@blockscout/points-types';
import type { UserInfo } from 'types/api/account';

import useApiFetch from 'lib/api/useApiFetch';
import { getResourceKey } from 'lib/api/useApiQuery';

interface WalletProof {
  message: string;
  signature: string;
  authNonce?: string;
}

interface Props {
  isAuth?: boolean;
  fetchProtectedResource: <T>(fetcher: (token?: string) => Promise<T>, token?: string) => Promise<T>;
}

export default function useWalletSignInSession({ isAuth, fetchProtectedResource }: Props) {
  const apiFetch = useApiFetch();
  const queryClient = useQueryClient();

  return React.useCallback(async({ message, signature, authNonce }: WalletProof) => {
    const authResource = isAuth ? 'general:auth_link_address' : 'general:auth_siwe_verify';
    const profile = await fetchProtectedResource((recaptchaToken?: string) => apiFetch<typeof authResource, UserInfo>(authResource, {
      fetchParams: {
        method: 'POST',
        body: { message, signature, recaptcha_response: recaptchaToken },
        headers: {
          ...(recaptchaToken && { 'recaptcha-v2-response': recaptchaToken }),
        },
      },
    }));

    if (!('name' in profile)) {
      throw Error('Something went wrong');
    }

    const queryKey = getResourceKey('general:user_info');
    await queryClient.cancelQueries({ queryKey, exact: true });
    queryClient.setQueryData(queryKey, profile);

    if (!authNonce) {
      return { profile, rewardsToken: undefined, rewardsAuthFailed: false };
    }

    try {
      const response = await apiFetch('rewards:login', {
        fetchParams: {
          method: 'POST',
          body: { nonce: authNonce, message, signature },
        },
      }) as rewards.AuthLoginResponse;
      return { profile, rewardsToken: response.token, rewardsAuthFailed: false };
    } catch (error) {
      return { profile, rewardsToken: undefined, rewardsAuthFailed: true };
    }
  }, [ apiFetch, fetchProtectedResource, isAuth, queryClient ]);
}
