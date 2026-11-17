import React from 'react';

import type { CeloEpochListItem } from 'types/api/epochs';

import config from 'configs/app';
import { TableBody, TableColumnHeader, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';
import TableScrollArea from 'ui/blocks/TableScrollArea';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';

import EpochsTableItem from './EpochsTableItem';

interface Props {
  items: Array<CeloEpochListItem>;
  isLoading?: boolean;
  top: number;
};

const EpochsTable = ({ items, isLoading, top }: Props) => {
  return (
    <TableScrollArea label="Epochs">
      <TableRoot aria-label="Epochs"
        minW="max-content" variant="scan"
        css={{ '& th:first-of-type, & td:first-of-type': { position: 'sticky', left: 0, bg: 'canvas' } }}>
        <TableHeaderSticky top={ top } bg="canvas">
          <TableRow>
            <TableColumnHeader w="var(--chakra-spacing-65)">
              Epoch
              <TimeFormatToggle/>
            </TableColumnHeader>
            <TableColumnHeader w="var(--chakra-spacing-30)">Status</TableColumnHeader>
            <TableColumnHeader w="var(--chakra-spacing-30)">Started</TableColumnHeader>
            <TableColumnHeader w="var(--chakra-spacing-37\.5)" isNumeric>First block</TableColumnHeader>
            <TableColumnHeader w="var(--chakra-spacing-37\.5)" isNumeric>Last block</TableColumnHeader>
            <TableColumnHeader w="var(--chakra-spacing-24)" isNumeric>Duration</TableColumnHeader>
            <TableColumnHeader w="var(--chakra-spacing-40)" isNumeric>Community { config.chain.currency.symbol }</TableColumnHeader>
            <TableColumnHeader w="var(--chakra-spacing-40)" isNumeric>Carbon offset { config.chain.currency.symbol }</TableColumnHeader>
            <TableColumnHeader w="var(--chakra-spacing-40)" isNumeric>Total { config.chain.currency.symbol }</TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { items.map((item, index) => {
            return (
              <EpochsTableItem
                key={ item.number + (isLoading ? String(index) : '') }
                item={ item }
                isLoading={ isLoading }
              />
            );
          }) }
        </TableBody>
      </TableRoot>
    </TableScrollArea>
  );
};

export default EpochsTable;
