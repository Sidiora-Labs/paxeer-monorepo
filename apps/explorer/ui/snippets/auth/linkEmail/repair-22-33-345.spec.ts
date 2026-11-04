// @vitest-environment jsdom

import type { useUserUpdateRequest } from '@dynamic-labs/sdk-react-core';
import { QueryClient, QueryObserver } from '@tanstack/react-query';

import type { UserInfo } from 'types/api/account';

import { getResourceKey } from 'lib/api/useApiQuery';
import { toaster } from 'toolkit/chakra/toaster';

import { afterEach, expect, test } from 'vitest';

import applyLinkEmailUpdate from './applyLinkEmailUpdate';
import results from './provider-results.json';

type EmailUpdate = ReturnType<ReturnType<typeof useUserUpdateRequest>['updateUserWithModal']>;

const profile: UserInfo = {
  email: 'previous@example.org',
  name: 'Explorer account',
  nickname: 'explorer',
  address_hash: '0x1234',
  avatar: 'avatar.svg',
};
const key = getResourceKey('general:user_info');

afterEach(() => {
  toaster.remove();
});

function createProfileCache() {
  const queryClient = new QueryClient();
  queryClient.setQueryData(key, profile);
  return queryClient;
}

async function dynamicEmailSuccessPreservesProfileFields() {
  const queryClient = createProfileCache();
  const observer = new QueryObserver<UserInfo>(queryClient, { queryKey: key, enabled: false });
  const observed: Array<UserInfo | undefined> = [];
  const unsubscribe = observer.subscribe((result) => observed.push(result.data));
  const update: EmailUpdate = Promise.resolve(results.success);

  await expect(applyLinkEmailUpdate(update, queryClient)).resolves.toBeUndefined();

  expect(queryClient.getQueryData(key)).toEqual({ ...profile, email: results.success.email });
  expect(observed).toEqual([ { ...profile, email: results.success.email } ]);
  expect(toaster.getVisibleToasts()).toEqual([]);
  unsubscribe();
  queryClient.clear();
}

async function dynamicEmailCancellationIsNotAProviderError() {
  const queryClient = createProfileCache();
  const previous = queryClient.getQueryData(key);
  const update: EmailUpdate = Promise.reject(results.cancellation);

  await expect(applyLinkEmailUpdate(update, queryClient)).resolves.toBeUndefined();

  expect(queryClient.getQueryData(key)).toBe(previous);
  expect(toaster.getVisibleToasts()).toEqual([
    expect.objectContaining({ type: 'info', title: 'Email linking cancelled', description: 'Your profile has not changed.' }),
  ]);
  queryClient.clear();
}

async function dynamicEmailProviderRejectionsAreHandledAndRetryable() {
  const queryClient = createProfileCache();
  const previous = queryClient.getQueryData(key);

  for (const rejection of [ results.verificationRejection, results.validationRejection, new Error('Provider unavailable') ]) {
    const update: EmailUpdate = Promise.reject(rejection);
    await expect(applyLinkEmailUpdate(update, queryClient)).resolves.toBeUndefined();
    expect(queryClient.getQueryData(key)).toBe(previous);
    expect(toaster.getVisibleToasts()).toEqual([
      expect.objectContaining({
        type: 'error',
        title: 'Unable to link email',
        description: 'Please try linking your email again. If the problem persists, contact support.',
      }),
    ]);
    toaster.remove();
  }

  await applyLinkEmailUpdate(Promise.resolve(results.success), queryClient);
  expect(queryClient.getQueryData(key)).toEqual({ ...profile, email: results.success.email });
  expect(toaster.getVisibleToasts()).toEqual([]);
  queryClient.clear();
}

async function dynamicEmailRejectionDoesNotCreateAProfile() {
  const queryClient = new QueryClient();
  await expect(applyLinkEmailUpdate(Promise.reject(results.verificationRejection), queryClient)).resolves.toBeUndefined();
  expect(queryClient.getQueryData(key)).toBeUndefined();
  expect(queryClient.getQueryState(key)).toBeUndefined();
  expect(toaster.getVisibleToasts()).toEqual([
    expect.objectContaining({ type: 'error', title: 'Unable to link email' }),
  ]);
  queryClient.clear();
}

test(dynamicEmailSuccessPreservesProfileFields.name, dynamicEmailSuccessPreservesProfileFields);
test(dynamicEmailCancellationIsNotAProviderError.name, dynamicEmailCancellationIsNotAProviderError);
test(dynamicEmailProviderRejectionsAreHandledAndRetryable.name, dynamicEmailProviderRejectionsAreHandledAndRetryable);
test(dynamicEmailRejectionDoesNotCreateAProfile.name, dynamicEmailRejectionDoesNotCreateAProfile);
