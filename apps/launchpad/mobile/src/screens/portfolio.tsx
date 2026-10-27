import React from 'react';
import { Text } from 'react-native';
import {
  Card,
  Copyable,
  ListPage,
  ResourceStatus,
  useResource,
  ui,
  type Runtime,
} from '../navigation.tsx';
import type { PortfolioResponse } from '../runtime/routes/account.ts';
export function Portfolio({ runtime }: { runtime: Runtime }) {
  const state = useResource<PortfolioResponse>(
    runtime,
    'portfolio',
    (s) => runtime.services.portfolio(s),
    true,
  );
  const data = state.data?.data;
  return (
    <ListPage
      runtime={runtime}
      stateKey="private:portfolio"
      title="Portfolio"
      data={data?.items ?? []}
      keyOf={(item) => item.positionId}
      header={
        <>
          <ResourceStatus {...state} />
          {data && (
            <Card>
              <Text style={ui.text}>
                Pricing: {data.summary.pricingQuality} · Cost basis:{' '}
                {data.summary.basisQuality}
              </Text>
              <Copyable
                label="Known priced subtotal (USDL raw, 6 decimals)"
                value={data.summary.pricedSubtotalUsdlRaw}
              />
              <Copyable
                label="Realized profit/loss (USDL raw, signed)"
                value={
                  data.summary.realizedPnlRaw ?? 'Unavailable: incomplete basis'
                }
              />
              <Text style={ui.subtle}>
                Unpriced assets: {data.summary.unpricedAssetCount}
              </Text>
            </Card>
          )}
        </>
      }
      row={(item) => (
        <Card>
          <Copyable label="Token" value={item.tokenAddress} />
          <Copyable
            label="Token quantity (raw units)"
            value={item.balanceRaw}
          />
          <Text style={ui.text}>
            Pricing {item.pricingQuality}; basis {item.basisQuality}
          </Text>
          <Copyable
            label="Unrealized profit/loss (USDL raw, signed)"
            value={item.unrealizedPnlRaw ?? 'Unavailable'}
          />
        </Card>
      )}
    />
  );
}
