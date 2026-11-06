import { HStack } from '@chakra-ui/react';
import React from 'react';

import type { CeloEpochListItem } from 'types/api/epochs';

import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableCell, TableRow } from 'toolkit/chakra/table';
import BlockEntity from 'ui/shared/entities/block/BlockEntity';
import EpochEntity from 'ui/shared/entities/epoch/EpochEntity';
import CeloEpochStatus from 'ui/shared/statusTag/CeloEpochStatus';
import TimeWithTooltip from 'ui/shared/time/TimeWithTooltip';
import NativeCoinValue from 'ui/shared/value/NativeCoinValue';

interface Props {
  item: CeloEpochListItem;
  isLoading?: boolean;
};

const EpochsTableItem = ({ item, isLoading }: Props) => {
  return (
    <TableRow>
      <TableCell verticalAlign="middle">
        <HStack gap={ 2 }>
          <EpochEntity number={ String(item.number) } noIcon fontWeight={ 700 } isLoading={ isLoading }/>
          <Skeleton loading={ isLoading } color="text.secondary" fontWeight={ 500 }><span>{ item.type }</span></Skeleton>
          <TimeWithTooltip
            timestamp={ item.timestamp }
            isLoading={ isLoading }
            color="text.secondary"
            display="inline-block"
            fontWeight={ 400 }
          />
        </HStack>
      </TableCell>
      <TableCell verticalAlign="middle">
        <CeloEpochStatus
          isFinalized={ item.is_finalized }
          loading={ isLoading }
        />
      </TableCell>
      <TableCell color="text.muted"><Skeleton loading={ isLoading }>—</Skeleton></TableCell>
      <TableCell isNumeric><BlockEntity number={ item.start_block_number } noIcon isLoading={ isLoading } textStyle="number"/></TableCell>
      <TableCell isNumeric>
        { item.end_block_number !== null ? <BlockEntity number={ item.end_block_number } noIcon isLoading={ isLoading } textStyle="number"/> : '—' }
      </TableCell>
      <TableCell isNumeric color="text.muted"><Skeleton loading={ isLoading }>—</Skeleton></TableCell>
      <TableCell verticalAlign="middle" isNumeric>
        { item.distribution?.community_transfer?.value != null ? (
          <NativeCoinValue amount={ item.distribution.community_transfer.value } noSymbol loading={ isLoading }/>
        ) : '—' }
      </TableCell>
      <TableCell verticalAlign="middle" isNumeric>
        { item.distribution?.carbon_offsetting_transfer?.value != null ? (
          <NativeCoinValue amount={ item.distribution.carbon_offsetting_transfer.value } noSymbol loading={ isLoading }/>
        ) : '—' }
      </TableCell>
      <TableCell verticalAlign="middle" isNumeric>
        { item.distribution?.transfers_total?.value != null ? (
          <NativeCoinValue amount={ item.distribution.transfers_total.value } noSymbol loading={ isLoading }/>
        ) : '—' }
      </TableCell>
    </TableRow>
  );
};

export default EpochsTableItem;
