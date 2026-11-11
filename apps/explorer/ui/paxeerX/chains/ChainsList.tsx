import { Box, Flex, Text } from '@chakra-ui/react';
import { useQuery } from '@tanstack/react-query';
import React from 'react';

import paxeerX from 'configs/app/features/paxeerX';
import useApiQuery from 'lib/api/useApiQuery';
import { publicClient } from 'lib/web3/client';
import { Link } from 'toolkit/chakra/link';
import { TableBody, TableCell, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import DataFetchAlert from 'ui/shared/DataFetchAlert';
import PageTitle from 'ui/shared/Page/PageTitle';
import { ScanTableCard } from 'ui/shared/scan';

import { readChainLaunch } from './launch';
import type { ChainRecord } from './model';
import { chainListRow, parseChainsResponse, sortChains, statusLabel } from './model';

function chainRows(json: unknown) {
  return sortChains(parseChainsResponse(json));
}

function ChainRow({ chain }: { chain: ChainRecord }) {
  const launch = useQuery({
    queryKey: [ 'chainLaunch', chain.id ],
    queryFn: () => {
      if (!publicClient) throw new Error('Chain launch data is unavailable');
      return readChainLaunch(chain, publicClient);
    },
    enabled: paxeerX.isEnabled,
    retry: false,
  });
  const row = chainListRow(chain, launch.data?.tokenSymbol ?? null);

  return (
    <TableRow data-chain-id={ row.id }>
      <TableCell><Link href={ `/chains/${ row.id }` }>{ row.name }</Link></TableCell>
      <TableCell>{ row.chain_id }</TableCell>
      <TableCell>{ row.launchSymbol || (launch.isError ? 'Unavailable' : '') }</TableCell>
      <TableCell><Text wordBreak="break-all">{ row.operator }</Text></TableCell>
      <TableCell>{ statusLabel(row.status) }</TableCell>
      <TableCell>
        <Text>{ row.gas_allowance.toLocaleString() } gas</Text>
        <Text>{ row.byte_allowance.toLocaleString() } bytes</Text>
      </TableCell>
      <TableCell>{ row.created_height }</TableCell>
    </TableRow>
  );
}

export function ChainsList() {
  const query = useApiQuery('general:paxeer_x_chains', {
    queryOptions: { select: chainRows, enabled: paxeerX.isEnabled },
  });

  return (
    <>
      <Flex as="nav" aria-label={ paxeerX.navigation.text } gap={ 4 } mb={ 6 } alignItems="center">
        <Text fontWeight={ 600 }>{ paxeerX.navigation.text }</Text>
        { paxeerX.navigation.items.map((item) => (
          <Link key={ item.href } href={ item.href } aria-current="page">{ item.text }</Link>
        )) }
      </Flex>
      <PageTitle title="Chains"/>
      { query.isError && <DataFetchAlert/> }
      { query.isPending && <Text role="status">Loading chains…</Text> }
      { query.isSuccess && (
        <ScanTableCard title={ `${ query.data.length } chains` } note="Virtual chains, active first">
          { query.data.length === 0 ? (
            <Text px={ 4 } py={ 6 }>There are no virtual chains.</Text>
          ) : (
            <Box overflowX="auto">
              <TableRoot variant="scan" aria-label="Virtual chains">
                <TableHeader>
                  <TableRow>
                    <TableColumnHeader>Name</TableColumnHeader>
                    <TableColumnHeader>Chain ID</TableColumnHeader>
                    <TableColumnHeader>Launch asset</TableColumnHeader>
                    <TableColumnHeader>Operator</TableColumnHeader>
                    <TableColumnHeader>Status</TableColumnHeader>
                    <TableColumnHeader>Allowance</TableColumnHeader>
                    <TableColumnHeader>Created height</TableColumnHeader>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  { query.data.map((chain) => <ChainRow key={ chain.id } chain={ chain }/>) }
                </TableBody>
              </TableRoot>
            </Box>
          ) }
        </ScanTableCard>
      ) }
    </>
  );
}

export default ChainsList;
