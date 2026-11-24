import React, { useState } from 'react';
import {
  Button,
  Text,
  Card,
  Copyable,
  ErrorNotice,
  ListPage,
  ResourceStatus,
  PageControls,
  usePagination,
  showAction,
  useResource,
  useSurface,
  type Runtime,
} from '../navigation.tsx';
import { pageResourceKey } from '../runtime/flow-state.ts';
export function Watchlists({ runtime }: { runtime: Runtime }) {
  const { ui } = useSurface();
  const page = usePagination('watchlist');
  const state = useResource(
    runtime,
    pageResourceKey('watchlist', page.cursor),
    (s) => runtime.services.watchlist(s, page.cursor),
    true,
  );
  const [error, setError] = useState<string>();
  return (
    <ListPage
      runtime={runtime}
      stateKey="private:watchlist"
      title="Watchlist"
      data={state.data?.data.items ?? []}
      keyOf={(item) => item.poolAddress}
      header={
        <>
          <ResourceStatus {...state} />
          <PageControls
            page={page}
            nextCursor={state.data?.nextCursor}
            loading={state.loading}
            offline={state.offline}
          />
          {error && <ErrorNotice message={error} />}
          <Text style={ui.subtle}>
            Up to 500 watched pools per account.
          </Text>
        </>
      }
      row={(item) => (
        <Card>
          <Copyable label="Pool" value={item.poolAddress} />
          <Button
            label="Open token"
            onPress={() =>
              runtime.navigate({
                name: 'token',
                address: item.poolAddress,
              })
            }
          />
          <Button
            label="Remove from watchlist"
            disabled={state.offline}
            onPress={() =>
              showAction(
                () =>
                  runtime.services.unwatch(
                    item.poolAddress,
                    runtime.session.signal,
                  ),
                setError,
                () => {
                  page.reset();
                  state.reload();
                },
              )
            }
          />
        </Card>
      )}
    />
  );
}
