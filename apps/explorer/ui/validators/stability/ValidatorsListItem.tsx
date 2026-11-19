import React from 'react';

import type { ValidatorStability } from 'types/api/validators';

import { Skeleton } from 'toolkit/chakra/skeleton';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import ListItemMobileGrid from 'ui/shared/ListItemMobile/ListItemMobileGrid';
import ValidatorStatus from 'ui/shared/statusTag/ValidatorStabilityStatus';

interface Props {
  data: ValidatorStability;
  isLoading?: boolean;
}

const ValidatorsListItem = ({ data, isLoading }: Props) => {

  return (
    <ListItemMobileGrid.Container gridTemplateColumns="minmax(0, 1fr)" textStyle="small"
      css={{
        '& > *': { minWidth: 0, overflowWrap: 'anywhere' },
        '& a': { display: 'inline-flex', alignItems: 'center' },
        '& a, & button': { _motionReduce: { transitionProperty: 'none' } },
        '& [data-status="error"] > span': { color: 'text.primary' },
      }}
    >

      <ListItemMobileGrid.Label isLoading={ isLoading }>Validator</ListItemMobileGrid.Label>
      <ListItemMobileGrid.Value>
        <AddressEntity
          isLoading={ isLoading }
          address={ data.address }
          truncation="constant"
        />
      </ListItemMobileGrid.Value>

      { data.state != null && (
        <>
          <ListItemMobileGrid.Label isLoading={ isLoading }>Status</ListItemMobileGrid.Label>
          <ListItemMobileGrid.Value>
            <ValidatorStatus state={ data.state } isLoading={ isLoading }/>
          </ListItemMobileGrid.Value>
        </>
      ) }

      { data.blocks_validated_count != null && (
        <>
          <ListItemMobileGrid.Label isLoading={ isLoading }>Blocks</ListItemMobileGrid.Label>
          <ListItemMobileGrid.Value>
            <Skeleton loading={ isLoading }>
              { data.blocks_validated_count.toLocaleString() }
            </Skeleton>
          </ListItemMobileGrid.Value>
        </>
      ) }
    </ListItemMobileGrid.Container>
  );
};

export default React.memo(ValidatorsListItem);
