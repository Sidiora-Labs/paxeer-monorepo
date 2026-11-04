import { Box, Flex, Text } from '@chakra-ui/react';
import React from 'react';

import paxeerX from 'configs/app/features/paxeerX';
import useApiQuery from 'lib/api/useApiQuery';
import { Link } from 'toolkit/chakra/link';
import { TableBody, TableCell, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import DataFetchAlert from 'ui/shared/DataFetchAlert';
import PageTitle from 'ui/shared/Page/PageTitle';
import { ScanTableCard } from 'ui/shared/scan';

import { parseChainsResponse, sortChains, statusLabel } from './model';

function chainRows(json: unknown) {
  return sortChains(parseChainsResponse(json));
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
                    <TableColumnHeader>Operator</TableColumnHeader>
                    <TableColumnHeader>Status</TableColumnHeader>
                    <TableColumnHeader>Allowance</TableColumnHeader>
                    <TableColumnHeader>Created height</TableColumnHeader>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  { query.data.map((chain) => (
                    <TableRow key={ chain.id } data-chain-id={ chain.id }>
                      <TableCell><Link href={ `/chains/${ chain.id }` }>{ chain.name }</Link></TableCell>
                      <TableCell>{ chain.chain_id }</TableCell>
                      <TableCell><Text wordBreak="break-all">{ chain.operator }</Text></TableCell>
                      <TableCell>{ statusLabel(chain.status) }</TableCell>
                      <TableCell>
                        <Text>{ chain.gas_allowance.toLocaleString() } gas</Text>
                        <Text>{ chain.byte_allowance.toLocaleString() } bytes</Text>
                      </TableCell>
                      <TableCell>{ chain.created_height }</TableCell>
                    </TableRow>
                  )) }
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
