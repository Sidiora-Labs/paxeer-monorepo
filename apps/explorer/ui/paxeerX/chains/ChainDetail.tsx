import { Box, Flex, Grid, Text } from '@chakra-ui/react';
import { useQuery } from '@tanstack/react-query';
import React from 'react';

import config from 'configs/app';
import paxeerX from 'configs/app/features/paxeerX';
import useApiQuery from 'lib/api/useApiQuery';
import { publicClient } from 'lib/web3/client';
import { Button } from 'toolkit/chakra/button';
import { Link } from 'toolkit/chakra/link';
import { TableBody, TableCell, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import CopyToClipboard from 'ui/shared/CopyToClipboard';
import DataFetchAlert from 'ui/shared/DataFetchAlert';
import PageTitle from 'ui/shared/Page/PageTitle';
import { ScanKeyValue, ScanTableCard } from 'ui/shared/scan';

import type { ActivityKind, ChainEvent, ChainTransfer } from './detail';
import { addChainParams, parseActivityPage, parseChainRecord, parseSummary } from './detail';
import type { ChainLaunchView } from './launch';
import { readChainLaunch } from './launch';
import type { ChainRecord } from './model';
import { statusLabel } from './model';

function isTransfer(item: ChainTransfer | ChainEvent): item is ChainTransfer {
  return typeof item.kind === 'string';
}

function ActivityFeed({ id, kind }: { id: number; kind: ActivityKind }) {
  const resource = kind === 'transfers' ? 'general:paxeer_x_chain_transfers' : 'general:paxeer_x_chain_events';
  const selectRecent = React.useCallback((json: unknown) => {
    const page = parseActivityPage(json, kind, id);
    if (page.items.length > 20) {
      throw new Error('Recent activity exceeds the requested window');
    }
    for (let index = 1; index < page.items.length; index++) {
      if (BigInt(page.items[index - 1].id) <= BigInt(page.items[index].id)) {
        throw new Error('Recent activity must be ordered by descending row ID');
      }
    }
    return page.items;
  }, [ kind, id ]);
  const query = useApiQuery(resource, {
    pathParams: { id: String(id) },
    queryParams: { limit: 20, order: 'desc' },
    queryOptions: { select: selectRecent, enabled: paxeerX.isEnabled },
  });

  return (
    <ScanTableCard
      title={ kind === 'transfers' ? 'Recent transfers' : 'Recent events' }
      note="Latest 20, newest first"
    >
      { query.isPending && <Text p={ 4 } role="status">Loading { kind }…</Text> }
      { query.isError && <DataFetchAlert/> }
      { query.isSuccess && (query.data.length === 0 ? <Text p={ 4 }>No { kind } yet.</Text> : (
        <Box overflowX="auto">
          <TableRoot variant="scan" aria-label={ kind }>
            <TableHeader>
              <TableRow>
                <TableColumnHeader>Height</TableColumnHeader>
                <TableColumnHeader>{ kind === 'transfers' ? 'Transfer' : 'Event' }</TableColumnHeader>
                <TableColumnHeader>Account</TableColumnHeader>
                <TableColumnHeader>Transaction</TableColumnHeader>
              </TableRow>
            </TableHeader>
            <TableBody>
              { query.data.map((item) => {
                const transfer = isTransfer(item) ? item : undefined;
                return (
                  <TableRow key={ item.id }>
                    <TableCell>{ item.height_or_seq }</TableCell>
                    <TableCell>
                      { transfer ? (
                        <>
                          <Text>{ transfer.direction } · { transfer.amount } { transfer.asset }</Text>
                          <Text>{ transfer.kind }</Text>
                          { transfer.counterparty && <Text wordBreak="break-all">{ transfer.counterparty }</Text> }
                        </>
                      ) : !isTransfer(item) && <Text>{ item.name }</Text> }
                    </TableCell>
                    <TableCell><Text wordBreak="break-all">{ item.account }</Text></TableCell>
                    <TableCell><Text wordBreak="break-all">{ item.tx_id }</Text></TableCell>
                  </TableRow>
                );
              }) }
            </TableBody>
          </TableRoot>
        </Box>
      )) }
    </ScanTableCard>
  );
}

function ChainRecordCard({ record }: { record: ChainRecord }) {
  const [ walletState, setWalletState ] = React.useState<'idle' | 'pending' | 'added'>('idle');
  const [ walletError, setWalletError ] = React.useState<string>();
  const wallet = React.useMemo(() => {
    try {
      const rpcBase = config.chain.rpcUrls[0];
      if (!rpcBase) {
        throw new Error('The RPC URL is not configured.');
      }
      return { params: addChainParams(record, rpcBase, config.app.baseUrl), error: undefined };
    } catch (error) {
      return { params: undefined, error: error instanceof Error ? error.message : 'Invalid network configuration.' };
    }
  }, [ record ]);
  const handleAdd = React.useCallback(async() => {
    setWalletError(undefined);
    if (!window.ethereum) {
      setWalletError('Install an Ethereum-compatible wallet to add this chain.');
      return;
    }
    if (!wallet.params) {
      setWalletError(wallet.error);
      return;
    }
    setWalletState('pending');
    try {
      await window.ethereum.request({ method: 'wallet_addEthereumChain', params: [ wallet.params ] });
      setWalletState('added');
    } catch (error) {
      setWalletState('idle');
      setWalletError(error instanceof Error ? error.message : 'The wallet could not add this chain.');
    }
  }, [ wallet ]);

  return (
    <ScanTableCard title={ record.name }>
      <Grid templateColumns={{ base: '1fr', md: '180px 1fr' }} gap={ 3 } p={ 4 }>
        <ScanKeyValue label="Virtual chain ID">{ record.id }</ScanKeyValue>
        <ScanKeyValue label="Wallet chain ID">{ record.chain_id }</ScanKeyValue>
        <ScanKeyValue label="Status">{ statusLabel(record.status) }</ScanKeyValue>
        <ScanKeyValue label="Operator"><Text wordBreak="break-all">{ record.operator }</Text></ScanKeyValue>
        <ScanKeyValue label="Created height">{ record.created_height }</ScanKeyValue>
        <ScanKeyValue label="Bond (base units)">{ record.bond }</ScanKeyValue>
        <ScanKeyValue label="Allowance">{ record.gas_allowance } gas · { record.byte_allowance } bytes</ScanKeyValue>
        <ScanKeyValue label="Fee share">{ record.fee_share_bps } bps</ScanKeyValue>
        <ScanKeyValue label="RPC URL">
          { wallet.params && (
            <Flex alignItems="center" gap={ 2 }>
              <Text wordBreak="break-all">{ wallet.params.rpcUrls[0] }</Text>
              <CopyToClipboard text={ wallet.params.rpcUrls[0] } aria-label="Copy RPC URL"/>
            </Flex>
          ) }
          { wallet.error && <Text role="alert">{ wallet.error }</Text> }
        </ScanKeyValue>
      </Grid>
      <Flex px={ 4 } pb={ 4 } gap={ 4 } alignItems="center" flexWrap="wrap">
        <Button onClick={ handleAdd } loading={ walletState === 'pending' } disabled={ !wallet.params }>Add to wallet</Button>
        <Link href={ `${ config.app.origin }/launchpad` } external>Launch a chain</Link>
        { walletState === 'added' && <Text role="status">Chain added to wallet.</Text> }
        { walletError && <Text role="alert">{ walletError }</Text> }
      </Flex>
    </ScanTableCard>
  );
}

function LaunchSections({ launch }: { launch: ChainLaunchView }) {
  return (
    <>
      <ScanTableCard title="Launch asset">
        <Grid templateColumns={{ base: '1fr', md: '180px 1fr' }} gap={ 3 } p={ 4 }>
          <ScanKeyValue label="Token">
            <Link href={ `/token/${ launch.token }` } wordBreak="break-all">{ launch.tokenSymbol } · { launch.token }</Link>
          </ScanKeyValue>
          <ScanKeyValue label="Pool"><Link href={ `/address/${ launch.pool }` } wordBreak="break-all">{ launch.pool }</Link></ScanKeyValue>
          <ScanKeyValue label="Fee right">#{ launch.feeRightId.toString() }</ScanKeyValue>
          <ScanKeyValue label="Home chain">{ launch.homeChainId.toString() }</ScanKeyValue>
        </Grid>
      </ScanTableCard>
      <ScanTableCard title="Revenue epochs">
        { launch.lastEpoch ? (
          <Grid templateColumns={{ base: '1fr', md: '180px 1fr' }} gap={ 3 } p={ 4 }>
            <ScanKeyValue label="Last epoch">{ launch.lastEpoch.number.toString() }</ScanKeyValue>
            <ScanKeyValue label="Epoch amount">{ launch.lastEpoch.amountPax } PAX</ScanKeyValue>
            <ScanKeyValue label="Snapshot block">{ launch.lastEpoch.snapshotBlock.toString() }</ScanKeyValue>
          </Grid>
        ) : <Text p={ 4 }>No revenue epochs yet.</Text> }
      </ScanTableCard>
    </>
  );
}

export function ChainDetail({ id }: { id: number }) {
  const selectRecord = React.useCallback((json: unknown) => parseChainRecord(json, id), [ id ]);
  const selectSummary = React.useCallback((json: unknown) => parseSummary(json, id), [ id ]);
  const record = useApiQuery('general:paxeer_x_chain', {
    pathParams: { id: String(id) },
    queryOptions: { select: selectRecord, enabled: paxeerX.isEnabled },
  });
  const summary = useApiQuery('general:paxeer_x_chain_summary', {
    pathParams: { id: String(id) },
    queryOptions: { select: selectSummary, enabled: paxeerX.isEnabled && record.isSuccess },
  });
  const launch = useQuery({
    queryKey: [ 'chainLaunch', id ],
    queryFn: () => {
      if (!publicClient || !record.data) throw new Error('Chain launch data is unavailable');
      return readChainLaunch(record.data, publicClient);
    },
    enabled: paxeerX.isEnabled && record.isSuccess,
    retry: false,
  });

  return (
    <>
      <Link href="/chains">All chains</Link>
      <PageTitle title={ record.data?.name || `Chain ${ id }` }/>
      { record.isPending && <Text role="status">Loading chain…</Text> }
      { record.isError && (record.error.status === 404 ? <Text role="alert">Unknown virtual chain.</Text> : <DataFetchAlert/>) }
      { record.isSuccess && (
        <Flex direction="column" gap={ 6 }>
          <ChainRecordCard key={ id } record={ record.data }/>
          { launch.isPending && <Text role="status">Loading launch asset…</Text> }
          { launch.isError && <DataFetchAlert/> }
          { launch.isSuccess && launch.data && <LaunchSections launch={ launch.data }/> }
          <ScanTableCard title="Summary">
            { summary.isPending && <Text p={ 4 } role="status">Loading summary…</Text> }
            { summary.isError && <DataFetchAlert/> }
            { summary.isSuccess && (
              <Grid templateColumns={{ base: '1fr', md: '180px 1fr' }} gap={ 3 } p={ 4 }>
                <ScanKeyValue label="Transactions">{ summary.data.tx_count.toLocaleString() }</ScanKeyValue>
                <ScanKeyValue label="Gas used">{ summary.data.gas_used.toLocaleString() }</ScanKeyValue>
                <ScanKeyValue label="Last height">{ summary.data.last_height }</ScanKeyValue>
              </Grid>
            ) }
          </ScanTableCard>
          <Text fontSize="lg" fontWeight={ 600 }>Chain activity</Text>
          <ActivityFeed key={ `${ id }:transfers` } id={ id } kind="transfers"/>
          <ActivityFeed key={ `${ id }:events` } id={ id } kind="events"/>
        </Flex>
      ) }
    </>
  );
}

export default ChainDetail;
