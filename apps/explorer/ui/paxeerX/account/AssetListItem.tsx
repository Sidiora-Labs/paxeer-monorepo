import { Flex } from '@chakra-ui/react';
import React from 'react';

import type { PaxeerXBalance } from 'types/api/paxeerX';

import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableCell, TableRow } from 'toolkit/chakra/table';

import { assetLabel, formatAmount } from './utils';

export interface Props {
  item: PaxeerXBalance;
  isLoading?: boolean;
}

const AssetListItem = ({ item, isLoading }: Props) => {
  const label = assetLabel(item.asset);

  return (
    <TableRow data-asset={ item.asset.id }>
      <TableCell verticalAlign="middle">
        <Flex flexDirection="column" rowGap={ 1 } minW={ 0 }>
          <Skeleton loading={ isLoading } fontWeight={ 600 }>{ label }</Skeleton>
          { item.asset.id !== label && (
            <Skeleton loading={ isLoading } color="text.secondary" textStyle="xs" wordBreak="break-all" data-label="asset-id">
              { item.asset.id }
            </Skeleton>
          ) }
        </Flex>
      </TableCell>
      <TableCell verticalAlign="middle">
        <Skeleton loading={ isLoading } color="text.secondary" wordBreak="break-all">{ item.asset.denom }</Skeleton>
      </TableCell>
      <TableCell verticalAlign="middle" isNumeric>
        <Skeleton loading={ isLoading } display="inline-block" fontWeight={ 500 } data-label="total">
          { formatAmount(item.total, item.asset) }
        </Skeleton>
      </TableCell>
    </TableRow>
  );
};

export default React.memo(AssetListItem);
