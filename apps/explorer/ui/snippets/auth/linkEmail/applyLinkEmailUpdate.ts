import type { useUserUpdateRequest } from '@dynamic-labs/sdk-react-core';
import type { QueryClient } from '@tanstack/react-query';

import type { UserInfo } from 'types/api/account';

import { getResourceKey } from 'lib/api/useApiQuery';
import * as mixpanel from 'lib/mixpanel/index';
import { toaster } from 'toolkit/chakra/toaster';

type EmailUpdate = ReturnType<ReturnType<typeof useUserUpdateRequest>['updateUserWithModal']>;

export default async function applyLinkEmailUpdate(update: EmailUpdate, queryClient: QueryClient): Promise<void> {
  try {
    const fields = await update;
    const email = fields.email;
    if (typeof email !== 'string' || !email) {
      throw new Error('The provider did not return an email address');
    }

    queryClient.setQueryData(getResourceKey('general:user_info'), (prevData: UserInfo | undefined) => {
      return prevData ? { ...prevData, email } : prevData;
    });
    mixpanel.logEvent(mixpanel.EventTypes.ACCOUNT_LINK_INFO, {
      Status: 'Finished',
      Type: 'Email',
      Source: 'Profile dropdown',
    });
  } catch (error) {
    if (error === 'User cancelled') {
      toaster.info({ title: 'Email linking cancelled', description: 'Your profile has not changed.' });
      return;
    }

    toaster.error({
      title: 'Unable to link email',
      description: 'Please try linking your email again. If the problem persists, contact support.',
    });
  }
}
