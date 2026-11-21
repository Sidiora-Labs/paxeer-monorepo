import React from 'react';
import type { V2ContractTypes } from '../../../../../sdk/launchpad/packages/sdk/src/index.ts';
import {
  Button,
  Text,
  Card,
  Copyable,
  Field,
  ListPage,
  ResourceStatus,
  PageControls,
  usePagination,
  useResource,
  useRetainedInput,
  useSurface,
  type Runtime,
} from '../navigation.tsx';
import { pageResourceKey } from '../runtime/flow-state.ts';
type Row =
  | {
      kind: 'pool';
      value: V2ContractTypes['PoolListResponse']['data']['items'][number];
    }
  | {
      kind: 'search';
      value: V2ContractTypes['SearchResponse']['data']['items'][number];
    };
export function Discovery({ runtime }: { runtime: Runtime }) {
  const { ui } = useSurface();
  const page = usePagination('pools');
  const [query, setQuery] = useRetainedInput(
      runtime,
      'discovery',
      'query',
    ),
    [search, setSearch] = useRetainedInput(
      runtime,
      'discovery',
      'search',
    );
  const pools = useResource(
    runtime,
    pageResourceKey('pools', page.cursor),
    (s) => runtime.services.pools(s, page.cursor),
  );
  const results = useResource(runtime, `search:${search}`, (s) =>
    search
      ? runtime.services.search(search, s)
      : Promise.resolve(undefined),
  );
  const rows: Row[] = search
    ? (results.data?.data.items ?? []).map((value) => ({
        kind: 'search',
        value,
      }))
    : (pools.data?.data.items ?? []).map((value) => ({
        kind: 'pool',
        value,
      }));
  return (
    <ListPage
      runtime={runtime}
      stateKey={`discovery:${search}`}
      title="Discover"
      data={rows}
      keyOf={(row) =>
        row.kind === 'pool' ? row.value.poolAddress : row.value.id
      }
      header={
        <>
          <Field label="Search pools" value={query} onChange={setQuery} />
          <Button
            label="Search"
            onPress={() => setSearch(query.trim())}
          />
          <ResourceStatus {...(search ? results : pools)} />
          {!search && (
            <PageControls
              page={page}
              nextCursor={pools.data?.nextCursor}
              loading={pools.loading}
              offline={pools.offline}
            />
          )}
        </>
      }
      row={(row) =>
        row.kind === 'pool' ? (
          <Card>
            <Copyable label="Pool" value={row.value.poolAddress} />
            <Copyable label="Token" value={row.value.tokenAddress} />
            <Text style={ui.subtle}>
              Chain 125 · USDL quote · PAX gas
            </Text>
            <Button
              label="View token"
              onPress={(
                (address: string) => () =>
                  runtime.navigate({ name: 'token', address })
              )(row.value.poolAddress)}
            />
          </Card>
        ) : (
          <Card>
            <Text style={ui.text}>{row.value.label}</Text>
            {row.value.poolAddress && (
              <Button
                label="Open pool"
                onPress={(
                  (address: string) => () =>
                    runtime.navigate({ name: 'token', address })
                )(row.value.poolAddress)}
              />
            )}
          </Card>
        )
      }
    />
  );
}
