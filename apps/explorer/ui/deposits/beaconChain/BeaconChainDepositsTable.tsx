import React from 'react';

import type { DepositsItem } from 'types/api/deposits';

import config from 'configs/app';
import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import useLazyRenderedList from 'lib/hooks/useLazyRenderedList';
import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import BeaconChainDepositsTableItem from './BeaconChainDepositsTableItem';

const feature = config.features.beaconChain;

type Props = {
  top: number;
  isLoading?: boolean;
  items: Array<DepositsItem>;
  view: 'list' | 'address' | 'block';
};

const BeaconChainDepositsTable = ({ items, isLoading, top, view }: Props) => {
  const { cutRef, renderedItemsNum } = useLazyRenderedList(items, !isLoading);

  if (!feature.isEnabled || feature.withdrawalsOnly) {
    return null;
  }

  return (
    <AddressHighlightProvider>
      <ActivityTable label="Deposits" minW="var(--chakra-sizes-6xl)">
        <TableHeaderSticky top={ top }>
          <TableRow>
            <TableColumnHeader w="var(--chakra-spacing-48)">Transaction hash</TableColumnHeader>
            { view !== 'block' && <TableColumnHeader isNumeric>Block</TableColumnHeader> }
            { view !== 'block' && <TableColumnHeader w="var(--chakra-spacing-44)">Timestamp<TimeFormatToggle/></TableColumnHeader> }
            <TableColumnHeader isNumeric>{ `Value ${ feature.currency.symbol }` }</TableColumnHeader>
            { view !== 'address' && <TableColumnHeader w="var(--chakra-spacing-48)">From</TableColumnHeader> }
            <TableColumnHeader>PubKey</TableColumnHeader>
            <TableColumnHeader>Signature</TableColumnHeader>
            <TableColumnHeader>Status</TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { items.slice(0, renderedItemsNum).map((item, index) => (
            <BeaconChainDepositsTableItem key={ item.index + (isLoading ? String(index) : '') } item={ item } view={ view } isLoading={ isLoading }/>
          )) }
          <TableRow ref={ cutRef }/>
        </TableBody>
      </ActivityTable>
    </AddressHighlightProvider>
  );
};

export default BeaconChainDepositsTable;
