import { Box, chakra, Grid, GridItem } from '@chakra-ui/react';
import BigNumber from 'bignumber.js';
import React from 'react';

import config from 'configs/app';
import useApiQuery from 'lib/api/useApiQuery';
import { useMultichainContext } from 'lib/contexts/multichain';
import DataFetchAlert from 'ui/shared/DataFetchAlert';
import { ScanStatCard } from 'ui/shared/scan';

import { formatCounterUnits, formatCounterValue } from './counterFormatting';
import { explorerNumber } from './explorerCharts';

interface Counter {
  label: string;
  value: string | number | null | undefined;
  units?: string;
}

function CounterGroup({ source, counters, isError, isPending }: {
  source: string;
  counters: Array<Counter>;
  isError: boolean;
  isPending: boolean;
}) {
  return (
    <Box display="contents" data-stats-counter-source={ source }>
      { isError ? <GridItem role="alert" colSpan={{ base: 1, sm: 2, lg: 4 }}><DataFetchAlert/></GridItem> : counters.map((counter) => {
        const value = explorerNumber(counter.value);
        return (
          <ScanStatCard
            key={ counter.label }
            label={ counter.label }
            value={ value === undefined ? <chakra.span color="text.muted">—</chakra.span> :
              `${ formatCounterValue(String(value)) }${ formatCounterUnits(counter.units) }` }
            isLoading={ isPending }
          />
        );
      }) }
    </Box>
  );
}

export default function ExplorerNumberWidgetsList() {
  const context = useMultichainContext();
  const currency = (context?.chain.app_config || config).chain.currency;
  const stats = useApiQuery('general:stats');
  const transactions = useApiQuery('general:txs_stats');
  const markets = useApiQuery('general:paxeer_x_feed_markets');
  const data = stats.data;
  const txs = transactions.data;
  const blockTime = explorerNumber(data?.average_block_time);

  function fee(value: string | undefined): string | undefined {
    return explorerNumber(value) === undefined ? undefined : BigNumber(value!).shiftedBy(-currency.decimals).toFixed();
  }

  const networkCounters: Array<Counter> = [
    { label: 'Total blocks', value: data?.total_blocks },
    { label: 'Total addresses', value: data?.total_addresses },
    { label: 'Total transactions', value: data?.total_transactions },
    { label: 'Transactions today', value: data?.transactions_today },
    { label: 'Average block time', value: blockTime === undefined ? undefined : blockTime / 1000, units: 's' },
    { label: 'Total gas used', value: data?.total_gas_used },
    { label: 'Gas used today', value: data?.gas_used_today },
    { label: 'Network utilisation', value: data?.network_utilization_percentage, units: '%' },
    { label: 'Gas price (slow)', value: data?.gas_prices?.slow?.price, units: 'Gwei' },
    { label: 'Gas price (average)', value: data?.gas_prices?.average?.price, units: 'Gwei' },
    { label: 'Gas price (fast)', value: data?.gas_prices?.fast?.price, units: 'Gwei' },
    { label: 'PAX price', value: data?.coin_price, units: 'USD' },
    { label: 'PAX market value', value: data?.market_cap, units: 'USD' },
    { label: 'SID price', value: data?.secondary_coin_price, units: 'USD' },
    { label: 'SID market value', value: data?.secondary_coin_market_cap, units: 'USD' },
  ];
  const transactionCounters: Array<Counter> = [
    { label: 'Transactions (24h)', value: txs?.transactions_count_24h },
    { label: 'Total transactions (transaction statistics)', value: txs?.total_transactions_count },
    { label: 'Pending transactions', value: txs?.pending_transactions_count },
    { label: 'Total transaction fees (24h)', value: fee(txs?.transaction_fees_sum_24h), units: currency.symbol },
    { label: 'Average transaction fee (24h)', value: fee(txs?.transaction_fees_avg_24h), units: currency.symbol },
    { label: 'Price updates (24h)', value: txs?.price_updates_count_24h },
    { label: 'Total price updates', value: txs?.total_price_updates_count },
  ];

  return (
    <Grid
      data-stats-number-widgets
      gridTemplateColumns={{ base: 'repeat(1, minmax(0, 1fr))', sm: 'repeat(2, minmax(0, 1fr))', lg: 'repeat(4, minmax(0, 1fr))' }}
      gap={{ base: 2, lg: 3 }}
    >
      <CounterGroup source="stats" counters={ networkCounters } isError={ stats.isError } isPending={ stats.isPending }/>
      <CounterGroup source="transactions" counters={ transactionCounters } isError={ transactions.isError } isPending={ transactions.isPending }/>
      <CounterGroup source="markets" counters={ [ { label: 'Price feed markets', value: markets.data?.items?.length } ] }
        isError={ markets.isError } isPending={ markets.isPending }/>
    </Grid>
  );
}
