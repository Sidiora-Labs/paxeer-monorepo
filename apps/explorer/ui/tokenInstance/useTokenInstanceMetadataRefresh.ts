import { useQueryClient } from '@tanstack/react-query';
import React from 'react';

import type { TokenInstance } from 'types/api/token';

import useApiFetch from 'lib/api/useApiFetch';
import { getResourceKey } from 'lib/api/useApiQuery';
import getErrorMessage from 'lib/errors/getErrorMessage';
import { useSocket } from 'lib/socket/context';
import { toaster } from 'toolkit/chakra/toaster';
import { MINUTE, SECOND } from 'toolkit/utils/consts';
import useReCaptcha from 'ui/shared/reCaptcha/useReCaptcha';

import { useMetadataUpdateContext } from './contexts/metadataUpdate';

const TOAST_ID = 'token-instance-metadata-fetcher';

export interface TokenInstanceMetadataNotFetched {
  token_id: string;
  reason: 'retry_cooldown' | 'error';
}

export default function useTokenInstanceMetadataRefresh(hash: string, id: string) {
  const { status, setStatus } = useMetadataUpdateContext() || {};
  const { socket } = useSocket() || {};
  const apiFetch = useApiFetch();
  const queryClient = useQueryClient();
  const recaptcha = useReCaptcha();
  const { fetchProtectedResource } = recaptcha;
  const isUpdating = status === 'MODAL_OPENED' || status === 'WAITING_FOR_RESPONSE';
  const statusRef = React.useRef(status);
  statusRef.current = status;

  React.useEffect(() => {
    if (!isUpdating) {
      return;
    }

    let active = true;
    let finished = false;
    let completionReceived = false;
    let completionFetchPending = false;
    let timedOut = false;
    let requestStarted = statusRef.current === 'WAITING_FOR_RESPONSE';
    let reconciliation: Promise<void> | undefined;
    const queryKey = getResourceKey('general:token_instance', { pathParams: { hash, id } });
    const previousInstance = queryClient.getQueryData<TokenInstance>(queryKey);
    const previousMetadata = JSON.stringify(previousInstance?.metadata);

    const fail = (description = 'The refreshing process has failed. Please try again.') => {
      if (!active || finished) {
        return;
      }
      finished = true;
      window.clearTimeout(timeoutId);
      setStatus?.('ERROR');
      toaster.error({ id: TOAST_ID, title: 'Error', description, duration: 5 * SECOND });
    };

    const reconcile = () => {
      if (!active || finished || reconciliation) {
        return;
      }

      const completionAtStart = completionReceived;
      reconciliation = (async() => {
        try {
          await queryClient.cancelQueries({ queryKey, exact: true });
          await queryClient.invalidateQueries({ queryKey, exact: true, refetchType: 'none' });
          const instance = await queryClient.fetchQuery({
            queryKey,
            queryFn: ({ signal }) => apiFetch<'general:token_instance', TokenInstance>('general:token_instance', {
              pathParams: { hash, id },
              fetchParams: { signal },
            }) as Promise<TokenInstance>,
            staleTime: 0,
            retry: false,
          });

          if (!active || finished) {
            return;
          }
          if (!completionAtStart && previousInstance && JSON.stringify(instance.metadata) === previousMetadata) {
            if (timedOut && !completionFetchPending) {
              fail('The refreshing process has not completed. Please try again.');
            }
            return;
          }
          finished = true;
          window.clearTimeout(timeoutId);
          setStatus?.('SUCCESS');
          toaster.success({
            id: TOAST_ID,
            title: 'Success!',
            description: 'Metadata has been refreshed',
            duration: 5 * SECOND,
          });
        } catch (error) {
          fail(getErrorMessage(error) || 'Unable to retrieve refreshed metadata. Please try again.');
        } finally {
          reconciliation = undefined;
          if (completionFetchPending) {
            completionFetchPending = false;
            reconcile();
          }
        }
      })();
    };

    const timeoutId = window.setTimeout(() => {
      timedOut = true;
      if (requestStarted) {
        reconcile();
      } else {
        fail('Unable to subscribe to metadata updates. Please try again.');
      }
    }, 2 * MINUTE);

    const channel = socket?.channel(`token_instances:${ hash.toLowerCase() }`);
    channel?.on('fetched_token_instance_metadata', (payload: { token_id: string }) => {
      if (String(payload.token_id) !== id) {
        return;
      }
      completionReceived = true;
      if (reconciliation) {
        completionFetchPending = true;
      } else {
        reconcile();
      }
    });
    channel?.on('not_fetched_token_instance_metadata', (payload: TokenInstanceMetadataNotFetched) => {
      if (String(payload.token_id) !== id) {
        return;
      }
      fail(payload.reason === 'retry_cooldown' ?
        'Metadata refresh is on cooldown. Please wait before trying again.' :
        'Unable to fetch token instance metadata. Please try again.');
    });

    channel?.join().receive('ok', () => {
      if (!active || finished) {
        return;
      }
      if (requestStarted) {
        reconcile();
        return;
      }

      requestStarted = true;
      toaster.loading({
        id: TOAST_ID,
        title: 'Please wait',
        description: 'Refetching metadata request sent',
        duration: Infinity,
      });

      fetchProtectedResource((recaptchaToken?: string) => apiFetch('general:token_instance_refresh_metadata', {
        pathParams: { hash, id },
        fetchParams: {
          method: 'PATCH',
          body: { recaptcha_response: recaptchaToken },
          headers: { ...(recaptchaToken && { 'recaptcha-v2-response': recaptchaToken }) },
        },
      })).then(() => {
        if (active && !finished && !completionReceived) {
          setStatus?.('WAITING_FOR_RESPONSE');
        }
      }).catch((error: unknown) => {
        if (!completionReceived) {
          fail(getErrorMessage(error) || 'Unable to initialize metadata update');
        }
      });
    }).receive('error', () => fail('Unable to subscribe to metadata updates. Please try again.'));

    return () => {
      active = false;
      window.clearTimeout(timeoutId);
      channel?.leave();
    };
  }, [ apiFetch, fetchProtectedResource, hash, id, isUpdating, queryClient, setStatus, socket ]);

  React.useEffect(() => () => toaster.remove(TOAST_ID), []);

  return recaptcha;
}
