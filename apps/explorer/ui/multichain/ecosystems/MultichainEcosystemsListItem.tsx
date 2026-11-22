import { chakra, Grid, HStack } from '@chakra-ui/react';
import React from 'react';

import type * as multichain from '@blockscout/multichain-aggregator-types';
import type { ClusterChainConfig } from 'types/multichain';

import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { Stat } from 'toolkit/chakra/stat';
import { TruncatedText } from 'toolkit/components/truncation/TruncatedText';
import { spacing } from 'toolkit/theme/foundations/spacing';
import CopyToClipboard from 'ui/shared/CopyToClipboard';
import ChainIcon from 'ui/shared/externalChains/ChainIcon';
import ListItemMobile from 'ui/shared/ListItemMobile/ListItemMobile';

interface Props {
  data: multichain.ChainMetrics;
  chainInfo: ClusterChainConfig | undefined;
  isLoading?: boolean;
}

const MultichainEcosystemsListItem = ({ data, chainInfo, isLoading }: Props) => {
  const activeAddresses = {
    value: data.active_accounts?.current_full_week ? Number(data.active_accounts.current_full_week) : 0,
    delta: data.active_accounts?.wow_diff_percent ? Number(data.active_accounts.wow_diff_percent) : 0,
  };

  const newAddresses = {
    value: data.new_addresses?.current_full_week ? Number(data.new_addresses.current_full_week) : 0,
    delta: data.new_addresses?.wow_diff_percent ? Number(data.new_addresses.wow_diff_percent) : 0,
  };

  const dailyTransactions = {
    value: data.daily_transactions?.current_full_week ? Number(data.daily_transactions.current_full_week) : 0,
    delta: data.daily_transactions?.wow_diff_percent ? Number(data.daily_transactions.wow_diff_percent) : 0,
  };

  return (
    <ListItemMobile rowGap={ 3 } py={ 4 } fontSize="body" lineHeight="small" alignItems="stretch" containerType="inline-size">
      <HStack justifyContent="space-between" fontWeight="semibold" flexWrap="wrap" minW={ 0 }>
        <HStack maxW="full" minW={ 0 } flex="1 1 var(--chakra-spacing-30)">
          <ChainIcon data={ chainInfo } isLoading={ isLoading }/>
          <Link
            data-touch-inline
            href={ chainInfo?.explorer_url }
            external
            loading={ isLoading }
            maxW="100%"
          >
            <TruncatedText text={ chainInfo?.name ?? 'Unknown chain' } loading={ isLoading }/>
          </Link>
        </HStack>
        <HStack gap={ 0 } flexShrink={ 0 }>
          <Skeleton loading={ isLoading } color="text.secondary"><span>{ data.chain_id }</span></Skeleton>
          <CopyToClipboard text={ String(data.chain_id) } isLoading={ isLoading }/>
        </HStack>
      </HStack>
      <Grid
        w="full"
        fontVariantNumeric="tabular-nums"
        minW={ 0 }
        gridTemplateColumns="minmax(0, 1fr)"
        columnGap={ 2 }
        rowGap={ 3 }
        css={{
          [ `@container (min-width: ${ spacing['75'].value })` ]: {
            gridTemplateColumns: 'calc(var(--chakra-spacing-1) * 35) minmax(0, 1fr)',
            '& > [data-metric-value]': {
              flexDirection: 'row-reverse',
              justifyContent: 'flex-start',
              textAlign: 'right',
            },
          },
        }}
      >
        <Skeleton loading={ isLoading } fontWeight="medium">
          <span>Active addresses</span>
          <chakra.span color="text.secondary"> 7D</chakra.span>
        </Skeleton>
        <HStack data-metric-value gap={ 1 } flexWrap="wrap" minW={ 0 }>
          <Skeleton loading={ isLoading }>
            <span>{ activeAddresses.value.toLocaleString() }</span>
          </Skeleton>
          { activeAddresses.delta ? (
            <Skeleton loading={ isLoading }>
              <Stat.Root positive={ activeAddresses.delta > 0 }>
                <Stat.ValueText>
                  { activeAddresses.delta.toFixed(2) }%
                </Stat.ValueText>
                { activeAddresses.delta > 0 ? <Stat.UpIndicator/> : <Stat.DownIndicator/> }
              </Stat.Root>
            </Skeleton>
          ) : null }
        </HStack>

        <Skeleton loading={ isLoading } fontWeight="medium">
          <span>New addresses</span>
          <chakra.span color="text.secondary"> 7D</chakra.span>
        </Skeleton>
        <HStack data-metric-value gap={ 1 } flexWrap="wrap" minW={ 0 }>
          <Skeleton loading={ isLoading }>
            <span>{ newAddresses.value.toLocaleString() }</span>
          </Skeleton>
          { newAddresses.delta ? (
            <Skeleton loading={ isLoading }>
              <Stat.Root positive={ newAddresses.delta > 0 }>
                <Stat.ValueText>
                  { newAddresses.delta.toFixed(2) }%
                </Stat.ValueText>
                { newAddresses.delta > 0 ? <Stat.UpIndicator/> : <Stat.DownIndicator/> }
              </Stat.Root>
            </Skeleton>
          ) : null }
        </HStack>

        <Skeleton loading={ isLoading } fontWeight="medium">
          <span>Daily txs</span>
          <chakra.span color="text.secondary"> 7D</chakra.span>
        </Skeleton>
        <HStack data-metric-value gap={ 1 } flexWrap="wrap" minW={ 0 }>
          <Skeleton loading={ isLoading }>
            <span>{ dailyTransactions.value.toLocaleString() }</span>
          </Skeleton>
          { dailyTransactions.delta ? (
            <Skeleton loading={ isLoading }>
              <Stat.Root positive={ dailyTransactions.delta > 0 }>
                <Stat.ValueText>
                  { dailyTransactions.delta.toFixed(2) }%
                </Stat.ValueText>
                { dailyTransactions.delta > 0 ? <Stat.UpIndicator/> : <Stat.DownIndicator/> }
              </Stat.Root>
            </Skeleton>
          ) : null }
        </HStack>

        <Skeleton loading={ isLoading } fontWeight="medium">
          <span>TPS</span>
        </Skeleton>
        <Skeleton data-metric-value loading={ isLoading }>
          <span>{ Number(data.tps ?? 0).toLocaleString(undefined, { maximumFractionDigits: 2 }) }</span>
        </Skeleton>
      </Grid>
    </ListItemMobile>
  );
};

export default React.memo(MultichainEcosystemsListItem);
