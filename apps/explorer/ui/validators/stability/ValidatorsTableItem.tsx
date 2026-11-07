import React from 'react';

import type { ValidatorStability } from 'types/api/validators';

import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableCell, TableRow } from 'toolkit/chakra/table';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import ValidatorStatus from 'ui/shared/statusTag/ValidatorStabilityStatus';

interface Props {
  data: ValidatorStability;
  isLoading?: boolean;
  columns: { status: boolean; blocks: boolean };
}

const ValidatorsTableItem = ({ data, isLoading, columns }: Props) => {
  return (
    <TableRow>
      <TableCell verticalAlign="middle">
        <AddressEntity
          address={ data.address }
          isLoading={ isLoading }
          truncation="constant"
        />
      </TableCell>
      { columns.status && (
        <TableCell>{ data.state == null ? '—' : <ValidatorStatus state={ data.state } isLoading={ isLoading }/> }</TableCell>
      ) }
      { columns.blocks && (
        <TableCell isNumeric><Skeleton loading={ isLoading }>{ data.blocks_validated_count?.toLocaleString() ?? '—' }</Skeleton></TableCell>
      ) }
    </TableRow>
  );
};

export default React.memo(ValidatorsTableItem);
