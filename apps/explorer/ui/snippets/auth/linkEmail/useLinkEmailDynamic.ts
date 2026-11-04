import { useUserUpdateRequest } from '@dynamic-labs/sdk-react-core';
import { useQueryClient } from '@tanstack/react-query';
import React from 'react';

import * as mixpanel from 'lib/mixpanel/index';

import applyLinkEmailUpdate from './applyLinkEmailUpdate';

export default function useLinkEmailDynamic(): () => void {
  const { updateUserWithModal } = useUserUpdateRequest();
  const queryClient = useQueryClient();

  return React.useCallback(() => {
    mixpanel.logEvent(mixpanel.EventTypes.ACCOUNT_LINK_INFO, {
      Status: 'Started',
      Type: 'Email',
      Source: 'Profile dropdown',
    });
    void applyLinkEmailUpdate(updateUserWithModal([ 'email' ]), queryClient);
  }, [ queryClient, updateUserWithModal ]);
}
