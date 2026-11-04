import React from 'react';
import type { UseFormReturn } from 'react-hook-form';

import type { FormFields } from './types';
import type { SmartContract, SmartContractVerificationResponse } from 'types/api/contract';

import useApiFetch from 'lib/api/useApiFetch';
import capitalizeFirstLetter from 'lib/capitalizeFirstLetter';
import getErrorMessage from 'lib/errors/getErrorMessage';
import getErrorObjPayload from 'lib/errors/getErrorObjPayload';
import useSocketChannel from 'lib/socket/useSocketChannel';
import useSocketMessage from 'lib/socket/useSocketMessage';

import { formatSocketErrors, prepareRequestBody } from './utils';

export const VERIFICATION_POLL_INTERVAL = 3000;
export const VERIFICATION_TIMEOUT = 120000;

export interface VerificationSubmission {
  address: string;
  resolve: () => void;
  controller: AbortController;
  poll?: ReturnType<typeof setTimeout>;
  timeout?: ReturnType<typeof setTimeout>;
}

interface Params {
  submissionRef: React.MutableRefObject<VerificationSubmission | undefined>;
  address: string;
  isDisabled: boolean;
  formApi: Pick<UseFormReturn<FormFields>, 'getValues' | 'setError'>;
  onSuccess: (address: string) => void;
}

export default function useVerificationSubmission({ submissionRef, address, isDisabled, formApi, onSuccess }: Params) {
  const apiFetch = useApiFetch();
  const { getValues, setError } = formApi;
  const onSuccessRef = React.useRef(onSuccess);
  onSuccessRef.current = onSuccess;

  const finish = React.useCallback((submission: VerificationSubmission) => {
    if (submissionRef.current !== submission) {
      return false;
    }
    submissionRef.current = undefined;
    clearTimeout(submission.poll);
    clearTimeout(submission.timeout);
    submission.controller.abort();
    submission.resolve();
    return true;
  }, [ submissionRef ]);

  const showResult = React.useCallback((payload: SmartContractVerificationResponse) => {
    const submission = submissionRef.current;
    if (!submission || !finish(submission)) {
      return;
    }
    if (payload.status === 'success') {
      onSuccessRef.current(submission.address);
      return;
    }
    const errors = formatSocketErrors(payload.errors).filter(Boolean).filter(([ field ]) => getValues(field));
    if (errors.length) {
      errors.forEach(([ field, error ]) => setError(field, error));
    } else {
      const message = Object.values(payload.errors).map(value => value.join(', ')).join('\n\n');
      setError('root', { message: capitalizeFirstLetter(message) || 'Contract verification failed. Please try again.' });
    }
  }, [ submissionRef, finish, getValues, setError ]);

  const handleSocketError = React.useCallback(() => {
    const submission = submissionRef.current;
    if (submission && finish(submission)) {
      setError('root', { message: 'Verification connection lost. Check the contract status or try again.' });
    }
  }, [ submissionRef, finish, setError ]);

  const channel = useSocketChannel({
    topic: `addresses:${ address?.toLowerCase() }`,
    isDisabled,
    onSocketClose: handleSocketError,
    onSocketError: handleSocketError,
  });
  useSocketMessage({ channel, event: 'verification_result', handler: showResult });

  React.useEffect(() => () => {
    const submission = submissionRef.current;
    if (submission) {
      finish(submission);
    }
  }, [ address, isDisabled, finish, submissionRef ]);

  return React.useCallback((data: FormFields) => {
    if (submissionRef.current) {
      return Promise.resolve();
    }
    return new Promise<void>((resolve) => {
      const submission: VerificationSubmission = {
        address: data.address.toLowerCase(), resolve, controller: new AbortController(),
      };
      submissionRef.current = submission;
      const signal = submission.controller.signal;
      submission.timeout = setTimeout(() => {
        if (finish(submission)) {
          setError('root', { message: 'Verification result is taking too long. Check the contract status or try again.' });
        }
      }, VERIFICATION_TIMEOUT);

      const poll = async() => {
        try {
          const contract = await apiFetch<'general:contract', SmartContract>('general:contract', {
            pathParams: { hash: submission.address }, fetchParams: { signal },
          });
          if (submissionRef.current !== submission) {
            return;
          }
          if ('is_verified' in contract && contract.is_verified && !contract.is_partially_verified) {
            showResult({ status: 'success' });
            return;
          }
        } catch {
          if (submissionRef.current === submission) {
            setError('root', { message: 'Unable to check verification status. Retrying...' });
          }
        }
        if (submissionRef.current === submission) {
          submission.poll = setTimeout(poll, VERIFICATION_POLL_INTERVAL);
        }
      };

      const send = async() => {
        try {
          await apiFetch('general:contract_verification_via', {
            pathParams: { method: data.method[0], hash: submission.address },
            fetchParams: { method: 'POST', body: prepareRequestBody(data), signal },
          });
          if (submissionRef.current === submission) {
            submission.poll = setTimeout(poll, VERIFICATION_POLL_INTERVAL);
          }
        } catch (error) {
          if (!finish(submission)) {
            return;
          }
          const payload = getErrorObjPayload<{ message?: string }>(error);
          setError('root', { message: getErrorMessage(payload) || getErrorMessage(error) || 'Unable to submit verification. Please try again.' });
        }
      };
      void send();
    });
  }, [ apiFetch, submissionRef, finish, setError, showResult ]);
}
