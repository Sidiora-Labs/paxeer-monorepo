import { Box, Flex } from '@chakra-ui/react';
import React from 'react';

import type { ValidatorBlackfort } from 'types/api/validators';

import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableCell, TableRow } from 'toolkit/chakra/table';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import NativeCoinValue from 'ui/shared/value/NativeCoinValue';

interface Props {
  data: ValidatorBlackfort;
  isLoading?: boolean;
  columns: { stake: boolean; delegated: boolean; commission: boolean };
}

const ValidatorsTableItem = ({ data, isLoading, columns }: Props) => {
  return (
    <TableRow>
      <TableCell verticalAlign="middle">
        <Flex alignItems="center" gap={ 2 } minW={ 0 }>
          <AddressEntity
            address={ data.address }
            isLoading={ isLoading }
            truncation="constant"
          />
          { data.name && <Box textStyle="small" color="text.secondary" truncate title={ data.name }>{ data.name }</Box> }
        </Flex>
      </TableCell>
      { columns.stake && (
        <TableCell isNumeric>{ data.self_bonded_amount == null ? '—' : <NativeCoinValue amount={ data.self_bonded_amount } loading={ isLoading } noSymbol/> }</TableCell>
      ) }
      { columns.delegated && (
        <TableCell isNumeric>{ data.delegated_amount == null ? '—' : <NativeCoinValue amount={ data.delegated_amount } loading={ isLoading } noSymbol/> }</TableCell>
      ) }
      { columns.commission && (
        <TableCell isNumeric><Skeleton loading={ isLoading }>{ data.commission == null ? '—' : `${ data.commission / 100 }%` }</Skeleton></TableCell>
      ) }
    </TableRow>
  );
};

export default React.memo(ValidatorsTableItem);
