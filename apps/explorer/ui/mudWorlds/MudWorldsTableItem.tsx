import React from 'react';

import type { MudWorldItem } from 'types/api/mudWorlds';

import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableCell, TableRow } from 'toolkit/chakra/table';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import NativeCoinValue from 'ui/shared/value/NativeCoinValue';

type Props = { item: MudWorldItem; isLoading?: boolean };

const MudWorldsTableItem = ({ item, isLoading }: Props) => {
  return (
    <TableRow>
      <TableCell verticalAlign="middle">
        <AddressEntity address={ item.address } isLoading={ isLoading } fontWeight="bold"/>
      </TableCell>
      <TableCell isNumeric>
        <NativeCoinValue
          amount={ item.coin_balance }
          noSymbol
          loading={ isLoading }
          lineHeight="h2"
        />
      </TableCell>
      <TableCell isNumeric>
        <Skeleton loading={ isLoading } display="inline-block" lineHeight="h2">
          { Number(item.transactions_count).toLocaleString() }
        </Skeleton>
      </TableCell>
    </TableRow>
  );
};

export default MudWorldsTableItem;
