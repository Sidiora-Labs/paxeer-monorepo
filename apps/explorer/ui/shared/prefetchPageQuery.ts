import type { QueryClient } from '@tanstack/react-query';

import buildUrl from 'nextjs/utils/buildUrl';

import type { ResourceName, ResourcePayload } from 'lib/api/resources';
import type { GetResourceKeyParams } from 'lib/api/useApiQuery';
import { getResourceKey } from 'lib/api/useApiQuery';

export default async function prefetchPageQuery<R extends ResourceName>(
  client: QueryClient,
  resource: R,
  params: GetResourceKeyParams<R> = {},
) {
  await client.prefetchQuery({
    queryKey: getResourceKey(resource, params),
    queryFn: async() => {
      const url = buildUrl(resource, params.pathParams);
      const requestUrl = new URL(url);
      Object.entries(params.queryParams ?? {}).forEach(([ key, value ]) => {
        if (Array.isArray(value)) {
          value.forEach((item) => requestUrl.searchParams.append(key, item));
        } else if (value !== undefined && value !== null) {
          requestUrl.searchParams.append(key, String(value));
        }
      });
      const response = await fetch(requestUrl, { signal: AbortSignal.timeout(5_000) });
      if (!response.ok) {
        throw new Error(`${ response.status } ${ response.statusText }`);
      }
      return await response.json() as ResourcePayload<R>;
    },
  });
}
