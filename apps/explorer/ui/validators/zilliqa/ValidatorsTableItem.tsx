import React from 'react';

import type { ValidatorsZilliqaItem } from 'types/api/validators';

import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableCell, TableRow } from 'toolkit/chakra/table';
import ValidatorEntity from 'ui/shared/entities/validator/ValidatorEntity';
import NativeCoinValue from 'ui/shared/value/NativeCoinValue';

interface Props {
  data: ValidatorsZilliqaItem;
  isLoading?: boolean;
  columns: { index: boolean; stake: boolean };
}

const ValidatorsTableItem = ({ data, isLoading, columns }: Props) => {
  return (
    <TableRow>
      <TableCell verticalAlign="middle">
        <ValidatorEntity id={ data.bls_public_key } isLoading={ isLoading } fontWeight="700"/>
      </TableCell>
      { columns.index && <TableCell isNumeric><Skeleton loading={ isLoading }>{ data.index ?? '—' }</Skeleton></TableCell> }
      { columns.stake && <TableCell isNumeric>{ data.balance == null ? '—' : <NativeCoinValue amount={ data.balance } loading={ isLoading } noSymbol/> }</TableCell> }
    </TableRow>
  );
};

export default React.memo(ValidatorsTableItem);
