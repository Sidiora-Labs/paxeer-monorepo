import type { DehydratedState } from '@tanstack/react-query';
import { hashKey } from '@tanstack/react-query';
import React from 'react';

import type { ResourceName, ResourcePayload } from 'lib/api/resources';
import type { GetResourceKeyParams } from 'lib/api/useApiQuery';
import { getResourceKey } from 'lib/api/useApiQuery';

const PageQuerySnapshot = React.createContext<DehydratedState | undefined>(undefined);

export const PageQuerySnapshotProvider = PageQuerySnapshot.Provider;

export function usePageQuerySnapshot<R extends ResourceName>(resource: R, params?: GetResourceKeyParams<R>) {
  const snapshot = React.useContext(PageQuerySnapshot);
  const queryHash = hashKey(getResourceKey(resource, params));
  return snapshot?.queries.find((query) => query.queryHash === queryHash)?.state.data as ResourcePayload<R> | undefined;
}
