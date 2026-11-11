import React, { useState } from 'react';
import { Linking, Text } from 'react-native';
import {
  Button,
  Card,
  Copyable,
  ErrorNotice,
  ListPage,
  ResourceStatus,
  showAction,
  useResource,
  ui,
  type Runtime,
} from '../navigation.tsx';
import type { PortfolioResponse } from '../runtime/routes/account.ts';
import {
  claimableChainPositions,
  claimChainRevenueBatch,
} from '../runtime/chain-launch.ts';
export function Portfolio({ runtime }: { runtime: Runtime }) {
  const [claimError, setClaimError] = useState<string>();
  const [claimHash, setClaimHash] = useState<string>();
  const state = useResource<PortfolioResponse>(
    runtime,
    'portfolio',
    (s) => runtime.services.portfolio(s),
    true,
  );
  const data = state.data?.data;
  const tokens = data?.items.map((item) => item.tokenAddress) ?? [];
  const revenue = useResource(
    runtime,
    `private:portfolio-chain-revenue:${tokens.join(',')}`,
    (s) =>
      data && runtime.session.account
        ? claimableChainPositions(
            runtime.services,
            tokens,
            runtime.session.account,
            s,
          )
        : Promise.resolve([]),
    true,
  );
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
          <Button
            label="Launch a chain in the web app"
            onPress={() =>
              void showAction(
                () =>
                  Linking.openURL(
                    `${runtime.services.config.canonicalOrigin}/create/chain`,
                  ),
                setClaimError,
              )
            }
          />
          {claimError && <ErrorNotice message={claimError} />}
          {claimHash && (
            <Copyable
              label="Chain revenue claim transaction"
              value={claimHash}
            />
          )}
          {data && <ResourceStatus {...revenue} />}
          {revenue.data?.map((position) => (
            <Card key={position.chain.chain}>
              <Copyable label="Chain token" value={position.chain.token} />
              <Copyable label="Registry number" value={position.chain.chain} />
              <Copyable
                label="Claimable chain revenue (wei)"
                value={position.total}
              />
              {position.epochs
                .filter((epoch) => BigInt(epoch.claimableAmount) > 0n)
                .map((epoch) => (
                  <Text key={epoch.epoch} style={ui.subtle}>
                    Epoch {epoch.epoch}: {epoch.claimableAmount} wei
                  </Text>
                ))}
              <Button
                label={`Claim chain ${position.chain.chain} revenue`}
                disabled={
                  !runtime.wallet?.sendTransaction ||
                  revenue.offline ||
                  !!runtime.pending.current
                }
                onPress={() =>
                  void showAction(async () => {
                    if (!runtime.wallet || !runtime.session.account)
                      throw new Error('Connected wallet required');
                    const result = await claimChainRevenueBatch(
                      runtime.wallet,
                      runtime.services,
                      position.chain.chain,
                      position.epochs
                        .filter((epoch) => BigInt(epoch.claimableAmount) > 0n)
                        .map((epoch) => epoch.epoch),
                      runtime.session.account,
                      runtime.session.signal,
                    );
                    setClaimHash(result.transactionHash);
                    revenue.reload();
                  }, setClaimError)
                }
              />
            </Card>
          ))}
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
